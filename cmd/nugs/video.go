package main

// Video wrappers delegating to internal/download during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import (
	"context"

	"github.com/jmagar/nugs-cli/internal/download"
)

func video(ctx context.Context, videoID, uguID string, cfg *Config, streamParams *StreamParams, meta *AlbArtResp, isLstream bool, progressBox *ProgressBoxState) error {
	return download.Video(ctx, videoID, uguID, cfg, streamParams, meta, isLstream, progressBox, buildDownloadDeps())
}
