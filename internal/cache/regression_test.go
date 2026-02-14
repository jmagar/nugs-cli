package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jmagar/nugs-cli/internal/model"
)

// buildCatalog creates a test catalog using the actual model type to ensure
// compile-time safety if the model struct changes.
func buildCatalog(containerID, artistID int, artistName string) *model.LatestCatalogResp {
	catalog := &model.LatestCatalogResp{MethodName: "catalog.latest"}
	catalog.Response.RecentItems = append(catalog.Response.RecentItems, struct {
		ContainerInfo          string `json:"containerInfo"`
		ArtistName             string `json:"artistName"`
		ShowDateFormattedShort string `json:"showDateFormattedShort"`
		ArtistID               int    `json:"artistID"`
		ContainerID            int    `json:"containerID"`
		PerformanceDateStr     string `json:"performanceDateStr"`
		PostedDate             string `json:"postedDate"`
		VenueCity              string `json:"venueCity"`
		VenueState             string `json:"venueState"`
		Venue                  string `json:"venue"`
		PageURL                string `json:"pageURL"`
		CategoryID             int    `json:"categoryID"`
		ImageURL               string `json:"imageURL"`
	}{
		ContainerInfo:      "Show",
		ArtistName:         artistName,
		ArtistID:           artistID,
		ContainerID:        containerID,
		PerformanceDateStr: "2026-02-10",
	})
	return catalog
}

func TestWriteCatalogCache_ConcurrentWriters_Consistency(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	formatDurationFn := func(d time.Duration) string {
		return d.String()
	}

	const writerCount = 12
	errCh := make(chan error, writerCount)
	var wg sync.WaitGroup

	for i := 0; i < writerCount; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			catalog := buildCatalog(1000+i, 2000+i, "Artist")
			errCh <- WriteCatalogCache(catalog, time.Duration(i)*time.Millisecond, formatDurationFn)
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent cache writes timed out")
	}
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("WriteCatalogCache failed: %v", err)
		}
	}

	cacheDir, err := GetCacheDir()
	if err != nil {
		t.Fatalf("GetCacheDir failed: %v", err)
	}

	// Cache index files use these names by design:
	//   artists_index.json  - artist name -> ID lookup
	//   containers_index.json - containerID -> artist mapping
	files := []string{
		"catalog.json",
		"catalog-meta.json",
		"artists_index.json",
		"containers_index.json",
	}

	for _, name := range files {
		path := filepath.Join(cacheDir, name)
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("failed reading %s: %v", name, readErr)
		}
		if !json.Valid(data) {
			t.Fatalf("%s is not valid JSON", name)
		}
	}

	catalog, err := ReadCatalogCache()
	if err != nil {
		t.Fatalf("ReadCatalogCache failed: %v", err)
	}
	if catalog == nil || len(catalog.Response.RecentItems) == 0 {
		t.Fatal("catalog cache is empty after writes")
	}

	meta, err := ReadCacheMeta()
	if err != nil {
		t.Fatalf("ReadCacheMeta failed: %v", err)
	}
	if meta == nil {
		t.Fatal("cache metadata missing after writes")
	}

	tmpFiles, err := filepath.Glob(filepath.Join(cacheDir, "*.tmp"))
	if err != nil {
		t.Fatalf("glob for tmp files failed: %v", err)
	}
	if len(tmpFiles) != 0 {
		t.Fatalf("unexpected leftover temp files: %v", tmpFiles)
	}
}
