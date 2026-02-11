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
	meta, err := api.GetArtistMeta(ctx, artistId)
	if err != nil {
		ui.PrintError("Failed to get artist metadata")
		return err
	}
	if len(meta) == 0 {
		return errors.New(
			"The API didn't return any artist metadata.")
	}
	fmt.Println(meta[0].Response.Containers[0].ArtistName)
	albumTotal := GetAlbumTotal(meta)

	// Create batch state for multi-album progress tracking (Tier 4 enhancement)
	batchState := &model.BatchProgressState{
		TotalAlbums: albumTotal,
		StartTime:   time.Now(),
	}

	// Create ONE progress box for the entire batch (reused across all albums)
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

	albumCount := 0
	for _, _meta := range meta {
		for _, container := range _meta.Response.Containers {
			if deps.WaitIfPausedOrCancelled != nil {
				if err := deps.WaitIfPausedOrCancelled(); err != nil {
					return err
				}
			}

			albumCount++
			batchState.CurrentAlbum = albumCount
			batchState.CurrentTitle = container.ContainerInfo

			// Pass the shared progress box to reuse it (no new boxes created!)
			if cfg.SkipVideos {
				err = Album(ctx, "", cfg, streamParams, container, batchState, sharedProgressBox, deps)
			} else {
				// Can't re-use this metadata as it doesn't have any product info for videos.
				err = Album(ctx, strconv.Itoa(container.ContainerID), cfg, streamParams, nil, batchState, sharedProgressBox, deps)
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

	for trackNum, track := range meta.Items {
		if deps.WaitIfPausedOrCancelled != nil {
			if err := deps.WaitIfPausedOrCancelled(); err != nil {
				return err
			}
		}
		trackNum++
		err := ProcessTrack(ctx,
			plistPath, trackNum, trackTotal, cfg, &track.Track, streamParams, progressBox, deps)
		if err != nil {
			if deps.IsCrawlCancelledErr != nil && deps.IsCrawlCancelledErr(err) {
				return err
			}
			helpers.HandleErr("Track failed.", err, false)
		}
	}

	// Upload to rclone if enabled
	if cfg.RcloneEnabled && deps.UploadToRclone != nil {
		// Playlists don't have artist folder structure
		err = deps.UploadToRclone(plistPath, "", cfg, progressBox, false)
		if err != nil {
			helpers.HandleErr("Upload failed.", err, false)
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
	err = Video(ctx, showId, uguID, cfg, streamParams, nil, true, nil, deps)
	return err
}
