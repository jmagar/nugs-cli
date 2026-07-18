package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/jmagar/nugs-cli/internal/model"
)

const artistListCacheFile = "artist-list.json"

const (
	defaultArtistCacheMaxAge   = 30 * 24 * time.Hour
	defaultArtistCacheMaxBytes = int64(256 << 20)
)

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

		if err := atomicWriteFile(cachePath, data); err != nil {
			return err
		}
		cacheDir := filepath.Dir(filepath.Dir(cachePath))
		if err := updateFullCatalogArtistLocked(cacheDir, artistID, pages); err != nil {
			return err
		}
		return pruneArtistMetaCacheLocked(filepath.Dir(cachePath), defaultArtistCacheMaxAge, defaultArtistCacheMaxBytes, cachePath)
	})
}

// PruneArtistMetaCache removes expired entries and then evicts least-recently
// modified entries until the cache is within maxBytes.
func PruneArtistMetaCache(maxAge time.Duration, maxBytes int64) error {
	return WithCacheLock(func() error {
		cacheDir, err := GetCacheDir()
		if err != nil {
			return err
		}
		return pruneArtistMetaCacheLocked(filepath.Join(cacheDir, "artists"), maxAge, maxBytes, "")
	})
}

func pruneArtistMetaCacheLocked(dir string, maxAge time.Duration, maxBytes int64, preserve string) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	type candidate struct {
		path string
		info os.FileInfo
	}
	files := make([]candidate, 0, len(entries))
	var total int64
	now := time.Now()
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, statErr := entry.Info()
		if statErr != nil {
			continue
		}
		if path != preserve && maxAge > 0 && now.Sub(info.ModTime()) > maxAge {
			_ = os.Remove(path)
			continue
		}
		files = append(files, candidate{path: path, info: info})
		total += info.Size()
	}
	sort.Slice(files, func(i, j int) bool { return files[i].info.ModTime().Before(files[j].info.ModTime()) })
	for _, file := range files {
		if maxBytes <= 0 || total <= maxBytes || file.path == preserve {
			continue
		}
		if err := os.Remove(file.path); err == nil {
			total -= file.info.Size()
		}
	}
	return nil
}
