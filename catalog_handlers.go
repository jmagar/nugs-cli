package main

// Catalog handler wrappers delegating to internal/catalog during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import "github.com/jmagar/nugs-cli/internal/catalog"

// buildCatalogDeps wires root-level callbacks into the internal/catalog package.
func buildCatalogDeps() *catalog.Deps {
	return &catalog.Deps{
		RemotePathExists:       remotePathExists,
		ListRemoteArtistFolders: listRemoteArtistFolders,
		Album:                  album,
		Playlist:               playlist,
		SetCurrentProgressBox:  setCurrentProgressBox,
		GetShowMediaType:       getShowMediaType,
		FormatDuration:         formatDuration,
		GetArtistMetaCached:    getArtistMetaCached,
	}
}

func analyzeArtistCatalog(artistID string, cfg *Config, jsonLevel string, mediaFilter MediaType) (*ArtistCatalogAnalysis, error) {
	return catalog.AnalyzeArtistCatalog(artistID, cfg, jsonLevel, mediaFilter, buildCatalogDeps())
}

func catalogUpdate(jsonLevel string) error {
	return catalog.CatalogUpdate(jsonLevel, buildCatalogDeps())
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

func catalogGapsForArtist(artistId string, cfg *Config, jsonLevel string, idsOnly bool, mediaFilter MediaType) error {
	return catalog.CatalogGapsForArtist(artistId, cfg, jsonLevel, idsOnly, mediaFilter, buildCatalogDeps())
}

func catalogGaps(artistIds []string, cfg *Config, jsonLevel string, idsOnly bool, mediaFilter MediaType) error {
	return catalog.CatalogGaps(artistIds, cfg, jsonLevel, idsOnly, mediaFilter, buildCatalogDeps())
}

func catalogGapsFill(artistId string, cfg *Config, streamParams *StreamParams, jsonLevel string, mediaFilter MediaType) error {
	return catalog.CatalogGapsFill(artistId, cfg, streamParams, jsonLevel, mediaFilter, buildCatalogDeps())
}

func catalogCoverage(artistIds []string, cfg *Config, jsonLevel string, mediaFilter MediaType) error {
	return catalog.CatalogCoverage(artistIds, cfg, jsonLevel, mediaFilter, buildCatalogDeps())
}

func catalogList(artistIds []string, cfg *Config, jsonLevel string, mediaFilter MediaType) error {
	return catalog.CatalogList(artistIds, cfg, jsonLevel, mediaFilter, buildCatalogDeps())
}

func catalogListForArtist(artistId string, cfg *Config, jsonLevel string, mediaFilter MediaType) error {
	return catalog.CatalogListForArtist(artistId, cfg, jsonLevel, mediaFilter, buildCatalogDeps())
}
