package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func buildTestArtistMeta(artistID int, artistName string, shows []*AlbArtResp) []*ArtistMeta {
	meta := &ArtistMeta{}
	meta.Response.ArtistID = artistID
	meta.Response.Containers = shows
	return []*ArtistMeta{meta}
}

func withTempHome(t *testing.T) string {
	t.Helper()
	origHome := os.Getenv("HOME")
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Cleanup(func() {
		_ = os.Setenv("HOME", origHome)
	})
	return tempHome
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = orig

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return buf.String()
}

func TestAnalyzeArtistCatalog_UsesCacheAndComputesCounts(t *testing.T) {
	withTempHome(t)
	outPath := t.TempDir()

	shows := []*AlbArtResp{
		{
			ArtistName:                    "Test Artist",
			ContainerInfo:                 "2024-01-01 Venue",
			ContainerID:                   101,
			PerformanceDate:               "24/01/01",
			PerformanceDateShortYearFirst: "24/01/01",
		},
		{
			ArtistName:                    "Test Artist",
			ContainerInfo:                 "2023-01-01 Venue",
			ContainerID:                   102,
			PerformanceDate:               "23/01/01",
			PerformanceDateShortYearFirst: "23/01/01",
		},
	}

	if err := writeArtistMetaCache("1125", buildTestArtistMeta(1125, "Test Artist", shows)); err != nil {
		t.Fatalf("failed to seed artist cache: %v", err)
	}

	artistFolder := sanitise("Test Artist")
	downloadedFolder := buildAlbumFolderName("Test Artist", "2024-01-01 Venue")
	if err := os.MkdirAll(filepath.Join(outPath, artistFolder, downloadedFolder), 0755); err != nil {
		t.Fatalf("failed to seed downloaded folder: %v", err)
	}

	cfg := &Config{
		OutPath:         outPath,
		RcloneEnabled:   false,
		RcloneRemote:    "",
		RclonePath:      "",
		RcloneTransfers: 0,
	}

	analysis, err := analyzeArtistCatalog("1125", cfg, "")
	if err != nil {
		t.Fatalf("analyzeArtistCatalog failed: %v", err)
	}

	if !analysis.CacheUsed {
		t.Fatalf("expected cache to be used")
	}
	if analysis.TotalShows != 2 {
		t.Fatalf("expected total shows=2, got %d", analysis.TotalShows)
	}
	if analysis.Downloaded != 1 {
		t.Fatalf("expected downloaded=1, got %d", analysis.Downloaded)
	}
	if analysis.Missing != 1 {
		t.Fatalf("expected missing=1, got %d", analysis.Missing)
	}
	if len(analysis.MissingShows) != 1 || analysis.MissingShows[0].Show.ContainerID != 102 {
		t.Fatalf("expected only container 102 missing, got %+v", analysis.MissingShows)
	}
}

func TestCatalogGapsForArtist_DefaultOutputIsMissingListOnly(t *testing.T) {
	withTempHome(t)
	outPath := t.TempDir()

	shows := []*AlbArtResp{
		{
			ArtistName:                    "Test Artist",
			ContainerInfo:                 "2024-01-01 Venue",
			ContainerID:                   201,
			PerformanceDate:               "24/01/01",
			PerformanceDateShortYearFirst: "24/01/01",
		},
		{
			ArtistName:                    "Test Artist",
			ContainerInfo:                 "2023-01-01 Venue",
			ContainerID:                   202,
			PerformanceDate:               "23/01/01",
			PerformanceDateShortYearFirst: "23/01/01",
		},
	}

	if err := writeArtistMetaCache("3344", buildTestArtistMeta(3344, "Test Artist", shows)); err != nil {
		t.Fatalf("failed to seed artist cache: %v", err)
	}

	cfg := &Config{
		OutPath:       outPath,
		RcloneEnabled: false,
	}

	out := captureStdout(t, func() {
		if err := catalogGapsForArtist("3344", cfg, "", false); err != nil {
			t.Fatalf("catalogGapsForArtist failed: %v", err)
		}
	})

	if !strings.Contains(out, "ID") {
		t.Fatalf("expected table output with ID column, got: %s", out)
	}
	if strings.Contains(out, "Gap Analysis") {
		t.Fatalf("did not expect gap summary header in default output: %s", out)
	}
	if strings.Contains(out, "Total Shows") {
		t.Fatalf("did not expect coverage summary in default output: %s", out)
	}
}

func TestGetArtistMetaCached_UsesFreshCache(t *testing.T) {
	withTempHome(t)

	shows := []*AlbArtResp{
		{
			ArtistName:                    "Cache Artist",
			ContainerInfo:                 "Cache Show",
			ContainerID:                   999,
			PerformanceDate:               "24/01/01",
			PerformanceDateShortYearFirst: "24/01/01",
		},
	}
	if err := writeArtistMetaCache("555", buildTestArtistMeta(555, "Cache Artist", shows)); err != nil {
		t.Fatalf("failed to seed artist cache: %v", err)
	}

	pages, cacheUsed, cacheStaleUse, err := getArtistMetaCached("555", 24*time.Hour)
	if err != nil {
		t.Fatalf("getArtistMetaCached failed: %v", err)
	}
	if !cacheUsed {
		t.Fatalf("expected cacheUsed=true")
	}
	if cacheStaleUse {
		t.Fatalf("expected cacheStaleUse=false for fresh cache")
	}
	if len(pages) != 1 || len(pages[0].Response.Containers) != 1 {
		t.Fatalf("unexpected cache pages shape: %+v", pages)
	}
}
