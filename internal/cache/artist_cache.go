package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jmagar/nugs-cli/internal/model"
)

// GetArtistMetaCachePath returns the path for an artist's cached metadata.
func GetArtistMetaCachePath(artistID string) (string, error) {
	cacheDir, err := GetCacheDir()
	if err != nil {
		return "", err
	}
	artistsDir := filepath.Join(cacheDir, "artists")
	if err := os.MkdirAll(artistsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create artist cache directory: %w", err)
	}
	return filepath.Join(artistsDir, fmt.Sprintf("artist_%s.json", artistID)), nil
}

// ReadArtistMetaCache reads cached artist metadata pages.
func ReadArtistMetaCache(artistID string) ([]*model.ArtistMeta, time.Time, error) {
	cachePath, err := GetArtistMetaCachePath(artistID)
	if err != nil {
		return nil, time.Time{}, err
	}
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, time.Time{}, err
	}
	var cached model.ArtistMetaCache
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to parse artist cache: %w", err)
	}
	return cached.Pages, cached.CachedAt, nil
}

// WriteArtistMetaCache writes artist metadata pages to cache.
func WriteArtistMetaCache(artistID string, pages []*model.ArtistMeta) error {
	cachePath, err := GetArtistMetaCachePath(artistID)
	if err != nil {
		return err
	}

	cached := model.ArtistMetaCache{
		ArtistID: artistID,
		CachedAt: time.Now(),
		Pages:    pages,
	}
	data, err := json.Marshal(cached)
	if err != nil {
		return fmt.Errorf("failed to marshal artist cache: %w", err)
	}

	tmpPath := cachePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp artist cache: %w", err)
	}
	if err := os.Rename(tmpPath, cachePath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to rename artist cache: %w", err)
	}
	return nil
}
