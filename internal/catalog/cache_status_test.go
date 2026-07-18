package catalog

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmagar/nugs-cli/internal/cache"
	"github.com/jmagar/nugs-cli/internal/testutil"
)

func TestCatalogCacheStatusUsesManifestGeneration(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := cache.WriteCatalogCache(buildUpdateCatalog([]showSpec{{1, 2, "Artist", "2025-01-01", "Show"}}), 0, time.Duration.String); err != nil {
		t.Fatal(err)
	}
	cacheDir, err := cache.GetCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "catalog.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("test requires no legacy catalog path, stat err = %v", err)
	}

	var callErr error
	stdout := testutil.CaptureStdout(t, func() {
		callErr = CatalogCacheStatus("normal", &Deps{FormatDuration: time.Duration.String})
	})
	if callErr != nil {
		t.Fatal(callErr)
	}
	var payload struct {
		Exists        bool  `json:"exists"`
		FileSizeBytes int64 `json:"fileSizeBytes"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("invalid status JSON %q: %v", stdout, err)
	}
	if !payload.Exists || payload.FileSizeBytes <= 0 {
		t.Fatalf("unexpected cache status: %+v", payload)
	}
}
