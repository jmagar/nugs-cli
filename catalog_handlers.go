package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Catalog Command Handlers

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
			"success":      true,
			"totalShows":   len(catalog.Response.RecentItems),
			"updateTime":   formatDuration(updateDuration),
			"cacheDir":     cacheDir,
		}
		jsonData, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(jsonData))
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
			jsonData, _ := json.MarshalIndent(output, "", "  ")
			fmt.Println(string(jsonData))
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
		jsonData, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(jsonData))
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
		jsonData, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(jsonData))
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
				"containerID":   item.ContainerID,
				"artistID":      item.ArtistID,
				"artistName":    item.ArtistName,
				"date":          item.ShowDateFormattedShort,
				"title":         item.ContainerInfo,
				"venue":         item.Venue,
				"venueCity":     item.VenueCity,
				"venueState":    item.VenueState,
			}
		}
		output := map[string]any{
			"shows": shows,
			"total": len(items),
			"limit": limit,
		}
		jsonData, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(jsonData))
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

// catalogGaps finds missing shows for an artist
func catalogGaps(artistId string, cfg *Config, jsonLevel string) error {
	// Check for --ids-only flag
	idsOnly := false
	for _, arg := range cfg.Urls {
		if arg == "--ids-only" {
			idsOnly = true
			break
		}
	}

	// Get all shows for this artist
	artistMetas, err := getArtistMeta(artistId)
	if err != nil {
		return fmt.Errorf("failed to get artist metadata: %w", err)
	}

	if len(artistMetas) == 0 {
		return fmt.Errorf("no shows found for artist %s", artistId)
	}

	artistName := ""
	var allShows []*AlbArtResp
	for _, meta := range artistMetas {
		allShows = append(allShows, meta.Response.Containers...)
		if artistName == "" && len(meta.Response.Containers) > 0 {
			artistName = meta.Response.Containers[0].ArtistName
		}
	}

	// Check which shows are missing
	var missingShows []*AlbArtResp
	downloadedCount := 0

	for _, show := range allShows {
		// Build expected path
		albumFolder := fmt.Sprintf("%s - %s", show.ArtistName, show.ContainerInfo)
		if len(albumFolder) > 120 {
			albumFolder = albumFolder[:120]
		}
		albumFolder = sanitise(albumFolder)
		albumPath := filepath.Join(cfg.OutPath, sanitise(show.ArtistName), albumFolder)

		// Check local existence
		_, err := os.Stat(albumPath)
		localExists := err == nil

		// Check remote if rclone enabled
		remoteExists := false
		if cfg.RcloneEnabled {
			remotePath := filepath.Join(sanitise(show.ArtistName), albumFolder)
			remoteExists, _ = remotePathExists(remotePath, cfg)
		}

		if localExists || remoteExists {
			downloadedCount++
		} else {
			missingShows = append(missingShows, show)
		}
	}

	// Sort missing shows by date (newest first)
	sort.Slice(missingShows, func(i, j int) bool {
		return missingShows[i].PerformanceDate > missingShows[j].PerformanceDate
	})

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
			"artistID":        artistId,
			"artistName":      artistName,
			"totalShows":      len(allShows),
			"downloaded":      downloadedCount,
			"downloadedPct":   float64(downloadedCount) / float64(len(allShows)) * 100,
			"missing":         len(missingShows),
			"missingPct":      float64(len(missingShows)) / float64(len(allShows)) * 100,
			"missingShows":    missingData,
		}
		jsonData, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(jsonData))
	} else {
		printHeader(fmt.Sprintf("Gap Analysis: %s", artistName))

		downloadedPct := float64(downloadedCount) / float64(len(allShows)) * 100
		missingPct := float64(len(missingShows)) / float64(len(allShows)) * 100

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
