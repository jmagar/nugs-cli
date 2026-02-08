package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/grafov/m3u8"
)

const bitrateRegex = `[\w]+(?:_(\d+)k_v\d+)`

var trackFallback = map[int]int{
	1: 2,
	2: 5,
	3: 2,
	4: 3,
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
	if err := waitIfPausedOrCancelled(); err != nil {
		return 0, err
	}
	var speed int64 = 0
	n := len(p)
	wc.Downloaded += int64(n)
	if wc.Total > 0 {
		percentage := float64(wc.Downloaded) / float64(wc.Total) * float64(100)
		wc.Percentage = int(percentage)
	}
	toDivideBy := time.Now().UnixMilli() - wc.StartTime
	if toDivideBy != 0 {
		speed = int64(wc.Downloaded) / toDivideBy * 1000
	}
	if wc.OnProgress != nil {
		wc.OnProgress(wc.Downloaded, wc.Total, speed)
	} else {
		printProgress(wc.Percentage, humanize.Bytes(uint64(speed)),
			humanize.Bytes(uint64(wc.Downloaded)), wc.TotalStr)
	}
	return n, nil
}

func downloadTrack(trackPath, _url string, onProgress func(downloaded, total, speed int64), printNewline bool) error {
	if err := waitIfPausedOrCancelled(); err != nil {
		return err
	}
	f, err := os.OpenFile(trackPath, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer f.Close()
	req, err := http.NewRequest(http.MethodGet, _url, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Referer", playerUrl)
	req.Header.Add("User-Agent", userAgent)
	req.Header.Add("Range", "bytes=0-")
	do, err := client.Do(req)
	if err != nil {
		return err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK && do.StatusCode != http.StatusPartialContent {
		return errors.New(do.Status)
	}
	totalBytes := do.ContentLength
	totalStr := "unknown"
	if totalBytes > 0 {
		totalStr = humanize.Bytes(uint64(totalBytes))
	}
	counter := &WriteCounter{
		Total:      totalBytes,
		TotalStr:   totalStr,
		StartTime:  time.Now().UnixMilli(),
		OnProgress: onProgress,
	}
	_, err = io.Copy(f, io.TeeReader(do.Body, counter))
	if printNewline {
		fmt.Println("")
	}
	return err
}

func extractBitrate(manUrl string) string {
	regex := regexp.MustCompile(bitrateRegex)
	match := regex.FindStringSubmatch(manUrl)
	if match != nil {
		return match[1]
	}
	return ""
}

func parseHlsMaster(qual *Quality) error {
	req, err := client.Get(qual.URL)
	if err != nil {
		return err
	}
	defer req.Body.Close()
	if req.StatusCode != http.StatusOK {
		return errors.New(req.Status)
	}

	playlist, _, err := m3u8.DecodeFrom(req.Body, true)
	if err != nil {
		return err
	}
	master := playlist.(*m3u8.MasterPlaylist)
	sort.Slice(master.Variants, func(x, y int) bool {
		return master.Variants[x].Bandwidth > master.Variants[y].Bandwidth
	})
	variantUri := master.Variants[0].URI
	bitrate := extractBitrate(variantUri)
	if bitrate == "" {
		return errors.New("no regex match for manifest bitrate")
	}
	qual.Specs = bitrate + " Kbps AAC"
	manBase, q, err := getManifestBase(qual.URL)
	if err != nil {
		return err
	}
	qual.URL = manBase + variantUri + q
	return nil
}

func getKey(keyUrl string) ([]byte, error) {
	req, err := client.Get(keyUrl)
	if err != nil {
		return nil, err
	}
	defer req.Body.Close()
	if req.StatusCode != http.StatusOK {
		return nil, errors.New(req.Status)
	}
	buf := make([]byte, 16)
	_, err = io.ReadFull(req.Body, buf)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func pkcs5Trimming(data []byte) []byte {
	padding := data[len(data)-1]
	return data[:len(data)-int(padding)]
}

func decryptTrack(key, iv []byte) ([]byte, error) {
	encData, err := os.ReadFile("temp_enc.ts")
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	ecb := cipher.NewCBCDecrypter(block, iv)
	decrypted := make([]byte, len(encData))
	printInfo("Decrypting...")
	ecb.CryptBlocks(decrypted, encData)
	return decrypted, nil
}

func tsToAac(decData []byte, outPath, ffmpegNameStr string) error {
	var errBuffer bytes.Buffer
	cmd := exec.Command(ffmpegNameStr, "-i", "pipe:", "-c:a", "copy", outPath)
	cmd.Stdin = bytes.NewReader(decData)
	cmd.Stderr = &errBuffer
	err := cmd.Run()
	if err != nil {
		errString := fmt.Sprintf("%s\n%s", err, errBuffer.String())
		return errors.New(errString)
	}
	return nil
}

func hlsOnly(trackPath, manUrl, ffmpegNameStr string, onProgress func(downloaded, total, speed int64), printNewline bool) error {
	req, err := client.Get(manUrl)
	if err != nil {
		return err
	}
	defer req.Body.Close()
	if req.StatusCode != http.StatusOK {
		return errors.New(req.Status)
	}
	playlist, _, err := m3u8.DecodeFrom(req.Body, true)
	if err != nil {
		return err
	}
	media := playlist.(*m3u8.MediaPlaylist)

	manBase, q, err := getManifestBase(manUrl)
	if err != nil {
		return err
	}
	tsUrl := manBase + media.Segments[0].URI + q

	key := media.Key
	keyBytes, err := getKey(manBase + key.URI)
	if err != nil {
		return err
	}

	iv, err := hex.DecodeString(key.IV[2:])
	if err != nil {
		return err
	}

	err = downloadTrack("temp_enc.ts", tsUrl, onProgress, printNewline)
	if err != nil {
		return err
	}
	decData, err := decryptTrack(keyBytes, iv)
	if err != nil {
		return err
	}
	err = os.Remove("temp_enc.ts")
	if err != nil {
		return err
	}
	err = tsToAac(decData, trackPath, ffmpegNameStr)
	return err
}

func checkIfHlsOnly(quals []*Quality) bool {
	for _, quality := range quals {
		if !strings.Contains(quality.URL, ".m3u8?") {
			return false
		}
	}
	return true
}

func processTrack(folPath string, trackNum, trackTotal int, cfg *Config, track *Track, streamParams *StreamParams, progressBox *ProgressBoxState) error {
	if err := waitIfPausedOrCancelled(); err != nil {
		return err
	}
	origWantFmt := cfg.Format
	wantFmt := origWantFmt
	var (
		quals      []*Quality
		chosenQual *Quality
	)
	// Call the stream meta endpoint four times to get all avail formats since the formats can shift.
	// This will ensure the right format's always chosen.
	for _, i := range [4]int{1, 4, 7, 10} {
		streamUrl, err := getStreamMeta(track.TrackID, 0, i, streamParams)
		if err != nil {
			printError("Failed to get track stream metadata")
			return err
		} else if streamUrl == "" {
			return errors.New("the api didn't return a track stream URL")
		}
		quality := queryQuality(streamUrl)
		if quality == nil {
			printError(fmt.Sprintf("The API returned an unsupported format: %s", streamUrl))
			continue
			//return errors.New("The API returned an unsupported format.")
		}
		quals = append(quals, quality)
		// if quality.Format == 6 {
		// 	isHlsOnly = true
		// 	break
		// }
	}

	if len(quals) == 0 {
		return errors.New("the api didn't return any formats")
	}

	isHlsOnly := checkIfHlsOnly(quals)

	if isHlsOnly {
		printInfo("HLS-only track. Only AAC is available, tags currently unsupported")
		chosenQual = quals[0]
		err := parseHlsMaster(chosenQual)
		if err != nil {
			return err
		}
	} else {
		for {
			chosenQual = getTrackQual(quals, wantFmt)
			if chosenQual != nil {
				break
			} else {
				// Fallback quality.
				wantFmt = trackFallback[wantFmt]
			}
		}
		// chosenQual is guaranteed non-nil after loop exit
		if wantFmt != origWantFmt && origWantFmt != 4 {
			printInfo("Unavailable in your chosen format")
			// Tier 3: Set quality fallback warning in progress box
			if progressBox != nil {
				fallbackMsg := fmt.Sprintf("Using %s (requested %s unavailable)",
					getQualityName(wantFmt), getQualityName(origWantFmt))
				progressBox.SetMessage(MessagePriorityWarning, fallbackMsg, 5*time.Second)
			}
		}
	}
	trackFname := fmt.Sprintf(
		"%02d. %s%s", trackNum, sanitise(track.SongTitle), chosenQual.Extension,
	)
	trackPath := filepath.Join(folPath, trackFname)
	exists, err := fileExists(trackPath)
	if err != nil {
		printError("Failed to check if track already exists locally")
		return err
	}
	if exists {
		printInfo(fmt.Sprintf("Track exists %s skipping", symbolArrow))
		if progressBox != nil {
			progressBox.SkippedTracks++
			// Tier 3: Set skip indicator message
			skipMsg := fmt.Sprintf("Skipped track %d - already exists", trackNum)
			progressBox.SetMessage(MessagePriorityStatus, skipMsg, 3*time.Second)
		}
		return nil
	}
	// Update progress box state
	if progressBox != nil {
		progressBox.TrackNumber = trackNum
		progressBox.TrackTotal = trackTotal
		progressBox.TrackName = track.SongTitle
		progressBox.TrackFormat = chosenQual.Specs
		progressBox.RcloneEnabled = cfg.RcloneEnabled
	}

	showProgress := func(downloaded, total, speed int64) {
		if progressBox == nil {
			// Fallback to old progress display if no box provided
			trackPercentage := 0
			trackTotalStr := "unknown"
			if total > 0 {
				trackPercentage = int((float64(downloaded) / float64(total)) * 100)
				if trackPercentage > 100 {
					trackPercentage = 100
				}
				trackTotalStr = humanize.Bytes(uint64(total))
			}
			downloadedLabel := fmt.Sprintf("T%02d/%02d %s",
				trackNum, trackTotal, humanize.Bytes(uint64(downloaded)))
			printProgress(trackPercentage, humanize.Bytes(uint64(speed)), downloadedLabel, trackTotalStr)
			return
		}

		// Update progress box with download progress
		trackPercentage := 0
		trackTotalStr := "unknown"
		if total > 0 {
			trackPercentage = int((float64(downloaded) / float64(total)) * 100)
			if trackPercentage > 100 {
				trackPercentage = 100
			}
			trackTotalStr = humanize.Bytes(uint64(total))
		}

		trackProgress := 0.0
		if total > 0 {
			trackProgress = float64(downloaded) / float64(total)
			if trackProgress > 1 {
				trackProgress = 1
			}
		}
		showPercentage := int(((float64(trackNum-1) + trackProgress) / float64(trackTotal)) * 100)
		if showPercentage > 100 {
			showPercentage = 100
		}

		// Lock for atomic update of all progress fields
		progressBox.mu.Lock()
		progressBox.DownloadPercent = trackPercentage
		progressBox.DownloadSpeed = humanize.Bytes(uint64(speed))
		progressBox.Downloaded = humanize.Bytes(uint64(downloaded))
		progressBox.DownloadTotal = trackTotalStr
		progressBox.ShowPercent = showPercentage

		// Calculate accumulated download (completed tracks + current track progress)
		accumulatedBytes := progressBox.AccumulatedBytes + downloaded
		progressBox.ShowDownloaded = humanize.Bytes(uint64(accumulatedBytes))

		// Calculate ETA for current track
		if speed > 0 && total > 0 && downloaded < total {
			// Update speed history for smoothing
			progressBox.SpeedHistory = updateSpeedHistory(progressBox.SpeedHistory, float64(speed))

			// Calculate remaining bytes for current track
			remaining := total - downloaded
			progressBox.DownloadETA = calculateETA(progressBox.SpeedHistory, remaining)
		} else {
			progressBox.DownloadETA = ""
		}
		progressBox.mu.Unlock()

		// Render the updated progress box (outside lock to avoid holding during I/O)
		renderProgressBox(progressBox)
	}
	var trackSize int64
	if isHlsOnly {
		err = hlsOnly(trackPath, chosenQual.URL, cfg.FfmpegNameStr, showProgress, false)
	} else {
		err = downloadTrack(trackPath, chosenQual.URL, showProgress, false)
	}
	if err != nil {
		printError("Failed to download track")
		return err
	}

	// Update accumulated bytes after successful download
	if progressBox != nil {
		// Get the actual file size
		if stat, err := os.Stat(trackPath); err == nil {
			trackSize = stat.Size()
		}
		progressBox.AccumulatedBytes += trackSize
		progressBox.AccumulatedTracks++
	}

	return nil
}

// album downloads an album or show from Nugs.net using the provided albumID.
// If albumID is empty, uses the provided artResp metadata instead of fetching it.
// The function creates artist and album directories, downloads all tracks, and optionally
// uploads to rclone if configured. Skips download if the show already exists locally or on remote.
// Returns an error if metadata fetching, directory creation, or any track download fails.
// preCalculateShowSize calculates the total size of all tracks in a show
// Uses parallel HEAD requests with 8-concurrent semaphore and 5-second timeout per request
// Returns total size in bytes and error. Gracefully degrades if calculation fails.
func preCalculateShowSize(tracks []Track, streamParams *StreamParams, cfg *Config) (int64, error) {
	if cfg.SkipSizePreCalculation {
		return 0, nil
	}

	var totalSize int64
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Semaphore to limit concurrent requests to 8
	sem := make(chan struct{}, 8)

	// Context with timeout for overall operation (tracks * 5 seconds, max 60 seconds)
	timeout := time.Duration(len(tracks)) * 5 * time.Second
	if timeout > 60*time.Second {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for _, track := range tracks {
		wg.Add(1)
		go func(t Track) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			// Try to get stream URL (using format 1 as a representative format)
			streamUrl, err := getStreamMeta(t.TrackID, 0, 1, streamParams)
			if err != nil || streamUrl == "" {
				return
			}

			// Create HEAD request with timeout
			reqCtx, reqCancel := context.WithTimeout(ctx, 5*time.Second)
			defer reqCancel()

			req, err := http.NewRequestWithContext(reqCtx, http.MethodHead, streamUrl, nil)
			if err != nil {
				return
			}

			resp, err := client.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			// Get content length
			if resp.ContentLength > 0 {
				mu.Lock()
				totalSize += resp.ContentLength
				mu.Unlock()
			}
		}(track)
	}

	wg.Wait()

	return totalSize, nil
}

func album(albumID string, cfg *Config, streamParams *StreamParams, artResp *AlbArtResp, batchState *BatchProgressState, progressBox *ProgressBoxState) error {
	var (
		meta   *AlbArtResp
		tracks []Track
	)
	if albumID == "" {
		meta = artResp
		tracks = meta.Songs
	} else {
		_meta, err := getAlbumMeta(albumID)
		if err != nil {
			printError("Failed to get metadata")
			return err
		}
		meta = _meta.Response
		tracks = meta.Tracks
	}

	trackTotal := len(tracks)

	skuID := getVideoSku(meta.Products)

	if skuID == 0 && trackTotal < 1 {
		return errors.New("release has no tracks or videos")
	}

	// Determine what to download based on defaultOutputs config and show media type
	mediaPreference := ParseMediaType(cfg.DefaultOutputs)
	if mediaPreference == MediaTypeUnknown {
		mediaPreference = MediaTypeAudio // default to audio
	}

	showMediaType := getShowMediaType(meta)
	downloadAudio := false
	downloadVideo := false

	if mediaPreference == MediaTypeBoth {
		downloadAudio = showMediaType.HasAudio()
		downloadVideo = showMediaType.HasVideo()
	} else if mediaPreference == MediaTypeVideo {
		downloadVideo = showMediaType.HasVideo()
	} else { // audio (default)
		downloadAudio = showMediaType.HasAudio()
	}

	// Legacy flag overrides
	if cfg.SkipVideos {
		downloadVideo = false
	}
	if cfg.ForceVideo {
		downloadVideo = true
		downloadAudio = false
	}

	// Handle video-only shows
	if skuID != 0 && trackTotal < 1 {
		if cfg.SkipVideos || !downloadVideo {
			printInfo("Video-only album, skipped")
			return nil
		}
		// Video-only album - no track progress to show
		return video(albumID, "", cfg, streamParams, meta, false, nil)
	}
	// Create artist directory
	artistFolder := sanitise(meta.ArtistName)
	artistPath := filepath.Join(cfg.OutPath, artistFolder)
	err := makeDirs(artistPath)
	if err != nil {
		printError("Failed to make artist folder")
		return err
	}

	albumFolder := buildAlbumFolderName(meta.ArtistName, meta.ContainerInfo)
	fmt.Println(albumFolder)
	if len([]rune(meta.ArtistName+" - "+strings.TrimRight(meta.ContainerInfo, " "))) > 120 {
		fmt.Println(
			"Album folder name was chopped because it exceeds 120 characters.")
	}
	albumPath := filepath.Join(artistPath, albumFolder)

	// Check if show already exists locally
	if stat, err := os.Stat(albumPath); err == nil && stat.IsDir() {
		printInfo(fmt.Sprintf("Show already exists locally %s skipping", symbolArrow))
		return nil
	}

	// Check if show already exists on remote
	remoteShowPath := artistFolder + "/" + albumFolder
	printInfo(fmt.Sprintf("Checking remote for: %s%s%s", colorCyan, albumFolder, colorReset))
	exists, err := remotePathExists(remoteShowPath, cfg, false)
	if err != nil {
		printWarning(fmt.Sprintf("Failed to check remote: %v", err))
		// Continue with download even if remote check fails
	} else if exists {
		printSuccess(fmt.Sprintf("Show found on remote %s skipping", symbolArrow))
		return nil
	}

	err = makeDirs(albumPath)
	if err != nil {
		printError("Failed to make album folder")
		return err
	}

	// Pre-calculate total show size (unless disabled)
	totalShowSize := int64(0)
	showTotalStr := "calculating..."
	if !cfg.SkipSizePreCalculation {
		printInfo("Pre-calculating total show size...")
		calculatedSize, err := preCalculateShowSize(tracks, streamParams, cfg)
		if err == nil && calculatedSize > 0 {
			totalShowSize = calculatedSize
			showTotalStr = humanize.Bytes(uint64(totalShowSize))
		}
	}

	// Reuse existing progress box (for batch operations) or create new one (for single downloads)
	showNumStr := meta.PerformanceDateShort
	if batchState != nil && batchState.TotalAlbums > 1 {
		showNumStr = fmt.Sprintf("Show %d/%d: %s", batchState.CurrentAlbum, batchState.TotalAlbums, meta.PerformanceDateShort)
	}

	// Track if we created a new progress box (for cleanup)
	createdNewProgressBox := progressBox == nil

	if progressBox == nil {
		// Single download - create new progress box
		progressBox = &ProgressBoxState{
			ShowTitle:      meta.ContainerInfo,
			ShowNumber:     showNumStr,
			RcloneEnabled:  cfg.RcloneEnabled,
			ShowDownloaded: "0 B",
			ShowTotal:      showTotalStr,
			BatchState:     batchState,
			StartTime:      time.Now(),
			TrackTotal:     trackTotal,
			RenderInterval: defaultProgressRenderInterval,
		}
		// Tier 3: Register global progress box for crawl control access
		setCurrentProgressBox(progressBox)
		defer func() {
			if createdNewProgressBox {
				setCurrentProgressBox(nil)
			}
		}()
	} else {
		// Batch download - reuse existing progress box with clean reset
		// Use ResetForNewAlbum to ensure all fields are properly cleared
		progressBox.ResetForNewAlbum(meta.ContainerInfo, showNumStr, trackTotal, totalShowSize)

		// Set show totals (not part of generic reset since they're album-specific)
		progressBox.mu.Lock()
		progressBox.ShowDownloaded = "0 B"
		progressBox.ShowTotal = showTotalStr
		progressBox.mu.Unlock()
	}

	// Download audio tracks if requested
	if downloadAudio && trackTotal > 0 {
		for trackNum, track := range tracks {
			if err := waitIfPausedOrCancelled(); err != nil {
				return err
			}
			trackNum++
			err := processTrack(
				albumPath, trackNum, trackTotal, cfg, &track, streamParams, progressBox)
			if err != nil {
				if isCrawlCancelledErr(err) {
					return err
				}
				// Track error count
				progressBox.ErrorTracks++
				handleErr("Track failed.", err, false)
			}
		}

		// Mark audio completion (but don't show summary yet if uploading or downloading video)
		if trackTotal > 0 {
			progressBox.IsComplete = true
			progressBox.CompletionTime = time.Now()
			progressBox.TotalDuration = time.Since(progressBox.StartTime)
		}

		// Upload to rclone if enabled
		if cfg.RcloneEnabled {
			err = uploadToRclone(albumPath, artistFolder, cfg, progressBox, false)
			if err != nil {
				handleErr("Upload failed.", err, false)
			}
		}

		// Show completion summary AFTER upload (or immediately if no upload) and no video pending
		if trackTotal > 0 && !downloadVideo {
			renderCompletionSummary(progressBox)
			fmt.Println("") // Final newline after completion summary
		}
	}

	// Download video if requested
	if downloadVideo && skuID != 0 {
		if downloadAudio && trackTotal > 0 {
			fmt.Println("") // Spacing between audio and video
			printInfo("Downloading video...")
		}
		err = video(albumID, "", cfg, streamParams, meta, false, progressBox)
		if err != nil {
			handleErr("Video download failed.", err, false)
		}
	}

	return nil
}

func getAlbumTotal(meta []*ArtistMeta) int {
	var total int
	for _, _meta := range meta {
		total += len(_meta.Response.Containers)
	}
	return total
}
