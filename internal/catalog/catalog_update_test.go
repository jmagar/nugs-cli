package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmagar/nugs-cli/internal/cache"
	"github.com/jmagar/nugs-cli/internal/model"
	"github.com/jmagar/nugs-cli/internal/testutil"
)

// buildUpdateCatalog builds a catalog with the given shows for CatalogUpdate tests.
// Each show is specified as (containerID, artistID, artistName, date, title).
type showSpec struct {
	containerID int
	artistID    int
	artistName  string
	date        string
	title       string
}

func buildUpdateCatalog(shows []showSpec) *model.LatestCatalogResp {
	cat := &model.LatestCatalogResp{MethodName: "catalog.latest"}
	for _, s := range shows {
		cat.Response.RecentItems = append(cat.Response.RecentItems, struct {
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
			ContainerID:            s.containerID,
			ArtistID:               s.artistID,
			ArtistName:             s.artistName,
			ShowDateFormattedShort: s.date,
			ContainerInfo:          s.title,
		})
	}
	return cat
}

func noDurationFmt(time.Duration) string { return "0s" }

func makeDeps(fetchCatalog func(context.Context) (*model.LatestCatalogResp, error)) *Deps {
	return &Deps{
		FetchCatalog:   fetchCatalog,
		FormatDuration: noDurationFmt,
	}
}

// TestCatalogUpdate_FirstUpdate verifies that when no previous cache exists,
// the output notes it's a first update rather than showing a diff.
func TestCatalogUpdate_FirstUpdate(t *testing.T) {
	testutil.WithTempHome(t)

	newCatalog := buildUpdateCatalog([]showSpec{
		{1001, 500, "Billy Strings", "2025-01-01", "Show A"},
		{1002, 500, "Billy Strings", "2025-02-01", "Show B"},
	})

	stdout := testutil.CaptureStdout(t, func() {
		err := CatalogUpdate(context.Background(), "", makeDeps(func(_ context.Context) (*model.LatestCatalogResp, error) {
			return newCatalog, nil
		}))
		if err != nil {
			t.Fatalf("CatalogUpdate() error = %v", err)
		}
	})

	if !strings.Contains(stdout, "First catalog update") {
		t.Errorf("expected first-update message in output, got:\n%s", stdout)
	}
	// The first-update message may contain "new shows" in prose, but should not
	// contain a count like "2 new show(s)" from the diff table header.
	if strings.Contains(stdout, "new show(s) since last update") {
		t.Errorf("expected no diff table on first update, got:\n%s", stdout)
	}
}

// TestCatalogUpdate_NoNewShows verifies that when all shows are already known
// the output reports no new shows.
func TestCatalogUpdate_NoNewShows(t *testing.T) {
	testutil.WithTempHome(t)

	shows := []showSpec{
		{1001, 500, "Billy Strings", "2025-01-01", "Show A"},
		{1002, 500, "Billy Strings", "2025-02-01", "Show B"},
	}

	// Write existing cache with the same shows.
	if err := cache.WriteCatalogCache(buildUpdateCatalog(shows), 0, noDurationFmt); err != nil {
		t.Fatalf("WriteCatalogCache() error = %v", err)
	}

	stdout := testutil.CaptureStdout(t, func() {
		err := CatalogUpdate(context.Background(), "", makeDeps(func(_ context.Context) (*model.LatestCatalogResp, error) {
			return buildUpdateCatalog(shows), nil
		}))
		if err != nil {
			t.Fatalf("CatalogUpdate() error = %v", err)
		}
	})

	if !strings.Contains(stdout, "No new shows since last update") {
		t.Errorf("expected no-new-shows message, got:\n%s", stdout)
	}
}

// TestCatalogUpdate_NewShowsDisplayed verifies new shows appear in the output.
func TestCatalogUpdate_NewShowsDisplayed(t *testing.T) {
	testutil.WithTempHome(t)

	oldShows := []showSpec{
		{1001, 500, "Billy Strings", "2025-01-01", "Old Show A"},
	}
	newShows := []showSpec{
		{1001, 500, "Billy Strings", "2025-01-01", "Old Show A"},
		{1002, 501, "Grateful Dead", "2025-06-15", "New Show B"},
		{1003, 501, "Grateful Dead", "2025-07-04", "New Show C"},
	}

	// Write existing cache with only the old show.
	if err := cache.WriteCatalogCache(buildUpdateCatalog(oldShows), 0, noDurationFmt); err != nil {
		t.Fatalf("WriteCatalogCache() error = %v", err)
	}

	stdout := testutil.CaptureStdout(t, func() {
		err := CatalogUpdate(context.Background(), "", makeDeps(func(_ context.Context) (*model.LatestCatalogResp, error) {
			return buildUpdateCatalog(newShows), nil
		}))
		if err != nil {
			t.Fatalf("CatalogUpdate() error = %v", err)
		}
	})

	if !strings.Contains(stdout, "2 new show") {
		t.Errorf("expected '2 new show(s)' in output, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Grateful Dead") {
		t.Errorf("expected 'Grateful Dead' in new-shows table, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "New Show B") {
		t.Errorf("expected 'New Show B' in new-shows table, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "New Show C") {
		t.Errorf("expected 'New Show C' in new-shows table, got:\n%s", stdout)
	}
	// Old show must NOT appear in the new-shows section.
	if strings.Contains(stdout, "Old Show A") {
		t.Errorf("old show should not appear in new-shows table, got:\n%s", stdout)
	}
}

// TestCatalogUpdate_JSONMode_NewShowsList verifies JSON output includes newShowsList.
func TestCatalogUpdate_JSONMode_NewShowsList(t *testing.T) {
	testutil.WithTempHome(t)

	oldShows := []showSpec{
		{1001, 500, "Billy Strings", "2025-01-01", "Old Show"},
	}
	allShows := []showSpec{
		{1001, 500, "Billy Strings", "2025-01-01", "Old Show"},
		{1002, 501, "Grateful Dead", "2025-06-15", "New Show"},
	}

	if err := cache.WriteCatalogCache(buildUpdateCatalog(oldShows), 0, noDurationFmt); err != nil {
		t.Fatalf("WriteCatalogCache() error = %v", err)
	}

	stdout := testutil.CaptureStdout(t, func() {
		err := CatalogUpdate(context.Background(), "normal", makeDeps(func(_ context.Context) (*model.LatestCatalogResp, error) {
			return buildUpdateCatalog(allShows), nil
		}))
		if err != nil {
			t.Fatalf("CatalogUpdate() error = %v", err)
		}
	})

	var payload struct {
		Success     bool             `json:"success"`
		FirstUpdate bool             `json:"firstUpdate"`
		TotalShows  int              `json:"totalShows"`
		NewShows    int              `json:"newShows"`
		NewShowList []map[string]any `json:"newShowsList"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("JSON parse error: %v\noutput: %s", err, stdout)
	}
	if !payload.Success {
		t.Error("success = false, want true")
	}
	if payload.FirstUpdate {
		t.Error("firstUpdate = true, want false")
	}
	if payload.TotalShows != 2 {
		t.Errorf("totalShows = %d, want 2", payload.TotalShows)
	}
	if payload.NewShows != 1 {
		t.Errorf("newShows = %d, want 1", payload.NewShows)
	}
	if len(payload.NewShowList) != 1 {
		t.Fatalf("len(newShowsList) = %d, want 1", len(payload.NewShowList))
	}
	if payload.NewShowList[0]["artistName"] != "Grateful Dead" {
		t.Errorf("newShowsList[0].artistName = %v, want 'Grateful Dead'", payload.NewShowList[0]["artistName"])
	}
}

// TestCatalogUpdate_JSONMode_FirstUpdate verifies JSON output on first update.
func TestCatalogUpdate_JSONMode_FirstUpdate(t *testing.T) {
	testutil.WithTempHome(t)

	stdout := testutil.CaptureStdout(t, func() {
		err := CatalogUpdate(context.Background(), "normal", makeDeps(func(_ context.Context) (*model.LatestCatalogResp, error) {
			return buildUpdateCatalog([]showSpec{{1001, 500, "Billy Strings", "2025-01-01", "Show A"}}), nil
		}))
		if err != nil {
			t.Fatalf("CatalogUpdate() error = %v", err)
		}
	})

	var payload struct {
		Success     bool `json:"success"`
		FirstUpdate bool `json:"firstUpdate"`
		NewShows    int  `json:"newShows"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("JSON parse error: %v\noutput: %s", err, stdout)
	}
	if !payload.FirstUpdate {
		t.Error("firstUpdate = false, want true")
	}
	if payload.NewShows != 0 {
		t.Errorf("newShows = %d, want 0 on first update", payload.NewShows)
	}
}

// TestCatalogUpdate_JSONMode_NoNewShows verifies newShowsList is [] not absent.
func TestCatalogUpdate_JSONMode_NoNewShows(t *testing.T) {
	testutil.WithTempHome(t)

	shows := []showSpec{{1001, 500, "Billy Strings", "2025-01-01", "Show A"}}
	if err := cache.WriteCatalogCache(buildUpdateCatalog(shows), 0, noDurationFmt); err != nil {
		t.Fatalf("WriteCatalogCache() error = %v", err)
	}

	stdout := testutil.CaptureStdout(t, func() {
		err := CatalogUpdate(context.Background(), "normal", makeDeps(func(_ context.Context) (*model.LatestCatalogResp, error) {
			return buildUpdateCatalog(shows), nil
		}))
		if err != nil {
			t.Fatalf("CatalogUpdate() error = %v", err)
		}
	})

	// Parse as raw map to detect key presence vs. value.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("JSON parse error: %v\noutput: %s", err, stdout)
	}
	if _, ok := raw["newShowsList"]; !ok {
		t.Error("newShowsList key missing from JSON output; want [] not absent")
	}

	var list []any
	if err := json.Unmarshal(raw["newShowsList"], &list); err != nil {
		t.Fatalf("newShowsList is not a JSON array: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("len(newShowsList) = %d, want 0", len(list))
	}
}

// TestCatalogUpdate_FetchError propagates the API error.
func TestCatalogUpdate_FetchError(t *testing.T) {
	testutil.WithTempHome(t)

	fetchErr := errors.New("network timeout")
	err := CatalogUpdate(context.Background(), "", makeDeps(func(_ context.Context) (*model.LatestCatalogResp, error) {
		return nil, fetchErr
	}))

	if err == nil {
		t.Fatal("CatalogUpdate() error = nil, want fetch error")
	}
	if !strings.Contains(err.Error(), "network timeout") {
		t.Errorf("error = %q, want to contain 'network timeout'", err.Error())
	}
}

// TestCatalogUpdate_OldIndexReadError treats a corrupt index as first update.
func TestCatalogUpdate_OldIndexReadError(t *testing.T) {
	testutil.WithTempHome(t)

	// Write a corrupt containers_index.json so ReadContainersIndex returns an error.
	cacheDir, err := cache.GetCacheDir()
	if err != nil {
		t.Fatalf("GetCacheDir() error = %v", err)
	}
	// Write valid catalog.json so WriteCatalogCache doesn't choke, but corrupt the index.
	// We do this by first writing a valid cache, then corrupting the index file.
	if err := cache.WriteCatalogCache(
		buildUpdateCatalog([]showSpec{{999, 100, "Artist", "2020-01-01", "Show"}}),
		0, noDurationFmt,
	); err != nil {
		t.Fatalf("WriteCatalogCache() error = %v", err)
	}

	// Now corrupt the containers_index so ReadContainersIndex fails.
	indexPath := filepath.Join(cacheDir, "containers_index.json")
	if err := os.WriteFile(indexPath, []byte("{bad json"), 0644); err != nil {
		t.Fatalf("failed to corrupt index: %v", err)
	}

	// The update should still succeed, falling back to first-update behavior.
	var callErr error
	stdout := testutil.CaptureStdout(t, func() {
		callErr = CatalogUpdate(context.Background(), "", makeDeps(func(_ context.Context) (*model.LatestCatalogResp, error) {
			return buildUpdateCatalog([]showSpec{{1001, 500, "Billy Strings", "2025-01-01", "New Show"}}), nil
		}))
	})

	if callErr != nil {
		t.Fatalf("CatalogUpdate() error = %v, want nil (corrupt old index is non-fatal)", callErr)
	}
	if !strings.Contains(stdout, "First catalog update") {
		t.Errorf("expected first-update fallback message, got:\n%s", stdout)
	}
}

