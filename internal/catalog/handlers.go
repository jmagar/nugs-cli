package catalog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jmagar/nugs-cli/internal/api"
	"github.com/jmagar/nugs-cli/internal/cache"
	"github.com/jmagar/nugs-cli/internal/helpers"
	"github.com/jmagar/nugs-cli/internal/model"
	"github.com/jmagar/nugs-cli/internal/ui"
)

// ArtistMetaCacheTTL is the TTL used for artist metadata caching.
const ArtistMetaCacheTTL = 24 * time.Hour

// gapFillErrorContext captures diagnostic information when a gap-fill download
// fails. It helps distinguish between several failure modes:
//   - Preorder/placeholder shows (availabilityType != "available", zero tracks/products)
//   - Naming/path mismatches (content exists locally or remotely but wasn't detected)
//   - Network failures (remote check returned an error)
//   - Missing content (metadata has no downloadable tracks or videos)
//
// Fields are populated progressively by buildGapFillErrorContext: metadata fields
// are always set, local checks run next, and remote fields are only populated
// when RcloneEnabled is true and the remote check completes.
type gapFillErrorContext struct {
	// Metadata from the show's API response, used to detect preorder/placeholder shows.
	AvailabilityType string `json:"availabilityType,omitempty"` // e.g. "available", "preorder"
	ActiveState      string `json:"activeState,omitempty"`      // show lifecycle state from API
	Tracks           int    `json:"tracks"`                     // number of audio tracks in metadata
	Products         int    `json:"products"`                   // number of product SKUs (purchase options)
	ProductFormats   int    `json:"productFormats"`             // number of format variants (FLAC, ALAC, etc.)

	// Local filesystem existence checks against expected download paths.
	LocalAudioPath   string `json:"localAudioPath"`   // expected path: outPath/artist/album
	LocalAudioExists bool   `json:"localAudioExists"` // true if directory found on disk
	LocalVideoPath   string `json:"localVideoPath"`   // expected path: videoOutPath/artist/album
	LocalVideoExists bool   `json:"localVideoExists"` // true if directory found on disk

	// Remote storage checks, only populated when cfg.RcloneEnabled is true.
	RemoteRelativePath string `json:"remoteRelativePath,omitempty"` // artist/album relative path
	RemoteAudioPath    string `json:"remoteAudioPath,omitempty"`    // full rclone URI for audio (remote:base/artist/album)
	RemoteAudioExists  bool   `json:"remoteAudioExists"`            // true if found on remote
	RemoteAudioError   string `json:"remoteAudioError,omitempty"`   // error message if audio check failed
	RemoteVideoPath    string `json:"remoteVideoPath,omitempty"`    // full rclone URI for video
	RemoteVideoExists  bool   `json:"remoteVideoExists"`            // true if found on remote
	RemoteVideoError   string `json:"remoteVideoError,omitempty"`   // error message if video check failed
}

func deriveGapFillReasonHint(err error, info gapFillErrorContext) string {
	if info.AvailabilityType != "" &&
		!strings.EqualFold(info.AvailabilityType, model.AvailableAvailabilityType) &&
		info.Tracks == 0 && info.Products == 0 && info.ProductFormats == 0 {
		return "Preorder/placeholder container (not released yet)"
	}
	if info.LocalAudioExists || info.LocalVideoExists {
		return "Already exists locally (naming/path mismatch likely)"
	}
	if info.RemoteAudioExists || info.RemoteVideoExists {
		return "Already exists on remote (naming/path mismatch likely)"
	}
	if info.RemoteAudioError != "" || info.RemoteVideoError != "" {
		return "Remote existence check failed"
	}
	if errors.Is(err, model.ErrReleaseHasNoContent) {
		return "Metadata has no downloadable tracks/videos yet"
	}
	return "No content found at expected local/remote paths"
}

func joinRemotePath(remote, base, relative string) string {
	trimmedBase := strings.TrimRight(base, "/")
	trimmedRelative := strings.TrimLeft(relative, "/")
	if trimmedBase == "" {
		return fmt.Sprintf("%s:%s", remote, trimmedRelative)
	}
	return fmt.Sprintf("%s:%s/%s", remote, trimmedBase, trimmedRelative)
}

func buildGapFillErrorContext(ctx context.Context, show *model.AlbArtResp, cfg *model.Config, deps *Deps) gapFillErrorContext {
	resolver := helpers.NewConfigPathResolver(cfg)
	localAudioPath := resolver.LocalShowPath(show, model.MediaTypeAudio)
	localVideoPath := resolver.LocalShowPath(show, model.MediaTypeVideo)

	info := gapFillErrorContext{
		AvailabilityType: show.AvailabilityTypeStr,
		ActiveState:      show.ActiveState,
		Tracks:           len(show.Tracks),
		Products:         len(show.Products),
		ProductFormats:   len(show.ProductFormatList),
		LocalAudioPath:   localAudioPath,
		LocalVideoPath:   localVideoPath,
	}

	if _, err := os.Stat(localAudioPath); err == nil {
		info.LocalAudioExists = true
	}
	if _, err := os.Stat(localVideoPath); err == nil {
		info.LocalVideoExists = true
	}

	if !cfg.RcloneEnabled || deps.RemotePathExists == nil {
		return info
	}

	remoteRelativePath := resolver.RemoteShowPath(show)
	info.RemoteRelativePath = remoteRelativePath
	info.RemoteAudioPath = joinRemotePath(cfg.RcloneRemote, helpers.GetRcloneBasePath(cfg, false), remoteRelativePath)
	info.RemoteVideoPath = joinRemotePath(cfg.RcloneRemote, helpers.GetRcloneBasePath(cfg, true), remoteRelativePath)

	audioExists, audioErr := deps.RemotePathExists(ctx, path.Clean(remoteRelativePath), cfg, false)
	if audioErr != nil {
		info.RemoteAudioError = audioErr.Error()
	} else {
		info.RemoteAudioExists = audioExists
	}

	videoExists, videoErr := deps.RemotePathExists(ctx, path.Clean(remoteRelativePath), cfg, true)
	if videoErr != nil {
		info.RemoteVideoError = videoErr.Error()
	} else {
		info.RemoteVideoExists = videoExists
	}

	return info
}

// AnalyzeArtistCatalog analyzes an artist's catalog with optional media type filtering.
func AnalyzeArtistCatalog(ctx context.Context, artistID string, cfg *model.Config, jsonLevel string, mediaFilter model.MediaType, deps *Deps) (*model.ArtistCatalogAnalysis, error) {
	return AnalyzeArtistCatalogMediaAware(ctx, artistID, cfg, jsonLevel, mediaFilter, deps)
}

// CatalogUpdate fetches and caches the latest catalog.
func CatalogUpdate(ctx context.Context, jsonLevel string, deps *Deps) error {
	startTime := time.Now()
	catalog, err := api.GetLatestCatalog(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch catalog: %w", err)
	}

	updateDuration := time.Since(startTime)
	err = cache.WriteCatalogCache(catalog, updateDuration, deps.FormatDuration)
	if err != nil {
		return err
	}

	cacheDir, _ := cache.GetCacheDir()

	if jsonLevel != "" {
		output := map[string]any{
			"success":    true,
			"totalShows": len(catalog.Response.RecentItems),
			"updateTime": deps.FormatDuration(updateDuration),
			"cacheDir":   cacheDir,
		}
		if err := PrintJSON(output); err != nil {
			return err
		}
	} else {
		fmt.Printf("\u2713 Catalog updated successfully\n")
		fmt.Printf("  Total shows: %s%d%s\n", ui.ColorGreen, len(catalog.Response.RecentItems), ui.ColorReset)
		fmt.Printf("  Update time: %s%s%s\n", ui.ColorCyan, deps.FormatDuration(updateDuration), ui.ColorReset)
		fmt.Printf("  Cache location: %s\n", cacheDir)
	}
	return nil
}

// CatalogCacheStatus shows cache status and metadata.
func CatalogCacheStatus(jsonLevel string, deps *Deps) error {
	meta, err := cache.ReadCacheMeta()
	if err != nil {
		return err
	}

	if meta == nil {
		if jsonLevel != "" {
			output := map[string]any{"exists": false}
			if err := PrintJSON(output); err != nil {
				return err
			}
		} else {
			fmt.Println("No cache found - run 'nugs catalog update' first")
		}
		return nil
	}

	cacheDir, _ := cache.GetCacheDir()
	catalogPath := filepath.Join(cacheDir, "catalog.json")
	fileInfo, err := os.Stat(catalogPath)
	if err != nil {
		return fmt.Errorf("failed to stat cache file: %w", err)
	}

	age := time.Since(meta.LastUpdated)
	ageHuman := deps.FormatDuration(age)
	fileSizeBytes := fileInfo.Size()
	fileSizeHuman := fmt.Sprintf("%.1f MB", float64(fileSizeBytes)/(1024*1024))

	if jsonLevel != "" {
		output := map[string]any{
			"exists":        true,
			"lastUpdated":   meta.LastUpdated.Format(time.RFC3339),
			"ageSeconds":    int(age.Seconds()),
			"ageHuman":      ageHuman,
			"totalShows":    meta.TotalShows,
			"totalArtists":  meta.TotalArtists,
			"cacheVersion":  meta.CacheVersion,
			"fileSizeBytes": fileSizeBytes,
			"fileSizeHuman": fileSizeHuman,
			"cacheDir":      cacheDir,
		}
		if err := PrintJSON(output); err != nil {
			return err
		}
	} else {
		ui.PrintHeader("Catalog Cache Status")

		ui.PrintKeyValue("Location", cacheDir, ui.ColorCyan)
		ui.PrintKeyValue("Last Updated", fmt.Sprintf("%s (%s ago)", meta.LastUpdated.Format("2006-01-02 15:04:05"), ageHuman), "")
		ui.PrintKeyValue("Total Shows", fmt.Sprintf("%d", meta.TotalShows), ui.ColorGreen)
		ui.PrintKeyValue("Artists", fmt.Sprintf("%d unique", meta.TotalArtists), ui.ColorCyan)
		ui.PrintKeyValue("Cache Size", fileSizeHuman, "")
		ui.PrintKeyValue("Version", meta.CacheVersion, "")

		if age.Hours() > 24 {
			fmt.Println()
			ui.PrintWarning(fmt.Sprintf("Cache is over 24 hours old - consider running '%snugs catalog update%s'", ui.ColorBold, ui.ColorReset))
		}
		fmt.Println()
	}
	return nil
}

// CatalogStats shows catalog statistics.
func CatalogStats(_ context.Context, jsonLevel string) error {
	catalog, err := cache.ReadCatalogCache()
	if err != nil {
		return err
	}

	// Build statistics
	showsPerArtist := make(map[int]int)
	artistNames := make(map[int]string)
	var earliestTime, latestTime time.Time

	for _, item := range catalog.Response.RecentItems {
		showsPerArtist[item.ArtistID]++
		artistNames[item.ArtistID] = item.ArtistName

		if item.PerformanceDateStr != "" {
			t, err := time.Parse("Jan 02, 2006", item.PerformanceDateStr)
			if err == nil {
				if earliestTime.IsZero() || t.Before(earliestTime) {
					earliestTime = t
				}
				if latestTime.IsZero() || t.After(latestTime) {
					latestTime = t
				}
			}
		}
	}

	var earliestDate, latestDate string
	if !earliestTime.IsZero() {
		earliestDate = earliestTime.Format("Jan 02, 2006")
	}
	if !latestTime.IsZero() {
		latestDate = latestTime.Format("Jan 02, 2006")
	}

	type artistCount struct {
		id    int
		name  string
		count int
	}
	var sortedArtists []artistCount
	for id, count := range showsPerArtist {
		sortedArtists = append(sortedArtists, artistCount{id, artistNames[id], count})
	}
	sort.Slice(sortedArtists, func(i, j int) bool {
		return sortedArtists[i].count > sortedArtists[j].count
	})

	top10 := sortedArtists
	if len(top10) > 10 {
		top10 = sortedArtists[:10]
	}

	if jsonLevel != "" {
		topArtists := make([]map[string]any, len(top10))
		for i, a := range top10 {
			topArtists[i] = map[string]any{
				"artistID":   a.id,
				"artistName": a.name,
				"showCount":  a.count,
			}
		}
		output := map[string]any{
			"totalShows":   len(catalog.Response.RecentItems),
			"totalArtists": len(showsPerArtist),
			"dateRange": map[string]string{
				"earliest": earliestDate,
				"latest":   latestDate,
			},
			"topArtists": topArtists,
		}
		if err := PrintJSON(output); err != nil {
			return err
		}
	} else {
		ui.PrintHeader("Catalog Statistics")

		ui.PrintKeyValue("Total Shows", fmt.Sprintf("%d", len(catalog.Response.RecentItems)), ui.ColorGreen)
		ui.PrintKeyValue("Total Artists", fmt.Sprintf("%d unique", len(showsPerArtist)), ui.ColorCyan)
		ui.PrintKeyValue("Date Range", fmt.Sprintf("%s to %s", earliestDate, latestDate), "")

		ui.PrintSection("Top 10 Artists by Show Count")

		table := ui.NewTable([]ui.TableColumn{
			{Header: "ID", Width: 10, Align: "right"},
			{Header: "Artist", Width: 50, Align: "left"},
			{Header: "Shows", Width: 10, Align: "right"},
		})

		for _, a := range top10 {
			table.AddRow(
				fmt.Sprintf("%d", a.id),
				a.name,
				fmt.Sprintf("%s%d%s", ui.ColorGreen, a.count, ui.ColorReset),
			)
		}

		table.Print()
		fmt.Println()
	}
	return nil
}

// CatalogLatest shows latest additions to catalog.
func CatalogLatest(_ context.Context, limit int, jsonLevel string) error {
	catalog, err := cache.ReadCatalogCache()
	if err != nil {
		return err
	}

	items := catalog.Response.RecentItems
	if len(items) > limit {
		items = items[:limit]
	}

	if jsonLevel != "" {
		shows := make([]map[string]any, len(items))
		for i, item := range items {
			shows[i] = map[string]any{
				"containerID": item.ContainerID,
				"artistID":    item.ArtistID,
				"artistName":  item.ArtistName,
				"date":        item.ShowDateFormattedShort,
				"title":       item.ContainerInfo,
				"venue":       item.Venue,
				"venueCity":   item.VenueCity,
				"venueState":  item.VenueState,
			}
		}
		output := map[string]any{
			"shows": shows,
			"total": len(items),
			"limit": limit,
		}
		if err := PrintJSON(output); err != nil {
			return err
		}
	} else {
		fmt.Printf("\nLatest %d Shows in Catalog:\n\n", len(items))
		for i, item := range items {
			artistName := item.ArtistName
			if len(artistName) > 24 {
				artistName = artistName[:21] + "..."
			}
			title := item.ContainerInfo
			if len(title) > 40 {
				title = title[:37] + "..."
			}
			location := item.Venue
			if item.VenueCity != "" {
				location = item.VenueCity + ", " + item.VenueState
			}
			if len(location) > 28 {
				location = location[:25] + "..."
			}
			fmt.Printf("  %2d. %s%-6d%s %-26s %-12s %-42s %-30s\n",
				i+1,
				ui.ColorPurple,
				item.ArtistID,
				ui.ColorReset,
				artistName,
				item.ShowDateFormattedShort,
				title,
				location)
		}
	}
	return nil
}

// CatalogGapsForArtist finds missing shows for a single artist.
func CatalogGapsForArtist(ctx context.Context, artistId string, cfg *model.Config, jsonLevel string, idsOnly bool, mediaFilter model.MediaType, deps *Deps) error {
	analysis, err := AnalyzeArtistCatalog(ctx, artistId, cfg, jsonLevel, mediaFilter, deps)
	if err != nil {
		return err
	}

	if idsOnly {
		for _, status := range analysis.MissingShows {
			fmt.Println(status.Show.ContainerID)
		}
	} else if jsonLevel != "" {
		missingData := make([]map[string]any, len(analysis.MissingShows))
		for i, status := range analysis.MissingShows {
			show := status.Show
			missingData[i] = map[string]any{
				"containerID": show.ContainerID,
				"date":        show.PerformanceDateShortYearFirst,
				"title":       show.ContainerInfo,
				"venue":       show.Venue,
			}
		}
		output := map[string]any{
			"artistID":      artistId,
			"artistName":    analysis.ArtistName,
			"totalShows":    analysis.TotalShows,
			"downloaded":    analysis.Downloaded,
			"downloadedPct": analysis.DownloadPct,
			"missing":       analysis.Missing,
			"missingPct":    analysis.MissingPct,
			"missingShows":  missingData,
			"cacheUsed":     analysis.CacheUsed,
			"cacheStaleUse": analysis.CacheStaleUse,
		}
		if err := PrintJSON(output); err != nil {
			return err
		}
	} else {
		if len(analysis.MissingShows) == 0 {
			fmt.Println("No missing shows")
			return nil
		}

		table := ui.NewTable([]ui.TableColumn{
			{Header: "Type", Width: 6, Align: "center"},
			{Header: "ID", Width: 10, Align: "right"},
			{Header: "Date", Width: 14, Align: "left"},
			{Header: "Title", Width: 55, Align: "left"},
		})

		for _, status := range analysis.MissingShows {
			show := status.Show
			mediaIndicator := ui.GetMediaTypeIndicator(status.MediaType)
			table.AddRow(
				mediaIndicator,
				fmt.Sprintf("%d", show.ContainerID),
				show.PerformanceDateShortYearFirst,
				show.ContainerInfo,
			)
		}
		table.Print()

		fmt.Printf("\n%sLegend:%s %s Audio  %s Video  %s Both\n",
			ui.ColorCyan, ui.ColorReset, ui.SymbolAudio, ui.SymbolVideo, ui.SymbolBoth)
	}
	return nil
}

// CatalogGaps finds missing shows for one or more artists.
func CatalogGaps(ctx context.Context, artistIds []string, cfg *model.Config, jsonLevel string, idsOnly bool, mediaFilter model.MediaType, deps *Deps) error {
	for i, artistId := range artistIds {
		if i > 0 && jsonLevel == "" && !idsOnly {
			fmt.Println()
			fmt.Println(strings.Repeat("\u2500", 80))
			fmt.Println()
		}

		err := CatalogGapsForArtist(ctx, artistId, cfg, jsonLevel, idsOnly, mediaFilter, deps)
		if err != nil {
			if len(artistIds) > 1 {
				if jsonLevel == "" {
					ui.PrintWarning(fmt.Sprintf("Failed to analyze artist %s: %v", artistId, err))
				}
				continue
			}
			return err
		}
	}

	return nil
}

// CatalogGapsFill downloads all missing shows for an artist.
func CatalogGapsFill(ctx context.Context, artistId string, cfg *model.Config, streamParams *model.StreamParams, jsonLevel string, mediaFilter model.MediaType, deps *Deps) error {
	analysis, err := AnalyzeArtistCatalog(ctx, artistId, cfg, jsonLevel, mediaFilter, deps)
	if err != nil {
		return err
	}
	missingShows := analysis.MissingShows

	if len(missingShows) == 0 {
		if jsonLevel != "" {
			output := map[string]any{
				"success":    true,
				"artistID":   artistId,
				"artistName": analysis.ArtistName,
				"totalShows": analysis.TotalShows,
				"downloaded": 0,
				"message":    "No missing shows found",
				"cacheUsed":  analysis.CacheUsed,
			}
			if err := PrintJSON(output); err != nil {
				return err
			}
		} else {
			ui.PrintSuccess(fmt.Sprintf("All shows already downloaded for %s%s%s", ui.ColorCyan, analysis.ArtistName, ui.ColorReset))
		}
		return nil
	}

	if jsonLevel == "" {
		ui.PrintHeader(fmt.Sprintf("Filling Gaps: %s", analysis.ArtistName))
		ui.PrintKeyValue("Total Missing", fmt.Sprintf("%d shows", len(missingShows)), ui.ColorYellow)
		fmt.Println()
	}

	successCount := 0
	failedCount := 0
	var failedShows []map[string]any
	interrupted := false

	batchState := &model.BatchProgressState{
		TotalAlbums: len(missingShows),
		StartTime:   time.Now(),
	}

	sharedProgressBox := &model.ProgressBoxState{
		RcloneEnabled:  cfg.RcloneEnabled,
		BatchState:     batchState,
		StartTime:      time.Now(),
		RenderInterval: model.DefaultProgressRenderInterval,
	}
	if deps.SetCurrentProgressBox != nil {
		deps.SetCurrentProgressBox(sharedProgressBox)
	}
	defer func() {
		if deps.SetCurrentProgressBox != nil {
			deps.SetCurrentProgressBox(nil)
		}
	}()

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	for i, status := range missingShows {
		show := status.Show
		select {
		case <-ctx.Done():
			interrupted = true
			if jsonLevel == "" {
				fmt.Println()
				ui.PrintWarning("Interrupted by user - stopping after current download")
			}
		default:
		}
		if interrupted {
			break
		}

		batchState.CurrentAlbum = i + 1
		batchState.CurrentTitle = show.ContainerInfo

		err := deps.Album(ctx, fmt.Sprintf("%d", show.ContainerID), cfg, streamParams, nil, batchState, sharedProgressBox)
		if err != nil {
			errorCtx := buildGapFillErrorContext(ctx, show, cfg, deps)
			reasonHint := deriveGapFillReasonHint(err, errorCtx)
			failedCount++
			batchState.Failed++
			failedShows = append(failedShows, map[string]any{
				"containerID": show.ContainerID,
				"date":        show.PerformanceDateShortYearFirst,
				"title":       show.ContainerInfo,
				"error":       err.Error(),
				"reasonHint":  reasonHint,
				"context":     errorCtx,
			})
			if jsonLevel == "" {
				ui.PrintError(fmt.Sprintf("Failed to download show %d: %v", show.ContainerID, err))
				ui.PrintWarning(fmt.Sprintf("Reason hint: %s", reasonHint))
				ui.PrintInfo(fmt.Sprintf("Failure context: availability=%s activeState=%s tracks=%d products=%d productFormats=%d",
					errorCtx.AvailabilityType, errorCtx.ActiveState, errorCtx.Tracks, errorCtx.Products, errorCtx.ProductFormats))
				// Redact full paths to avoid PII leakage in logs
				ui.PrintInfo(fmt.Sprintf("Local check: audioExists=%t path=%s", errorCtx.LocalAudioExists, filepath.Base(errorCtx.LocalAudioPath)))
				ui.PrintInfo(fmt.Sprintf("Local check: videoExists=%t path=%s", errorCtx.LocalVideoExists, filepath.Base(errorCtx.LocalVideoPath)))
				if cfg.RcloneEnabled {
					ui.PrintInfo(fmt.Sprintf("Remote check: audioExists=%t path=%s err=%s",
						errorCtx.RemoteAudioExists, filepath.Base(errorCtx.RemoteAudioPath), errorCtx.RemoteAudioError))
					ui.PrintInfo(fmt.Sprintf("Remote check: videoExists=%t path=%s err=%s",
						errorCtx.RemoteVideoExists, filepath.Base(errorCtx.RemoteVideoPath), errorCtx.RemoteVideoError))
				}
			}
		} else {
			successCount++
			batchState.Complete++
		}
	}

	attempted := successCount + failedCount
	remaining := len(missingShows) - attempted

	if jsonLevel != "" {
		output := map[string]any{
			"success":       failedCount == 0 && !interrupted,
			"interrupted":   interrupted,
			"artistID":      artistId,
			"artistName":    analysis.ArtistName,
			"totalShows":    analysis.TotalShows,
			"totalMissing":  len(missingShows),
			"attempted":     attempted,
			"downloaded":    successCount,
			"failed":        failedCount,
			"remaining":     remaining,
			"failedShows":   failedShows,
			"cacheUsed":     analysis.CacheUsed,
			"cacheStaleUse": analysis.CacheStaleUse,
		}
		if err := PrintJSON(output); err != nil {
			return err
		}
	} else {
		fmt.Println()
		ui.PrintHeader("Download Summary")
		if interrupted {
			ui.PrintWarning("Download was interrupted by user")
		}
		ui.PrintKeyValue("Total Missing", fmt.Sprintf("%d", len(missingShows)), ui.ColorCyan)
		ui.PrintKeyValue("Attempted", fmt.Sprintf("%d", attempted), ui.ColorCyan)
		ui.PrintKeyValue("Successfully Downloaded", fmt.Sprintf("%d", successCount), ui.ColorGreen)
		if failedCount > 0 {
			ui.PrintKeyValue("Failed", fmt.Sprintf("%d", failedCount), ui.ColorRed)
			fmt.Println()
			ui.PrintSection("Failed Downloads")
			for _, failed := range failedShows {
				fmt.Printf("  %s%s%s - %s\n",
					ui.ColorRed, failed["date"], ui.ColorReset, failed["title"])
			}
		}
		if remaining > 0 {
			ui.PrintKeyValue("Remaining", fmt.Sprintf("%d (re-run to continue)", remaining), ui.ColorYellow)
		}
		fmt.Println()
	}

	return nil
}

// CatalogCoverage shows download coverage statistics for artists.
func CatalogCoverage(ctx context.Context, artistIds []string, cfg *model.Config, jsonLevel string, mediaFilter model.MediaType, deps *Deps) error {
	type coverageStats struct {
		artistID        string
		artistName      string
		totalShows      int
		downloadedCount int
		coveragePct     float64
	}

	var allStats []coverageStats
	var remoteScanErr error

	if len(artistIds) == 0 {
		if jsonLevel == "" {
			ui.PrintWarning("No artist IDs provided - scanning local and remote artist folders...")
		}

		discoveredArtistDirs := make(map[string]struct{})

		entries, err := os.ReadDir(cfg.OutPath)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				artistPath := filepath.Join(cfg.OutPath, entry.Name())
				subEntries, readErr := os.ReadDir(artistPath)
				if readErr == nil && len(subEntries) > 0 {
					discoveredArtistDirs[entry.Name()] = struct{}{}
				}
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read output directory: %w", err)
		}

		remoteArtistDirs, err := ListAllRemoteArtistFolders(ctx, cfg)
		if err != nil {
			remoteScanErr = err
			if jsonLevel == "" {
				ui.PrintWarning(fmt.Sprintf("Remote artist scan failed: %v", err))
			}
		}
		for artistDir := range remoteArtistDirs {
			discoveredArtistDirs[artistDir] = struct{}{}
		}

		if len(discoveredArtistDirs) == 0 {
			if jsonLevel != "" {
				output := map[string]any{
					"artists": []map[string]any{},
					"total":   0,
					"message": "No downloaded artists found",
				}
				if err := PrintJSON(output); err != nil {
					return err
				}
			} else {
				fmt.Println("No downloaded artists found in local output or remote storage")
			}
			return nil
		}

		catalog, err := cache.ReadCatalogCache()
		if err != nil {
			return fmt.Errorf("failed to read catalog cache (run 'nugs catalog update' first): %w", err)
		}

		artistMapping := make(map[string]string)
		artistMappingNormalized := make(map[string]string)
		for _, item := range catalog.Response.RecentItems {
			normalizedName := helpers.Sanitise(item.ArtistName)
			artistID := fmt.Sprintf("%d", item.ArtistID)
			artistMapping[normalizedName] = artistID
			artistMappingNormalized[NormalizeArtistFolderKey(normalizedName)] = artistID
		}

		artistIDSet := make(map[string]struct{})
		unmatchedCount := 0
		for artistDir := range discoveredArtistDirs {
			if artistID, found := artistMapping[artistDir]; found {
				artistIDSet[artistID] = struct{}{}
				continue
			}
			if artistID, found := artistMappingNormalized[NormalizeArtistFolderKey(artistDir)]; found {
				artistIDSet[artistID] = struct{}{}
				continue
			}
			unmatchedCount++
		}

		for artistID := range artistIDSet {
			artistIds = append(artistIds, artistID)
		}
		sort.Strings(artistIds)

		if unmatchedCount > 0 && jsonLevel == "" {
			ui.PrintWarning(fmt.Sprintf("Skipped %d unmatched artist folder(s) not found in catalog mapping", unmatchedCount))
		}

		if len(artistIds) == 0 {
			if jsonLevel != "" {
				output := map[string]any{
					"artists": []map[string]any{},
					"total":   0,
					"message": "No downloaded artists found",
				}
				if err := PrintJSON(output); err != nil {
					return err
				}
			} else {
				fmt.Println("No downloaded artists found in output directory")
			}
			return nil
		}
	}

	for _, artistId := range artistIds {
		analysis, err := AnalyzeArtistCatalog(ctx, artistId, cfg, jsonLevel, mediaFilter, deps)
		if err != nil {
			if jsonLevel == "" {
				ui.PrintWarning(fmt.Sprintf("Failed to get metadata for artist %s: %v", artistId, err))
			}
			continue
		}

		allStats = append(allStats, coverageStats{
			artistID:        artistId,
			artistName:      analysis.ArtistName,
			totalShows:      analysis.TotalShows,
			downloadedCount: analysis.Downloaded,
			coveragePct:     analysis.DownloadPct,
		})
	}

	sort.Slice(allStats, func(i, j int) bool {
		return allStats[i].coveragePct > allStats[j].coveragePct
	})

	totalShows := 0
	totalDownloaded := 0
	for _, stats := range allStats {
		totalShows += stats.totalShows
		totalDownloaded += stats.downloadedCount
	}
	totalCoveragePct := 0.0
	if totalShows > 0 {
		totalCoveragePct = (float64(totalDownloaded) / float64(totalShows)) * 100
	}

	if jsonLevel != "" {
		artistsData := make([]map[string]any, len(allStats))
		for i, stats := range allStats {
			artistsData[i] = map[string]any{
				"artistID":   stats.artistID,
				"artistName": stats.artistName,
				"totalShows": stats.totalShows,
				"downloaded": stats.downloadedCount,
				"missing":    stats.totalShows - stats.downloadedCount,
				"coverage":   stats.coveragePct,
			}
		}
		output := map[string]any{
			"artists": artistsData,
			"total":   len(allStats),
			"summary": map[string]any{
				"downloaded": totalDownloaded,
				"totalShows": totalShows,
				"missing":    totalShows - totalDownloaded,
				"coverage":   totalCoveragePct,
			},
			"remoteScanError": nil,
		}
		if remoteScanErr != nil {
			output["remoteScanError"] = remoteScanErr.Error()
		}
		if err := PrintJSON(output); err != nil {
			return err
		}
	} else {
		ui.PrintHeader("Download Coverage Statistics")
		ui.PrintKeyValue("Artists", fmt.Sprintf("%d", len(allStats)), ui.ColorCyan)
		ui.PrintKeyValue("Overall", fmt.Sprintf("%d/%d (%.1f%%)", totalDownloaded, totalShows, totalCoveragePct), ui.ColorGreen)
		fmt.Println()

		table := ui.NewTable([]ui.TableColumn{
			{Header: "Artist ID", Width: 12, Align: "right"},
			{Header: "Artist Name", Width: 40, Align: "left"},
			{Header: "Downloaded", Width: 12, Align: "right"},
			{Header: "Total", Width: 10, Align: "right"},
			{Header: "Coverage", Width: 12, Align: "right"},
		})

		for _, stats := range allStats {
			coverageColor := ui.ColorGreen
			if stats.coveragePct < 50 {
				coverageColor = ui.ColorYellow
			}
			if stats.coveragePct < 25 {
				coverageColor = ui.ColorRed
			}

			table.AddRow(
				stats.artistID,
				stats.artistName,
				fmt.Sprintf("%d", stats.downloadedCount),
				fmt.Sprintf("%d", stats.totalShows),
				fmt.Sprintf("%s%.1f%%%s", coverageColor, stats.coveragePct, ui.ColorReset),
			)
		}

		table.Print()
		fmt.Println()
	}

	return nil
}

// CatalogList displays all shows for an artist with status indicators.
func CatalogList(ctx context.Context, artistIds []string, cfg *model.Config, jsonLevel string, mediaFilter model.MediaType, deps *Deps) error {
	for i, artistId := range artistIds {
		if i > 0 && jsonLevel == "" {
			fmt.Println()
			fmt.Println(strings.Repeat("\u2500", 80))
			fmt.Println()
		}

		err := CatalogListForArtist(ctx, artistId, cfg, jsonLevel, mediaFilter, deps)
		if err != nil {
			if len(artistIds) > 1 {
				if jsonLevel == "" {
					ui.PrintWarning(fmt.Sprintf("Failed to analyze artist %s: %v", artistId, err))
				}
				continue
			}
			return err
		}
	}

	return nil
}

// CatalogListForArtist displays all shows for a single artist with status indicators.
func CatalogListForArtist(ctx context.Context, artistId string, cfg *model.Config, jsonLevel string, mediaFilter model.MediaType, deps *Deps) error {
	analysis, err := AnalyzeArtistCatalog(ctx, artistId, cfg, jsonLevel, mediaFilter, deps)
	if err != nil {
		return err
	}

	if jsonLevel != "" {
		allShowsData := make([]map[string]any, len(analysis.Shows))
		for i, item := range analysis.Shows {
			show := item.Show
			allShowsData[i] = map[string]any{
				"containerID": show.ContainerID,
				"date":        show.PerformanceDateShortYearFirst,
				"title":       show.ContainerInfo,
				"venue":       show.Venue,
				"downloaded":  item.Downloaded,
			}
		}
		output := map[string]any{
			"artistID":      artistId,
			"artistName":    analysis.ArtistName,
			"totalShows":    analysis.TotalShows,
			"downloaded":    analysis.Downloaded,
			"downloadedPct": analysis.DownloadPct,
			"missing":       analysis.Missing,
			"missingPct":    analysis.MissingPct,
			"shows":         allShowsData,
			"cacheUsed":     analysis.CacheUsed,
			"cacheStaleUse": analysis.CacheStaleUse,
		}
		if err := PrintJSON(output); err != nil {
			return err
		}
	} else {
		ui.PrintHeader(fmt.Sprintf("Complete Catalog: %s", analysis.ArtistName))

		ui.PrintKeyValue("Total Shows", fmt.Sprintf("%d", analysis.TotalShows), ui.ColorCyan)
		ui.PrintKeyValue("Downloaded", fmt.Sprintf("%d (%.1f%%)", analysis.Downloaded, analysis.DownloadPct), ui.ColorGreen)
		ui.PrintKeyValue("Missing", fmt.Sprintf("%d (%.1f%%)", analysis.Missing, analysis.MissingPct), ui.ColorYellow)

		ui.PrintSection("All Shows")

		table := ui.NewTable([]ui.TableColumn{
			{Header: "Type", Width: 6, Align: "center"},
			{Header: "Status", Width: 8, Align: "center"},
			{Header: "ID", Width: 10, Align: "right"},
			{Header: "Date", Width: 14, Align: "left"},
			{Header: "Title", Width: 50, Align: "left"},
		})

		for _, item := range analysis.Shows {
			show := item.Show
			mediaIndicator := ui.GetMediaTypeIndicator(item.MediaType)
			status := ""
			if item.Downloaded {
				status = fmt.Sprintf("%s%s%s", ui.ColorGreen, ui.SymbolCheck, ui.ColorReset)
			} else {
				status = fmt.Sprintf("%s%s%s", ui.ColorRed, ui.SymbolCross, ui.ColorReset)
			}

			table.AddRow(
				mediaIndicator,
				status,
				fmt.Sprintf("%d", show.ContainerID),
				show.PerformanceDateShortYearFirst,
				show.ContainerInfo,
			)
		}

		table.Print()

		fmt.Printf("\n%sLegend:%s %s Audio  %s Video  %s Both  |  %s%s%s Downloaded  %s%s%s Missing\n",
			ui.ColorCyan, ui.ColorReset, ui.SymbolAudio, ui.SymbolVideo, ui.SymbolBoth,
			ui.ColorGreen, ui.SymbolCheck, ui.ColorReset, ui.ColorRed, ui.SymbolCross, ui.ColorReset)
		fmt.Printf("  To download: %snugs <container_id>%s\n\n",
			ui.ColorBold, ui.ColorReset)
	}

	return nil
}
