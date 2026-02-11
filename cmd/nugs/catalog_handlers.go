package main

// Catalog handler wrappers delegating to internal/catalog during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import (
	"context"

	"github.com/jmagar/nugs-cli/internal/catalog"
)

// buildCatalogDeps wires root-level callbacks into the internal/catalog package.
func buildCatalogDeps() *catalog.Deps {
	return &catalog.Deps{
		RemotePathExists:        remotePathExists,
		ListRemoteArtistFolders: listRemoteArtistFolders,
		Album:                   album,
		Playlist:                playlist,
		SetCurrentProgressBox:   setCurrentProgressBox,
		GetShowMediaType:        getShowMediaType,
		FormatDuration:          formatDuration,
		GetArtistMetaCached:     getArtistMetaCached,
	}
}

func analyzeArtistCatalog(ctx context.Context, artistID string, cfg *Config, jsonLevel string, mediaFilter MediaType) (*ArtistCatalogAnalysis, error) {
	return catalog.AnalyzeArtistCatalog(ctx, artistID, cfg, jsonLevel, mediaFilter, buildCatalogDeps())
}

func catalogUpdate(ctx context.Context, jsonLevel string) error {
	return catalog.CatalogUpdate(ctx, jsonLevel, buildCatalogDeps())
}

func catalogCacheStatus(jsonLevel string) error {
	return catalog.CatalogCacheStatus(jsonLevel, buildCatalogDeps())
}

func catalogStats(jsonLevel string) error {
	return catalog.CatalogStats(jsonLevel)
}

func catalogLatest(limit int, jsonLevel string) error {
	return catalog.CatalogLatest(limit, jsonLevel)
}

func catalogGapsForArtist(ctx context.Context, artistId string, cfg *Config, jsonLevel string, idsOnly bool, mediaFilter MediaType) error {
	return catalog.CatalogGapsForArtist(ctx, artistId, cfg, jsonLevel, idsOnly, mediaFilter, buildCatalogDeps())
}

func catalogGaps(ctx context.Context, artistIds []string, cfg *Config, jsonLevel string, idsOnly bool, mediaFilter MediaType) error {
	return catalog.CatalogGaps(ctx, artistIds, cfg, jsonLevel, idsOnly, mediaFilter, buildCatalogDeps())
}

func catalogGapsFill(ctx context.Context, artistId string, cfg *Config, streamParams *StreamParams, jsonLevel string, mediaFilter MediaType) error {
	return catalog.CatalogGapsFill(ctx, artistId, cfg, streamParams, jsonLevel, mediaFilter, buildCatalogDeps())
}

func catalogCoverage(ctx context.Context, artistIds []string, cfg *Config, jsonLevel string, mediaFilter MediaType) error {
	return catalog.CatalogCoverage(ctx, artistIds, cfg, jsonLevel, mediaFilter, buildCatalogDeps())
}

func catalogList(ctx context.Context, artistIds []string, cfg *Config, jsonLevel string, mediaFilter MediaType) error {
	return catalog.CatalogList(ctx, artistIds, cfg, jsonLevel, mediaFilter, buildCatalogDeps())
}

func catalogListForArtist(ctx context.Context, artistId string, cfg *Config, jsonLevel string, mediaFilter MediaType) error {
	return catalog.CatalogListForArtist(ctx, artistId, cfg, jsonLevel, mediaFilter, buildCatalogDeps())
}
