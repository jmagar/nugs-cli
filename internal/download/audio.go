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
	5: 1, // AAC fallback to ALAC
}

// updateWriteCounterProgress updates download progress on a WriteCounter and returns the computed speed.
func updateWriteCounterProgress(wc *model.WriteCounter, n int) int64 {
	wc.Downloaded += int64(n)
	if wc.Total > 0 {
		percentage := float64(wc.Downloaded) / float64(wc.Total) * float64(model.MaxProgressPercent)
		wc.Percentage = int(percentage)
	}
	var speed int64
	toDivideBy := time.Now().UnixMilli() - wc.StartTime
	if toDivideBy != 0 {
		speed = int64(wc.Downloaded) * model.KBpsDivisor / toDivideBy
	}
	return speed
}

// writeCounterWrite implements the io.Writer interface for WriteCounter.
// It updates download progress and calls the appropriate progress callback.
func writeCounterWrite(wc *model.WriteCounter, p []byte, deps *Deps) (int, error) {
	if deps.WaitIfPausedOrCancelled != nil {
		if err := deps.WaitIfPausedOrCancelled(); err != nil {
			return 0, err
		}
	}
	n := len(p)
	speed := updateWriteCounterProgress(wc, n)
	if wc.OnProgress != nil {
		wc.OnProgress(wc.Downloaded, wc.Total, speed)
	} else if deps.PrintProgress != nil {
		deps.PrintProgress(wc.Percentage, humanize.Bytes(uint64(speed)),
			humanize.Bytes(uint64(wc.Downloaded)), wc.TotalStr)
	}
	return n, nil
}

// DownloadTrack downloads a single audio track file from the given URL.
func DownloadTrack(ctx context.Context, trackPath, _url string, onProgress func(downloaded, total, speed int64), printNewline bool, deps *Deps) error {
	if deps.WaitIfPausedOrCancelled != nil {
		if err := deps.WaitIfPausedOrCancelled(); err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, _url, nil)
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
	f, err := os.OpenFile(trackPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	totalBytes := do.ContentLength
	totalStr := model.UnknownSizeLabelLower
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
	if err != nil {
		os.Remove(trackPath)
		return err
	}
	return nil
}

// writeCounterAdapter wraps WriteCounter to satisfy io.Writer using Deps.
type writeCounterAdapter struct {
	wc   *model.WriteCounter
	deps *Deps
}

func (a *writeCounterAdapter) Write(p []byte) (int, error) {
	return writeCounterWrite(a.wc, p, a.deps)
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
	master, ok := playlist.(*m3u8.MasterPlaylist)
	if !ok {
		return errors.New("expected HLS master playlist but got media playlist")
	}
	if len(master.Variants) == 0 {
		return errors.New("HLS master playlist has no variants")
	}
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
	// Verify data length is multiple of AES block size before decryption
	if len(encData)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("encrypted data length %d is not multiple of AES block size %d", len(encData), aes.BlockSize)
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
func HlsOnly(ctx context.Context, trackPath, manUrl, ffmpegNameStr string, onProgress func(downloaded, total, speed int64), printNewline bool, deps *Deps) error {
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
	media, ok := playlist.(*m3u8.MediaPlaylist)
	if !ok {
		return errors.New("expected HLS media playlist but got master playlist")
	}

	// Validate media playlist has segments and key before accessing
	if len(media.Segments) == 0 {
		return errors.New("HLS media playlist has no segments")
	}
	if media.Key == nil {
		return errors.New("HLS media playlist has no encryption key")
	}

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

	err = DownloadTrack(ctx, tempPath, tsUrl, onProgress, printNewline, deps)
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

// selectTrackQuality probes available formats and selects the best quality for a track.
// Returns the chosen quality, whether the track is HLS-only, and the resolved format ID.
func selectTrackQuality(ctx context.Context, track *model.Track, streamParams *model.StreamParams, wantFmt int) (*model.Quality, bool, int, error) {
	var quals []*model.Quality
	for _, i := range model.TrackStreamMetaFormatProbeOrder {
		streamUrl, err := api.GetStreamMeta(ctx, track.TrackID, 0, i, streamParams)
		if err != nil {
			ui.PrintError("Failed to get track stream metadata")
			return nil, false, wantFmt, err
		} else if streamUrl == "" {
			return nil, false, wantFmt, errors.New("the api didn't return a track stream URL")
		}
		quality := api.QueryQuality(streamUrl)
		if quality == nil {
			ui.PrintError(fmt.Sprintf("The API returned an unsupported format: %s", streamUrl))
			continue
		}
		quals = append(quals, quality)
	}
	if len(quals) == 0 {
		return nil, false, wantFmt, errors.New("the api didn't return any formats")
	}

	isHlsOnly := CheckIfHlsOnly(quals)
	if isHlsOnly {
		ui.PrintInfo("HLS-only track. Only AAC is available, tags currently unsupported")
		chosenQual := quals[0]
		if err := ParseHlsMaster(chosenQual); err != nil {
			return nil, true, wantFmt, err
		}
		return chosenQual, true, wantFmt, nil
	}

	// Non-HLS: try requested format with fallback chain
	for i := 0; i < model.MaxFormatFallbackAttempts; i++ {
		if chosen := api.GetTrackQual(quals, wantFmt); chosen != nil {
			return chosen, false, wantFmt, nil
		}
		nextFmt := trackFallback[wantFmt]
		if nextFmt == 0 || nextFmt == wantFmt {
			break
		}
		wantFmt = nextFmt
	}
	return nil, false, wantFmt, nil
}

// buildTrackProgressCallback creates a progress callback for track downloads.
// It updates both the progress box (if provided) and falls back to simple progress display.
func buildTrackProgressCallback(progressBox *model.ProgressBoxState, deps *Deps, trackNum, trackTotal int, accumulatedBeforeCurrent int64) func(downloaded, total, speed int64) {
	return func(downloaded, total, speed int64) {
		if progressBox == nil {
			trackPercentage := 0
			trackTotalStr := model.UnknownSizeLabelLower
			if total > 0 {
				trackPercentage = int((float64(downloaded) / float64(total)) * model.MaxProgressPercent)
				if trackPercentage > model.MaxProgressPercent {
					trackPercentage = model.MaxProgressPercent
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

		trackPercentage := 0
		trackTotalStr := model.UnknownSizeLabelLower
		if total > 0 {
			trackPercentage = int((float64(downloaded) / float64(total)) * model.MaxProgressPercent)
			if trackPercentage > model.MaxProgressPercent {
				trackPercentage = model.MaxProgressPercent
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
		showPercentage := int(((float64(trackNum-1) + trackProgress) / float64(trackTotal)) * model.MaxProgressPercent)
		if showPercentage > model.MaxProgressPercent {
			showPercentage = model.MaxProgressPercent
		}

		progressBox.Mu.Lock()
		progressBox.DownloadPercent = trackPercentage
		progressBox.DownloadSpeed = humanize.Bytes(uint64(speed))
		progressBox.Downloaded = humanize.Bytes(uint64(downloaded))
		progressBox.DownloadTotal = trackTotalStr
		progressBox.ShowPercent = showPercentage
		progressBox.ShowDownloaded = humanize.Bytes(uint64(accumulatedBeforeCurrent + downloaded))

		// Update ShowTotal dynamically based on actual downloaded size + current track size
		// This fixes incorrect pre-calculation that may have returned 0 or wrong values
		if total > 0 {
			// Calculate estimated total: completed bytes + current track + estimate for remaining tracks
			estimatedRemaining := total * int64(trackTotal-trackNum)
			estimatedTotal := accumulatedBeforeCurrent + downloaded + estimatedRemaining
			progressBox.ShowTotal = humanize.Bytes(uint64(estimatedTotal))
		}

		if speed > 0 && total > 0 && downloaded < total {
			if deps.UpdateSpeedHistory != nil {
				progressBox.SpeedHistory = deps.UpdateSpeedHistory(progressBox.SpeedHistory, float64(speed))
			}
			remaining := total - downloaded
			if deps.CalculateETA != nil {
				progressBox.DownloadETA = deps.CalculateETA(progressBox.SpeedHistory, remaining)
			}
		} else {
			progressBox.DownloadETA = ""
		}
		progressBox.Mu.Unlock()

		if deps.RenderProgressBox != nil {
			deps.RenderProgressBox(progressBox)
		}
	}
}

// ProcessTrack downloads a single track, handling quality selection and progress updates.
func ProcessTrack(ctx context.Context, folPath string, trackNum, trackTotal int, cfg *model.Config, track *model.Track, streamParams *model.StreamParams, progressBox *model.ProgressBoxState, deps *Deps) error {
	if deps.WaitIfPausedOrCancelled != nil {
		if err := deps.WaitIfPausedOrCancelled(); err != nil {
			return err
		}
	}
	origWantFmt := cfg.Format
	chosenQual, isHlsOnly, resolvedFmt, err := selectTrackQuality(ctx, track, streamParams, origWantFmt)
	if err != nil {
		return err
	}
	if !isHlsOnly && resolvedFmt != origWantFmt && origWantFmt != 4 {
		ui.PrintInfo("Unavailable in your chosen format")
		if progressBox != nil {
			fallbackMsg := fmt.Sprintf("Using %s (requested %s unavailable)",
				model.GetQualityName(resolvedFmt), model.GetQualityName(origWantFmt))
			progressBox.SetMessage(model.MessagePriorityWarning, fallbackMsg, model.StatusMessageDuration)
		}
	}
	if chosenQual == nil {
		return fmt.Errorf("no supported format found for track %d", trackNum)
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
			progressBox.Mu.Lock()
			progressBox.SkippedTracks++
			progressBox.Mu.Unlock()
			skipMsg := fmt.Sprintf("Skipped track %d - already exists", trackNum)
			progressBox.SetMessage(model.MessagePriorityStatus, skipMsg, model.SkipMessageDuration)
		}
		return nil
	}

	var accumulatedBeforeCurrent int64
	if progressBox != nil {
		progressBox.Mu.Lock()
		progressBox.TrackNumber = trackNum
		progressBox.TrackTotal = trackTotal
		progressBox.TrackName = track.SongTitle
		progressBox.TrackFormat = chosenQual.Specs
		progressBox.RcloneEnabled = cfg.RcloneEnabled
		accumulatedBeforeCurrent = progressBox.AccumulatedBytes
		progressBox.Mu.Unlock()
	}
	showProgress := buildTrackProgressCallback(progressBox, deps, trackNum, trackTotal, accumulatedBeforeCurrent)

	if isHlsOnly {
		err = HlsOnly(ctx, trackPath, chosenQual.URL, cfg.FfmpegNameStr, showProgress, false, deps)
	} else {
		err = DownloadTrack(ctx, trackPath, chosenQual.URL, showProgress, false, deps)
	}
	if err != nil {
		ui.PrintError("Failed to download track")
		return err
	}

	if progressBox != nil {
		var trackSize int64
		if stat, statErr := os.Stat(trackPath); statErr == nil {
			trackSize = stat.Size()
		}
		progressBox.Mu.Lock()
		progressBox.AccumulatedBytes += trackSize
		progressBox.AccumulatedTracks++
		progressBox.Mu.Unlock()
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
	sem := make(chan struct{}, model.PreCalcConcurrency)

	// Use background context for orchestration; per-request timeouts handle individual HEADs
	ctx := context.Background()

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
			reqCtx, reqCancel := context.WithTimeout(ctx, model.PreCalcPerRequestTimeout)
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

func resolveAlbumMetaAndTracks(ctx context.Context, albumID string, artResp *model.AlbArtResp) (*model.AlbArtResp, []model.Track, error) {
	if albumID == "" {
		return artResp, artResp.Songs, nil
	}
	meta, err := api.GetAlbumMeta(ctx, albumID)
	if err != nil {
		ui.PrintError("Failed to get metadata")
		return nil, nil, err
	}
	return meta.Response, meta.Response.Tracks, nil
}

func resolveAlbumDownloadModes(cfg *model.Config, meta *model.AlbArtResp) (bool, bool) {
	mediaPreference := model.ParseMediaType(cfg.DefaultOutputs)
	if mediaPreference == model.MediaTypeUnknown {
		mediaPreference = model.MediaTypeAudio
	}
	showMediaType := GetShowMediaType(meta)
	downloadAudio := mediaPreference != model.MediaTypeVideo && showMediaType.HasAudio()
	downloadVideo := mediaPreference != model.MediaTypeAudio && showMediaType.HasVideo()
	if mediaPreference == model.MediaTypeVideo {
		downloadAudio = false
	}
	if cfg.SkipVideos {
		downloadVideo = false
	}
	if cfg.ForceVideo {
		return false, true
	}
	return downloadAudio, downloadVideo
}

func handleVideoOnlyAlbum(ctx context.Context, albumID string, cfg *model.Config, streamParams *model.StreamParams, meta *model.AlbArtResp, trackTotal, skuID int, downloadVideo bool, deps *Deps) (bool, error) {
	if skuID == 0 || trackTotal > 0 {
		return false, nil
	}
	if cfg.SkipVideos || !downloadVideo {
		ui.PrintInfo("Video-only album, skipped")
		return true, nil
	}
	return true, Video(ctx, albumID, "", cfg, streamParams, meta, false, nil, deps)
}

func buildAlbumFolderName(meta *model.AlbArtResp) string {
	albumFolder := helpers.BuildAlbumFolderName(meta.ArtistName, meta.ContainerInfo)
	fmt.Println(albumFolder)
	fullName := meta.ArtistName + " - " + strings.TrimRight(meta.ContainerInfo, " ")
	if len([]rune(fullName)) > model.AlbumFolderMaxRunes {
		fmt.Printf("Album folder name was chopped because it exceeds %d characters.\n", model.AlbumFolderMaxRunes)
	}
	return albumFolder
}

func prepareAlbumPaths(ctx context.Context, cfg *model.Config, meta *model.AlbArtResp, deps *Deps) (string, string, bool, error) {
	artistFolder := helpers.Sanitise(meta.ArtistName)
	artistPath := filepath.Join(cfg.OutPath, artistFolder)
	if err := helpers.MakeDirs(artistPath); err != nil {
		ui.PrintError("Failed to make artist folder")
		return "", "", false, err
	}
	albumFolder := buildAlbumFolderName(meta)
	albumPath := filepath.Join(artistPath, albumFolder)
	if stat, statErr := os.Stat(albumPath); statErr == nil && stat.IsDir() {
		ui.PrintInfo(fmt.Sprintf("Show already exists locally %s skipping", ui.SymbolArrow))
		return "", "", true, nil
	}
	remoteShowPath := artistFolder + "/" + albumFolder
	ui.PrintInfo(fmt.Sprintf("Checking remote for: %s%s%s", ui.ColorCyan, albumFolder, ui.ColorReset))
	exists, err := deps.CheckRemotePathExists(ctx, remoteShowPath, cfg, false)
	if err != nil {
		ui.PrintWarning(fmt.Sprintf("Failed to check remote: %v", err))
	} else if exists {
		ui.PrintSuccess(fmt.Sprintf("Show found on remote %s skipping", ui.SymbolArrow))
		return "", "", true, nil
	}
	if err := helpers.MakeDirs(albumPath); err != nil {
		ui.PrintError("Failed to make album folder")
		return "", "", false, err
	}
	return artistFolder, albumPath, false, nil
}

func calculateAlbumShowSize(tracks []model.Track, streamParams *model.StreamParams, cfg *model.Config) (int64, string) {
	totalShowSize := int64(0)
	showTotalStr := model.CalculatingSizeLabel
	if cfg.SkipSizePreCalculation {
		return totalShowSize, showTotalStr
	}
	ui.PrintInfo("Pre-calculating total show size...")
	calculatedSize, err := PreCalculateShowSize(tracks, streamParams, cfg)
	if err == nil && calculatedSize > 0 {
		totalShowSize = calculatedSize
		showTotalStr = humanize.Bytes(uint64(totalShowSize))
	}
	return totalShowSize, showTotalStr
}

func buildAlbumShowNumber(meta *model.AlbArtResp, batchState *model.BatchProgressState) string {
	if batchState != nil && batchState.TotalAlbums > 1 {
		return fmt.Sprintf(model.BatchShowNumberFormat, batchState.CurrentAlbum, batchState.TotalAlbums, meta.PerformanceDateShort)
	}
	return meta.PerformanceDateShort
}

func prepareAlbumProgressBox(meta *model.AlbArtResp, cfg *model.Config, batchState *model.BatchProgressState, progressBox *model.ProgressBoxState, trackTotal int, totalShowSize int64, showTotalStr string, deps *Deps) (*model.ProgressBoxState, bool) {
	showNumStr := buildAlbumShowNumber(meta, batchState)
	created := progressBox == nil
	if progressBox == nil {
		progressBox = &model.ProgressBoxState{
			ShowTitle:      meta.ContainerInfo,
			ShowNumber:     showNumStr,
			RcloneEnabled:  cfg.RcloneEnabled,
			ShowDownloaded: model.ZeroBytesLabel,
			ShowTotal:      showTotalStr,
			BatchState:     batchState,
			StartTime:      time.Now(),
			TrackTotal:     trackTotal,
			RenderInterval: model.DefaultProgressRenderInterval,
		}
		if deps.SetCurrentProgressBox != nil {
			deps.SetCurrentProgressBox(progressBox)
		}
		return progressBox, created
	}
	progressBox.ResetForNewAlbum(meta.ContainerInfo, showNumStr, trackTotal, totalShowSize)
	progressBox.Mu.Lock()
	progressBox.ShowDownloaded = model.ZeroBytesLabel
	progressBox.ShowTotal = showTotalStr
	progressBox.Mu.Unlock()
	return progressBox, created
}

func downloadAlbumAudio(ctx context.Context, tracks []model.Track, albumPath, artistFolder string, cfg *model.Config, streamParams *model.StreamParams, progressBox *model.ProgressBoxState, downloadVideo bool, deps *Deps) error {
	trackTotal := len(tracks)
	for trackNum, track := range tracks {
		if deps.WaitIfPausedOrCancelled != nil {
			if err := deps.WaitIfPausedOrCancelled(); err != nil {
				return err
			}
		}
		trackNum++
		err := ProcessTrack(ctx, albumPath, trackNum, trackTotal, cfg, &track, streamParams, progressBox, deps)
		if err != nil {
			if deps.IsCrawlCancelledErr != nil && deps.IsCrawlCancelledErr(err) {
				return err
			}
			if progressBox != nil {
				progressBox.Mu.Lock()
				progressBox.ErrorTracks++
				progressBox.Mu.Unlock()
			}
			helpers.HandleErr("Track failed.", err, false)
		}
	}
	if progressBox != nil {
		progressBox.IsComplete = true
		progressBox.CompletionTime = time.Now()
		progressBox.TotalDuration = time.Since(progressBox.StartTime)
	}
	if cfg.RcloneEnabled {
		if err := deps.UploadPath(ctx, albumPath, artistFolder, cfg, progressBox, false); err != nil {
			helpers.HandleErr("Upload failed.", err, false)
		}
	}
	if !downloadVideo && deps.RenderCompletionSummary != nil {
		deps.RenderCompletionSummary(progressBox)
		fmt.Println("")
	}
	return nil
}

func downloadAlbumVideo(ctx context.Context, albumID string, cfg *model.Config, streamParams *model.StreamParams, meta *model.AlbArtResp, progressBox *model.ProgressBoxState, hadAudio bool, deps *Deps) {
	if hadAudio {
		fmt.Println("")
		ui.PrintInfo("Downloading video...")
	}
	err := Video(ctx, albumID, "", cfg, streamParams, meta, false, progressBox, deps)
	if err != nil {
		helpers.HandleErr("Video download failed.", err, false)
	}
}

// Album downloads an album or show from Nugs.net using the provided albumID.
// If albumID is empty, uses the provided artResp metadata instead of fetching it.
func Album(ctx context.Context, albumID string, cfg *model.Config, streamParams *model.StreamParams, artResp *model.AlbArtResp, batchState *model.BatchProgressState, progressBox *model.ProgressBoxState, deps *Deps) error {
	meta, tracks, err := resolveAlbumMetaAndTracks(ctx, albumID, artResp)
	if err != nil {
		return err
	}
	trackTotal := len(tracks)
	skuID := GetVideoSku(meta.Products)
	if skuID == 0 && trackTotal < 1 {
		return model.ErrReleaseHasNoContent
	}
	downloadAudio, downloadVideo := resolveAlbumDownloadModes(cfg, meta)
	handled, err := handleVideoOnlyAlbum(ctx, albumID, cfg, streamParams, meta, trackTotal, skuID, downloadVideo, deps)
	if handled {
		return err
	}

	artistFolder, albumPath, skipped, err := prepareAlbumPaths(ctx, cfg, meta, deps)
	if err != nil || skipped {
		return err
	}

	totalShowSize, showTotalStr := calculateAlbumShowSize(tracks, streamParams, cfg)
	progressBox, created := prepareAlbumProgressBox(meta, cfg, batchState, progressBox, trackTotal, totalShowSize, showTotalStr, deps)
	if created {
		defer func() {
			if deps.SetCurrentProgressBox != nil {
				deps.SetCurrentProgressBox(nil)
			}
		}()
	}
	if downloadAudio && trackTotal > 0 {
		if err := downloadAlbumAudio(ctx, tracks, albumPath, artistFolder, cfg, streamParams, progressBox, downloadVideo, deps); err != nil {
			return err
		}
	}
	if downloadVideo && skuID != 0 {
		downloadAlbumVideo(ctx, albumID, cfg, streamParams, meta, progressBox, downloadAudio && trackTotal > 0, deps)
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
