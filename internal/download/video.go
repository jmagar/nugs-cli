package download

import (
	"bytes"
	"context"
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

var durRegex = regexp.MustCompile(`Duration: ([\d:.]+)`)

var resFallback = map[string]string{
	"720":  "480",
	"1080": "720",
	"1440": "1080",
}

// GetVideoSku finds the video SKU ID from a product list.
func GetVideoSku(products []model.Product) int {
	for _, product := range products {
		formatStr := product.FormatStr
		if formatStr == model.VideoOnDemandFormatLabel || formatStr == model.LiveHDVideoFormatLabel {
			return product.SkuID
		}
	}
	return 0
}

// GetShowMediaType determines what media types a show offers (audio/video/both).
func GetShowMediaType(show *model.AlbArtResp) model.MediaType {
	hasVideo := false
	hasAudio := false

	// Primary source: products (present on catalog.container responses).
	if len(show.Products) > 0 {
		hasVideo = GetVideoSku(show.Products) != 0
		for _, product := range show.Products {
			if product.FormatStr != model.VideoOnDemandFormatLabel &&
				product.FormatStr != model.LiveHDVideoFormatLabel {
				hasAudio = true
				break
			}
		}
	}

	// Fallback source: productFormatList (present on catalog.containersAll responses).
	// This is critical for list media filtering because products are often null there.
	if len(show.ProductFormatList) > 0 {
		if GetLstreamSku(show.ProductFormatList) != 0 {
			hasVideo = true
		}
		for _, format := range show.ProductFormatList {
			if format == nil {
				continue
			}
			if format.FormatStr != model.VideoOnDemandFormatLabel &&
				format.FormatStr != model.LiveHDVideoFormatLabel {
				hasAudio = true
				break
			}
		}
	}

	// If both format lists are empty, we can't determine the media type.
	// However, since catalog.containersAll with availType includes shows based on availability,
	// if we have no format data, assume the show has both audio and video to be inclusive.
	// This ensures shows with incomplete metadata aren't incorrectly filtered out.
	if len(show.Products) == 0 && len(show.ProductFormatList) == 0 {
		return model.MediaTypeBoth
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
		if product.FormatStr == model.LiveHDVideoFormatLabel {
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
	if res == model.Res2160 {
		return model.Res4K
	}
	return res + model.Resp
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
	master, ok := playlist.(*m3u8.MasterPlaylist)
	if !ok {
		return nil, "", errors.New("expected HLS master playlist but got media playlist")
	}
	sort.Slice(master.Variants, func(x, y int) bool {
		return master.Variants[x].Bandwidth > master.Variants[y].Bandwidth
	})
	if wantRes == model.Res2160 {
		variant := master.Variants[0]
		parts := strings.SplitN(variant.Resolution, "x", 2)
		if len(parts) != 2 {
			return nil, "", fmt.Errorf("invalid resolution format: %s", variant.Resolution)
		}
		varRes := FormatRes(parts[1])
		return variant, varRes, nil
	}
	// Guard against infinite loop: max 10 fallback attempts
	maxFallbacks := model.MaxFormatFallbackAttempts
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
				parts := strings.SplitN(wantVariant.Resolution, "x", 2)
				if len(parts) == 2 {
					wantRes = FormatRes(parts[1])
				} else {
					wantRes = model.UnknownResolutionLabel
				}
			}
			break
		}
		wantRes = nextRes
	}
	// Final fallback: if still no variant after max attempts, use highest available
	if wantVariant == nil && len(master.Variants) > 0 {
		wantVariant = master.Variants[0]
		parts := strings.SplitN(wantVariant.Resolution, "x", 2)
		if len(parts) == 2 {
			wantRes = FormatRes(parts[1])
		} else {
			wantRes = model.UnknownResolutionLabel
		}
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
	media, ok := playlist.(*m3u8.MediaPlaylist)
	if !ok {
		return nil, errors.New("expected HLS media playlist but got master playlist")
	}
	for _, seg := range media.Segments {
		if seg == nil {
			break
		}
		segUrls = append(segUrls, seg.URI+query)
	}
	return segUrls, nil
}

// DownloadVideoFile downloads a video file from a URL with progress tracking.
func DownloadVideoFile(ctx context.Context, videoPath, _url string, onProgress func(downloaded, total, speed int64)) error {
	f, err := os.OpenFile(videoPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return err
	}
	startByte := stat.Size()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, _url, nil)
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

	// Server ignored Range header — restart from beginning
	if do.StatusCode == http.StatusOK && startByte > 0 {
		startByte = 0
		if err := f.Truncate(0); err != nil {
			return fmt.Errorf("failed to truncate file for full re-download: %w", err)
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("failed to seek to start: %w", err)
		}
	}

	if startByte > 0 {
		fmt.Printf("TS already exists locally, resuming from byte %d...\n", startByte)
		// Seek to the correct position before writing resumed data
		if _, err := f.Seek(int64(startByte), io.SeekStart); err != nil {
			return fmt.Errorf("failed to seek to resume position: %w", err)
		}
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
	n := len(p)
	speed := updateWriteCounterProgress(s.wc, n)
	if s.wc.OnProgress != nil {
		s.wc.OnProgress(s.wc.Downloaded, s.wc.Total, speed)
	}
	return n, nil
}

// DownloadLstream downloads a livestream video by downloading all segments.
func DownloadLstream(ctx context.Context, videoPath, baseUrl string, segUrls []string, onProgress func(segNum, segTotal int)) error {
	f, err := os.OpenFile(videoPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
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
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseUrl+segUrl, nil)
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
	return nil
}

// ExtractDuration extracts the duration string from ffmpeg output.
func ExtractDuration(errStr string) string {
	match := durRegex.FindStringSubmatch(errStr)
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
	// Check error properly - err can be nil on success
	if err != nil && err.Error() != "exit status 1" {
		return 0, fmt.Errorf("ffmpeg execution failed: %w", err)
	}
	errStr := errBuffer.String()
	ok := strings.HasSuffix(
		strings.TrimSpace(errStr), "At least one output file must be specified")
	if !ok {
		if err != nil {
			return 0, fmt.Errorf("ffmpeg error: %s\n%s", err.Error(), errStr)
		}
		return 0, fmt.Errorf("unexpected ffmpeg output: %s", errStr)
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
func GetNextChapStart(chapters []any, idx int) (float64, error) {
	if idx < 0 || idx >= len(chapters) {
		return 0, fmt.Errorf("chapter index %d out of range", idx)
	}
	m, ok := chapters[idx].(map[string]any)
	if !ok {
		return 0, fmt.Errorf("chapter at index %d is not a map", idx)
	}
	secs, ok := m["chapterSeconds"].(float64)
	if !ok {
		return 0, fmt.Errorf("chapterSeconds missing or not a number at index %d", idx)
	}
	return secs, nil
}

// WriteChapsFile writes an ffmpeg chapters metadata file and returns the temp file path.
func WriteChapsFile(chapters []any, dur int) (string, error) {
	f, err := os.CreateTemp("", "nugs-chapters-*.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create chapters temp file: %w", err)
	}
	defer f.Close()
	chapsPath := f.Name()

	_, err = f.WriteString(";FFMETADATA1\n")
	if err != nil {
		return chapsPath, err
	}
	chaptersCount := len(chapters)

	for i, chapter := range chapters {
		i++
		isLast := i == chaptersCount

		m, ok := chapter.(map[string]any)
		if !ok {
			return chapsPath, fmt.Errorf("chapter at index %d is not a map", i-1)
		}
		start, ok := m["chapterSeconds"].(float64)
		if !ok {
			return chapsPath, fmt.Errorf("chapterSeconds missing or not a number at index %d", i-1)
		}

		if !isLast {
			nextChapStart, nextErr := GetNextChapStart(chapters, i)
			if nextErr != nil {
				return chapsPath, nextErr
			}
			if nextChapStart <= start {
				continue
			}
			end := int(math.Round(nextChapStart)) - 1
			if end < 0 {
				end = 0
			}
			_, err = f.WriteString(fmt.Sprintf("\n[CHAPTER]\nTIMEBASE=1/1\nSTART=%d\nEND=%d\n",
				int(math.Round(start)), end))
		} else {
			_, err = f.WriteString(fmt.Sprintf("\n[CHAPTER]\nTIMEBASE=1/1\nSTART=%d\nEND=%d\n",
				int(math.Round(start)), dur))
		}
		if err != nil {
			return chapsPath, err
		}

		title, _ := m["chaptername"].(string)
		_, err = f.WriteString("TITLE=" + title + "\n")
		if err != nil {
			return chapsPath, err
		}
	}
	return chapsPath, nil
}

// TsToMp4 converts a TS file to MP4 using ffmpeg, optionally including chapters.
// chapsFilePath is the path to the chapters metadata file (empty if no chapters).
func TsToMp4(vidPathTs, vidPath, ffmpegNameStr, chapsFilePath string) error {
	var (
		errBuffer bytes.Buffer
		args      []string
	)
	if chapsFilePath != "" {
		args = []string{
			"-hide_banner", "-i", vidPathTs, "-f", "ffmetadata",
			"-i", chapsFilePath, "-map_metadata", "1", "-c", "copy", vidPath,
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
		if c.AvailabilityTypeStr == model.AvailableAvailabilityType && c.ContainerTypeStr == model.ShowContainerType {
			return c
		}
	}
	return nil
}

// ParseLstreamMeta parses livestream metadata into album metadata format.
func ParseLstreamMeta(_meta *model.ArtistMeta) (*model.AlbumMeta, error) {
	meta := GetLstreamContainer(_meta.Response.Containers)
	if meta == nil {
		return nil, errors.New("no available livestream container found")
	}
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
	return parsed, nil
}

// PrepareVideoProgressBox creates or reuses a progress box for video downloads.
func PrepareVideoProgressBox(meta *model.AlbArtResp, cfg *model.Config, progressBox *model.ProgressBoxState, deps *Deps) (*model.ProgressBoxState, bool) {
	if progressBox != nil {
		return progressBox, false
	}

	showNumber := model.VideoShowNumberDefault
	if meta.PerformanceDateShort != "" {
		showNumber = meta.PerformanceDateShort
	}

	box := &model.ProgressBoxState{
		ShowTitle:      meta.ContainerInfo,
		ShowNumber:     showNumber,
		TrackNumber:    1,
		TrackTotal:     1,
		TrackName:      model.VideoTrackNameDefault,
		ShowDownloaded: model.ZeroBytesLabel,
		ShowTotal:      model.UnknownSizeLabel,
		RcloneEnabled:  cfg.RcloneEnabled,
		StartTime:      time.Now(),
		RenderInterval: model.DefaultProgressRenderInterval,
	}

	box.SetPhase(model.PhaseVerify)
	if deps.SetCurrentProgressBox != nil {
		deps.SetCurrentProgressBox(box)
	}
	if deps.RenderProgressBox != nil {
		deps.RenderProgressBox(box)
	}

	return box, true
}

// resolveVideoMeta retrieves video metadata and determines the SKU ID.
func resolveVideoMeta(ctx context.Context, videoID string, _meta *model.AlbArtResp, isLstream bool) (*model.AlbArtResp, int, error) {
	var meta *model.AlbArtResp
	if _meta != nil {
		meta = _meta
	} else {
		m, err := api.GetAlbumMeta(ctx, videoID)
		if err != nil {
			ui.PrintError("Failed to get metadata")
			return nil, 0, err
		}
		meta = m.Response
	}
	var skuID int
	if isLstream {
		skuID = GetLstreamSku(meta.ProductFormatList)
	} else {
		skuID = GetVideoSku(meta.Products)
	}
	if skuID == 0 {
		return nil, 0, errors.New("no video available")
	}
	return meta, skuID, nil
}

// resolveManifestAndVariant fetches the manifest URL and selects the best video variant.
func resolveManifestAndVariant(ctx context.Context, meta *model.AlbArtResp, videoID, uguID string, skuID int, streamParams *model.StreamParams, cfg *model.Config) (string, *m3u8.Variant, string, error) {
	var (
		manifestUrl string
		err         error
	)
	if uguID == "" {
		manifestUrl, err = api.GetStreamMeta(ctx, meta.ContainerID, skuID, 0, streamParams)
	} else {
		manifestUrl, err = api.GetPurchasedManURL(ctx, skuID, videoID, streamParams.UserID, uguID)
	}
	if err != nil {
		ui.PrintError("Failed to get video file metadata")
		return "", nil, "", err
	}
	if manifestUrl == "" {
		return "", nil, "", errors.New("the api didn't return a video manifest url")
	}
	variant, retRes, err := ChooseVariant(manifestUrl, cfg.WantRes)
	if err != nil {
		ui.PrintError("Failed to get video master manifest")
		return "", nil, "", err
	}
	return manifestUrl, variant, retRes, nil
}

// prepareVideoPathsAndCheck creates directories and checks for existing files locally/remotely.
// Returns artistFolder, vidPathTs, vidPath, and whether the video should be skipped.
func prepareVideoPathsAndCheck(ctx context.Context, meta *model.AlbArtResp, videoFname, retRes string, cfg *model.Config, deps *Deps) (string, string, string, bool, error) {
	artistFolder := helpers.Sanitise(meta.ArtistName)
	artistPath := filepath.Join(helpers.GetVideoOutPath(cfg), artistFolder)
	if err := helpers.MakeDirs(artistPath); err != nil {
		ui.PrintError("Failed to make artist folder")
		return "", "", "", false, err
	}
	vidPathNoExt := filepath.Join(artistPath, helpers.Sanitise(videoFname+"_"+retRes))
	vidPathTs := vidPathNoExt + ".ts"
	vidPath := vidPathNoExt + ".mp4"
	exists, err := helpers.FileExists(vidPath)
	if err != nil {
		ui.PrintError("Failed to check if video already exists locally")
		return "", "", "", false, err
	}
	if exists {
		ui.PrintInfo(fmt.Sprintf("Video exists %s skipping", ui.SymbolArrow))
		return "", "", "", true, nil
	}
	if cfg.RcloneEnabled {
		remoteVideoPath := path.Join(artistFolder, filepath.Base(vidPath))
		ui.PrintInfo(fmt.Sprintf("Checking remote for video: %s%s%s", ui.ColorCyan, filepath.Base(vidPath), ui.ColorReset))
		remoteExists, checkErr := deps.CheckRemotePathExists(ctx, remoteVideoPath, cfg, true)
		if checkErr != nil {
			ui.PrintWarning(fmt.Sprintf("Failed to check remote video path: %v", checkErr))
		} else if remoteExists {
			ui.PrintSuccess(fmt.Sprintf("Video found on remote %s skipping", ui.SymbolArrow))
			return "", "", "", true, nil
		}
	}
	return artistFolder, vidPathTs, vidPath, false, nil
}

// downloadVideoContent downloads the video content (livestream segments or single file).
func downloadVideoContent(ctx context.Context, vidPathTs, manBaseUrl string, segUrls []string, isLstream bool, progressBox *model.ProgressBoxState, deps *Deps) error {
	if isLstream {
		return DownloadLstream(ctx, vidPathTs, manBaseUrl, segUrls, func(segNum, segTotal int) {
			if progressBox == nil {
				return
			}
			percent := 0
			if segTotal > 0 {
				percent = int(float64(segNum) / float64(segTotal) * float64(model.MaxProgressPercent))
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
	}
	return DownloadVideoFile(ctx, vidPathTs, manBaseUrl+segUrls[0], func(downloaded, total, speed int64) {
		if progressBox == nil {
			return
		}
		totalStr := model.UnknownSizeLabel
		percent := 0
		if total > 0 {
			totalStr = humanize.Bytes(uint64(total))
			percent = int(float64(downloaded) / float64(total) * float64(model.MaxProgressPercent))
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

// convertAndUploadVideo handles chapter extraction, TS-to-MP4 conversion, and optional upload.
func convertAndUploadVideo(ctx context.Context, vidPathTs, vidPath, artistFolder string, meta *model.AlbArtResp, cfg *model.Config, chapsAvail bool, progressBox *model.ProgressBoxState, deps *Deps) error {
	var chapsFilePath string
	if chapsAvail {
		dur, getDurErr := GetDuration(vidPathTs, cfg.FfmpegNameStr)
		if getDurErr != nil {
			ui.PrintError("Failed to get TS duration")
			return getDurErr
		}
		var err error
		chapsFilePath, err = WriteChapsFile(meta.VideoChapters, dur)
		if err != nil {
			if chapsFilePath != "" {
				os.Remove(chapsFilePath)
			}
			ui.PrintError("Failed to write chapters file")
			return err
		}
		defer os.Remove(chapsFilePath)
	}
	ui.PrintInfo("Putting into MP4 container...")
	if progressBox != nil {
		progressBox.SetPhase(model.PhaseVerify)
		progressBox.SetMessage(model.MessagePriorityStatus, model.VideoConvertStatusLabel, model.StatusMessageDuration)
		if deps.RenderProgressBox != nil {
			deps.RenderProgressBox(progressBox)
		}
	}
	if err := TsToMp4(vidPathTs, vidPath, cfg.FfmpegNameStr, chapsFilePath); err != nil {
		ui.PrintError("Failed to put TS into MP4 container")
		return err
	}
	if err := os.Remove(vidPathTs); err != nil {
		ui.PrintError("Failed to delete TS")
	}
	if cfg.RcloneEnabled {
		if progressBox != nil {
			progressBox.SetPhase(model.PhaseUpload)
			progressBox.SetMessage(model.MessagePriorityStatus, model.VideoUploadStatusLabel, model.StatusMessageDuration)
			if deps.RenderProgressBox != nil {
				deps.RenderProgressBox(progressBox)
			}
		}
		if err := deps.UploadPath(ctx, vidPath, artistFolder, cfg, progressBox, true); err != nil {
			helpers.HandleErr("Upload failed.", err, false)
		}
	}
	return nil
}

// Video downloads a video from Nugs.net using the provided videoID.
func Video(ctx context.Context, videoID, uguID string, cfg *model.Config, streamParams *model.StreamParams, _meta *model.AlbArtResp, isLstream bool, progressBox *model.ProgressBoxState, deps *Deps) error {
	meta, skuID, err := resolveVideoMeta(ctx, videoID, _meta, isLstream)
	if err != nil {
		return err
	}

	progressBox, createdProgressBox := PrepareVideoProgressBox(meta, cfg, progressBox, deps)
	if createdProgressBox {
		defer func() {
			if deps.SetCurrentProgressBox != nil {
				deps.SetCurrentProgressBox(nil)
			}
		}()
	}

	chapsAvail := !cfg.SkipChapters && len(meta.VideoChapters) > 0
	videoFname := helpers.BuildAlbumFolderName(meta.ArtistName, meta.ContainerInfo, model.VideoNameMaxRunes)
	fmt.Println(videoFname)
	if len([]rune(meta.ArtistName+" - "+strings.TrimRight(meta.ContainerInfo, " "))) > model.VideoNameMaxRunes {
		fmt.Printf("Video filename was chopped because it exceeds %d characters.\n", model.VideoNameMaxRunes)
	}

	manifestUrl, variant, retRes, err := resolveManifestAndVariant(ctx, meta, videoID, uguID, skuID, streamParams, cfg)
	if err != nil {
		return err
	}

	artistFolder, vidPathTs, vidPath, skipped, err := prepareVideoPathsAndCheck(ctx, meta, videoFname, retRes, cfg, deps)
	if err != nil || skipped {
		return err
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
	isLstream, err = api.IsLikelyLivestreamSegments(segUrls)
	if err != nil {
		return err
	}

	if !isLstream {
		fmt.Printf("%.3f FPS, ", variant.FrameRate)
	}
	fmt.Printf("%d Kbps, %s (%s)\n",
		variant.Bandwidth/model.KBpsDivisor, retRes, variant.Resolution)

	if progressBox != nil {
		progressBox.Mu.Lock()
		progressBox.TrackName = filepath.Base(vidPath)
		progressBox.TrackFormat = fmt.Sprintf("%d Kbps, %s (%s)", variant.Bandwidth/model.KBpsDivisor, retRes, variant.Resolution)
		progressBox.Downloaded = model.ZeroBytesLabel
		progressBox.DownloadTotal = model.UnknownSizeLabel
		progressBox.ShowDownloaded = model.ZeroBytesLabel
		progressBox.ShowTotal = model.UnknownSizeLabel
		progressBox.SetPhaseLocked(model.PhaseDownload)
		progressBox.Mu.Unlock()
		progressBox.SetMessage(model.MessagePriorityStatus, model.VideoDownloadStatusLabel, model.StatusMessageDuration)
		if deps.RenderProgressBox != nil {
			deps.RenderProgressBox(progressBox)
		}
	}

	if err := downloadVideoContent(ctx, vidPathTs, manBaseUrl, segUrls, isLstream, progressBox, deps); err != nil {
		ui.PrintError("Failed to download video segments")
		return err
	}

	if err := convertAndUploadVideo(ctx, vidPathTs, vidPath, artistFolder, meta, cfg, chapsAvail, progressBox, deps); err != nil {
		return err
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
