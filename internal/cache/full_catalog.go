package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jmagar/nugs-cli/internal/model"
)

const fullCatalogIndexFile = "full-catalog-index.json"

// FullCatalogIndex is the durable, API-independent source used by catalog
// coverage and gap analysis after the first complete crawl.
type FullCatalogIndex struct {
	Version   int                   `json:"version"`
	UpdatedAt time.Time             `json:"updatedAt"`
	Artists   map[string]FullArtist `json:"artists"`
}

type FullArtist struct {
	ArtistName string              `json:"artistName"`
	Pages      []*model.ArtistMeta `json:"pages"`
}

func ReadFullCatalogArtist(artistID string) ([]*model.ArtistMeta, string, error) {
	cacheDir, err := GetCacheDir()
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(filepath.Join(cacheDir, fullCatalogIndexFile))
	if err != nil {
		return nil, "", err
	}
	var index FullCatalogIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, "", fmt.Errorf("failed to parse full catalog index: %w", err)
	}
	artist, ok := index.Artists[artistID]
	if !ok {
		return nil, "", os.ErrNotExist
	}
	return artist.Pages, artist.ArtistName, nil
}

func updateFullCatalogArtistLocked(cacheDir, artistID string, pages []*model.ArtistMeta) error {
	path := filepath.Join(cacheDir, fullCatalogIndexFile)
	index := FullCatalogIndex{Version: 1, Artists: make(map[string]FullArtist)}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &index); err != nil {
			return fmt.Errorf("failed to parse full catalog index: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if index.Artists == nil {
		index.Artists = make(map[string]FullArtist)
	}
	name := ""
	for _, page := range pages {
		if page != nil {
			if value, ok := page.Response.ArtistName.(string); ok && value != "" {
				name = value
			}
			if name == "" && len(page.Response.Containers) > 0 && page.Response.Containers[0] != nil {
				name = page.Response.Containers[0].ArtistName
			}
		}
	}
	index.Artists[artistID] = FullArtist{ArtistName: name, Pages: pages}
	index.UpdatedAt = time.Now()
	data, err := json.Marshal(index)
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, data, 0644)
}
