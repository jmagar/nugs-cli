package main

// Command adapters for internal video download operations.

import (
	"context"

	"github.com/jmagar/nugs-cli/internal/download"
)

func video(ctx context.Context, videoID, uguID string, cfg *Config, streamParams *StreamParams, meta *AlbArtResp, isLstream bool, progressBox *ProgressBoxState) error {
	return download.Video(ctx, videoID, uguID, cfg, streamParams, meta, isLstream, progressBox, buildDownloadDeps())
}
