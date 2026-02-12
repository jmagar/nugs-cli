package catalog

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/jmagar/nugs-cli/internal/helpers"
	"github.com/jmagar/nugs-cli/internal/model"
	"github.com/jmagar/nugs-cli/internal/ui"
)

// ShowExistsForMedia checks if a show exists locally or remotely for a specific media type.
func ShowExistsForMedia(show *model.AlbArtResp, cfg *model.Config, mediaType model.MediaType, deps *Deps) bool {
	resolver := helpers.NewConfigPathResolver(cfg)

	// Determine if we're looking for video or audio content
	isVideo := mediaType.HasVideo()

	// Check local path (using media-specific base path)
	localPath := resolver.LocalShowPath(show, mediaType)
	if _, err := os.Stat(localPath); err == nil {
		return true
	}

	// Check remote path (if rclone enabled)
	if cfg.RcloneEnabled && deps.RemotePathExists != nil {
		remotePath := resolver.RemoteShowPath(show)
		exists, err := deps.RemotePathExists(remotePath, cfg, isVideo)
		if err != nil {
			WarnRemoteCheckError(err)
			return false
		}
		if exists {
			return true
		}
	}

	return false
}

// ShowExistsForMediaIndexed checks if a show exists using pre-built folder index (fast).
func ShowExistsForMediaIndexed(show *model.AlbArtResp, cfg *model.Config, mediaType model.MediaType, idx *ArtistPresenceIndex, deps *Deps) bool {
	albumFolder := helpers.BuildAlbumFolderName(show.ArtistName, show.ContainerInfo)
	resolver := helpers.NewConfigPathResolver(cfg)

	// Fast path: check local index
	if _, exists := idx.LocalFolders[albumFolder]; exists {
		return true
	}

	// Fast path: check remote index (if available and no error)
	if cfg.RcloneEnabled && idx.RemoteListErr == nil {
		if _, exists := idx.RemoteFolders[albumFolder]; exists {
			return true
		}
		return false // Index is valid, not found
	}

	// Fallback: remote index errored or rclone disabled, use slow per-show check
	if cfg.RcloneEnabled && idx.RemoteListErr != nil && deps.RemotePathExists != nil {
		isVideo := mediaType.HasVideo()
		remotePath := resolver.RemoteShowPath(show)
		exists, err := deps.RemotePathExists(remotePath, cfg, isVideo)
		if err != nil {
			WarnRemoteCheckError(err)
			return false
		}
		return exists
	}

	return false
}

// MatchesMediaFilter returns true if the show's media type matches the filter.
// If filter is Unknown or Both, all shows match (no filtering).
func MatchesMediaFilter(showMedia, filter model.MediaType) bool {
	if filter == model.MediaTypeUnknown || filter == model.MediaTypeBoth {
		return true
	}
	if filter == model.MediaTypeAudio {
		return showMedia.HasAudio()
	}
	if filter == model.MediaTypeVideo {
		return showMedia.HasVideo()
	}
	return false
}

// AnalyzeArtistCatalogMediaAware is the media-aware version of catalog analysis.
func AnalyzeArtistCatalogMediaAware(ctx context.Context, artistID string, cfg *model.Config, jsonLevel string, mediaFilter model.MediaType, deps *Deps) (*model.ArtistCatalogAnalysis, error) {
	if deps.GetArtistMetaCached == nil {
		return nil, fmt.Errorf("GetArtistMetaCached callback not configured")
	}

	const artistMetaCacheTTL = ArtistMetaCacheTTL
	artistMetas, cacheUsed, cacheStaleUse, err := deps.GetArtistMetaCached(ctx, artistID, artistMetaCacheTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to get artist metadata: %w", err)
	}

	if len(artistMetas) == 0 {
		return nil, fmt.Errorf("no shows found for artist %s", artistID)
	}

	allShows, artistName := CollectArtistShows(artistMetas)
	if len(allShows) == 0 {
		return nil, fmt.Errorf("no shows found for artist %s", artistID)
	}

	sort.Slice(allShows, func(i, j int) bool {
		return allShows[i].PerformanceDate > allShows[j].PerformanceDate
	})

	// Default to config default outputs if no filter specified
	if mediaFilter == model.MediaTypeUnknown {
		mediaFilter = model.ParseMediaType(cfg.DefaultOutputs)
		if mediaFilter == model.MediaTypeUnknown {
			mediaFilter = model.MediaTypeBoth // Ultimate fallback
		}
	}

	presenceIdx := BuildArtistPresenceIndex(artistName, cfg, deps, mediaFilter)
	if presenceIdx.RemoteListErr != nil && jsonLevel == "" {
		ui.PrintWarning(fmt.Sprintf("Remote artist folder bulk check failed, falling back to per-show checks: %v", presenceIdx.RemoteListErr))
	}

	analysis := &model.ArtistCatalogAnalysis{
		ArtistID:      artistID,
		ArtistName:    artistName,
		TotalShows:    0, // Will be calculated after filtering
		Shows:         make([]model.ShowStatus, 0, len(allShows)),
		MissingShows:  make([]model.ShowStatus, 0, len(allShows)),
		CacheUsed:     cacheUsed,
		CacheStaleUse: cacheStaleUse,
		MediaFilter:   mediaFilter,
	}

	for _, show := range allShows {
		// Detect show's media type
		var showMedia model.MediaType
		if deps.GetShowMediaType != nil {
			showMedia = deps.GetShowMediaType(show)
		} else {
			showMedia = model.MediaTypeAudio // fallback
		}

		// Apply media filter
		if !MatchesMediaFilter(showMedia, mediaFilter) {
			continue
		}

		// Use fast indexed existence check
		downloaded := ShowExistsForMediaIndexed(show, cfg, mediaFilter, &presenceIdx, deps)

		status := model.ShowStatus{
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
