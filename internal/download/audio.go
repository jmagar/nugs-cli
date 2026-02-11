package download

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
	"github.com/jmagar/nugs-cli/internal/api"
	"github.com/jmagar/nugs-cli/internal/helpers"
	"github.com/jmagar/nugs-cli/internal/model"
	"github.com/jmagar/nugs-cli/internal/ui"
)

var bitrateRegex = regexp.MustCompile(`[\w]+(?:_(\d+)k_v\d+)`)

var trackFallback = map[int]int{
	1: 2,
	2: 5,
	3: 2,
	4: 3,
}

// WriteCounterWrite implements the io.Writer interface for WriteCounter.
// It updates download progress and calls the appropriate progress callback.
func WriteCounterWrite(wc *model.WriteCounter, p []byte, deps *Deps) (int, error) {
	if deps.WaitIfPausedOrCancelled != nil {
		if err := deps.WaitIfPausedOrCancelled(); err != nil {
			return 0, err
		}
	}
	var speed int64
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
	} else if deps.PrintProgress != nil {
		deps.PrintProgress(wc.Percentage, humanize.Bytes(uint64(speed)),
			humanize.Bytes(uint64(wc.Downloaded)), wc.TotalStr)
	}
	return n, nil
}

// DownloadTrack downloads a single audio track file from the given URL.
func DownloadTrack(trackPath, _url string, onProgress func(downloaded, total, speed int64), printNewline bool, deps *Deps) error {
	if deps.WaitIfPausedOrCancelled != nil {
		if err := deps.WaitIfPausedOrCancelled(); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(trackPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	req, err := http.NewRequest(http.MethodGet, _url, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Referer", api.PlayerURL)
	req.Header.Add("User-Agent", api.UserAgent)
	req.Header.Add("Range", "bytes=0-")
	do, err := api.Client.Do(req)
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
	counter := &writeCounterAdapter{
		wc: &model.WriteCounter{
			Total:      totalBytes,
			TotalStr:   totalStr,
			StartTime:  time.Now().UnixMilli(),
			OnProgress: onProgress,
		},
		deps: deps,
	}
	_, err = io.Copy(f, io.TeeReader(do.Body, counter))
	if printNewline {
		fmt.Println("")
	}
	return err
}

// writeCounterAdapter wraps WriteCounter to satisfy io.Writer using Deps.
type writeCounterAdapter struct {
	wc   *model.WriteCounter
	deps *Deps
}

func (a *writeCounterAdapter) Write(p []byte) (int, error) {
	return WriteCounterWrite(a.wc, p, a.deps)
}

// ExtractBitrate extracts the bitrate from a manifest URL.
func ExtractBitrate(manUrl string) string {
	match := bitrateRegex.FindStringSubmatch(manUrl)
	if match != nil {
		return match[1]
	}
	return ""
}

// ParseHlsMaster parses an HLS master playlist and selects the best variant.
func ParseHlsMaster(qual *model.Quality) error {
	req, err := api.Client.Get(qual.URL)
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
	bitrate := ExtractBitrate(variantUri)
	if bitrate == "" {
		return errors.New("no regex match for manifest bitrate")
	}
	qual.Specs = bitrate + " Kbps AAC"
	manBase, q, err := GetManifestBase(qual.URL)
	if err != nil {
		return err
	}
	qual.URL = manBase + variantUri + q
	return nil
}

// GetKey retrieves an AES encryption key from the given URL.
func GetKey(keyUrl string) ([]byte, error) {
	req, err := api.Client.Get(keyUrl)
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

// Pkcs5Trimming removes PKCS5 padding from decrypted data.
func Pkcs5Trimming(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("pkcs5: empty data")
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > aes.BlockSize || padding > len(data) {
		return nil, fmt.Errorf("pkcs5: invalid padding value %d", padding)
	}
	return data[:len(data)-padding], nil
}

// DecryptTrack decrypts a downloaded encrypted TS track file.
func DecryptTrack(tempPath string, key, iv []byte) ([]byte, error) {
	encData, err := os.ReadFile(tempPath)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	ecb := cipher.NewCBCDecrypter(block, iv)
	decrypted := make([]byte, len(encData))
	ui.PrintInfo("Decrypting...")
	ecb.CryptBlocks(decrypted, encData)
	return decrypted, nil
}

// TsToAac converts a decrypted TS stream to AAC using ffmpeg.
func TsToAac(decData []byte, outPath, ffmpegNameStr string) error {
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

// HlsOnly downloads an HLS-only track (encrypted), decrypts it, and converts to AAC.
func HlsOnly(trackPath, manUrl, ffmpegNameStr string, onProgress func(downloaded, total, speed int64), printNewline bool, deps *Deps) error {
	req, err := api.Client.Get(manUrl)
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

	manBase, q, err := GetManifestBase(manUrl)
	if err != nil {
		return err
	}
	tsUrl := manBase + media.Segments[0].URI + q

	key := media.Key
	keyBytes, err := GetKey(manBase + key.URI)
	if err != nil {
		return err
	}

	iv, err := hex.DecodeString(key.IV[2:])
	if err != nil {
		return err
	}

	tempFile, err := os.CreateTemp("", "nugs-enc-*.ts")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	tempFile.Close()
	defer os.Remove(tempPath)

	err = DownloadTrack(tempPath, tsUrl, onProgress, printNewline, deps)
	if err != nil {
		return err
	}
	decData, err := DecryptTrack(tempPath, keyBytes, iv)
	if err != nil {
		return err
	}
	err = TsToAac(decData, trackPath, ffmpegNameStr)
	return err
}

// CheckIfHlsOnly checks if all qualities are HLS-only streams.
func CheckIfHlsOnly(quals []*model.Quality) bool {
	for _, quality := range quals {
		if !strings.Contains(quality.URL, ".m3u8?") {
			return false
		}
	}
	return true
}

// ProcessTrack downloads a single track, handling quality selection and progress updates.
func ProcessTrack(ctx context.Context, folPath string, trackNum, trackTotal int, cfg *model.Config, track *model.Track, streamParams *model.StreamParams, progressBox *model.ProgressBoxState, deps *Deps) error {
	if deps.WaitIfPausedOrCancelled != nil {
		if err := deps.WaitIfPausedOrCancelled(); err != nil {
			return err
		}
	}
	origWantFmt := cfg.Format
	wantFmt := origWantFmt
	var (
		quals      []*model.Quality
		chosenQual *model.Quality
	)
	// Call the stream meta endpoint four times to get all avail formats since the formats can shift.
	for _, i := range [4]int{1, 4, 7, 10} {
		streamUrl, err := api.GetStreamMeta(ctx, track.TrackID, 0, i, streamParams)
		if err != nil {
			ui.PrintError("Failed to get track stream metadata")
			return err
		} else if streamUrl == "" {
			return errors.New("the api didn't return a track stream URL")
		}
		quality := api.QueryQuality(streamUrl)
		if quality == nil {
			ui.PrintError(fmt.Sprintf("The API returned an unsupported format: %s", streamUrl))
			continue
		}
		quals = append(quals, quality)
	}

	if len(quals) == 0 {
		return errors.New("the api didn't return any formats")
	}

	isHlsOnly := CheckIfHlsOnly(quals)

	if isHlsOnly {
		ui.PrintInfo("HLS-only track. Only AAC is available, tags currently unsupported")
		chosenQual = quals[0]
		err := ParseHlsMaster(chosenQual)
		if err != nil {
			return err
		}
	} else {
		for {
			chosenQual = api.GetTrackQual(quals, wantFmt)
			if chosenQual != nil {
				break
			}
			// Fallback quality.
			wantFmt = trackFallback[wantFmt]
		}
		// chosenQual is guaranteed non-nil after loop exit
		if wantFmt != origWantFmt && origWantFmt != 4 {
			ui.PrintInfo("Unavailable in your chosen format")
			// Tier 3: Set quality fallback warning in progress box
			if progressBox != nil {
				fallbackMsg := fmt.Sprintf("Using %s (requested %s unavailable)",
					model.GetQualityName(wantFmt), model.GetQualityName(origWantFmt))
				progressBox.SetMessage(model.MessagePriorityWarning, fallbackMsg, 5*time.Second)
			}
		}
	}
	trackFname := fmt.Sprintf(
		"%02d. %s%s", trackNum, helpers.Sanitise(track.SongTitle), chosenQual.Extension,
	)
	trackPath := filepath.Join(folPath, trackFname)
	exists, err := helpers.FileExists(trackPath)
	if err != nil {
		ui.PrintError("Failed to check if track already exists locally")
		return err
	}
	if exists {
		ui.PrintInfo(fmt.Sprintf("Track exists %s skipping", ui.SymbolArrow))
		if progressBox != nil {
			progressBox.SkippedTracks++
			// Tier 3: Set skip indicator message
			skipMsg := fmt.Sprintf("Skipped track %d - already exists", trackNum)
			progressBox.SetMessage(model.MessagePriorityStatus, skipMsg, 3*time.Second)
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
			if deps.PrintProgress != nil {
				deps.PrintProgress(trackPercentage, humanize.Bytes(uint64(speed)), downloadedLabel, trackTotalStr)
			}
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
		progressBox.Mu.Lock()
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
			if deps.UpdateSpeedHistory != nil {
				progressBox.SpeedHistory = deps.UpdateSpeedHistory(progressBox.SpeedHistory, float64(speed))
			}

			// Calculate remaining bytes for current track
			remaining := total - downloaded
			if deps.CalculateETA != nil {
				progressBox.DownloadETA = deps.CalculateETA(progressBox.SpeedHistory, remaining)
			}
		} else {
			progressBox.DownloadETA = ""
		}
		progressBox.Mu.Unlock()

		// Render the updated progress box (outside lock to avoid holding during I/O)
		if deps.RenderProgressBox != nil {
			deps.RenderProgressBox(progressBox)
		}
	}

	if isHlsOnly {
		err = HlsOnly(trackPath, chosenQual.URL, cfg.FfmpegNameStr, showProgress, false, deps)
	} else {
		err = DownloadTrack(trackPath, chosenQual.URL, showProgress, false, deps)
	}
	if err != nil {
		ui.PrintError("Failed to download track")
		return err
	}

	// Update accumulated bytes after successful download
	if progressBox != nil {
		// Get the actual file size
		var trackSize int64
		if stat, statErr := os.Stat(trackPath); statErr == nil {
			trackSize = stat.Size()
		}
		progressBox.AccumulatedBytes += trackSize
		progressBox.AccumulatedTracks++
	}

	return nil
}

// PreCalculateShowSize calculates the total size of all tracks in a show.
// Uses parallel HEAD requests with 8-concurrent semaphore and 5-second timeout per request.
func PreCalculateShowSize(tracks []model.Track, streamParams *model.StreamParams, cfg *model.Config) (int64, error) {
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
		go func(t model.Track) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			// Try to get stream URL (using format 1 as a representative format)
			streamUrl, err := api.GetStreamMeta(ctx, t.TrackID, 0, 1, streamParams)
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

			resp, err := api.Client.Do(req)
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

// Album downloads an album or show from Nugs.net using the provided albumID.
// If albumID is empty, uses the provided artResp metadata instead of fetching it.
func Album(ctx context.Context, albumID string, cfg *model.Config, streamParams *model.StreamParams, artResp *model.AlbArtResp, batchState *model.BatchProgressState, progressBox *model.ProgressBoxState, deps *Deps) error {
	var (
		meta   *model.AlbArtResp
		tracks []model.Track
	)
	if albumID == "" {
		meta = artResp
		tracks = meta.Songs
	} else {
		_meta, err := api.GetAlbumMeta(ctx, albumID)
		if err != nil {
			ui.PrintError("Failed to get metadata")
			return err
		}
		meta = _meta.Response
		tracks = meta.Tracks
	}

	trackTotal := len(tracks)

	skuID := GetVideoSku(meta.Products)

	if skuID == 0 && trackTotal < 1 {
		return errors.New("release has no tracks or videos")
	}

	// Determine what to download based on defaultOutputs config and show media type
	mediaPreference := model.ParseMediaType(cfg.DefaultOutputs)
	if mediaPreference == model.MediaTypeUnknown {
		mediaPreference = model.MediaTypeAudio // default to audio
	}

	showMediaType := GetShowMediaType(meta)
	downloadAudio := false
	downloadVideo := false

	if mediaPreference == model.MediaTypeBoth {
		downloadAudio = showMediaType.HasAudio()
		downloadVideo = showMediaType.HasVideo()
	} else if mediaPreference == model.MediaTypeVideo {
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
			ui.PrintInfo("Video-only album, skipped")
			return nil
		}
		// Video-only album - no track progress to show
		return Video(ctx, albumID, "", cfg, streamParams, meta, false, nil, deps)
	}
	// Create artist directory
	artistFolder := helpers.Sanitise(meta.ArtistName)
	artistPath := filepath.Join(cfg.OutPath, artistFolder)
	err := helpers.MakeDirs(artistPath)
	if err != nil {
		ui.PrintError("Failed to make artist folder")
		return err
	}

	albumFolder := helpers.BuildAlbumFolderName(meta.ArtistName, meta.ContainerInfo)
	fmt.Println(albumFolder)
	if len([]rune(meta.ArtistName+" - "+strings.TrimRight(meta.ContainerInfo, " "))) > 120 {
		fmt.Println(
			"Album folder name was chopped because it exceeds 120 characters.")
	}
	albumPath := filepath.Join(artistPath, albumFolder)

	// Check if show already exists locally
	if stat, statErr := os.Stat(albumPath); statErr == nil && stat.IsDir() {
		ui.PrintInfo(fmt.Sprintf("Show already exists locally %s skipping", ui.SymbolArrow))
		return nil
	}

	// Check if show already exists on remote
	remoteShowPath := artistFolder + "/" + albumFolder
	ui.PrintInfo(fmt.Sprintf("Checking remote for: %s%s%s", ui.ColorCyan, albumFolder, ui.ColorReset))
	exists, err := deps.RemotePathExists(remoteShowPath, cfg, false)
	if err != nil {
		ui.PrintWarning(fmt.Sprintf("Failed to check remote: %v", err))
		// Continue with download even if remote check fails
	} else if exists {
		ui.PrintSuccess(fmt.Sprintf("Show found on remote %s skipping", ui.SymbolArrow))
		return nil
	}

	err = helpers.MakeDirs(albumPath)
	if err != nil {
		ui.PrintError("Failed to make album folder")
		return err
	}

	// Pre-calculate total show size (unless disabled)
	totalShowSize := int64(0)
	showTotalStr := "calculating..."
	if !cfg.SkipSizePreCalculation {
		ui.PrintInfo("Pre-calculating total show size...")
		calculatedSize, calcErr := PreCalculateShowSize(tracks, streamParams, cfg)
		if calcErr == nil && calculatedSize > 0 {
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
		progressBox = &model.ProgressBoxState{
			ShowTitle:      meta.ContainerInfo,
			ShowNumber:     showNumStr,
			RcloneEnabled:  cfg.RcloneEnabled,
			ShowDownloaded: "0 B",
			ShowTotal:      showTotalStr,
			BatchState:     batchState,
			StartTime:      time.Now(),
			TrackTotal:     trackTotal,
			RenderInterval: model.DefaultProgressRenderInterval,
		}
		// Tier 3: Register global progress box for crawl control access
		if deps.SetCurrentProgressBox != nil {
			deps.SetCurrentProgressBox(progressBox)
		}
		defer func() {
			if createdNewProgressBox && deps.SetCurrentProgressBox != nil {
				deps.SetCurrentProgressBox(nil)
			}
		}()
	} else {
		// Batch download - reuse existing progress box with clean reset
		progressBox.ResetForNewAlbum(meta.ContainerInfo, showNumStr, trackTotal, totalShowSize)

		// Set show totals (not part of generic reset since they're album-specific)
		progressBox.Mu.Lock()
		progressBox.ShowDownloaded = "0 B"
		progressBox.ShowTotal = showTotalStr
		progressBox.Mu.Unlock()
	}

	// Download audio tracks if requested
	if downloadAudio && trackTotal > 0 {
		for trackNum, track := range tracks {
			if deps.WaitIfPausedOrCancelled != nil {
				if err := deps.WaitIfPausedOrCancelled(); err != nil {
					return err
				}
			}
			trackNum++
			err := ProcessTrack(ctx,
				albumPath, trackNum, trackTotal, cfg, &track, streamParams, progressBox, deps)
			if err != nil {
				if deps.IsCrawlCancelledErr != nil && deps.IsCrawlCancelledErr(err) {
					return err
				}
				// Track error count
				progressBox.ErrorTracks++
				helpers.HandleErr("Track failed.", err, false)
			}
		}

		// Mark audio completion (but don't show summary yet if uploading or downloading video)
		if trackTotal > 0 {
			progressBox.IsComplete = true
			progressBox.CompletionTime = time.Now()
			progressBox.TotalDuration = time.Since(progressBox.StartTime)
		}

		// Upload to rclone if enabled
		if cfg.RcloneEnabled && deps.UploadToRclone != nil {
			err = deps.UploadToRclone(albumPath, artistFolder, cfg, progressBox, false)
			if err != nil {
				helpers.HandleErr("Upload failed.", err, false)
			}
		}

		// Show completion summary AFTER upload (or immediately if no upload) and no video pending
		if trackTotal > 0 && !downloadVideo {
			if deps.RenderCompletionSummary != nil {
				deps.RenderCompletionSummary(progressBox)
			}
			fmt.Println("") // Final newline after completion summary
		}
	}

	// Download video if requested
	if downloadVideo && skuID != 0 {
		if downloadAudio && trackTotal > 0 {
			fmt.Println("") // Spacing between audio and video
			ui.PrintInfo("Downloading video...")
		}
		err = Video(ctx, albumID, "", cfg, streamParams, meta, false, progressBox, deps)
		if err != nil {
			helpers.HandleErr("Video download failed.", err, false)
		}
	}

	return nil
}

// GetAlbumTotal counts the total number of containers across all artist metadata pages.
func GetAlbumTotal(meta []*model.ArtistMeta) int {
	var total int
	for _, _meta := range meta {
		total += len(_meta.Response.Containers)
	}
	return total
}
