package catalog

import (
	"context"
	"testing"
	"time"

	"github.com/jmagar/nugs-cli/internal/cache"
	"github.com/jmagar/nugs-cli/internal/model"
)

func TestAnalyzeArtistWarmIndexUsesZeroAPIRequests(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	page := &model.ArtistMeta{}
	page.Response.ArtistName = "Artist"
	page.Response.Containers = []*model.AlbArtResp{{
		ArtistName:          "Artist",
		ContainerID:         10,
		ContainerInfo:       "Show",
		PerformanceDate:     "2025-01-01",
		AvailabilityTypeStr: model.AvailableAvailabilityType,
		ProductFormatList:   []*model.ProductFormatList{{}},
	}}
	requests := 0
	deps := &Deps{
		GetArtistMetaCached: func(_ context.Context, _ string, _ time.Duration) ([]*model.ArtistMeta, bool, bool, error) {
			requests++
			if err := cache.WriteArtistMetaCache("1", []*model.ArtistMeta{page}); err != nil {
				return nil, false, false, err
			}
			return []*model.ArtistMeta{page}, false, false, nil
		},
		GetShowMediaType: func(*model.AlbArtResp) model.MediaType { return model.MediaTypeAudio },
	}
	cfg := &model.Config{OutPath: t.TempDir(), DefaultOutputs: "audio"}
	if _, err := AnalyzeArtistCatalogMediaAware(context.Background(), "1", cfg, "", model.MediaTypeAudio, deps); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("cold request count = %d, want 1", requests)
	}
	if _, err := AnalyzeArtistCatalogMediaAware(context.Background(), "1", cfg, "", model.MediaTypeAudio, deps); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("warm analysis made %d requests, want zero additional", requests)
	}
}

func TestAnalyzeArtistRefreshesShardOlderThanCatalogGeneration(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	oldPage := artistPageWithShow("Artist", 10, "Old Show")
	if err := cache.WriteArtistMetaCache("1", []*model.ArtistMeta{oldPage}); err != nil {
		t.Fatal(err)
	}
	latest := &model.LatestCatalogResp{}
	if err := cache.WriteCatalogCache(latest, 0, time.Duration.String); err != nil {
		t.Fatal(err)
	}

	freshPage := artistPageWithShow("Artist", 20, "New Show")
	requests := 0
	deps := &Deps{
		GetArtistMetaCached: func(_ context.Context, _ string, ttl time.Duration) ([]*model.ArtistMeta, bool, bool, error) {
			requests++
			if ttl != 0 {
				t.Fatalf("refresh TTL = %s, want 0 to bypass stale artist cache", ttl)
			}
			if err := cache.WriteArtistMetaCache("1", []*model.ArtistMeta{freshPage}); err != nil {
				return nil, false, false, err
			}
			return []*model.ArtistMeta{freshPage}, false, false, nil
		},
		GetShowMediaType: func(*model.AlbArtResp) model.MediaType { return model.MediaTypeAudio },
	}
	cfg := &model.Config{OutPath: t.TempDir(), DefaultOutputs: "audio"}
	analysis, err := AnalyzeArtistCatalogMediaAware(context.Background(), "1", cfg, "", model.MediaTypeAudio, deps)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("metadata refresh requests = %d, want 1", requests)
	}
	if len(analysis.Shows) != 1 || analysis.Shows[0].Show.ContainerID != 20 {
		t.Fatalf("analysis used stale shard: %+v", analysis.Shows)
	}
}

func artistPageWithShow(name string, containerID int, title string) *model.ArtistMeta {
	page := &model.ArtistMeta{}
	page.Response.ArtistName = name
	page.Response.Containers = []*model.AlbArtResp{{
		ArtistName:          name,
		ContainerID:         containerID,
		ContainerInfo:       title,
		PerformanceDate:     "2025-01-01",
		AvailabilityTypeStr: model.AvailableAvailabilityType,
		ProductFormatList:   []*model.ProductFormatList{{}},
	}}
	return page
}
