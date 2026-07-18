package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/jmagar/nugs-cli/internal/model"
)

const (
	fullCatalogShardDir      = "full-catalog-artists"
	legacyFullCatalogIndex   = "full-catalog-index.json"
	fullCatalogShardMaxAge   = 30 * 24 * time.Hour
	fullCatalogShardMaxBytes = int64(256 << 20)
)

type fullCatalogArtistShard struct {
	Version    int                 `json:"version"`
	UpdatedAt  time.Time           `json:"updatedAt"`
	ArtistID   string              `json:"artistID"`
	ArtistName string              `json:"artistName"`
	Pages      []*model.ArtistMeta `json:"pages"`
}

func fullCatalogArtistPath(cacheDir, artistID string) (string, error) {
	safeID := filepath.Base(artistID)
	if safeID != artistID || safeID == "." || safeID == ".." || safeID == "" {
		return "", fmt.Errorf("invalid artist ID: %q", artistID)
	}
	return filepath.Join(cacheDir, fullCatalogShardDir, "artist_"+safeID+".json"), nil
}

// ReadFullCatalogArtist reads one durable artist shard without loading the
// rest of the catalog into memory.
func ReadFullCatalogArtist(artistID string) ([]*model.ArtistMeta, string, time.Time, error) {
	cacheDir, err := GetCacheDir()
	if err != nil {
		return nil, "", time.Time{}, err
	}
	path, err := fullCatalogArtistPath(cacheDir, artistID)
	if err != nil {
		return nil, "", time.Time{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", time.Time{}, err
	}
	var shard fullCatalogArtistShard
	if err := json.Unmarshal(data, &shard); err != nil {
		return nil, "", time.Time{}, fmt.Errorf("failed to parse full catalog artist shard: %w", err)
	}
	if shard.Version != 1 || shard.ArtistID != artistID || shard.UpdatedAt.IsZero() {
		return nil, "", time.Time{}, fmt.Errorf("invalid full catalog artist shard for %s", artistID)
	}
	return shard.Pages, shard.ArtistName, shard.UpdatedAt, nil
}

// FullCatalogArtistFresh reports whether a shard is recent enough and was
// written after the latest catalog generation. A catalog update therefore
// invalidates older artist shards so watch/gap checks cannot hide new shows.
func FullCatalogArtistFresh(updatedAt time.Time, maxAge time.Duration) bool {
	if updatedAt.IsZero() || maxAge <= 0 || time.Since(updatedAt) > maxAge {
		return false
	}
	meta, err := ReadCacheMeta()
	if err != nil || meta == nil {
		return err == nil
	}
	return !updatedAt.Before(meta.LastUpdated)
}

func updateFullCatalogArtistLocked(cacheDir, artistID string, pages []*model.ArtistMeta) error {
	path, err := fullCatalogArtistPath(cacheDir, artistID)
	if err != nil {
		return err
	}
	name := ""
	for _, page := range pages {
		if page == nil {
			continue
		}
		if value, ok := page.Response.ArtistName.(string); ok && value != "" {
			name = value
		}
		if name == "" && len(page.Response.Containers) > 0 && page.Response.Containers[0] != nil {
			name = page.Response.Containers[0].ArtistName
		}
	}
	shard := fullCatalogArtistShard{
		Version:    1,
		UpdatedAt:  time.Now(),
		ArtistID:   artistID,
		ArtistName: name,
		Pages:      pages,
	}
	data, err := json.Marshal(shard)
	if err != nil {
		return err
	}
	if err := WriteFileAtomic(path, data, 0644); err != nil {
		return err
	}
	// The former monolithic file is neither bounded nor efficiently readable.
	_ = os.Remove(filepath.Join(cacheDir, legacyFullCatalogIndex))
	return pruneFullCatalogShardsLocked(filepath.Dir(path), fullCatalogShardMaxAge, fullCatalogShardMaxBytes, path)
}

func pruneFullCatalogShardsLocked(dir string, maxAge time.Duration, maxBytes int64, preserve string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
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
