package main

// Batch wrappers delegating to internal/download during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import "github.com/jmagar/nugs-cli/internal/download"

func artist(artistId string, cfg *Config, streamParams *StreamParams) error {
	return download.Artist(artistId, cfg, streamParams, buildDownloadDeps())
}

func playlist(plistId, legacyToken string, cfg *Config, streamParams *StreamParams, cat bool) error {
	return download.Playlist(plistId, legacyToken, cfg, streamParams, cat, buildDownloadDeps())
}

func paidLstream(query, uguID string, cfg *Config, streamParams *StreamParams) error {
	return download.PaidLstream(query, uguID, cfg, streamParams, buildDownloadDeps())
}
