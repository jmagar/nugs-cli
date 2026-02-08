package download

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/grafov/m3u8"
	"github.com/jmagar/nugs-cli/internal/api"
	"github.com/jmagar/nugs-cli/internal/helpers"
	"github.com/jmagar/nugs-cli/internal/model"
	"github.com/jmagar/nugs-cli/internal/ui"
)

const (
	chapsFileFname = "chapters_nugs_dl_tmp.txt"
	durRegex       = `Duration: ([\d:.]+)`
)

var resFallback = map[string]string{
	"720":  "480",
	"1080": "720",
	"1440": "1080",
}

// GetVideoSku finds the video SKU ID from a product list.
func GetVideoSku(products []model.Product) int {
	for _, product := range products {
		formatStr := product.FormatStr
		if formatStr == "VIDEO ON DEMAND" || formatStr == "LIVE HD VIDEO" {
			return product.SkuID
		}
	}
	return 0
}

// GetShowMediaType determines what media types a show offers (audio/video/both).
func GetShowMediaType(show *model.AlbArtResp) model.MediaType {
	// When Products is empty, the API didn't return product details.
	// Default to audio since every show on Nugs.net has audio.
	if len(show.Products) == 0 {
		return model.MediaTypeAudio
	}

	hasVideo := GetVideoSku(show.Products) != 0
	hasAudio := false

	// Check for audio products (non-video formats)
	for _, product := range show.Products {
		if product.FormatStr != "VIDEO ON DEMAND" &&
			product.FormatStr != "LIVE HD VIDEO" {
			hasAudio = true
			break
		}
	}

	if hasVideo && hasAudio {
		return model.MediaTypeBoth
	}
	if hasVideo {
		return model.MediaTypeVideo
	}
	return model.MediaTypeAudio
}

// GetLstreamSku finds the livestream SKU ID from a product format list.
func GetLstreamSku(products []*model.ProductFormatList) int {
	for _, product := range products {
		if product.FormatStr == "LIVE HD VIDEO" {
			return product.SkuID
		}
	}
	return 0
}

// GetVidVariant finds a video variant matching the desired resolution.
func GetVidVariant(variants []*m3u8.Variant, wantRes string) *m3u8.Variant {
	for _, variant := range variants {
		if strings.HasSuffix(variant.Resolution, "x"+wantRes) {
			return variant
		}
	}
	return nil
}

// FormatRes formats a resolution string for display.
func FormatRes(res string) string {
	if res == "2160" {
		return "4K"
	}
	return res + "p"
}

// ChooseVariant selects the best video variant from a manifest URL.
func ChooseVariant(manifestUrl, wantRes string) (*m3u8.Variant, string, error) {
	origWantRes := wantRes
	var wantVariant *m3u8.Variant
	req, err := api.Client.Get(manifestUrl)
	if err != nil {
		return nil, "", err
	}
	defer req.Body.Close()
	if req.StatusCode != http.StatusOK {
		return nil, "", errors.New(req.Status)
	}
	playlist, _, err := m3u8.DecodeFrom(req.Body, true)
	if err != nil {
		return nil, "", err
	}
	master := playlist.(*m3u8.MasterPlaylist)
	sort.Slice(master.Variants, func(x, y int) bool {
		return master.Variants[x].Bandwidth > master.Variants[y].Bandwidth
	})
	if wantRes == "2160" {
		variant := master.Variants[0]
		varRes := strings.SplitN(variant.Resolution, "x", 2)[1]
		varRes = FormatRes(varRes)
		return variant, varRes, nil
	}
	// Guard against infinite loop: max 10 fallback attempts
	maxFallbacks := 10
	for i := 0; i < maxFallbacks; i++ {
		wantVariant = GetVidVariant(master.Variants, wantRes)
		if wantVariant != nil {
			break
		}
		nextRes := resFallback[wantRes]
		// Guard: if no fallback exists or fallback doesn't progress, use highest available
		if nextRes == "" || nextRes == wantRes {
			if len(master.Variants) > 0 {
				wantVariant = master.Variants[0] // Highest bandwidth variant
				wantRes = strings.SplitN(wantVariant.Resolution, "x", 2)[1]
				wantRes = FormatRes(wantRes)
			}
			break
		}
		wantRes = nextRes
	}
	// Final fallback: if still no variant after max attempts, use highest available
	if wantVariant == nil && len(master.Variants) > 0 {
		wantVariant = master.Variants[0]
		wantRes = strings.SplitN(wantVariant.Resolution, "x", 2)[1]
		wantRes = FormatRes(wantRes)
	}
	if wantRes != origWantRes {
		ui.PrintInfo("Unavailable in your chosen format")
	}
	wantRes = FormatRes(wantRes)
	return wantVariant, wantRes, nil
}

// GetManifestBase extracts the base URL and query string from a manifest URL.
func GetManifestBase(manifestUrl string) (string, string, error) {
	u, err := url.Parse(manifestUrl)
	if err != nil {
		return "", "", err
	}
	p := u.Path
	lastPathIdx := strings.LastIndex(p, "/")
	base := u.Scheme + "://" + u.Host + p[:lastPathIdx+1]
	return base, "?" + u.RawQuery, nil
}

// GetSegUrls retrieves segment URLs from an HLS media playlist.
func GetSegUrls(manifestUrl, query string) ([]string, error) {
	var segUrls []string
	req, err := api.Client.Get(manifestUrl)
	if err != nil {
		return nil, err
	}
	defer req.Body.Close()
	if req.StatusCode != http.StatusOK {
		return nil, errors.New(req.Status)
	}
	playlist, _, err := m3u8.DecodeFrom(req.Body, true)
	if err != nil {
		return nil, err
	}
	media := playlist.(*m3u8.MediaPlaylist)
	for _, seg := range media.Segments {
		if seg == nil {
			break
		}
		segUrls = append(segUrls, seg.URI+query)
	}
	return segUrls, nil
}

// DownloadVideoFile downloads a video file from a URL with progress tracking.
func DownloadVideoFile(videoPath, _url string, onProgress func(downloaded, total, speed int64)) error {
	f, err := os.OpenFile(videoPath, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return err
	}
	startByte := stat.Size()

	req, err := http.NewRequest(http.MethodGet, _url, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Range", fmt.Sprintf("bytes=%d-", startByte))
	do, err := api.Client.Do(req)
	if err != nil {
		return err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK && do.StatusCode != http.StatusPartialContent {
		return errors.New(do.Status)
	}

	if startByte > 0 {
		fmt.Printf("TS already exists locally, resuming from byte %d...\n", startByte)
		startByte = 0
	}

	totalBytes := do.ContentLength
	counter := &model.WriteCounter{
		Total:      totalBytes,
		TotalStr:   humanize.Bytes(uint64(totalBytes)),
		StartTime:  time.Now().UnixMilli(),
		Downloaded: startByte,
		OnProgress: onProgress,
	}

	// Use a simple writer that doesn't need deps (video has its own progress callbacks)
	_, err = io.Copy(f, io.TeeReader(do.Body, &simpleWriteCounter{wc: counter}))
	if onProgress == nil {
		fmt.Println("")
	}
	return err
}

// simpleWriteCounter wraps WriteCounter for video downloads (no deps needed).
type simpleWriteCounter struct {
	wc *model.WriteCounter
}

func (s *simpleWriteCounter) Write(p []byte) (int, error) {
	var speed int64
	n := len(p)
	s.wc.Downloaded += int64(n)
	if s.wc.Total > 0 {
		percentage := float64(s.wc.Downloaded) / float64(s.wc.Total) * float64(100)
		s.wc.Percentage = int(percentage)
	}
	toDivideBy := time.Now().UnixMilli() - s.wc.StartTime
	if toDivideBy != 0 {
		speed = int64(s.wc.Downloaded) / toDivideBy * 1000
	}
	if s.wc.OnProgress != nil {
		s.wc.OnProgress(s.wc.Downloaded, s.wc.Total, speed)
	}
	return n, nil
}

// DownloadLstream downloads a livestream video by downloading all segments.
func DownloadLstream(videoPath, baseUrl string, segUrls []string, onProgress func(segNum, segTotal int)) error {
	f, err := os.OpenFile(videoPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer f.Close()
	segTotal := len(segUrls)
	for segNum, segUrl := range segUrls {
		segNum++
		if onProgress != nil {
			onProgress(segNum, segTotal)
		} else {
			fmt.Printf("\rSegment %d of %d.", segNum, segTotal)
		}
		req, err := http.NewRequest(http.MethodGet, baseUrl+segUrl, nil)
		if err != nil {
			return err
		}
		do, err := api.Client.Do(req)
		if err != nil {
			return err
		}
		if do.StatusCode != http.StatusOK {
			do.Body.Close()
			return errors.New(do.Status)
		}
		_, err = io.Copy(f, do.Body)
		do.Body.Close()
		if err != nil {
			return err
		}
	}
	if onProgress == nil {
		fmt.Println("")
	}
	return err
}

// ExtractDuration extracts the duration string from ffmpeg output.
func ExtractDuration(errStr string) string {
	regex := regexp.MustCompile(durRegex)
	match := regex.FindStringSubmatch(errStr)
	if match != nil {
		return match[1]
	}
	return ""
}

// ParseDuration converts a duration string (HH:MM:SS.ms) to seconds.
func ParseDuration(dur string) (int, error) {
	dur = strings.Replace(dur, ":", "h", 1)
	dur = strings.Replace(dur, ":", "m", 1)
	dur = strings.Replace(dur, ".", "s", 1)
	dur += "ms"
	d, err := time.ParseDuration(dur)
	if err != nil {
		return 0, err
	}
	rounded := math.Round(d.Seconds())
	return int(rounded), nil
}

// GetDuration gets the duration of a TS file using ffmpeg.
func GetDuration(tsPath, ffmpegNameStr string) (int, error) {
	var errBuffer bytes.Buffer
	args := []string{"-hide_banner", "-i", tsPath}
	cmd := exec.Command(ffmpegNameStr, args...)
	cmd.Stderr = &errBuffer
	// Return code's always 1 as we're not providing any output files.
	err := cmd.Run()
	if err.Error() != "exit status 1" {
		return 0, err
	}
	errStr := errBuffer.String()
	ok := strings.HasSuffix(
		strings.TrimSpace(errStr), "At least one output file must be specified")
	if !ok {
		errString := fmt.Sprintf("%s\n%s", err, errStr)
		return 0, errors.New(errString)
	}
	dur := ExtractDuration(errStr)
	if dur == "" {
		return 0, errors.New("No regex match.")
	}
	durSecs, err := ParseDuration(dur)
	if err != nil {
		return 0, err
	}
	return durSecs, nil
}

// GetNextChapStart gets the start time of a chapter by index.
func GetNextChapStart(chapters []any, idx int) float64 {
	for i, chapter := range chapters {
		if i == idx {
			m := chapter.(map[string]any)
			return m["chapterSeconds"].(float64)
		}
	}
	return 0
}

// WriteChapsFile writes an ffmpeg chapters metadata file.
func WriteChapsFile(chapters []any, dur int) error {
	f, err := os.OpenFile(chapsFileFname, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(";FFMETADATA1\n")
	if err != nil {
		return err
	}
	chaptersCount := len(chapters)

	var nextChapStart float64

	for i, chapter := range chapters {
		i++
		isLast := i == chaptersCount

		// casting to struct won't work.
		m := chapter.(map[string]any)
		start := m["chapterSeconds"].(float64)

		if !isLast {
			nextChapStart = GetNextChapStart(chapters, i)
			if nextChapStart <= start {
				continue
			}
		}

		_, err := f.WriteString("\n[CHAPTER]\n")
		if err != nil {
			return err
		}
		_, err = f.WriteString("TIMEBASE=1/1\n")
		if err != nil {
			return err
		}

		startLine := fmt.Sprintf("START=%d\n", int(math.Round(start)))
		_, err = f.WriteString(startLine)
		if err != nil {
			return err
		}
		if isLast {
			endLine := fmt.Sprintf("END=%d\n", dur)
			_, err = f.WriteString(endLine)
			if err != nil {
				return err
			}
		} else {
			endLine := fmt.Sprintf("END=%d\n", int(math.Round(nextChapStart)-1))
			_, err = f.WriteString(endLine)
			if err != nil {
				return err
			}
		}
		_, err = f.WriteString("TITLE=" + m["chaptername"].(string) + "\n")
		if err != nil {
			return err
		}
	}
	return nil
}

// TsToMp4 converts a TS file to MP4 using ffmpeg, optionally including chapters.
func TsToMp4(vidPathTs, vidPath, ffmpegNameStr string, chapAvail bool) error {
	var (
		errBuffer bytes.Buffer
		args      []string
	)
	if chapAvail {
		args = []string{
			"-hide_banner", "-i", vidPathTs, "-f", "ffmetadata",
			"-i", chapsFileFname, "-map_metadata", "1", "-c", "copy", vidPath,
		}
	} else {
		args = []string{"-hide_banner", "-i", vidPathTs, "-c", "copy", vidPath}
	}
	cmd := exec.Command(ffmpegNameStr, args...)
	cmd.Stderr = &errBuffer
	err := cmd.Run()
	if err != nil {
		errString := fmt.Sprintf("%s\n%s", err, errBuffer.String())
		return errors.New(errString)
	}
	return nil
}

// GetLstreamContainer finds the latest available livestream container.
func GetLstreamContainer(containers []*model.AlbArtResp) *model.AlbArtResp {
	for i := len(containers) - 1; i >= 0; i-- {
		c := containers[i]
		if c.AvailabilityTypeStr == "AVAILABLE" && c.ContainerTypeStr == "Show" {
			return c
		}
	}
	return nil
}

// ParseLstreamMeta parses livestream metadata into album metadata format.
func ParseLstreamMeta(_meta *model.ArtistMeta) *model.AlbumMeta {
	meta := GetLstreamContainer(_meta.Response.Containers)
	parsed := &model.AlbumMeta{
		Response: &model.AlbArtResp{
			ArtistName:        meta.ArtistName,
			ContainerInfo:     meta.ContainerInfo,
			ContainerID:       meta.ContainerID,
			VideoChapters:     meta.VideoChapters,
			Products:          meta.Products,
			ProductFormatList: meta.ProductFormatList,
		},
	}
	return parsed
}

// PrepareVideoProgressBox creates or reuses a progress box for video downloads.
func PrepareVideoProgressBox(meta *model.AlbArtResp, cfg *model.Config, progressBox *model.ProgressBoxState, deps *Deps) (*model.ProgressBoxState, bool) {
	if progressBox != nil {
		return progressBox, false
	}

	showNumber := "Video"
	if meta.PerformanceDateShort != "" {
		showNumber = meta.PerformanceDateShort
	}

	box := &model.ProgressBoxState{
		ShowTitle:      meta.ContainerInfo,
		ShowNumber:     showNumber,
		TrackNumber:    1,
		TrackTotal:     1,
		TrackName:      "Video Stream",
		ShowDownloaded: "0 B",
		ShowTotal:      "Unknown",
		RcloneEnabled:  cfg.RcloneEnabled,
		StartTime:      time.Now(),
		RenderInterval: model.DefaultProgressRenderInterval,
	}

	box.SetPhase("verify", ui.ColorYellow)
	if deps.SetCurrentProgressBox != nil {
		deps.SetCurrentProgressBox(box)
	}
	if deps.RenderProgressBox != nil {
		deps.RenderProgressBox(box)
	}

	return box, true
}

// Video downloads a video from Nugs.net using the provided videoID.
func Video(videoID, uguID string, cfg *model.Config, streamParams *model.StreamParams, _meta *model.AlbArtResp, isLstream bool, progressBox *model.ProgressBoxState, deps *Deps) error {
	var (
		chapsAvail  bool
		skuID       int
		manifestUrl string
		meta        *model.AlbArtResp
		err         error
	)

	if _meta != nil {
		meta = _meta
	} else {
		m, err := api.GetAlbumMeta(videoID)
		if err != nil {
			ui.PrintError("Failed to get metadata")
			return err
		}
		meta = m.Response
	}

	progressBox, createdProgressBox := PrepareVideoProgressBox(meta, cfg, progressBox, deps)
	if createdProgressBox {
		defer func() {
			if deps.SetCurrentProgressBox != nil {
				deps.SetCurrentProgressBox(nil)
			}
		}()
	}

	if !cfg.SkipChapters {
		chapsAvail = !reflect.ValueOf(meta.VideoChapters).IsZero()
	}

	videoFname := helpers.BuildAlbumFolderName(meta.ArtistName, meta.ContainerInfo, 110)
	fmt.Println(videoFname)
	if len([]rune(meta.ArtistName+" - "+strings.TrimRight(meta.ContainerInfo, " "))) > 110 {
		fmt.Println(
			"Video filename was chopped because it exceeds 110 characters.")
	}
	if isLstream {
		skuID = GetLstreamSku(meta.ProductFormatList)
	} else {
		skuID = GetVideoSku(meta.Products)
	}
	if skuID == 0 {
		return errors.New("no video available")
	}
	if uguID == "" {
		manifestUrl, err = api.GetStreamMeta(
			meta.ContainerID, skuID, 0, streamParams)
	} else {
		manifestUrl, err = api.GetPurchasedManURL(skuID, videoID, streamParams.UserID, uguID)
	}

	if err != nil {
		ui.PrintError("Failed to get video file metadata")
		return err
	} else if manifestUrl == "" {
		return errors.New("the api didn't return a video manifest url")
	}
	variant, retRes, err := ChooseVariant(manifestUrl, cfg.WantRes)
	if err != nil {
		ui.PrintError("Failed to get video master manifest")
		return err
	}

	// Create artist directory
	artistFolder := helpers.Sanitise(meta.ArtistName)
	artistPath := filepath.Join(helpers.GetVideoOutPath(cfg), artistFolder)
	err = helpers.MakeDirs(artistPath)
	if err != nil {
		ui.PrintError("Failed to make artist folder")
		return err
	}

	vidPathNoExt := filepath.Join(artistPath, helpers.Sanitise(videoFname+"_"+retRes))
	vidPathTs := vidPathNoExt + ".ts"
	vidPath := vidPathNoExt + ".mp4"
	exists, err := helpers.FileExists(vidPath)
	if err != nil {
		ui.PrintError("Failed to check if video already exists locally")
		return err
	}
	if exists {
		ui.PrintInfo(fmt.Sprintf("Video exists %s skipping", ui.SymbolArrow))
		return nil
	}
	if cfg.RcloneEnabled {
		remoteVideoPath := path.Join(artistFolder, filepath.Base(vidPath))
		ui.PrintInfo(fmt.Sprintf("Checking remote for video: %s%s%s", ui.ColorCyan, filepath.Base(vidPath), ui.ColorReset))
		remoteExists, checkErr := deps.RemotePathExists(remoteVideoPath, cfg, true)
		if checkErr != nil {
			ui.PrintWarning(fmt.Sprintf("Failed to check remote video path: %v", checkErr))
		} else if remoteExists {
			ui.PrintSuccess(fmt.Sprintf("Video found on remote %s skipping", ui.SymbolArrow))
			return nil
		}
	}
	manBaseUrl, query, err := GetManifestBase(manifestUrl)
	if err != nil {
		ui.PrintError("Failed to get video manifest base URL")
		return err
	}

	segUrls, err := GetSegUrls(manBaseUrl+variant.URI, query)
	if err != nil {
		ui.PrintError("Failed to get video segment URLs")
		return err
	}
	// Player album page videos aren't always only the first seg for the entire vid.
	isLstream, err = api.IsLikelyLivestreamSegments(segUrls)
	if err != nil {
		return err
	}

	if !isLstream {
		fmt.Printf("%.3f FPS, ", variant.FrameRate)
	}
	fmt.Printf("%d Kbps, %s (%s)\n",
		variant.Bandwidth/1000, retRes, variant.Resolution)

	if progressBox != nil {
		progressBox.Mu.Lock()
		progressBox.TrackName = filepath.Base(vidPath)
		progressBox.TrackFormat = fmt.Sprintf("%d Kbps, %s (%s)", variant.Bandwidth/1000, retRes, variant.Resolution)
		progressBox.Downloaded = "0 B"
		progressBox.DownloadTotal = "Unknown"
		progressBox.ShowDownloaded = "0 B"
		progressBox.ShowTotal = "Unknown"
		progressBox.SetPhase("download", ui.ColorGreen)
		progressBox.Mu.Unlock()
		progressBox.SetMessage(model.MessagePriorityStatus, "Downloading video stream", 5*time.Second)
		if deps.RenderProgressBox != nil {
			deps.RenderProgressBox(progressBox)
		}
	}

	if isLstream {
		err = DownloadLstream(vidPathTs, manBaseUrl, segUrls, func(segNum, segTotal int) {
			if progressBox == nil {
				return
			}
			percent := 0
			if segTotal > 0 {
				percent = int(float64(segNum) / float64(segTotal) * 100)
			}
			progressBox.Mu.Lock()
			progressBox.DownloadPercent = percent
			progressBox.Downloaded = fmt.Sprintf("%d segments", segNum)
			progressBox.DownloadTotal = fmt.Sprintf("%d segments", segTotal)
			progressBox.ShowPercent = percent
			progressBox.ShowDownloaded = progressBox.Downloaded
			progressBox.ShowTotal = progressBox.DownloadTotal
			progressBox.Mu.Unlock()
			if deps.RenderProgressBox != nil {
				deps.RenderProgressBox(progressBox)
			}
		})
	} else {
		err = DownloadVideoFile(vidPathTs, manBaseUrl+segUrls[0], func(downloaded, total, speed int64) {
			if progressBox == nil {
				return
			}
			totalStr := "Unknown"
			percent := 0
			if total > 0 {
				totalStr = humanize.Bytes(uint64(total))
				percent = int(float64(downloaded) / float64(total) * 100)
			}

			progressBox.Mu.Lock()
			progressBox.DownloadPercent = percent
			progressBox.DownloadSpeed = humanize.Bytes(uint64(speed))
			progressBox.Downloaded = humanize.Bytes(uint64(downloaded))
			progressBox.DownloadTotal = totalStr
			progressBox.ShowPercent = percent
			progressBox.ShowDownloaded = progressBox.Downloaded
			progressBox.ShowTotal = progressBox.DownloadTotal
			progressBox.Mu.Unlock()
			if deps.RenderProgressBox != nil {
				deps.RenderProgressBox(progressBox)
			}
		})
	}
	if err != nil {
		ui.PrintError("Failed to download video segments")
		return err
	}
	if chapsAvail {
		dur, getDurErr := GetDuration(vidPathTs, cfg.FfmpegNameStr)
		if getDurErr != nil {
			ui.PrintError("Failed to get TS duration")
			return getDurErr
		}
		err = WriteChapsFile(meta.VideoChapters, dur)
		if err != nil {
			ui.PrintError("Failed to write chapters file")
			return err
		}
	}
	ui.PrintInfo("Putting into MP4 container...")
	if progressBox != nil {
		progressBox.SetPhase("verify", ui.ColorYellow)
		progressBox.SetMessage(model.MessagePriorityStatus, "Converting TS to MP4", 5*time.Second)
		if deps.RenderProgressBox != nil {
			deps.RenderProgressBox(progressBox)
		}
	}
	err = TsToMp4(vidPathTs, vidPath, cfg.FfmpegNameStr, chapsAvail)
	if err != nil {
		ui.PrintError("Failed to put TS into MP4 container")
		return err
	}
	if chapsAvail {
		err = os.Remove(chapsFileFname)
		if err != nil {
			ui.PrintError("Failed to delete chapters file")
		}
	}
	err = os.Remove(vidPathTs)
	if err != nil {
		ui.PrintError("Failed to delete TS")
	}

	// Upload to rclone if enabled
	if cfg.RcloneEnabled && deps.UploadToRclone != nil {
		if progressBox != nil {
			progressBox.SetPhase("upload", ui.ColorBlue)
			progressBox.SetMessage(model.MessagePriorityStatus, "Uploading video to rclone", 5*time.Second)
			if deps.RenderProgressBox != nil {
				deps.RenderProgressBox(progressBox)
			}
		}
		// Upload the video file to the artist folder on remote
		err = deps.UploadToRclone(vidPath, artistFolder, cfg, progressBox, true)
		if err != nil {
			helpers.HandleErr("Upload failed.", err, false)
		}
	}

	if progressBox != nil {
		progressBox.Mu.Lock()
		progressBox.IsComplete = true
		progressBox.CompletionTime = time.Now()
		progressBox.TotalDuration = time.Since(progressBox.StartTime)
		progressBox.Mu.Unlock()
	}

	if createdProgressBox && deps.RenderCompletionSummary != nil {
		deps.RenderCompletionSummary(progressBox)
		fmt.Println("")
	}

	return nil
}
