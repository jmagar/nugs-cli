package main

// Cache wrappers delegating to internal/cache during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import (
	"time"

	"github.com/jmagar/nugs-cli/internal/cache"
	"github.com/jmagar/nugs-cli/internal/model"
)

func getArtistMetaCachePath(artistID string) (string, error) {
	return cache.GetArtistMetaCachePath(artistID)
}

func readArtistMetaCache(artistID string) ([]*ArtistMeta, time.Time, error) {
	return cache.ReadArtistMetaCache(artistID)
}

func writeArtistMetaCache(artistID string, pages []*ArtistMeta) error {
	return cache.WriteArtistMetaCache(artistID, pages)
}

func getCacheDir() (string, error) {
	return cache.GetCacheDir()
}

func readCacheMeta() (*CacheMeta, error) {
	return cache.ReadCacheMeta()
}

func readCatalogCache() (*LatestCatalogResp, error) {
	return cache.ReadCatalogCache()
}

func writeCatalogCache(catalog *model.LatestCatalogResp, updateDuration time.Duration) error {
	return cache.WriteCatalogCache(catalog, updateDuration, formatDuration)
}
