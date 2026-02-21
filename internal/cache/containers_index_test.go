package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmagar/nugs-cli/internal/model"
)

func noDuration(time.Duration) string { return "" }

func TestReadContainersIndex_NoCacheFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	idx, err := ReadContainersIndex()
	if err != nil {
		t.Fatalf("ReadContainersIndex() error = %v, want nil", err)
	}
	if idx == nil {
		t.Fatal("ReadContainersIndex() returned nil index, want empty index")
	}
	if len(idx.Containers) != 0 {
		t.Fatalf("ReadContainersIndex() len(Containers) = %d, want 0", len(idx.Containers))
	}
}

func TestReadContainersIndex_ValidCache(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Build a two-show catalog and write it via the normal cache write path.
	catalog := buildCatalog(1001, 500, "Billy Strings")
	catalog.Response.RecentItems = append(catalog.Response.RecentItems, catalog.Response.RecentItems[0])
	catalog.Response.RecentItems[1].ContainerID = 1002
	catalog.Response.RecentItems[1].ArtistID = 501
	catalog.Response.RecentItems[1].ArtistName = "Grateful Dead"

	if err := WriteCatalogCache(catalog, 0, noDuration); err != nil {
		t.Fatalf("WriteCatalogCache() error = %v", err)
	}

	idx, err := ReadContainersIndex()
	if err != nil {
		t.Fatalf("ReadContainersIndex() error = %v", err)
	}
	if len(idx.Containers) != 2 {
		t.Fatalf("len(Containers) = %d, want 2", len(idx.Containers))
	}
	if _, ok := idx.Containers[1001]; !ok {
		t.Error("missing container 1001")
	}
	if _, ok := idx.Containers[1002]; !ok {
		t.Error("missing container 1002")
	}
	if idx.Containers[1001].ArtistName != "Billy Strings" {
		t.Errorf("ArtistName = %q, want %q", idx.Containers[1001].ArtistName, "Billy Strings")
	}
}

func TestReadContainersIndex_CorruptJSON(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cacheDir, err := GetCacheDir()
	if err != nil {
		t.Fatalf("GetCacheDir() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "containers_index.json"), []byte("{corrupt"), 0644); err != nil {
		t.Fatalf("failed to write corrupt file: %v", err)
	}

	_, err = ReadContainersIndex()
	if err == nil {
		t.Fatal("ReadContainersIndex() error = nil, want parse error")
	}
}

func TestReadContainersIndex_NilContainersFieldInitialized(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cacheDir, err := GetCacheDir()
	if err != nil {
		t.Fatalf("GetCacheDir() error = %v", err)
	}

	// Write JSON with explicit null containers map.
	data, _ := json.Marshal(model.ContainersIndex{Containers: nil})
	if err := os.WriteFile(filepath.Join(cacheDir, "containers_index.json"), data, 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	idx, err := ReadContainersIndex()
	if err != nil {
		t.Fatalf("ReadContainersIndex() error = %v", err)
	}
	if idx.Containers == nil {
		t.Fatal("Containers map is nil, want initialized empty map")
	}
}
