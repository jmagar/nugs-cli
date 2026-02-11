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
	metaPath := filepath.Join(cacheDir, "catalog_meta.json")

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

		// Write catalog.json atomically using temp file
		catalogPath := filepath.Join(cacheDir, "catalog.json")
		catalogData, err := json.Marshal(catalog)
		if err != nil {
			return fmt.Errorf("failed to marshal catalog: %w", err)
		}

		tmpCatalogPath := catalogPath + ".tmp"
		err = os.WriteFile(tmpCatalogPath, catalogData, 0644)
		if err != nil {
			return fmt.Errorf("failed to write temp catalog: %w", err)
		}

		err = os.Rename(tmpCatalogPath, catalogPath)
		if err != nil {
			_ = os.Remove(tmpCatalogPath)
			return fmt.Errorf("failed to rename catalog: %w", err)
		}

		// Count unique artists
		artistSet := make(map[int]bool)
		for _, item := range catalog.Response.RecentItems {
			artistSet[item.ArtistID] = true
		}

		// Write catalog_meta.json atomically
		meta := model.CacheMeta{
			LastUpdated:    time.Now(),
			CacheVersion:   "v1.0.0",
			TotalShows:     len(catalog.Response.RecentItems),
			TotalArtists:   len(artistSet),
			ApiMethod:      "catalog.latest",
			UpdateDuration: formatDurationFn(updateDuration),
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
			_ = os.Remove(tmpMetaPath)
			return fmt.Errorf("failed to rename metadata: %w", err)
		}

		// Build indexes (also atomic)
		err = BuildArtistIndex(catalog)
		if err != nil {
			return err
		}
		err = BuildContainerIndex(catalog)
		if err != nil {
			return err
		}

		return nil
	})
}

// BuildArtistIndex creates artist name -> ID lookup index.
func BuildArtistIndex(catalog *model.LatestCatalogResp) error {
	cacheDir, err := GetCacheDir()
	if err != nil {
		return err
	}

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

	tmpPath := indexPath + ".tmp"
	err = os.WriteFile(tmpPath, indexData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write temp artist index: %w", err)
	}

	err = os.Rename(tmpPath, indexPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to rename artist index: %w", err)
	}

	return nil
}

// BuildContainerIndex creates containerID -> artist mapping.
func BuildContainerIndex(catalog *model.LatestCatalogResp) error {
	cacheDir, err := GetCacheDir()
	if err != nil {
		return err
	}

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

	tmpPath := indexPath + ".tmp"
	err = os.WriteFile(tmpPath, indexData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write temp container index: %w", err)
	}

	err = os.Rename(tmpPath, indexPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to rename container index: %w", err)
	}

	return nil
}
