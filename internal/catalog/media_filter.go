package catalog

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/jmagar/nugs-cli/internal/helpers"
	"github.com/jmagar/nugs-cli/internal/model"
	"github.com/jmagar/nugs-cli/internal/ui"
)

const (
	perArtistExistenceCheckConcurrency = 8
	coverageArtistConcurrency          = 6
)

// ShowExistsForMedia checks if a show exists locally or remotely for a specific media type.
func ShowExistsForMedia(ctx context.Context, show *model.AlbArtResp, cfg *model.Config, mediaType model.MediaType, deps *Deps) bool {
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
		exists, err := deps.RemotePathExists(ctx, remotePath, cfg, isVideo)
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
func ShowExistsForMediaIndexed(ctx context.Context, show *model.AlbArtResp, cfg *model.Config, mediaType model.MediaType, idx *ArtistPresenceIndex, deps *Deps) bool {
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

	// Fallback: the bulk remote folder listing failed (e.g., network timeout,
	// invalid remote config, permission error), so the pre-built index is
	// incomplete. We must check each show individually via rclone to avoid
	// false negatives that would cause re-downloading already-uploaded shows.
	if cfg.RcloneEnabled && idx.RemoteListErr != nil && deps.RemotePathExists != nil {
		remotePath := resolver.RemoteShowPath(show)

		// Determine which remote locations to check based on media type.
		// Audio and video may be stored in separate remote base paths
		// (rclonePath vs rcloneVideoPath), so the isVideo flag selects
		// which remote to query. For explicit types, check only the
		// corresponding remote. For "both" or "unknown", check both
		// audio (isVideo=false) then video (isVideo=true) to ensure we
		// find the show regardless of which format was uploaded.
		remoteTargets := []bool{mediaType == model.MediaTypeVideo}
		if mediaType == model.MediaTypeBoth || mediaType == model.MediaTypeUnknown {
			remoteTargets = []bool{false, true} // check audio first, then video
		}

		var lastErr error
		for _, isVideo := range remoteTargets {
			exists, err := deps.RemotePathExists(ctx, remotePath, cfg, isVideo)
			if err != nil {
				WarnRemoteCheckError(err)
				lastErr = err
				continue // try next target; one remote may work even if the other fails
			}
			if exists {
				return true // found in at least one remote location
			}
		}
		_ = lastErr
		return false // not found in any checked remote location
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

// IsShowDownloadable returns true only when metadata indicates content is
// currently downloadable/streamable (audio tracks or a video SKU).
func IsShowDownloadable(show *model.AlbArtResp) bool {
	if show == nil {
		return false
	}
	if show.AvailabilityTypeStr != "" &&
		!strings.EqualFold(show.AvailabilityTypeStr, model.AvailableAvailabilityType) {
		return false
	}
	if len(show.Tracks) > 0 || len(show.Songs) > 0 {
		return true
	}
	if len(show.Products) > 0 {
		return true
	}
	if len(show.ProductFormatList) > 0 {
		return true
	}
	return false
}

// classifyShows iterates all shows, applies media filtering, and populates the analysis
// with show statuses and download counts.
func classifyShows(ctx context.Context, allShows []*model.AlbArtResp, mediaFilter model.MediaType, presenceIdx *ArtistPresenceIndex, cfg *model.Config, deps *Deps, analysis *model.ArtistCatalogAnalysis) {
	type candidate struct {
		show      *model.AlbArtResp
		showMedia model.MediaType
	}

	candidates := make([]candidate, 0, len(allShows))
	for _, show := range allShows {
		if !IsShowDownloadable(show) {
			continue
		}
		var showMedia model.MediaType
		if deps.GetShowMediaType != nil {
			showMedia = deps.GetShowMediaType(show)
		} else {
			showMedia = model.MediaTypeAudio
		}

		if !MatchesMediaFilter(showMedia, mediaFilter) {
			continue
		}
		candidates = append(candidates, candidate{show: show, showMedia: showMedia})
	}

	if len(candidates) == 0 {
		return
	}

	statuses := make([]model.ShowStatus, len(candidates))
	workerCount := min(perArtistExistenceCheckConcurrency, len(candidates))
	jobs := make(chan int)
	var wg sync.WaitGroup

	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				candidateItem := candidates[idx]
				downloaded := ShowExistsForMediaIndexed(ctx, candidateItem.show, cfg, mediaFilter, presenceIdx, deps)
				statuses[idx] = model.ShowStatus{
					Show:       candidateItem.show,
					Downloaded: downloaded,
					MediaType:  candidateItem.showMedia,
				}
			}
		}()
	}

	for i := range candidates {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return
		case jobs <- i:
		}
	}
	close(jobs)
	wg.Wait()

	for _, status := range statuses {
		analysis.Shows = append(analysis.Shows, status)
		if status.Downloaded {
			analysis.Downloaded++
		} else {
			analysis.MissingShows = append(analysis.MissingShows, status)
		}
	}
}

// AnalyzeArtistCatalogMediaAware is the media-aware version of catalog analysis.
func AnalyzeArtistCatalogMediaAware(ctx context.Context, artistID string, cfg *model.Config, jsonLevel string, mediaFilter model.MediaType, deps *Deps) (*model.ArtistCatalogAnalysis, error) {
	if deps.GetArtistMetaCached == nil {
		return nil, fmt.Errorf("GetArtistMetaCached callback not configured")
	}

	artistMetas, cacheUsed, cacheStaleUse, err := deps.GetArtistMetaCached(ctx, artistID, ArtistMetaCacheTTL)
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

	presenceIdx := BuildArtistPresenceIndex(ctx, artistName, cfg, deps, mediaFilter)
	if presenceIdx.RemoteListErr != nil && jsonLevel == "" {
		ui.PrintWarning(fmt.Sprintf("Remote artist folder bulk check failed, falling back to per-show checks: %v", presenceIdx.RemoteListErr))
	}

	analysis := &model.ArtistCatalogAnalysis{
		ArtistID:      artistID,
		ArtistName:    artistName,
		Shows:         make([]model.ShowStatus, 0, len(allShows)),
		MissingShows:  make([]model.ShowStatus, 0, len(allShows)),
		CacheUsed:     cacheUsed,
		CacheStaleUse: cacheStaleUse,
		MediaFilter:   mediaFilter,
	}

	classifyShows(ctx, allShows, mediaFilter, &presenceIdx, cfg, deps, analysis)

	analysis.TotalShows = len(analysis.Shows)
	analysis.Missing = len(analysis.MissingShows)
	if analysis.TotalShows > 0 {
		analysis.DownloadPct = float64(analysis.Downloaded) / float64(analysis.TotalShows) * 100
		analysis.MissingPct = float64(analysis.Missing) / float64(analysis.TotalShows) * 100
	}

	return analysis, nil
}
