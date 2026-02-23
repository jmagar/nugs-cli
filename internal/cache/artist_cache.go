package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jmagar/nugs-cli/internal/model"
)

const artistListCacheFile = "artist-list.json"

// ReadArtistListCache reads the cached catalog.artists response.
// Returns the response, the time it was cached, and any error.
// A missing cache file returns os.ErrNotExist.
func ReadArtistListCache() (*model.ArtistListResp, time.Time, error) {
	cacheDir, err := GetCacheDir()
	if err != nil {
		return nil, time.Time{}, err
	}
	data, err := os.ReadFile(filepath.Join(cacheDir, artistListCacheFile))
	if err != nil {
		return nil, time.Time{}, err
	}
	var cached model.ArtistListCache
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to parse artist list cache: %w", err)
	}
	return cached.Resp, cached.CachedAt, nil
}

// WriteArtistListCache atomically writes the catalog.artists response to cache.
func WriteArtistListCache(resp *model.ArtistListResp) error {
	cacheDir, err := GetCacheDir()
	if err != nil {
		return err
	}
	cached := model.ArtistListCache{
		CachedAt: time.Now(),
		Resp:     resp,
	}
	data, err := json.Marshal(cached)
	if err != nil {
		return fmt.Errorf("failed to marshal artist list cache: %w", err)
	}
	return atomicWriteFile(filepath.Join(cacheDir, artistListCacheFile), data)
}

// GetArtistMetaCachePath returns the path for an artist's cached metadata.
// Validates artistID to prevent directory traversal attacks.
func GetArtistMetaCachePath(artistID string) (string, error) {
	safeID := filepath.Base(artistID)
	if safeID != artistID || safeID == "." || safeID == ".." || safeID == "" {
		return "", fmt.Errorf("invalid artist ID: %q", artistID)
	}

	cacheDir, err := GetCacheDir()
	if err != nil {
		return "", err
	}
	artistsDir := filepath.Join(cacheDir, "artists")
	if err := os.MkdirAll(artistsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create artist cache directory: %w", err)
	}
	return filepath.Join(artistsDir, fmt.Sprintf("artist_%s.json", safeID)), nil
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
// Uses file locking and unique temp files for concurrent safety.
func WriteArtistMetaCache(artistID string, pages []*model.ArtistMeta) error {
	return WithCacheLock(func() error {
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

		return atomicWriteFile(cachePath, data)
	})
}
