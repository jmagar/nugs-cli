package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"time"
)

func artist(artistId string, cfg *Config, streamParams *StreamParams) error {
	meta, err := getArtistMeta(artistId)
	if err != nil {
		printError("Failed to get artist metadata")
		return err
	}
	if len(meta) == 0 {
		return errors.New(
			"The API didn't return any artist metadata.")
	}
	fmt.Println(meta[0].Response.Containers[0].ArtistName)
	albumTotal := getAlbumTotal(meta)

	// Create batch state for multi-album progress tracking (Tier 4 enhancement)
	batchState := &BatchProgressState{
		TotalAlbums: albumTotal,
		StartTime:   time.Now(),
	}

	// Create ONE progress box for the entire batch (reused across all albums)
	sharedProgressBox := &ProgressBoxState{
		RcloneEnabled:  cfg.RcloneEnabled,
		BatchState:     batchState,
		StartTime:      time.Now(),
		RenderInterval: defaultProgressRenderInterval,
	}
	setCurrentProgressBox(sharedProgressBox)
	defer setCurrentProgressBox(nil)

	albumCount := 0
	for _, _meta := range meta {
		for _, container := range _meta.Response.Containers {
			if err := waitIfPausedOrCancelled(); err != nil {
				return err
			}

			albumCount++
			batchState.CurrentAlbum = albumCount
			batchState.CurrentTitle = container.ContainerInfo

			// Pass the shared progress box to reuse it (no new boxes created!)
			if cfg.SkipVideos {
				err = album("", cfg, streamParams, container, batchState, sharedProgressBox)
			} else {
				// Can't re-use this metadata as it doesn't have any product info for videos.
				err = album(strconv.Itoa(container.ContainerID), cfg, streamParams, nil, batchState, sharedProgressBox)
			}
			if err != nil {
				batchState.Failed++
				if isCrawlCancelledErr(err) {
					return err
				}
				handleErr("Item failed.", err, false)
			} else {
				batchState.Complete++
			}
		}
	}
	return nil
}

func playlist(plistId, legacyToken string, cfg *Config, streamParams *StreamParams, cat bool) error {
	_meta, err := getPlistMeta(plistId, cfg.Email, legacyToken, cat)
	if err != nil {
		printError("Failed to get playlist metadata")
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
	plistPath := filepath.Join(cfg.OutPath, sanitise(plistName))
	err = makeDirs(plistPath)
	if err != nil {
		printError("Failed to make playlist folder")
		return err
	}
	trackTotal := len(meta.Items)

	// Initialize progress box for this playlist
	progressBox := &ProgressBoxState{
		ShowTitle:      plistName,
		ShowNumber:     "Downloading Playlist",
		RcloneEnabled:  cfg.RcloneEnabled,
		ShowDownloaded: "0 B",
		ShowTotal:      "calculating...",
		RenderInterval: defaultProgressRenderInterval,
	}
	// Tier 3: Register global progress box for crawl control access
	setCurrentProgressBox(progressBox)
	defer setCurrentProgressBox(nil)

	for trackNum, track := range meta.Items {
		if err := waitIfPausedOrCancelled(); err != nil {
			return err
		}
		trackNum++
		err := processTrack(
			plistPath, trackNum, trackTotal, cfg, &track.Track, streamParams, progressBox)
		if err != nil {
			if isCrawlCancelledErr(err) {
				return err
			}
			handleErr("Track failed.", err, false)
		}
	}

	// Upload to rclone if enabled
	if cfg.RcloneEnabled {
		// Playlists don't have artist folder structure
		err = uploadToRclone(plistPath, "", cfg, progressBox, false)
		if err != nil {
			handleErr("Upload failed.", err, false)
		}
	}

	return nil
}

func paidLstream(query, uguID string, cfg *Config, streamParams *StreamParams) error {
	showId, err := parsePaidLstreamShowID(query)
	if err != nil {
		return err
	}
	err = video(showId, uguID, cfg, streamParams, nil, true, nil)
	return err
}
