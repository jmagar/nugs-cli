package main

// Command adapters for cache operations.

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
