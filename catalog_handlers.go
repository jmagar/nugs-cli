package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Catalog Command Handlers

// showExists checks if a show has been downloaded locally or exists on remote storage
func showExists(show *AlbArtResp, cfg *Config) bool {
	albumFolder := fmt.Sprintf("%s - %s", show.ArtistName, show.ContainerInfo)
	if len(albumFolder) > 120 {
		albumFolder = albumFolder[:120]
	}
	albumFolder = sanitise(albumFolder)

	// Determine base path: use rclonePath if specified, otherwise outPath
	basePath := cfg.OutPath
	if cfg.RclonePath != "" {
		basePath = cfg.RclonePath
	}

	albumPath := filepath.Join(basePath, sanitise(show.ArtistName), albumFolder)

	// Check local existence
	_, err := os.Stat(albumPath)
	if err == nil {
		return true
	}

	// Check remote if rclone enabled
	if cfg.RcloneEnabled {
		remotePath := filepath.Join(sanitise(show.ArtistName), albumFolder)
		remoteExists, _ := remotePathExists(remotePath, cfg)
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
			fmt.Printf("  %2d. %-26s %-12s %-42s %-30s\n",
				i+1,
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

	// Get all shows for this artist
	artistMetas, err := getArtistMeta(artistId)
	if err != nil {
		return fmt.Errorf("failed to get artist metadata: %w", err)
	}

	if len(artistMetas) == 0 {
		return fmt.Errorf("no shows found for artist %s", artistId)
	}

	allShows, artistName := collectArtistShows(artistMetas)

	if len(allShows) == 0 {
		return fmt.Errorf("no shows found for artist %s", artistId)
	}

	// Check which shows are missing
	var missingShows []*AlbArtResp
	downloadedCount := 0

	for _, show := range allShows {
		if showExists(show, cfg) {
			downloadedCount++
		} else {
			missingShows = append(missingShows, show)
		}
	}

	// Sort missing shows by date (newest first)
	sort.Slice(missingShows, func(i, j int) bool {
		return missingShows[i].PerformanceDate > missingShows[j].PerformanceDate
	})

	totalShows := len(allShows)
	downloadedPct := float64(downloadedCount) / float64(totalShows) * 100
	missingPct := float64(len(missingShows)) / float64(totalShows) * 100

	// Output results
	if idsOnly {
		for _, show := range missingShows {
			fmt.Println(show.ContainerID)
		}
	} else if jsonLevel != "" {
		missingData := make([]map[string]any, len(missingShows))
		for i, show := range missingShows {
			missingData[i] = map[string]any{
				"containerID": show.ContainerID,
				"date":        show.PerformanceDateShortYearFirst,
				"title":       show.ContainerInfo,
				"venue":       show.Venue,
			}
		}
		output := map[string]any{
			"artistID":      artistId,
			"artistName":    artistName,
			"totalShows":    totalShows,
			"downloaded":    downloadedCount,
			"downloadedPct": downloadedPct,
			"missing":       len(missingShows),
			"missingPct":    missingPct,
			"missingShows":  missingData,
		}
		if err := printJSON(output); err != nil {
			return err
		}
	} else {
		printHeader(fmt.Sprintf("Gap Analysis: %s", artistName))

		printKeyValue("Total Shows", fmt.Sprintf("%d", len(allShows)), colorCyan)
		printKeyValue("Downloaded", fmt.Sprintf("%d (%.1f%%)", downloadedCount, downloadedPct), colorGreen)
		printKeyValue("Missing", fmt.Sprintf("%d (%.1f%%)", len(missingShows), missingPct), colorYellow)

		if len(missingShows) > 0 {
			printSection("Missing Shows")

			table := NewTable([]TableColumn{
				{Header: "ID", Width: 10, Align: "right"},
				{Header: "Date", Width: 14, Align: "left"},
				{Header: "Title", Width: 60, Align: "left"},
			})

			for _, show := range missingShows {
				table.AddRow(
					fmt.Sprintf("%d", show.ContainerID),
					show.PerformanceDateShortYearFirst,
					show.ContainerInfo,
				)
			}

			table.Print()

			fmt.Printf("\n%s%s%s To download: %snugs <container_id>%s\n",
				colorCyan, symbolInfo, colorReset, colorBold, colorReset)
			fmt.Printf("  Example: %snugs %d%s\n\n",
				colorGreen, missingShows[0].ContainerID, colorReset)
		}
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
	// Get all shows for this artist
	artistMetas, err := getArtistMeta(artistId)
	if err != nil {
		return fmt.Errorf("failed to get artist metadata: %w", err)
	}

	if len(artistMetas) == 0 {
		return fmt.Errorf("no shows found for artist %s", artistId)
	}

	allShows, artistName := collectArtistShows(artistMetas)

	if len(allShows) == 0 {
		return fmt.Errorf("no shows found for artist %s", artistId)
	}

	// Check which shows are missing
	var missingShows []*AlbArtResp
	for _, show := range allShows {
		if !showExists(show, cfg) {
			missingShows = append(missingShows, show)
		}
	}

	// Sort missing shows by date (newest first)
	sort.Slice(missingShows, func(i, j int) bool {
		return missingShows[i].PerformanceDate > missingShows[j].PerformanceDate
	})

	if len(missingShows) == 0 {
		if jsonLevel != "" {
			output := map[string]any{
				"success":    true,
				"artistID":   artistId,
				"artistName": artistName,
				"totalShows": len(allShows),
				"downloaded": 0,
				"message":    "No missing shows found",
			}
			if err := printJSON(output); err != nil {
				return err
			}
		} else {
			printSuccess(fmt.Sprintf("All shows already downloaded for %s%s%s", colorCyan, artistName, colorReset))
		}
		return nil
	}

	// Display summary and start downloading
	if jsonLevel == "" {
		printHeader(fmt.Sprintf("Filling Gaps: %s", artistName))
		printKeyValue("Total Missing", fmt.Sprintf("%d shows", len(missingShows)), colorYellow)
		fmt.Println()
	}

	successCount := 0
	failedCount := 0
	var failedShows []map[string]any
	interrupted := false

	// Set up signal handling for graceful Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	defer signal.Stop(sigChan)

	for i, show := range missingShows {
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

		if jsonLevel == "" {
			fmt.Printf("%s%s Downloading %d/%d:%s %s%s%s - %s\n",
				colorBold, symbolDownload, i+1, len(missingShows), colorReset,
				colorCyan, show.PerformanceDateShortYearFirst, colorReset,
				show.ContainerInfo)
		}

		err := album(fmt.Sprintf("%d", show.ContainerID), cfg, streamParams, nil)
		if err != nil {
			failedCount++
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
		}
	}

	// Display final summary
	attempted := successCount + failedCount
	remaining := len(missingShows) - attempted

	if jsonLevel != "" {
		output := map[string]any{
			"success":      failedCount == 0 && !interrupted,
			"interrupted":  interrupted,
			"artistID":     artistId,
			"artistName":   artistName,
			"totalShows":   len(allShows),
			"totalMissing": len(missingShows),
			"attempted":    attempted,
			"downloaded":   successCount,
			"failed":       failedCount,
			"remaining":    remaining,
			"failedShows":  failedShows,
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
		// Get all artists from downloaded directories
		entries, err := os.ReadDir(cfg.OutPath)
		if err != nil {
			return fmt.Errorf("failed to read output directory: %w", err)
		}

		// For each artist directory, we need to find their artist ID
		// This requires checking the catalog or making API calls
		if jsonLevel == "" {
			printWarning("No artist IDs provided - scanning downloaded artists...")
		}

		// Read catalog to get artist mappings
		catalog, err := readCatalogCache()
		if err != nil {
			return fmt.Errorf("failed to read catalog cache (run 'nugs catalog update' first): %w", err)
		}

		// Build artist name -> ID mapping
		artistMapping := make(map[string]string)
		for _, item := range catalog.Response.RecentItems {
			normalizedName := sanitise(item.ArtistName)
			artistMapping[normalizedName] = fmt.Sprintf("%d", item.ArtistID)
		}

		// Check each directory
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			artistDir := entry.Name()
			if artistID, found := artistMapping[artistDir]; found {
				// Check if there are any downloads
				artistPath := filepath.Join(cfg.OutPath, artistDir)
				subEntries, _ := os.ReadDir(artistPath)
				if len(subEntries) > 0 {
					artistIds = append(artistIds, artistID)
				}
			}
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
		artistMetas, err := getArtistMeta(artistId)
		if err != nil {
			if jsonLevel == "" {
				printWarning(fmt.Sprintf("Failed to get metadata for artist %s: %v", artistId, err))
			}
			continue
		}

		if len(artistMetas) == 0 {
			continue
		}

		allShows, artistName := collectArtistShows(artistMetas)

		// Count downloaded shows
		downloadedCount := 0
		for _, show := range allShows {
			if showExists(show, cfg) {
				downloadedCount++
			}
		}

		coveragePct := 0.0
		if len(allShows) > 0 {
			coveragePct = float64(downloadedCount) / float64(len(allShows)) * 100
		}

		allStats = append(allStats, coverageStats{
			artistID:        artistId,
			artistName:      artistName,
			totalShows:      len(allShows),
			downloadedCount: downloadedCount,
			coveragePct:     coveragePct,
		})
	}

	// Sort by coverage percentage (descending)
	sort.Slice(allStats, func(i, j int) bool {
		return allStats[i].coveragePct > allStats[j].coveragePct
	})

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
		}
		if err := printJSON(output); err != nil {
			return err
		}
	} else {
		printHeader("Download Coverage Statistics")

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
	// Get all shows for this artist
	artistMetas, err := getArtistMeta(artistId)
	if err != nil {
		return fmt.Errorf("failed to get artist metadata: %w", err)
	}

	if len(artistMetas) == 0 {
		return fmt.Errorf("no shows found for artist %s", artistId)
	}

	allShows, artistName := collectArtistShows(artistMetas)

	if len(allShows) == 0 {
		return fmt.Errorf("no shows found for artist %s", artistId)
	}

	// Sort shows by date (newest first)
	sort.Slice(allShows, func(i, j int) bool {
		return allShows[i].PerformanceDate > allShows[j].PerformanceDate
	})

	// Calculate statistics
	downloadedCount := 0
	var downloadedShows []*AlbArtResp
	var missingShows []*AlbArtResp

	for _, show := range allShows {
		if showExists(show, cfg) {
			downloadedCount++
			downloadedShows = append(downloadedShows, show)
		} else {
			missingShows = append(missingShows, show)
		}
	}

	totalShows := len(allShows)
	downloadedPct := float64(downloadedCount) / float64(totalShows) * 100
	missingPct := float64(len(missingShows)) / float64(totalShows) * 100

	// Output results
	if jsonLevel != "" {
		allShowsData := make([]map[string]any, len(allShows))
		for i, show := range allShows {
			allShowsData[i] = map[string]any{
				"containerID": show.ContainerID,
				"date":        show.PerformanceDateShortYearFirst,
				"title":       show.ContainerInfo,
				"venue":       show.Venue,
				"downloaded":  showExists(show, cfg),
			}
		}
		output := map[string]any{
			"artistID":      artistId,
			"artistName":    artistName,
			"totalShows":    totalShows,
			"downloaded":    downloadedCount,
			"downloadedPct": downloadedPct,
			"missing":       len(missingShows),
			"missingPct":    missingPct,
			"shows":         allShowsData,
		}
		if err := printJSON(output); err != nil {
			return err
		}
	} else {
		printHeader(fmt.Sprintf("Complete Catalog: %s", artistName))

		printKeyValue("Total Shows", fmt.Sprintf("%d", totalShows), colorCyan)
		printKeyValue("Downloaded", fmt.Sprintf("%d (%.1f%%)", downloadedCount, downloadedPct), colorGreen)
		printKeyValue("Missing", fmt.Sprintf("%d (%.1f%%)", len(missingShows), missingPct), colorYellow)

		printSection("All Shows")

		table := NewTable([]TableColumn{
			{Header: "Status", Width: 8, Align: "center"},
			{Header: "ID", Width: 10, Align: "right"},
			{Header: "Date", Width: 14, Align: "left"},
			{Header: "Title", Width: 55, Align: "left"},
		})

		for _, show := range allShows {
			status := ""
			if showExists(show, cfg) {
				status = fmt.Sprintf("%s✓%s", colorGreen, colorReset)
			} else {
				status = fmt.Sprintf("%s✗%s", colorRed, colorReset)
			}

			table.AddRow(
				status,
				fmt.Sprintf("%d", show.ContainerID),
				show.PerformanceDateShortYearFirst,
				show.ContainerInfo,
			)
		}

		table.Print()

		fmt.Printf("\n%s%s%s Legend: %s✓%s Downloaded  %s✗%s Missing\n",
			colorCyan, symbolInfo, colorReset,
			colorGreen, colorReset,
			colorRed, colorReset)
		fmt.Printf("  To download: %snugs <container_id>%s\n\n",
			colorBold, colorReset)
	}

	return nil
}
