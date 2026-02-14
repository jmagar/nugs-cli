package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jmagar/nugs-cli/internal/model"
)

// GetCacheDir returns the cache directory path, creating it if needed.
func GetCacheDir() (string, error) {
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

// ReadCacheMeta reads the cache metadata file.
func ReadCacheMeta() (*model.CacheMeta, error) {
	cacheDir, err := GetCacheDir()
	if err != nil {
		return nil, err
	}
	metaPath := filepath.Join(cacheDir, "catalog-meta.json")

	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No cache yet
		}
		return nil, fmt.Errorf("failed to read cache metadata: %w", err)
	}

	var meta model.CacheMeta
	err = json.Unmarshal(data, &meta)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cache metadata: %w", err)
	}
	return &meta, nil
}

// ReadCatalogCache reads the cached catalog data.
func ReadCatalogCache() (*model.LatestCatalogResp, error) {
	cacheDir, err := GetCacheDir()
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

	var catalog model.LatestCatalogResp
	err = json.Unmarshal(data, &catalog)
	if err != nil {
		return nil, fmt.Errorf("failed to parse catalog cache: %w", err)
	}
	return &catalog, nil
}

// WriteCatalogCache writes the catalog and metadata to cache.
// Uses file locking to prevent corruption from concurrent writes.
// The formatDurationFn parameter formats the update duration for metadata.
func WriteCatalogCache(catalog *model.LatestCatalogResp, updateDuration time.Duration, formatDurationFn func(time.Duration) string) error {
	return WithCacheLock(func() error {
		cacheDir, err := GetCacheDir()
		if err != nil {
			return err
		}

		if err := writeCatalogJSON(cacheDir, catalog); err != nil {
			return err
		}

		if err := writeCatalogMeta(cacheDir, catalog, updateDuration, formatDurationFn); err != nil {
			return err
		}

		if err := buildArtistIndex(cacheDir, catalog); err != nil {
			return err
		}
		return buildContainerIndex(cacheDir, catalog)
	})
}

// writeCatalogJSON writes catalog.json atomically using a temp file.
func writeCatalogJSON(cacheDir string, catalog *model.LatestCatalogResp) error {
	catalogPath := filepath.Join(cacheDir, "catalog.json")
	catalogData, err := json.Marshal(catalog)
	if err != nil {
		return fmt.Errorf("failed to marshal catalog: %w", err)
	}
	return atomicWriteFile(catalogPath, catalogData)
}

// writeCatalogMeta writes catalog-meta.json atomically.
func writeCatalogMeta(cacheDir string, catalog *model.LatestCatalogResp, updateDuration time.Duration, formatDurationFn func(time.Duration) string) error {
	artistSet := make(map[int]bool)
	for _, item := range catalog.Response.RecentItems {
		artistSet[item.ArtistID] = true
	}

	meta := model.CacheMeta{
		LastUpdated:    time.Now(),
		CacheVersion:   "v1.0.0",
		TotalShows:     len(catalog.Response.RecentItems),
		TotalArtists:   len(artistSet),
		APIMethod:      "catalog.latest",
		UpdateDuration: formatDurationFn(updateDuration),
	}
	metaPath := filepath.Join(cacheDir, "catalog-meta.json")
	metaData, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	return atomicWriteFile(metaPath, metaData)
}

// atomicWriteFile writes data to a temp file then atomically renames it.
func atomicWriteFile(targetPath string, data []byte) error {
	tmpFile, err := os.CreateTemp(filepath.Dir(targetPath), "nugs_cache_*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}
	return nil
}

// buildArtistIndex creates artist name -> ID lookup index (artists_index.json).
// Must be called within WithCacheLock.
func buildArtistIndex(cacheDir string, catalog *model.LatestCatalogResp) error {
	index := make(map[string]int)
	for _, item := range catalog.Response.RecentItems {
		normalizedName := strings.ToLower(strings.TrimSpace(item.ArtistName))
		index[normalizedName] = item.ArtistID
	}

	artistIndex := model.ArtistsIndex{Index: index}
	indexPath := filepath.Join(cacheDir, "artists_index.json")
	indexData, err := json.Marshal(artistIndex)
	if err != nil {
		return fmt.Errorf("failed to marshal artist index: %w", err)
	}
	return atomicWriteFile(indexPath, indexData)
}

// buildContainerIndex creates containerID -> artist mapping (containers_index.json).
// Must be called within WithCacheLock.
func buildContainerIndex(cacheDir string, catalog *model.LatestCatalogResp) error {
	containers := make(map[int]model.ContainerIndexEntry)
	for _, item := range catalog.Response.RecentItems {
		containers[item.ContainerID] = model.ContainerIndexEntry{
			ArtistID:        item.ArtistID,
			ArtistName:      item.ArtistName,
			ContainerInfo:   item.ContainerInfo,
			PerformanceDate: item.PerformanceDateStr,
		}
	}

	containerIndex := model.ContainersIndex{Containers: containers}
	indexPath := filepath.Join(cacheDir, "containers_index.json")
	indexData, err := json.Marshal(containerIndex)
	if err != nil {
		return fmt.Errorf("failed to marshal container index: %w", err)
	}
	return atomicWriteFile(indexPath, indexData)
}
