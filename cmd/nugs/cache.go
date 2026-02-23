package main

// Cache wrappers delegating to internal/cache during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import (
	"time"

	"github.com/jmagar/nugs-cli/internal/cache"
)

func readArtistMetaCache(artistID string) ([]*ArtistMeta, time.Time, error) {
	return cache.ReadArtistMetaCache(artistID)
}

func writeArtistMetaCache(artistID string, pages []*ArtistMeta) error {
	return cache.WriteArtistMetaCache(artistID, pages)
}
