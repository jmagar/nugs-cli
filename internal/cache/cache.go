package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jmagar/nugs-cli/internal/model"
)

const (
	catalogManifestFile  = "catalog-current.json"
	catalogGenerationDir = "catalog-generations"
)

type catalogManifest struct {
	Generation string `json:"generation"`
}

func catalogArtifactPath(cacheDir, name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(cacheDir, catalogManifestFile))
	if err != nil {
		if os.IsNotExist(err) {
			return filepath.Join(cacheDir, name), nil
		}
		return "", fmt.Errorf("failed to read catalog manifest: %w", err)
	}
	var manifest catalogManifest
	if err := json.Unmarshal(data, &manifest); err != nil || filepath.Base(manifest.Generation) != manifest.Generation || manifest.Generation == "" {
		return "", fmt.Errorf("invalid catalog manifest")
	}
	return filepath.Join(cacheDir, catalogGenerationDir, manifest.Generation, name), nil
}

// GetCacheDir returns the cache directory path, creating it if needed.
func GetCacheDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	cacheDir := filepath.Join(homeDir, ".cache", "nugs")
	err = os.MkdirAll(cacheDir, 0700)
	if err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}
	if err := os.Chmod(cacheDir, 0700); err != nil {
		return "", fmt.Errorf("failed to secure cache directory: %w", err)
	}
	return cacheDir, nil
}

// ReadCacheMeta reads the cache metadata file.
func ReadCacheMeta() (*model.CacheMeta, error) {
	cacheDir, err := GetCacheDir()
	if err != nil {
		return nil, err
	}
	metaPath, err := catalogArtifactPath(cacheDir, "catalog-meta.json")
	if err != nil {
		return nil, err
	}

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
	catalogPath, err := catalogArtifactPath(cacheDir, "catalog.json")
	if err != nil {
		return nil, err
	}

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
	cacheDir, err := GetCacheDir()
	if err != nil {
		return err
	}

	prepared, err := prepareCatalogCacheData(catalog, updateDuration, formatDurationFn)
	if err != nil {
		return err
	}

	return WithCacheLock(func() error {
		return writePreparedCatalogCache(cacheDir, prepared)
	})
}

type preparedCatalogCacheData struct {
	catalogData   []byte
	metaData      []byte
	artistIdxData []byte
	containerData []byte
}

func prepareCatalogCacheData(catalog *model.LatestCatalogResp, updateDuration time.Duration, formatDurationFn func(time.Duration) string) (*preparedCatalogCacheData, error) {
	catalogData, err := json.Marshal(catalog)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal catalog: %w", err)
	}

	artistSet := make(map[int]bool, len(catalog.Response.RecentItems))
	artistIndex := make(map[string]int, len(catalog.Response.RecentItems))
	containers := make(map[int]model.ContainerIndexEntry, len(catalog.Response.RecentItems))
	for _, item := range catalog.Response.RecentItems {
		artistSet[item.ArtistID] = true
		artistIndex[strings.ToLower(strings.TrimSpace(item.ArtistName))] = item.ArtistID
		containers[item.ContainerID] = model.ContainerIndexEntry{
			ArtistID:        item.ArtistID,
			ArtistName:      item.ArtistName,
			ContainerInfo:   item.ContainerInfo,
			PerformanceDate: item.PerformanceDateStr,
		}
	}

	meta := model.CacheMeta{
		LastUpdated:    time.Now(),
		CacheVersion:   "v1.0.0",
		TotalShows:     len(catalog.Response.RecentItems),
		TotalArtists:   len(artistSet),
		APIMethod:      "catalog.latest",
		UpdateDuration: formatDurationFn(updateDuration),
	}

	metaData, err := json.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}
	artistIdxData, err := json.Marshal(model.ArtistsIndex{Index: artistIndex})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal artist index: %w", err)
	}
	containerData, err := json.Marshal(model.ContainersIndex{Containers: containers})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal container index: %w", err)
	}

	return &preparedCatalogCacheData{
		catalogData:   catalogData,
		metaData:      metaData,
		artistIdxData: artistIdxData,
		containerData: containerData,
	}, nil
}

func writePreparedCatalogCache(cacheDir string, prepared *preparedCatalogCacheData) error {
	return writePreparedCatalogCacheWithFailpoint(cacheDir, prepared, -1)
}

// writePreparedCatalogCacheWithFailpoint exists so tests can prove that a
// failure before the manifest swap never exposes a mixed generation.
func writePreparedCatalogCacheWithFailpoint(cacheDir string, prepared *preparedCatalogCacheData, failAfter int) error {
	root := filepath.Join(cacheDir, catalogGenerationDir)
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}
	generation, err := os.MkdirTemp(root, "generation-")
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(generation)
		}
	}()
	artifacts := []struct {
		name string
		data []byte
	}{
		{"catalog.json", prepared.catalogData},
		{"catalog-meta.json", prepared.metaData},
		{"artists_index.json", prepared.artistIdxData},
		{"containers_index.json", prepared.containerData},
	}
	for i, artifact := range artifacts {
		if failAfter == i {
			return fmt.Errorf("injected catalog generation failure after %d artifacts", i)
		}
		if err := WriteFileAtomic(filepath.Join(generation, artifact.name), artifact.data, 0644); err != nil {
			return err
		}
	}
	manifestData, err := json.Marshal(catalogManifest{Generation: filepath.Base(generation)})
	if err != nil {
		return err
	}
	if err := WriteFileAtomic(filepath.Join(cacheDir, catalogManifestFile), manifestData, 0644); err != nil {
		return err
	}
	committed = true
	pruneCatalogGenerations(root, filepath.Base(generation))
	return nil
}

func pruneCatalogGenerations(root, current string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() > entries[j].Name() })
	kept := 0
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == current || kept < 1 {
			if entry.IsDir() && entry.Name() != current {
				kept++
			}
			continue
		}
		_ = os.RemoveAll(filepath.Join(root, entry.Name()))
	}
}

// ReadContainersIndex reads the containers index file.
// Returns an empty index (not an error) when no cache exists yet.
func ReadContainersIndex() (*model.ContainersIndex, error) {
	cacheDir, err := GetCacheDir()
	if err != nil {
		return nil, err
	}
	indexPath, err := catalogArtifactPath(cacheDir, "containers_index.json")
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &model.ContainersIndex{Containers: map[int]model.ContainerIndexEntry{}}, nil
		}
		return nil, fmt.Errorf("failed to read containers index: %w", err)
	}
	var idx model.ContainersIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("failed to parse containers index: %w", err)
	}
	if idx.Containers == nil {
		idx.Containers = map[int]model.ContainerIndexEntry{}
	}
	return &idx, nil
}
