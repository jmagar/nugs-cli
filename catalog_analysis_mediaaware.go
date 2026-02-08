package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// showExistsForMedia checks if a show exists locally or remotely for a specific media type.
// Uses getOutPathForMedia() to get the correct local path based on media type.
// Uses getRclonePathForMedia() to get the correct remote path based on media type.
// Returns true if the show exists either locally or remotely for the specified media type.
// This function performs per-show filesystem checks (slower). Use showExistsForMediaIndexed when possible.
func showExistsForMedia(show *AlbArtResp, cfg *Config, mediaType MediaType) bool {
	albumFolder := buildAlbumFolderName(show.ArtistName, show.ContainerInfo)

	// Determine if we're looking for video or audio content
	isVideo := mediaType.HasVideo()

	// Check local path (using media-specific base path)
	localBase := getOutPathForMedia(cfg, mediaType)
	localPath := filepath.Join(localBase, sanitise(show.ArtistName), albumFolder)
	if _, err := os.Stat(localPath); err == nil {
		return true
	}

	// Check remote path (if rclone enabled)
	if cfg.RcloneEnabled {
		remotePath := filepath.Join(sanitise(show.ArtistName), albumFolder)
		// remotePathExists handles the base path internally via isVideo parameter
		exists, err := remotePathExists(remotePath, cfg, isVideo)
		if err != nil {
			warnRemoteCheckError(err)
			return false
		}
		if exists {
			return true
		}
	}

	return false
}

// showExistsForMediaIndexed checks if a show exists using pre-built folder index (fast).
// Falls back to per-show checks if index is unavailable or errored.
// For 430-show artists, this is 430x faster (1 readdir vs 430 stat calls).
func showExistsForMediaIndexed(show *AlbArtResp, cfg *Config, mediaType MediaType, idx *artistPresenceIndex) bool {
	albumFolder := buildAlbumFolderName(show.ArtistName, show.ContainerInfo)

	// Fast path: check local index
	if _, exists := idx.localFolders[albumFolder]; exists {
		return true
	}

	// Fast path: check remote index (if available and no error)
	if cfg.RcloneEnabled && idx.remoteListErr == nil {
		if _, exists := idx.remoteFolders[albumFolder]; exists {
			return true
		}
		return false // Index is valid, not found
	}

	// Fallback: remote index errored or rclone disabled, use slow per-show check
	if cfg.RcloneEnabled && idx.remoteListErr != nil {
		isVideo := mediaType.HasVideo()
		remotePath := filepath.Join(sanitise(show.ArtistName), albumFolder)
		exists, err := remotePathExists(remotePath, cfg, isVideo)
		if err != nil {
			warnRemoteCheckError(err)
			return false
		}
		return exists
	}

	return false
}

// matchesMediaFilter returns true if the show's media type matches the filter.
// If filter is Unknown or Both, all shows match (no filtering).
func matchesMediaFilter(showMedia, filter MediaType) bool {
	if filter == MediaTypeUnknown || filter == MediaTypeBoth {
		return true // No filter or "both" means show everything
	}
	if filter == MediaTypeAudio {
		return showMedia.HasAudio()
	}
	if filter == MediaTypeVideo {
		return showMedia.HasVideo()
	}
	return false
}

// analyzeArtistCatalogMediaAware is the media-aware version of analyzeArtistCatalog.
// It replaces the old version and adds media type filtering capabilities.
func analyzeArtistCatalogMediaAware(artistID string, cfg *Config, jsonLevel string, mediaFilter MediaType) (*ArtistCatalogAnalysis, error) {
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

	// Default to config default outputs if no filter specified
	if mediaFilter == MediaTypeUnknown {
		mediaFilter = ParseMediaType(cfg.DefaultOutputs)
		if mediaFilter == MediaTypeUnknown {
			mediaFilter = MediaTypeBoth // Ultimate fallback
		}
	}

	presenceIdx := buildArtistPresenceIndex(artistName, cfg)
	if presenceIdx.remoteListErr != nil && jsonLevel == "" {
		printWarning(fmt.Sprintf("Remote artist folder bulk check failed, falling back to per-show checks: %v", presenceIdx.remoteListErr))
	}

	analysis := &ArtistCatalogAnalysis{
		ArtistID:      artistID,
		ArtistName:    artistName,
		TotalShows:    0, // Will be calculated after filtering
		Shows:         make([]ShowStatus, 0, len(allShows)),
		MissingShows:  make([]ShowStatus, 0, len(allShows)),
		CacheUsed:     cacheUsed,
		CacheStaleUse: cacheStaleUse,
		MediaFilter:   mediaFilter,
	}

	for _, show := range allShows {
		// Detect show's media type
		showMedia := getShowMediaType(show)

		// Apply media filter
		if !matchesMediaFilter(showMedia, mediaFilter) {
			continue // Skip shows that don't match filter
		}

		// Use fast indexed existence check (430x faster for large catalogs)
		downloaded := showExistsForMediaIndexed(show, cfg, mediaFilter, &presenceIdx)

		status := ShowStatus{
			Show:       show,
			Downloaded: downloaded,
			MediaType:  showMedia,
		}
		analysis.Shows = append(analysis.Shows, status)
		if downloaded {
			analysis.Downloaded++
			continue
		}
		analysis.MissingShows = append(analysis.MissingShows, status)
	}

	analysis.TotalShows = len(analysis.Shows)
	analysis.Missing = len(analysis.MissingShows)
	if analysis.TotalShows > 0 {
		analysis.DownloadPct = float64(analysis.Downloaded) / float64(analysis.TotalShows) * 100
		analysis.MissingPct = float64(analysis.Missing) / float64(analysis.TotalShows) * 100
	}

	return analysis, nil
}
