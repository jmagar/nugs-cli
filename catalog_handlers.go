package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Catalog Command Handlers

const artistMetaCacheTTL = 24 * time.Hour

type artistPresenceIndex struct {
	artistFolder  string
	localFolders  map[string]struct{}
	remoteFolders map[string]struct{}
	remoteListErr error
}

var remoteCheckWarnOnce sync.Once

func warnRemoteCheckError(err error) {
	remoteCheckWarnOnce.Do(func() {
		printWarning(fmt.Sprintf("Remote existence checks failed; treating as not found. First error: %v", err))
	})
}

func listAllRemoteArtistFolders(cfg *Config) (map[string]struct{}, error) {
	folders := make(map[string]struct{})
	if !cfg.RcloneEnabled {
		return folders, nil
	}

	remoteDest := cfg.RcloneRemote + ":" + cfg.RclonePath
	cmd := exec.Command("rclone", "lsf", remoteDest, "--dirs-only")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 3 {
			return folders, nil
		}
		return nil, fmt.Errorf("failed to list remote artist folders: %w", err)
	}

	for _, line := range strings.Split(string(output), "\n") {
		trimmed := strings.TrimSuffix(strings.TrimSpace(line), "/")
		if trimmed == "" {
			continue
		}
		folders[trimmed] = struct{}{}
	}
	return folders, nil
}

func normalizeArtistFolderKey(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// showExists checks if a show has been downloaded locally or exists on remote storage
func showExists(show *AlbArtResp, cfg *Config) bool {
	albumFolder := buildAlbumFolderName(show.ArtistName, show.ContainerInfo)
	albumPath := filepath.Join(cfg.OutPath, sanitise(show.ArtistName), albumFolder)

	// Check local existence
	_, err := os.Stat(albumPath)
	if err == nil {
		return true
	}

	// Check remote if rclone enabled
	if cfg.RcloneEnabled {
		remotePath := filepath.Join(sanitise(show.ArtistName), albumFolder)
		remoteExists, err := remotePathExists(remotePath, cfg, false)
		if err != nil {
			warnRemoteCheckError(err)
			return false
		}
		if remoteExists {
			return true
		}
	}

	return false
}

// collectArtistShows gathers all shows and artist name from artist metadata responses
func collectArtistShows(artistMetas []*ArtistMeta) (allShows []*AlbArtResp, artistName string) {
	for _, meta := range artistMetas {
		allShows = append(allShows, meta.Response.Containers...)
		if artistName == "" && len(meta.Response.Containers) > 0 {
			artistName = meta.Response.Containers[0].ArtistName
		}
	}
	return allShows, artistName
}

func buildArtistPresenceIndex(artistName string, cfg *Config) artistPresenceIndex {
	idx := artistPresenceIndex{
		artistFolder:  sanitise(artistName),
		localFolders:  make(map[string]struct{}),
		remoteFolders: make(map[string]struct{}),
	}

	localArtistPath := filepath.Join(cfg.OutPath, idx.artistFolder)
	if entries, err := os.ReadDir(localArtistPath); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			idx.localFolders[entry.Name()] = struct{}{}
		}
	}

	if cfg.RcloneEnabled {
		remoteFolders, err := listRemoteArtistFolders(idx.artistFolder, cfg)
		if err != nil {
			idx.remoteListErr = err
		} else {
			idx.remoteFolders = remoteFolders
		}
	}

	return idx
}

func isShowDownloaded(show *AlbArtResp, idx artistPresenceIndex, cfg *Config) bool {
	albumFolder := buildAlbumFolderName(show.ArtistName, show.ContainerInfo)

	if _, ok := idx.localFolders[albumFolder]; ok {
		return true
	}
	if _, ok := idx.remoteFolders[albumFolder]; ok {
		return true
	}

	// Fallback for remote-list failures to preserve correctness.
	if cfg.RcloneEnabled && idx.remoteListErr != nil {
		remotePath := filepath.Join(idx.artistFolder, albumFolder)
		remoteExists, err := remotePathExists(remotePath, cfg, false)
		if err != nil {
			warnRemoteCheckError(err)
			return false
		}
		return remoteExists
	}

	return false
}

func analyzeArtistCatalog(artistID string, cfg *Config, jsonLevel string) (*ArtistCatalogAnalysis, error) {
	artistMetas, cacheUsed, cacheStaleUse, err := getArtistMetaCached(artistID, artistMetaCacheTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to get artist metadata: %w", err)
	}

	if len(artistMetas) == 0 {
		return nil, fmt.Errorf("no shows found for artist %s", artistID)
	}

	allShows, artistName := collectArtistShows(artistMetas)
	if len(allShows) == 0 {
		return nil, fmt.Errorf("no shows found for artist %s", artistID)
	}

	sort.Slice(allShows, func(i, j int) bool {
		return allShows[i].PerformanceDate > allShows[j].PerformanceDate
	})

	presenceIdx := buildArtistPresenceIndex(artistName, cfg)
	if presenceIdx.remoteListErr != nil && jsonLevel == "" {
		printWarning(fmt.Sprintf("Remote artist folder bulk check failed, falling back to per-show checks: %v", presenceIdx.remoteListErr))
	}

	analysis := &ArtistCatalogAnalysis{
		ArtistID:      artistID,
		ArtistName:    artistName,
		TotalShows:    len(allShows),
		Shows:         make([]ShowStatus, 0, len(allShows)),
		MissingShows:  make([]ShowStatus, 0, len(allShows)),
		CacheUsed:     cacheUsed,
		CacheStaleUse: cacheStaleUse,
	}

	for _, show := range allShows {
		downloaded := isShowDownloaded(show, presenceIdx, cfg)
		status := ShowStatus{
			Show:       show,
			Downloaded: downloaded,
		}
		analysis.Shows = append(analysis.Shows, status)
		if downloaded {
			analysis.Downloaded++
			continue
		}
		analysis.MissingShows = append(analysis.MissingShows, status)
	}

	analysis.Missing = len(analysis.MissingShows)
	if analysis.TotalShows > 0 {
		analysis.DownloadPct = float64(analysis.Downloaded) / float64(analysis.TotalShows) * 100
		analysis.MissingPct = float64(analysis.Missing) / float64(analysis.TotalShows) * 100
	}

	return analysis, nil
}

// printJSON marshals data to JSON and prints it, handling errors properly
func printJSON(data any) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Println(string(jsonData))
	return nil
}

// catalogUpdate fetches and caches the latest catalog
func catalogUpdate(jsonLevel string) error {
	startTime := time.Now()
	catalog, err := getLatestCatalog()
	if err != nil {
		return fmt.Errorf("failed to fetch catalog: %w", err)
	}

	updateDuration := time.Since(startTime)
	err = writeCatalogCache(catalog, updateDuration)
	if err != nil {
		return err
	}

	cacheDir, _ := getCacheDir()

	if jsonLevel != "" {
		output := map[string]any{
			"success":    true,
			"totalShows": len(catalog.Response.RecentItems),
			"updateTime": formatDuration(updateDuration),
			"cacheDir":   cacheDir,
		}
		if err := printJSON(output); err != nil {
			return err
		}
	} else {
		fmt.Printf("✓ Catalog updated successfully\n")
		fmt.Printf("  Total shows: %s%d%s\n", colorGreen, len(catalog.Response.RecentItems), colorReset)
		fmt.Printf("  Update time: %s%s%s\n", colorCyan, formatDuration(updateDuration), colorReset)
		fmt.Printf("  Cache location: %s\n", cacheDir)
	}
	return nil
}

// catalogCacheStatus shows cache status and metadata
func catalogCacheStatus(jsonLevel string) error {
	meta, err := readCacheMeta()
	if err != nil {
		return err
	}

	if meta == nil {
		if jsonLevel != "" {
			output := map[string]any{"exists": false}
			if err := printJSON(output); err != nil {
				return err
			}
		} else {
			fmt.Println("No cache found - run 'nugs catalog update' first")
		}
		return nil
	}

	cacheDir, _ := getCacheDir()
	catalogPath := filepath.Join(cacheDir, "catalog.json")
	fileInfo, err := os.Stat(catalogPath)
	if err != nil {
		return fmt.Errorf("failed to stat cache file: %w", err)
	}

	age := time.Since(meta.LastUpdated)
	ageHuman := formatDuration(age)
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
		if err := printJSON(output); err != nil {
			return err
		}
	} else {
		printHeader("Catalog Cache Status")

		printKeyValue("Location", cacheDir, colorCyan)
		printKeyValue("Last Updated", fmt.Sprintf("%s (%s ago)", meta.LastUpdated.Format("2006-01-02 15:04:05"), ageHuman), "")
		printKeyValue("Total Shows", fmt.Sprintf("%d", meta.TotalShows), colorGreen)
		printKeyValue("Artists", fmt.Sprintf("%d unique", meta.TotalArtists), colorCyan)
		printKeyValue("Cache Size", fileSizeHuman, "")
		printKeyValue("Version", meta.CacheVersion, "")

		if age.Hours() > 24 {
			fmt.Println()
			printWarning(fmt.Sprintf("Cache is over 24 hours old - consider running '%snugs catalog update%s'", colorBold, colorReset))
		}
		fmt.Println()
	}
	return nil
}

// catalogStats shows catalog statistics
func catalogStats(jsonLevel string) error {
	catalog, err := readCatalogCache()
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

		// Parse performance date (format: "Jan 02, 2006")
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

	// Format dates for display
	var earliestDate, latestDate string
	if !earliestTime.IsZero() {
		earliestDate = earliestTime.Format("Jan 02, 2006")
	}
	if !latestTime.IsZero() {
		latestDate = latestTime.Format("Jan 02, 2006")
	}

	// Sort artists by show count
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

	// Get top 10
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
		if err := printJSON(output); err != nil {
			return err
		}
	} else {
		printHeader("Catalog Statistics")

		printKeyValue("Total Shows", fmt.Sprintf("%d", len(catalog.Response.RecentItems)), colorGreen)
		printKeyValue("Total Artists", fmt.Sprintf("%d unique", len(showsPerArtist)), colorCyan)
		printKeyValue("Date Range", fmt.Sprintf("%s to %s", earliestDate, latestDate), "")

		printSection("Top 10 Artists by Show Count")

		table := NewTable([]TableColumn{
			{Header: "ID", Width: 10, Align: "right"},
			{Header: "Artist", Width: 50, Align: "left"},
			{Header: "Shows", Width: 10, Align: "right"},
		})

		for _, a := range top10 {
			table.AddRow(
				fmt.Sprintf("%d", a.id),
				a.name,
				fmt.Sprintf("%s%d%s", colorGreen, a.count, colorReset),
			)
		}

		table.Print()
		fmt.Println()
	}
	return nil
}

// catalogLatest shows latest additions to catalog
func catalogLatest(limit int, jsonLevel string) error {
	catalog, err := readCatalogCache()
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
		if err := printJSON(output); err != nil {
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
				colorPurple,
				item.ArtistID,
				colorReset,
				artistName,
				item.ShowDateFormattedShort,
				title,
				location)
		}
	}
	return nil
}

// catalogGapsForArtist finds missing shows for a single artist.
// Returns structured gap data for aggregation by the caller.
func catalogGapsForArtist(artistId string, cfg *Config, jsonLevel string, idsOnly bool) error {
	analysis, err := analyzeArtistCatalog(artistId, cfg, jsonLevel)
	if err != nil {
		return err
	}

	// Output results
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
		if err := printJSON(output); err != nil {
			return err
		}
	} else {
		if len(analysis.MissingShows) == 0 {
			fmt.Println("No missing shows")
			return nil
		}

		table := NewTable([]TableColumn{
		{Header: "Type", Width: 6, Align: "center"},
		{Header: "ID", Width: 10, Align: "right"},
		{Header: "Date", Width: 14, Align: "left"},
		{Header: "Title", Width: 55, Align: "left"},
	})

	for _, status := range analysis.MissingShows {
		show := status.Show
		mediaIndicator := getMediaTypeIndicator(status.MediaType)
		table.AddRow(
			mediaIndicator,
			fmt.Sprintf("%d", show.ContainerID),
			show.PerformanceDateShortYearFirst,
			show.ContainerInfo,
		)
	}
	table.Print()

	fmt.Printf("\n%sLegend:%s %s Audio  %s Video  %s Both\n",
		colorCyan, colorReset, symbolAudio, symbolVideo, symbolBoth)
	}
	return nil
}

// catalogGaps finds missing shows for one or more artists
func catalogGaps(artistIds []string, cfg *Config, jsonLevel string, idsOnly bool) error {
	// Process each artist
	for i, artistId := range artistIds {
		// Add separator between artists (except before first)
		if i > 0 && jsonLevel == "" && !idsOnly {
			fmt.Println()
			fmt.Println(strings.Repeat("─", 80))
			fmt.Println()
		}

		err := catalogGapsForArtist(artistId, cfg, jsonLevel, idsOnly)
		if err != nil {
			// For multiple artists, continue on error but print warning
			if len(artistIds) > 1 {
				if jsonLevel == "" {
					printWarning(fmt.Sprintf("Failed to analyze artist %s: %v", artistId, err))
				}
				continue
			}
			return err
		}
	}

	return nil
}

// catalogGapsFill downloads all missing shows for an artist
func catalogGapsFill(artistId string, cfg *Config, streamParams *StreamParams, jsonLevel string) error {
	analysis, err := analyzeArtistCatalog(artistId, cfg, jsonLevel)
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
			if err := printJSON(output); err != nil {
				return err
			}
		} else {
			printSuccess(fmt.Sprintf("All shows already downloaded for %s%s%s", colorCyan, analysis.ArtistName, colorReset))
		}
		return nil
	}

	// Display summary and start downloading
	if jsonLevel == "" {
		printHeader(fmt.Sprintf("Filling Gaps: %s", analysis.ArtistName))
		printKeyValue("Total Missing", fmt.Sprintf("%d shows", len(missingShows)), colorYellow)
		fmt.Println()
	}

	successCount := 0
	failedCount := 0
	var failedShows []map[string]any
	interrupted := false

	// Create batch state for multi-show progress tracking (Tier 4 enhancement)
	batchState := &BatchProgressState{
		TotalAlbums: len(missingShows),
		StartTime:   time.Now(),
	}

	// Create ONE progress box for the entire batch (reused across all shows)
	sharedProgressBox := &ProgressBoxState{
		RcloneEnabled:  cfg.RcloneEnabled,
		BatchState:     batchState,
		StartTime:      time.Now(),
		RenderInterval: defaultProgressRenderInterval,
	}
	setCurrentProgressBox(sharedProgressBox)
	defer setCurrentProgressBox(nil)

	// Set up signal handling for graceful Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	defer signal.Stop(sigChan)

	for i, status := range missingShows {
		show := status.Show
		// Check for interrupt before starting next download
		select {
		case <-sigChan:
			interrupted = true
			if jsonLevel == "" {
				fmt.Println()
				printWarning("Interrupted by user - stopping after current download")
			}
		default:
		}
		if interrupted {
			break
		}

		// Update batch state
		batchState.CurrentAlbum = i + 1
		batchState.CurrentTitle = show.ContainerInfo

		// Pass the shared progress box to reuse it (no new boxes created!)
		err := album(fmt.Sprintf("%d", show.ContainerID), cfg, streamParams, nil, batchState, sharedProgressBox)
		if err != nil {
			failedCount++
			batchState.Failed++
			failedShows = append(failedShows, map[string]any{
				"containerID": show.ContainerID,
				"date":        show.PerformanceDateShortYearFirst,
				"title":       show.ContainerInfo,
				"error":       err.Error(),
			})
			if jsonLevel == "" {
				printError(fmt.Sprintf("Failed to download show %d: %v", show.ContainerID, err))
			}
		} else {
			successCount++
			batchState.Complete++
		}
	}

	// Display final summary
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
		if err := printJSON(output); err != nil {
			return err
		}
	} else {
		fmt.Println()
		printHeader("Download Summary")
		if interrupted {
			printWarning("Download was interrupted by user")
		}
		printKeyValue("Total Missing", fmt.Sprintf("%d", len(missingShows)), colorCyan)
		printKeyValue("Attempted", fmt.Sprintf("%d", attempted), colorCyan)
		printKeyValue("Successfully Downloaded", fmt.Sprintf("%d", successCount), colorGreen)
		if failedCount > 0 {
			printKeyValue("Failed", fmt.Sprintf("%d", failedCount), colorRed)
			fmt.Println()
			printSection("Failed Downloads")
			for _, failed := range failedShows {
				fmt.Printf("  %s%s%s - %s\n",
					colorRed, failed["date"], colorReset, failed["title"])
			}
		}
		if remaining > 0 {
			printKeyValue("Remaining", fmt.Sprintf("%d (re-run to continue)", remaining), colorYellow)
		}
		fmt.Println()
	}

	return nil
}

// catalogCoverage shows download coverage statistics for artists
func catalogCoverage(artistIds []string, cfg *Config, jsonLevel string) error {
	type coverageStats struct {
		artistID        string
		artistName      string
		totalShows      int
		downloadedCount int
		coveragePct     float64
	}

	var allStats []coverageStats

	// If no artist IDs provided, find all artists with downloads
	if len(artistIds) == 0 {
		if jsonLevel == "" {
			printWarning("No artist IDs provided - scanning local and remote artist folders...")
		}

		discoveredArtistDirs := make(map[string]struct{})

		// Local artist folders.
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

		// Remote artist folders (for delete-after-upload or remote-only collections).
		remoteArtistDirs, err := listAllRemoteArtistFolders(cfg)
		if err != nil && jsonLevel == "" {
			printWarning(fmt.Sprintf("Remote artist scan failed: %v", err))
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
				if err := printJSON(output); err != nil {
					return err
				}
			} else {
				fmt.Println("No downloaded artists found in local output or remote storage")
			}
			return nil
		}

		// Read catalog to get artist mappings
		catalog, err := readCatalogCache()
		if err != nil {
			return fmt.Errorf("failed to read catalog cache (run 'nugs catalog update' first): %w", err)
		}

		// Build artist folder -> ID mapping
		artistMapping := make(map[string]string)
		artistMappingNormalized := make(map[string]string)
		for _, item := range catalog.Response.RecentItems {
			normalizedName := sanitise(item.ArtistName)
			artistID := fmt.Sprintf("%d", item.ArtistID)
			artistMapping[normalizedName] = artistID
			artistMappingNormalized[normalizeArtistFolderKey(normalizedName)] = artistID
		}

		artistIDSet := make(map[string]struct{})
		unmatchedCount := 0
		for artistDir := range discoveredArtistDirs {
			if artistID, found := artistMapping[artistDir]; found {
				artistIDSet[artistID] = struct{}{}
				continue
			}
			if artistID, found := artistMappingNormalized[normalizeArtistFolderKey(artistDir)]; found {
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
			printWarning(fmt.Sprintf("Skipped %d unmatched artist folder(s) not found in catalog mapping", unmatchedCount))
		}

		if len(artistIds) == 0 {
			if jsonLevel != "" {
				output := map[string]any{
					"artists": []map[string]any{},
					"total":   0,
					"message": "No downloaded artists found",
				}
				if err := printJSON(output); err != nil {
					return err
				}
			} else {
				fmt.Println("No downloaded artists found in output directory")
			}
			return nil
		}
	}

	// Get coverage stats for each artist
	for _, artistId := range artistIds {
		analysis, err := analyzeArtistCatalog(artistId, cfg, jsonLevel)
		if err != nil {
			if jsonLevel == "" {
				printWarning(fmt.Sprintf("Failed to get metadata for artist %s: %v", artistId, err))
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

	// Sort by coverage percentage (descending)
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

	// Output results
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
		}
		if err := printJSON(output); err != nil {
			return err
		}
	} else {
		printHeader("Download Coverage Statistics")
		printKeyValue("Artists", fmt.Sprintf("%d", len(allStats)), colorCyan)
		printKeyValue("Overall", fmt.Sprintf("%d/%d (%.1f%%)", totalDownloaded, totalShows, totalCoveragePct), colorGreen)
		fmt.Println()

		table := NewTable([]TableColumn{
			{Header: "Artist ID", Width: 12, Align: "right"},
			{Header: "Artist Name", Width: 40, Align: "left"},
			{Header: "Downloaded", Width: 12, Align: "right"},
			{Header: "Total", Width: 10, Align: "right"},
			{Header: "Coverage", Width: 12, Align: "right"},
		})

		for _, stats := range allStats {
			coverageColor := colorGreen
			if stats.coveragePct < 50 {
				coverageColor = colorYellow
			}
			if stats.coveragePct < 25 {
				coverageColor = colorRed
			}

			table.AddRow(
				stats.artistID,
				stats.artistName,
				fmt.Sprintf("%d", stats.downloadedCount),
				fmt.Sprintf("%d", stats.totalShows),
				fmt.Sprintf("%s%.1f%%%s", coverageColor, stats.coveragePct, colorReset),
			)
		}

		table.Print()
		fmt.Println()
	}

	return nil
}

// catalogList displays all shows for an artist with status indicators
func catalogList(artistIds []string, cfg *Config, jsonLevel string) error {
	// Process each artist
	for i, artistId := range artistIds {
		// Add separator between artists (except before first)
		if i > 0 && jsonLevel == "" {
			fmt.Println()
			fmt.Println(strings.Repeat("─", 80))
			fmt.Println()
		}

		err := catalogListForArtist(artistId, cfg, jsonLevel)
		if err != nil {
			// For multiple artists, continue on error but print warning
			if len(artistIds) > 1 {
				if jsonLevel == "" {
					printWarning(fmt.Sprintf("Failed to analyze artist %s: %v", artistId, err))
				}
				continue
			}
			return err
		}
	}

	return nil
}

// catalogListForArtist displays all shows for a single artist with status indicators
func catalogListForArtist(artistId string, cfg *Config, jsonLevel string) error {
	analysis, err := analyzeArtistCatalog(artistId, cfg, jsonLevel)
	if err != nil {
		return err
	}

	// Output results
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
		if err := printJSON(output); err != nil {
			return err
		}
	} else {
		printHeader(fmt.Sprintf("Complete Catalog: %s", analysis.ArtistName))

		printKeyValue("Total Shows", fmt.Sprintf("%d", analysis.TotalShows), colorCyan)
		printKeyValue("Downloaded", fmt.Sprintf("%d (%.1f%%)", analysis.Downloaded, analysis.DownloadPct), colorGreen)
		printKeyValue("Missing", fmt.Sprintf("%d (%.1f%%)", analysis.Missing, analysis.MissingPct), colorYellow)

		printSection("All Shows")

		table := NewTable([]TableColumn{
		{Header: "Type", Width: 6, Align: "center"},
		{Header: "Status", Width: 8, Align: "center"},
		{Header: "ID", Width: 10, Align: "right"},
		{Header: "Date", Width: 14, Align: "left"},
		{Header: "Title", Width: 50, Align: "left"},
	})

	for _, item := range analysis.Shows {
		show := item.Show
		mediaIndicator := getMediaTypeIndicator(item.MediaType)
		status := ""
		if item.Downloaded {
			status = fmt.Sprintf("%s%s%s", colorGreen, symbolCheck, colorReset)
		} else {
			status = fmt.Sprintf("%s%s%s", colorRed, symbolCross, colorReset)
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
		colorCyan, colorReset, symbolAudio, symbolVideo, symbolBoth,
		colorGreen, symbolCheck, colorReset, colorRed, symbolCross, colorReset)

		fmt.Printf("\n%s%s%s Legend: %s%s%s Downloaded  %s%s%s Missing\n",
			colorCyan, symbolInfo, colorReset,
			colorGreen, symbolCheck, colorReset,
			colorRed, symbolCross, colorReset)
		fmt.Printf("  To download: %snugs <container_id>%s\n\n",
			colorBold, colorReset)
	}

	return nil
}
