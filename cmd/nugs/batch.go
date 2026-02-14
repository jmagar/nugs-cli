package main

// Batch wrappers delegating to internal/download during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import (
	"context"

	"github.com/jmagar/nugs-cli/internal/download"
)

func artist(ctx context.Context, artistID string, cfg *Config, streamParams *StreamParams) error {
	return download.Artist(ctx, artistID, cfg, streamParams, buildDownloadDeps())
}

func playlist(ctx context.Context, plistID, legacyToken string, cfg *Config, streamParams *StreamParams, cat bool) error {
	return download.Playlist(ctx, plistID, legacyToken, cfg, streamParams, cat, buildDownloadDeps())
}

func paidLstream(ctx context.Context, query, uguID string, cfg *Config, streamParams *StreamParams) error {
	return download.PaidLstream(ctx, query, uguID, cfg, streamParams, buildDownloadDeps())
}
