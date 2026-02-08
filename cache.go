package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func getArtistMetaCachePath(artistID string) (string, error) {
	cacheDir, err := getCacheDir()
	if err != nil {
		return "", err
	}
	artistsDir := filepath.Join(cacheDir, "artists")
	if err := os.MkdirAll(artistsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create artist cache directory: %w", err)
	}
	return filepath.Join(artistsDir, fmt.Sprintf("artist_%s.json", artistID)), nil
}

func readArtistMetaCache(artistID string) ([]*ArtistMeta, time.Time, error) {
	cachePath, err := getArtistMetaCachePath(artistID)
	if err != nil {
		return nil, time.Time{}, err
	}
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, time.Time{}, err
	}
	var cached ArtistMetaCache
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to parse artist cache: %w", err)
	}
	return cached.Pages, cached.CachedAt, nil
}

func writeArtistMetaCache(artistID string, pages []*ArtistMeta) error {
	cachePath, err := getArtistMetaCachePath(artistID)
	if err != nil {
		return err
	}

	cached := ArtistMetaCache{
		ArtistID: artistID,
		CachedAt: time.Now(),
		Pages:    pages,
	}
	data, err := json.MarshalIndent(cached, "", "  ")
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

// getCacheDir returns the cache directory path, creating it if needed
func getCacheDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	cacheDir := filepath.Join(homeDir, ".cache", "nugs")
	err = os.MkdirAll(cacheDir, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}
	return cacheDir, nil
}

// readCacheMeta reads the cache metadata file
func readCacheMeta() (*CacheMeta, error) {
	cacheDir, err := getCacheDir()
	if err != nil {
		return nil, err
	}
	metaPath := filepath.Join(cacheDir, "catalog_meta.json")

	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No cache yet
		}
		return nil, fmt.Errorf("failed to read cache metadata: %w", err)
	}

	var meta CacheMeta
	err = json.Unmarshal(data, &meta)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cache metadata: %w", err)
	}
	return &meta, nil
}

// readCatalogCache reads the cached catalog data
func readCatalogCache() (*LatestCatalogResp, error) {
	cacheDir, err := getCacheDir()
	if err != nil {
		return nil, err
	}
	catalogPath := filepath.Join(cacheDir, "catalog.json")

	data, err := os.ReadFile(catalogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no cache found - run 'nugs catalog update' first")
		}
		return nil, fmt.Errorf("failed to read catalog cache: %w", err)
	}

	var catalog LatestCatalogResp
	err = json.Unmarshal(data, &catalog)
	if err != nil {
		return nil, fmt.Errorf("failed to parse catalog cache: %w", err)
	}
	return &catalog, nil
}

// writeCatalogCache writes the catalog and metadata to cache
// Uses file locking to prevent corruption from concurrent writes
func writeCatalogCache(catalog *LatestCatalogResp, updateDuration time.Duration) error {
	// Acquire lock for the entire cache write operation
	return WithCacheLock(func() error {
		cacheDir, err := getCacheDir()
		if err != nil {
			return err
		}

		// Write catalog.json atomically using temp file
		catalogPath := filepath.Join(cacheDir, "catalog.json")
		catalogData, err := json.MarshalIndent(catalog, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal catalog: %w", err)
		}

		// Write to temp file first
		tmpCatalogPath := catalogPath + ".tmp"
		err = os.WriteFile(tmpCatalogPath, catalogData, 0644)
		if err != nil {
			return fmt.Errorf("failed to write temp catalog: %w", err)
		}

		// Atomic rename
		err = os.Rename(tmpCatalogPath, catalogPath)
		if err != nil {
			_ = os.Remove(tmpCatalogPath) // Best effort cleanup
			return fmt.Errorf("failed to rename catalog: %w", err)
		}

		// Count unique artists
		artistSet := make(map[int]bool)
		for _, item := range catalog.Response.RecentItems {
			artistSet[item.ArtistID] = true
		}

		// Write catalog_meta.json atomically
		meta := CacheMeta{
			LastUpdated:    time.Now(),
			CacheVersion:   "v1.0.0",
			TotalShows:     len(catalog.Response.RecentItems),
			TotalArtists:   len(artistSet),
			ApiMethod:      "catalog.latest",
			UpdateDuration: formatDuration(updateDuration),
		}
		metaPath := filepath.Join(cacheDir, "catalog_meta.json")
		metaData, err := json.MarshalIndent(meta, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}

		tmpMetaPath := metaPath + ".tmp"
		err = os.WriteFile(tmpMetaPath, metaData, 0644)
		if err != nil {
			return fmt.Errorf("failed to write temp metadata: %w", err)
		}

		err = os.Rename(tmpMetaPath, metaPath)
		if err != nil {
			_ = os.Remove(tmpMetaPath) // Best effort cleanup
			return fmt.Errorf("failed to rename metadata: %w", err)
		}

		// Build indexes (also atomic)
		err = buildArtistIndex(catalog)
		if err != nil {
			return err
		}
		err = buildContainerIndex(catalog)
		if err != nil {
			return err
		}

		return nil
	})
}

// buildArtistIndex creates artist name → ID lookup index
// Uses atomic write (temp file + rename) to prevent corruption
func buildArtistIndex(catalog *LatestCatalogResp) error {
	cacheDir, err := getCacheDir()
	if err != nil {
		return err
	}

	index := make(map[string]int)
	for _, item := range catalog.Response.RecentItems {
		normalizedName := strings.ToLower(strings.TrimSpace(item.ArtistName))
		index[normalizedName] = item.ArtistID
	}

	artistIndex := ArtistsIndex{Index: index}
	indexPath := filepath.Join(cacheDir, "artists_index.json")
	indexData, err := json.MarshalIndent(artistIndex, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal artist index: %w", err)
	}

	// Atomic write via temp file
	tmpPath := indexPath + ".tmp"
	err = os.WriteFile(tmpPath, indexData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write temp artist index: %w", err)
	}

	err = os.Rename(tmpPath, indexPath)
	if err != nil {
		_ = os.Remove(tmpPath) // Best effort cleanup
		return fmt.Errorf("failed to rename artist index: %w", err)
	}

	return nil
}

// buildContainerIndex creates containerID → artist mapping
// Uses atomic write (temp file + rename) to prevent corruption
func buildContainerIndex(catalog *LatestCatalogResp) error {
	cacheDir, err := getCacheDir()
	if err != nil {
		return err
	}

	containers := make(map[int]ContainerIndexEntry)
	for _, item := range catalog.Response.RecentItems {
		containers[item.ContainerID] = ContainerIndexEntry{
			ArtistID:        item.ArtistID,
			ArtistName:      item.ArtistName,
			ContainerInfo:   item.ContainerInfo,
			PerformanceDate: item.PerformanceDateStr,
		}
	}

	containerIndex := ContainersIndex{Containers: containers}
	indexPath := filepath.Join(cacheDir, "containers_index.json")
	indexData, err := json.MarshalIndent(containerIndex, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal container index: %w", err)
	}

	// Atomic write via temp file
	tmpPath := indexPath + ".tmp"
	err = os.WriteFile(tmpPath, indexData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write temp container index: %w", err)
	}

	err = os.Rename(tmpPath, indexPath)
	if err != nil {
		_ = os.Remove(tmpPath) // Best effort cleanup
		return fmt.Errorf("failed to rename container index: %w", err)
	}

	return nil
}
