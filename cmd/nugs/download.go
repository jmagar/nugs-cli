package main

// Download wrappers delegating to internal/download during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import (
	"context"

	"github.com/jmagar/nugs-cli/internal/download"
	"github.com/jmagar/nugs-cli/internal/model"
)

// buildDownloadDeps creates the Deps struct wiring root-package callbacks
// into the internal/download package.
func buildDownloadDeps() *download.Deps {
	return &download.Deps{
		WaitIfPausedOrCancelled: waitIfPausedOrCancelled,
		IsCrawlCancelledErr:     isCrawlCancelledErr,
		SetCurrentProgressBox:   setCurrentProgressBox,
		RenderProgressBox:       renderProgressBox,
		RenderCompletionSummary: renderCompletionSummary,
		UploadToRclone:          uploadToRclone,
		RemotePathExists:        remotePathExists,
		PrintProgress:           printProgress,
		UpdateSpeedHistory:      updateSpeedHistory,
		CalculateETA:            calculateETA,
	}
}

func album(ctx context.Context, albumID string, cfg *Config, streamParams *StreamParams, artResp *AlbArtResp, batchState *BatchProgressState, progressBox *ProgressBoxState) error {
	return download.Album(ctx, albumID, cfg, streamParams, artResp, batchState, progressBox, buildDownloadDeps())
}

func processTrack(ctx context.Context, folPath string, trackNum, trackTotal int, cfg *Config, track *Track, streamParams *StreamParams, progressBox *ProgressBoxState) error {
	return download.ProcessTrack(ctx, folPath, trackNum, trackTotal, cfg, track, streamParams, progressBox, buildDownloadDeps())
}

func preCalculateShowSize(tracks []Track, streamParams *StreamParams, cfg *Config) (int64, error) {
	return download.PreCalculateShowSize(tracks, streamParams, cfg)
}

func getAlbumTotal(meta []*ArtistMeta) int {
	return download.GetAlbumTotal(meta)
}

func getVideoSku(products []Product) int {
	return download.GetVideoSku(products)
}

func getShowMediaType(show *AlbArtResp) model.MediaType {
	return download.GetShowMediaType(show)
}
