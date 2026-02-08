package main

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
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/grafov/m3u8"
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

func getVideoSku(products []Product) int {
	for _, product := range products {
		formatStr := product.FormatStr
		if formatStr == "VIDEO ON DEMAND" || formatStr == "LIVE HD VIDEO" {
			return product.SkuID
		}
	}
	return 0
}

func getLstreamSku(products []*ProductFormatList) int {
	for _, product := range products {
		if product.FormatStr == "LIVE HD VIDEO" {
			return product.SkuID
		}
	}
	return 0
}

func getVidVariant(variants []*m3u8.Variant, wantRes string) *m3u8.Variant {
	for _, variant := range variants {
		if strings.HasSuffix(variant.Resolution, "x"+wantRes) {
			return variant
		}
	}
	return nil
}

func formatRes(res string) string {
	if res == "2160" {
		return "4K"
	} else {
		return res + "p"
	}
}

func chooseVariant(manifestUrl, wantRes string) (*m3u8.Variant, string, error) {
	origWantRes := wantRes
	var wantVariant *m3u8.Variant
	req, err := client.Get(manifestUrl)
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
		varRes = formatRes(varRes)
		return variant, varRes, nil
	}
	for {
		wantVariant = getVidVariant(master.Variants, wantRes)
		if wantVariant != nil {
			break
		} else {
			wantRes = resFallback[wantRes]
		}
	}
	// wantVariant is guaranteed non-nil after loop exit
	if wantRes != origWantRes {
		printInfo("Unavailable in your chosen format")
	}
	wantRes = formatRes(wantRes)
	return wantVariant, wantRes, nil
}

func getManifestBase(manifestUrl string) (string, string, error) {
	u, err := url.Parse(manifestUrl)
	if err != nil {
		return "", "", err
	}
	path := u.Path
	lastPathIdx := strings.LastIndex(path, "/")
	base := u.Scheme + "://" + u.Host + path[:lastPathIdx+1]
	return base, "?" + u.RawQuery, nil
}

func getSegUrls(manifestUrl, query string) ([]string, error) {
	var segUrls []string
	req, err := client.Get(manifestUrl)
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

func downloadVideo(videoPath, _url string) error {
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
	do, err := client.Do(req)
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
	counter := &WriteCounter{
		Total:      totalBytes,
		TotalStr:   humanize.Bytes(uint64(totalBytes)),
		StartTime:  time.Now().UnixMilli(),
		Downloaded: startByte,
	}
	_, err = io.Copy(f, io.TeeReader(do.Body, counter))
	fmt.Println("")
	return err
}

func downloadLstream(videoPath, baseUrl string, segUrls []string) error {
	f, err := os.OpenFile(videoPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer f.Close()
	segTotal := len(segUrls)
	for segNum, segUrl := range segUrls {
		segNum++
		fmt.Printf("\rSegment %d of %d.", segNum, segTotal)
		req, err := http.NewRequest(http.MethodGet, baseUrl+segUrl, nil)
		if err != nil {
			return err
		}
		do, err := client.Do(req)
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
	fmt.Println("")
	return err
}

func extractDuration(errStr string) string {
	regex := regexp.MustCompile(durRegex)
	match := regex.FindStringSubmatch(errStr)
	if match != nil {
		return match[1]
	}
	return ""
}

func parseDuration(dur string) (int, error) {
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

// Horrible, but best way without ffprobe.
// My native Go duration calculation's too slow. Is there a way without having to iterate over all the packets?
func getDuration(tsPath, ffmpegNameStr string) (int, error) {
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
	dur := extractDuration(errStr)
	if dur == "" {
		return 0, errors.New("No regex match.")
	}
	durSecs, err := parseDuration(dur)
	if err != nil {
		return 0, err
	}
	return durSecs, nil
}

func getNextChapStart(chapters []any, idx int) float64 {
	for i, chapter := range chapters {
		if i == idx {
			m := chapter.(map[string]any)
			return m["chapterSeconds"].(float64)
		}
	}
	return 0
}

func writeChapsFile(chapters []any, dur int) error {
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
			nextChapStart = getNextChapStart(chapters, i)
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

// There are native MPEG demuxers and MP4 muxers for Go, but they're too slow.
func tsToMp4(VidPathTs, vidPath, ffmpegNameStr string, chapAvail bool) error {
	var (
		errBuffer bytes.Buffer
		args      []string
	)
	if chapAvail {
		args = []string{
			"-hide_banner", "-i", VidPathTs, "-f", "ffmetadata",
			"-i", chapsFileFname, "-map_metadata", "1", "-c", "copy", vidPath,
		}
	} else {
		args = []string{"-hide_banner", "-i", VidPathTs, "-c", "copy", vidPath}
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

func getLstreamContainer(containers []*AlbArtResp) *AlbArtResp {
	for i := len(containers) - 1; i >= 0; i-- {
		c := containers[i]
		if c.AvailabilityTypeStr == "AVAILABLE" && c.ContainerTypeStr == "Show" {
			return c
		}
	}
	return nil
}

func parseLstreamMeta(_meta *ArtistMeta) *AlbumMeta {
	meta := getLstreamContainer(_meta.Response.Containers)
	parsed := &AlbumMeta{
		Response: &AlbArtResp{
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

// video downloads a video from Nugs.net using the provided videoID.
// If _meta is provided, uses it instead of fetching metadata. uguID is used for purchased videos.
// The isLstream parameter indicates whether this is a livestream video.
// Downloads video in the highest quality matching cfg.VideoFormat, processes chapters if available,
// converts from TS to MP4 container, and optionally uploads to rclone if configured.
// Returns an error if metadata fetching, download, conversion, or upload fails.
func video(videoID, uguID string, cfg *Config, streamParams *StreamParams, _meta *AlbArtResp, isLstream bool, progressBox *ProgressBoxState) error {
	var (
		chapsAvail  bool
		skuID       int
		manifestUrl string
		meta        *AlbArtResp
		err         error
	)

	if _meta != nil {
		meta = _meta
	} else {
		m, err := getAlbumMeta(videoID)
		if err != nil {
			printError("Failed to get metadata")
			return err
		}
		meta = m.Response
	}

	if !cfg.SkipChapters {
		chapsAvail = !reflect.ValueOf(meta.VideoChapters).IsZero()
	}

	videoFname := buildAlbumFolderName(meta.ArtistName, meta.ContainerInfo, 110)
	fmt.Println(videoFname)
	if len([]rune(meta.ArtistName+" - "+strings.TrimRight(meta.ContainerInfo, " "))) > 110 {
		fmt.Println(
			"Video filename was chopped because it exceeds 110 characters.")
	}
	if isLstream {
		skuID = getLstreamSku(meta.ProductFormatList)
	} else {
		skuID = getVideoSku(meta.Products)
	}
	if skuID == 0 {
		return errors.New("no video available")
	}
	if uguID == "" {
		manifestUrl, err = getStreamMeta(
			meta.ContainerID, skuID, 0, streamParams)
	} else {
		manifestUrl, err = getPurchasedManUrl(skuID, videoID, streamParams.UserID, uguID)
	}

	if err != nil {
		printError("Failed to get video file metadata")
		return err
	} else if manifestUrl == "" {
		return errors.New("the api didn't return a video manifest url")
	}
	variant, retRes, err := chooseVariant(manifestUrl, cfg.WantRes)
	if err != nil {
		printError("Failed to get video master manifest")
		return err
	}

	// Create artist directory
	artistFolder := sanitise(meta.ArtistName)
	artistPath := filepath.Join(cfg.OutPath, artistFolder)
	err = makeDirs(artistPath)
	if err != nil {
		printError("Failed to make artist folder")
		return err
	}

	vidPathNoExt := filepath.Join(artistPath, sanitise(videoFname+"_"+retRes))
	VidPathTs := vidPathNoExt + ".ts"
	vidPath := vidPathNoExt + ".mp4"
	exists, err := fileExists(vidPath)
	if err != nil {
		printError("Failed to check if video already exists locally")
		return err
	}
	if exists {
		printInfo(fmt.Sprintf("Video exists %s skipping", symbolArrow))
		return nil
	}
	manBaseUrl, query, err := getManifestBase(manifestUrl)
	if err != nil {
		printError("Failed to get video manifest base URL")
		return err
	}

	segUrls, err := getSegUrls(manBaseUrl+variant.URI, query)
	if err != nil {
		printError("Failed to get video segment URLs")
		return err
	}
	// Player album page videos aren't always only the first seg for the entire vid.
	isLstream, err = isLikelyLivestreamSegments(segUrls)
	if err != nil {
		return err
	}

	if !isLstream {
		fmt.Printf("%.3f FPS, ", variant.FrameRate)
	}
	fmt.Printf("%d Kbps, %s (%s)\n",
		variant.Bandwidth/1000, retRes, variant.Resolution)
	if isLstream {
		err = downloadLstream(VidPathTs, manBaseUrl, segUrls)
	} else {
		err = downloadVideo(VidPathTs, manBaseUrl+segUrls[0])
	}
	if err != nil {
		printError("Failed to download video segments")
		return err
	}
	if chapsAvail {
		dur, err := getDuration(VidPathTs, cfg.FfmpegNameStr)
		if err != nil {
			printError("Failed to get TS duration")
			return err
		}
		err = writeChapsFile(meta.VideoChapters, dur)
		if err != nil {
			printError("Failed to write chapters file")
			return err
		}
	}
	printInfo("Putting into MP4 container...")
	err = tsToMp4(VidPathTs, vidPath, cfg.FfmpegNameStr, chapsAvail)
	if err != nil {
		printError("Failed to put TS into MP4 container")
		return err
	}
	if chapsAvail {
		err = os.Remove(chapsFileFname)
		if err != nil {
			printError("Failed to delete chapters file")
		}
	}
	err = os.Remove(VidPathTs)
	if err != nil {
		printError("Failed to delete TS")
	}

	// Upload to rclone if enabled
	if cfg.RcloneEnabled {
		// Upload the video file to the artist folder on remote
		err = uploadToRclone(vidPath, artistFolder, cfg, progressBox)
		if err != nil {
			handleErr("Upload failed.", err, false)
		}
	}

	return nil
}
