package cache

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
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

func TestCatalogManifestRetainsActualPreviousGeneration(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := WriteCatalogCache(buildCatalog(1, 1, "one"), 0, time.Duration.String); err != nil {
		t.Fatal(err)
	}
	cacheDir, _ := GetCacheDir()
	readManifest := func() catalogManifest {
		data, err := os.ReadFile(filepath.Join(cacheDir, catalogManifestFile))
		if err != nil {
			t.Fatal(err)
		}
		var manifest catalogManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			t.Fatal(err)
		}
		return manifest
	}
	first := readManifest()
	if err := WriteCatalogCache(buildCatalog(2, 2, "two"), 0, time.Duration.String); err != nil {
		t.Fatal(err)
	}
	second := readManifest()
	if second.Previous != first.Generation {
		t.Fatalf("previous generation = %q, want %q", second.Previous, first.Generation)
	}
	for _, generation := range []string{second.Generation, second.Previous} {
		if _, err := os.Stat(filepath.Join(cacheDir, catalogGenerationDir, generation)); err != nil {
			t.Fatalf("retained generation %q: %v", generation, err)
		}
	}
}

func TestCatalogGenerationConcurrentReadersAndWriters(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := WriteCatalogCache(buildCatalog(1, 1, "initial"), 0, time.Duration.String); err != nil {
		t.Fatal(err)
	}
	const writers = 16
	const readers = 6
	start := make(chan struct{})
	errs := make(chan error, writers+readers*120)
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- WriteCatalogCache(buildCatalog(100+i, 200+i, "writer"), 0, time.Duration.String)
		}()
	}
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for range 40 {
				if _, err := ReadCatalogCache(); err != nil {
					errs <- err
				}
				if _, err := ReadCacheMeta(); err != nil {
					errs <- err
				}
				if _, err := ReadContainersIndex(); err != nil {
					errs <- err
				}
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent catalog access failed: %v", err)
		}
	}
}

func TestFullCatalogUsesBoundedArtistShards(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	page := &model.ArtistMeta{}
	for _, artistID := range []string{"1", "2", "3"} {
		page.Response.ArtistName = "Artist " + artistID
		if err := WriteArtistMetaCache(artistID, []*model.ArtistMeta{page}); err != nil {
			t.Fatal(err)
		}
	}
	cacheDir, _ := GetCacheDir()
	dir := filepath.Join(cacheDir, fullCatalogShardDir)
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 3 {
		t.Fatalf("artist shards = %d, err=%v; want 3", len(entries), err)
	}
	if err := WithCacheLock(func() error { return pruneFullCatalogShardsLocked(dir, 0, 1, "") }); err != nil {
		t.Fatal(err)
	}
	entries, err = os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("bounded shard prune retained %d files, want 0", len(entries))
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
	pages, name, _, err := ReadFullCatalogArtist("1")
	if err != nil || len(pages) != 1 || name != "Artist" {
		t.Fatalf("durable full index lost: pages=%d name=%q err=%v", len(pages), name, err)
	}
	cacheDir, _ := GetCacheDir()
	shardPath, _ := fullCatalogArtistPath(cacheDir, "1")
	if _, err := os.Stat(shardPath); err != nil {
		t.Fatal(err)
	}
}
