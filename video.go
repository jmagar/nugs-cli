package main

// Video wrappers delegating to internal/download during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import "github.com/jmagar/nugs-cli/internal/download"

func video(videoID, uguID string, cfg *Config, streamParams *StreamParams, _meta *AlbArtResp, isLstream bool, progressBox *ProgressBoxState) error {
	return download.Video(videoID, uguID, cfg, streamParams, _meta, isLstream, progressBox, buildDownloadDeps())
}
