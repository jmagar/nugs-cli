package cache

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmagar/nugs-cli/internal/model"
)

func TestCatalogGenerationFailureKeepsPreviousSnapshot(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	oldCatalog := buildCatalog(1, 101, "old")
	if err := WriteCatalogCache(oldCatalog, 0, time.Duration.String); err != nil {
		t.Fatal(err)
	}
	cacheDir, _ := GetCacheDir()
	prepared, err := prepareCatalogCacheData(buildCatalog(2, 202, "new"), 0, time.Duration.String)
	if err != nil {
		t.Fatal(err)
	}
	if err := WithCacheLock(func() error {
		return writePreparedCatalogCacheWithFailpoint(cacheDir, prepared, 2)
	}); err == nil {
		t.Fatal("expected injected failure")
	}
	got, err := ReadCatalogCache()
	if err != nil {
		t.Fatal(err)
	}
	if id := got.Response.RecentItems[0].ContainerID; id != 1 {
		t.Fatalf("mixed/new generation exposed: containerID=%d", id)
	}
	idx, err := ReadContainersIndex()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := idx.Containers[1]; !ok {
		t.Fatal("previous index was not preserved")
	}
}

func TestArtistCacheEvictionPreservesDurableFullIndex(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	page := &model.ArtistMeta{}
	page.Response.ArtistName = "Artist"
	if err := WriteArtistMetaCache("1", []*model.ArtistMeta{page}); err != nil {
		t.Fatal(err)
	}
	path, _ := GetArtistMetaCachePath("1")
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if err := PruneArtistMetaCache(24*time.Hour, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("artist cache was not evicted: %v", err)
	}
	pages, name, err := ReadFullCatalogArtist("1")
	if err != nil || len(pages) != 1 || name != "Artist" {
		t.Fatalf("durable full index lost: pages=%d name=%q err=%v", len(pages), name, err)
	}
	cacheDir, _ := GetCacheDir()
	if _, err := os.Stat(filepath.Join(cacheDir, fullCatalogIndexFile)); err != nil {
		t.Fatal(err)
	}
}
