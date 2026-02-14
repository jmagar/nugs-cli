package download

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jmagar/nugs-cli/internal/api"
	"github.com/jmagar/nugs-cli/internal/helpers"
	"github.com/jmagar/nugs-cli/internal/model"
	"github.com/jmagar/nugs-cli/internal/ui"
)

// Artist downloads all albums for an artist.
func Artist(ctx context.Context, artistId string, cfg *model.Config, streamParams *model.StreamParams, deps *Deps) error {
	// Use availType=2 to get complete catalog (both audio and video shows)
	meta, err := api.GetArtistMetaWithAvailType(ctx, artistId, 2)
	if err != nil {
		ui.PrintError("Failed to get artist metadata")
		return err
	}
	if len(meta) == 0 {
		return errors.New("The API didn't return any artist metadata.")
	}
	if len(meta[0].Response.Containers) == 0 {
		return errors.New("The API didn't return any containers for this artist.")
	}
	fmt.Println(meta[0].Response.Containers[0].ArtistName)

	batchState, progressBox := prepareBatchProgress(meta, cfg, deps)
	defer func() {
		if deps.SetCurrentProgressBox != nil {
			deps.SetCurrentProgressBox(nil)
		}
	}()

	return processArtistAlbums(ctx, meta, cfg, streamParams, batchState, progressBox, deps)
}

// prepareBatchProgress initialises the shared batch and progress state for an artist download.
func prepareBatchProgress(meta []*model.ArtistMeta, cfg *model.Config, deps *Deps) (*model.BatchProgressState, *model.ProgressBoxState) {
	albumTotal := GetAlbumTotal(meta)
	batchState := &model.BatchProgressState{
		TotalAlbums: albumTotal,
		StartTime:   time.Now(),
	}
	progressBox := &model.ProgressBoxState{
		RcloneEnabled:  cfg.RcloneEnabled,
		BatchState:     batchState,
		StartTime:      time.Now(),
		RenderInterval: model.DefaultProgressRenderInterval,
	}
	if deps.SetCurrentProgressBox != nil {
		deps.SetCurrentProgressBox(progressBox)
	}
	return batchState, progressBox
}

// processArtistAlbums iterates over all containers in the artist metadata, downloading each album.
func processArtistAlbums(ctx context.Context, meta []*model.ArtistMeta, cfg *model.Config, streamParams *model.StreamParams, batchState *model.BatchProgressState, progressBox *model.ProgressBoxState, deps *Deps) error {
	albumCount := 0
	for _, m := range meta {
		for _, container := range m.Response.Containers {
			if deps.WaitIfPausedOrCancelled != nil {
				if err := deps.WaitIfPausedOrCancelled(); err != nil {
					return err
				}
			}

			albumCount++
			batchState.CurrentAlbum = albumCount
			batchState.CurrentTitle = container.ContainerInfo

			var err error
			if cfg.SkipVideos {
				err = Album(ctx, "", cfg, streamParams, container, batchState, progressBox, deps)
			} else {
				// Can't re-use this metadata as it doesn't have any product info for videos.
				err = Album(ctx, strconv.Itoa(container.ContainerID), cfg, streamParams, nil, batchState, progressBox, deps)
			}
			if err != nil {
				batchState.Failed++
				if deps.IsCrawlCancelledErr != nil && deps.IsCrawlCancelledErr(err) {
					return err
				}
				helpers.HandleErr("Item failed.", err, false)
			} else {
				batchState.Complete++
			}
		}
	}
	return nil
}

// Playlist downloads all tracks in a playlist.
func Playlist(ctx context.Context, plistId, legacyToken string, cfg *model.Config, streamParams *model.StreamParams, cat bool, deps *Deps) error {
	_meta, err := api.GetPlistMeta(ctx, plistId, cfg.Email, legacyToken, cat)
	if err != nil {
		ui.PrintError("Failed to get playlist metadata")
		return err
	}
	meta := _meta.Response
	plistName := meta.PlayListName
	fmt.Println(plistName)
	if len(plistName) > 120 {
		plistName = plistName[:120]
		fmt.Println(
			"Playlist folder name was chopped because it exceeds 120 characters.")
	}
	plistPath := filepath.Join(cfg.OutPath, helpers.Sanitise(plistName))
	err = helpers.MakeDirs(plistPath)
	if err != nil {
		ui.PrintError("Failed to make playlist folder")
		return err
	}
	trackTotal := len(meta.Items)

	// Initialize progress box for this playlist
	progressBox := &model.ProgressBoxState{
		ShowTitle:      plistName,
		ShowNumber:     "Downloading Playlist",
		RcloneEnabled:  cfg.RcloneEnabled,
		ShowDownloaded: "0 B",
		ShowTotal:      "calculating...",
		RenderInterval: model.DefaultProgressRenderInterval,
	}
	// Tier 3: Register global progress box for crawl control access
	if deps.SetCurrentProgressBox != nil {
		deps.SetCurrentProgressBox(progressBox)
	}
	defer func() {
		if deps.SetCurrentProgressBox != nil {
			deps.SetCurrentProgressBox(nil)
		}
	}()

	tracks := make([]*model.Track, trackTotal)
	for i := range meta.Items {
		tracks[i] = &meta.Items[i].Track
	}
	if err := processPlaylistTracks(ctx, tracks, plistPath, cfg, streamParams, progressBox, deps); err != nil {
		return err
	}

	// Upload to rclone if enabled
	if cfg.RcloneEnabled {
		// Playlists don't have artist folder structure
		err = deps.UploadPath(ctx, plistPath, "", cfg, progressBox, false)
		if err != nil {
			helpers.HandleErr("Upload failed.", err, false)
		}
	}

	return nil
}

// processPlaylistTracks iterates over playlist tracks, downloading each with pause/cancel support.
func processPlaylistTracks(ctx context.Context, tracks []*model.Track, plistPath string, cfg *model.Config, streamParams *model.StreamParams, progressBox *model.ProgressBoxState, deps *Deps) error {
	trackTotal := len(tracks)
	for trackNum, track := range tracks {
		if deps.WaitIfPausedOrCancelled != nil {
			if err := deps.WaitIfPausedOrCancelled(); err != nil {
				return err
			}
		}
		trackNum++
		if err := ProcessTrack(ctx, plistPath, trackNum, trackTotal, cfg, track, streamParams, progressBox, deps); err != nil {
			if deps.IsCrawlCancelledErr != nil && deps.IsCrawlCancelledErr(err) {
				return err
			}
			helpers.HandleErr("Track failed.", err, false)
		}
	}
	return nil
}

// PaidLstream downloads a paid livestream video.
func PaidLstream(ctx context.Context, query, uguID string, cfg *model.Config, streamParams *model.StreamParams, deps *Deps) error {
	showId, err := api.ParsePaidLstreamShowID(query)
	if err != nil {
		return err
	}
	return Video(ctx, showId, uguID, cfg, streamParams, nil, true, nil, deps)
}
