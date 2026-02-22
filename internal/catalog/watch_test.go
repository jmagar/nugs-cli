package catalog

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmagar/nugs-cli/internal/config"
	"github.com/jmagar/nugs-cli/internal/model"
	"github.com/jmagar/nugs-cli/internal/testutil"
)

// notifyCapture returns a notify func and pointers to the captured title, message, priority.
// gotCalled is set to true if the func is invoked at all.
func notifyCapture() (func(ctx context.Context, title, message string, priority int) error, *bool, *string, *string, *int) {
	called := false
	title := ""
	message := ""
	priority := 0
	fn := func(_ context.Context, t, m string, p int) error {
		called = true
		title = t
		message = m
		priority = p
		return nil
	}
	return fn, &called, &title, &message, &priority
}

// resetLoadedConfigPath clears the global config path so WriteConfig uses HOME.
func resetLoadedConfigPath(t *testing.T) {
	t.Helper()
	orig := config.LoadedConfigPath
	config.LoadedConfigPath = ""
	t.Cleanup(func() { config.LoadedConfigPath = orig })
}

// TestWatchAdd tests adding artists to the watch list.
func TestWatchAdd(t *testing.T) {
	tests := []struct {
		name           string
		initial        []string
		addID          string
		wantErr        bool
		wantErrContain string
		wantList       []string
	}{
		{
			name:     "adds new artist",
			initial:  []string{},
			addID:    "1125",
			wantList: []string{"1125"},
		},
		{
			name:     "adds second artist",
			initial:  []string{"1125"},
			addID:    "461",
			wantList: []string{"1125", "461"},
		},
		{
			name:     "duplicate is idempotent",
			initial:  []string{"1125"},
			addID:    "1125",
			wantList: []string{"1125"}, // unchanged
		},
		{
			name:           "rejects non-numeric ID",
			initial:        []string{},
			addID:          "billy-strings",
			wantErr:        true,
			wantErrContain: "must be a number",
			wantList:       []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testutil.WithTempHome(t)
			resetLoadedConfigPath(t)

			cfg := &model.Config{WatchedArtists: append([]string{}, tc.initial...)}
			err := WatchAdd(cfg, tc.addID)

			if tc.wantErr {
				if err == nil {
					t.Fatal("WatchAdd() error = nil, want error")
				}
				if tc.wantErrContain != "" && !strings.Contains(err.Error(), tc.wantErrContain) {
					t.Errorf("WatchAdd() error = %q, want to contain %q", err.Error(), tc.wantErrContain)
				}
			} else {
				if err != nil {
					t.Fatalf("WatchAdd() error = %v", err)
				}
			}

			if len(cfg.WatchedArtists) != len(tc.wantList) {
				t.Errorf("WatchedArtists = %v, want %v", cfg.WatchedArtists, tc.wantList)
				return
			}
			for i, id := range tc.wantList {
				if cfg.WatchedArtists[i] != id {
					t.Errorf("WatchedArtists[%d] = %q, want %q", i, cfg.WatchedArtists[i], id)
				}
			}
		})
	}
}

// TestWatchRemove tests removing artists from the watch list.
func TestWatchRemove(t *testing.T) {
	tests := []struct {
		name           string
		initial        []string
		removeID       string
		wantErr        bool
		wantErrContain string
		wantList       []string
	}{
		{
			name:     "removes existing artist",
			initial:  []string{"1125", "461"},
			removeID: "461",
			wantList: []string{"1125"},
		},
		{
			name:     "removes only artist",
			initial:  []string{"1125"},
			removeID: "1125",
			wantList: []string{},
		},
		{
			name:           "errors when not found",
			initial:        []string{"1125"},
			removeID:       "461",
			wantErr:        true,
			wantErrContain: "not in the watch list",
			wantList:       []string{"1125"}, // unchanged
		},
		{
			name:           "errors on empty list",
			initial:        []string{},
			removeID:       "1125",
			wantErr:        true,
			wantErrContain: "not in the watch list",
			wantList:       []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testutil.WithTempHome(t)
			resetLoadedConfigPath(t)

			cfg := &model.Config{WatchedArtists: append([]string{}, tc.initial...)}
			err := WatchRemove(cfg, tc.removeID)

			if tc.wantErr {
				if err == nil {
					t.Fatal("WatchRemove() error = nil, want error")
				}
				if tc.wantErrContain != "" && !strings.Contains(err.Error(), tc.wantErrContain) {
					t.Errorf("WatchRemove() error = %q, want to contain %q", err.Error(), tc.wantErrContain)
				}
			} else {
				if err != nil {
					t.Fatalf("WatchRemove() error = %v", err)
				}
			}

			wantLen := len(tc.wantList)
			if len(cfg.WatchedArtists) != wantLen {
				t.Errorf("WatchedArtists = %v, want %v", cfg.WatchedArtists, tc.wantList)
				return
			}
			for i, id := range tc.wantList {
				if cfg.WatchedArtists[i] != id {
					t.Errorf("WatchedArtists[%d] = %q, want %q", i, cfg.WatchedArtists[i], id)
				}
			}
		})
	}
}

// TestWatchIntervalOrDefault confirms the default value is "1h".
func TestWatchIntervalOrDefault(t *testing.T) {
	if got := watchIntervalOrDefault(&model.Config{}); got != "1h" {
		t.Errorf("watchIntervalOrDefault({}) = %q, want %q", got, "1h")
	}
	if got := watchIntervalOrDefault(&model.Config{WatchInterval: "30m"}); got != "30m" {
		t.Errorf("watchIntervalOrDefault({WatchInterval:30m}) = %q, want %q", got, "30m")
	}
}

// TestWatchCheck_EmptyList verifies WatchCheck returns early when no artists are watched.
func TestWatchCheck_EmptyList(t *testing.T) {
	testutil.WithTempHome(t)

	fetchCalled := false
	deps := &Deps{
		FetchCatalog: func(_ context.Context) (*model.LatestCatalogResp, error) {
			fetchCalled = true
			return buildUpdateCatalog(nil), nil
		},
		FormatDuration: noDurationFmt,
	}

	cfg := &model.Config{WatchedArtists: []string{}}
	err := WatchCheck(context.Background(), cfg, &model.StreamParams{}, "", model.MediaTypeUnknown, deps)
	if err != nil {
		t.Fatalf("WatchCheck() error = %v", err)
	}
	if fetchCalled {
		t.Error("FetchCatalog should not be called when watch list is empty")
	}
}

// TestWatchCheck_CatalogUpdateFailureIsNonFatal verifies WatchCheck continues to gap-fill
// when the catalog update fails.
func TestWatchCheck_CatalogUpdateFailureIsNonFatal(t *testing.T) {
	testutil.WithTempHome(t)

	fetchErr := errors.New("network timeout")
	artistMetaCalled := false

	deps := &Deps{
		FetchCatalog: func(_ context.Context) (*model.LatestCatalogResp, error) {
			return nil, fetchErr
		},
		FormatDuration: noDurationFmt,
		// GetArtistMetaCached is called by AnalyzeArtistCatalog inside CatalogGapsFill.
		// Return empty pages so AnalyzeArtistCatalog reports 0 missing shows.
		GetArtistMetaCached: func(_ context.Context, _ string, _ time.Duration) ([]*model.ArtistMeta, bool, bool, error) {
			artistMetaCalled = true
			return []*model.ArtistMeta{emptyArtistMeta("Billy Strings")}, false, false, nil
		},
		GetShowMediaType: func(_ *model.AlbArtResp) model.MediaType { return model.MediaTypeAudio },
	}

	cfg := &model.Config{
		WatchedArtists: []string{"1125"},
		OutPath:        t.TempDir(),
	}
	err := WatchCheck(context.Background(), cfg, &model.StreamParams{}, "", model.MediaTypeUnknown, deps)
	if err != nil {
		t.Fatalf("WatchCheck() returned error on catalog failure: %v", err)
	}
	if !artistMetaCalled {
		t.Error("GetArtistMetaCached should be called even when catalog update fails")
	}
}

// TestWatchCheck_PerArtistFailureContinues verifies WatchCheck continues to the next artist
// when one artist's gap-fill fails.
func TestWatchCheck_PerArtistFailureContinues(t *testing.T) {
	testutil.WithTempHome(t)

	callOrder := []string{}

	deps := &Deps{
		FetchCatalog: func(_ context.Context) (*model.LatestCatalogResp, error) {
			return buildUpdateCatalog(nil), nil
		},
		FormatDuration: noDurationFmt,
		GetArtistMetaCached: func(_ context.Context, artistID string, _ time.Duration) ([]*model.ArtistMeta, bool, bool, error) {
			callOrder = append(callOrder, artistID)
			if artistID == "1125" {
				return nil, false, false, errors.New("artist meta fetch failed")
			}
			return []*model.ArtistMeta{emptyArtistMeta("Grateful Dead")}, false, false, nil
		},
		GetShowMediaType: func(_ *model.AlbArtResp) model.MediaType { return model.MediaTypeAudio },
	}

	cfg := &model.Config{
		WatchedArtists: []string{"1125", "461"},
		OutPath:        t.TempDir(),
	}
	err := WatchCheck(context.Background(), cfg, &model.StreamParams{}, "", model.MediaTypeUnknown, deps)
	if err != nil {
		t.Fatalf("WatchCheck() returned error when per-artist failure should be non-fatal: %v", err)
	}

	// Both artists must have been attempted.
	if len(callOrder) < 2 {
		t.Errorf("expected both artists to be attempted, callOrder = %v", callOrder)
	}
}

// TestWatchCheck_ContextCancellationStopsLoop verifies WatchCheck respects ctx cancellation
// between artists.
func TestWatchCheck_ContextCancellationStopsLoop(t *testing.T) {
	testutil.WithTempHome(t)

	callCount := 0
	deps := &Deps{
		FetchCatalog: func(_ context.Context) (*model.LatestCatalogResp, error) {
			return buildUpdateCatalog(nil), nil
		},
		FormatDuration: noDurationFmt,
		GetArtistMetaCached: func(_ context.Context, _ string, _ time.Duration) ([]*model.ArtistMeta, bool, bool, error) {
			callCount++
			return []*model.ArtistMeta{emptyArtistMeta("Artist")}, false, false, nil
		},
		GetShowMediaType: func(_ *model.AlbArtResp) model.MediaType { return model.MediaTypeAudio },
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	cfg := &model.Config{
		WatchedArtists: []string{"1125", "461", "1045"},
		OutPath:        t.TempDir(),
	}
	err := WatchCheck(ctx, cfg, &model.StreamParams{}, "", model.MediaTypeUnknown, deps)
	if err == nil {
		t.Fatal("WatchCheck() error = nil, want context.Canceled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("WatchCheck() error = %v, want context.Canceled", err)
	}
	// With a pre-cancelled context, no artist should be processed.
	if callCount > 0 {
		t.Errorf("GetArtistMetaCached called %d times, want 0 for pre-cancelled context", callCount)
	}
}

// emptyArtistMeta returns an ArtistMeta with no containers (shows) for the given artist name.
func emptyArtistMeta(artistName string) *model.ArtistMeta {
	m := &model.ArtistMeta{}
	m.Response.Containers = nil
	// ArtistName in the struct is typed `any` — CollectArtistShows reads it from containers.
	_ = artistName
	return m
}

// TestToSystemdDuration verifies that Go duration strings are converted to
// systemd-safe time(7) format, with 'm' (Go minutes) becoming 'min' (systemd minutes).
func TestToSystemdDuration(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"1h", "1h", false},
		{"6h", "6h", false},
		{"30m", "30min", false},    // critical: Go 'm' → systemd 'min', not months
		{"90m", "1h30min", false},  // mixed hours + minutes
		{"45m", "45min", false},
		{"3600s", "1h", false},
		{"1h30m", "1h30min", false},
		{"30s", "30s", false},
		{"0s", "", true},           // zero duration rejected
		{"-1h", "", true},          // negative rejected
		{"2x", "", true},           // invalid format
		{"every day", "", true},    // completely invalid
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			// toSystemdDuration is only compiled on Linux; skip gracefully on other platforms.
			got, err := toSystemdDuration(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("toSystemdDuration(%q) error = nil, want error", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("toSystemdDuration(%q) error = %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("toSystemdDuration(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestWatchEnableUnitContent verifies the generated systemd unit files contain
// the correct ExecStart path and OnUnitActiveSec value.
// Uses writeWatchUnitFiles directly to avoid requiring a live systemd session.
func TestWatchEnableUnitContent(t *testing.T) {
	testutil.WithTempHome(t)

	cfg := &model.Config{
		WatchedArtists: []string{"1125"},
		WatchInterval:  "6h",
	}

	unitDir := t.TempDir()
	fakeBin := "/usr/local/bin/nugs"

	if err := writeWatchUnitFiles(cfg, unitDir, fakeBin); err != nil {
		if strings.Contains(err.Error(), "require Linux") {
			t.Skip("systemd unit files not supported on this platform")
		}
		t.Fatalf("writeWatchUnitFiles() error = %v", err)
	}

	// Verify service file contains ExecStart pointing to our binary.
	servicePath := filepath.Join(unitDir, "nugs-watch.service")
	serviceContent, err := os.ReadFile(servicePath)
	if err != nil {
		t.Fatalf("failed to read service file: %v", err)
	}
	wantExecStart := "ExecStart=" + fakeBin + " watch check"
	if !strings.Contains(string(serviceContent), wantExecStart) {
		t.Errorf("service file missing %q:\n%s", wantExecStart, serviceContent)
	}

	// Verify timer file uses the configured interval (6h stays "6h" after conversion).
	timerPath := filepath.Join(unitDir, "nugs-watch.timer")
	timerContent, err := os.ReadFile(timerPath)
	if err != nil {
		t.Fatalf("failed to read timer file: %v", err)
	}
	if !strings.Contains(string(timerContent), "OnUnitActiveSec=6h") {
		t.Errorf("timer file does not contain 'OnUnitActiveSec=6h':\n%s", timerContent)
	}
}

// TestSendWatchSummary verifies the notification logic: correct title, priority, and
// silent behaviour for each outcome combination.
func TestSendWatchSummary(t *testing.T) {
	tests := []struct {
		name         string
		downloaded   int
		failed       int
		errs         []string
		wantCalled   bool
		wantTitle    string
		wantPriority int
		wantContains []string // substrings that must appear in message
		wantAbsent   []string // substrings that must NOT appear in message
	}{
		{
			name:       "nothing new — silent",
			downloaded: 0, failed: 0, errs: nil,
			wantCalled: false,
		},
		{
			name:         "downloads only",
			downloaded:   3, failed: 0, errs: nil,
			wantCalled:   true,
			wantTitle:    "Nugs Watch",
			wantPriority: 5,
			wantContains: []string{"3 new show(s)"},
			wantAbsent:   []string{"failed", "error"},
		},
		{
			name:         "downloads with some failures",
			downloaded:   2, failed: 1, errs: nil,
			wantCalled:   true,
			wantTitle:    "Nugs Watch",
			wantPriority: 5,
			wantContains: []string{"2 new show(s)", "1 failed"},
		},
		{
			name:         "downloads with artist errors",
			downloaded:   4, failed: 0, errs: []string{"1125: timeout"},
			wantCalled:   true,
			wantTitle:    "Nugs Watch",
			wantPriority: 5,
			wantContains: []string{"4 new show(s)", "1 artist error(s)"},
		},
		{
			name:         "download failures only — no artist errors",
			downloaded:   0, failed: 2, errs: nil,
			wantCalled:   true,
			wantTitle:    "Nugs Watch Error",
			wantPriority: 7,
			wantContains: []string{"2 download failure(s)"},
		},
		{
			name:         "artist errors only — no download failures",
			downloaded:   0, failed: 0, errs: []string{"461: network error"},
			wantCalled:   true,
			wantTitle:    "Nugs Watch Error",
			wantPriority: 7,
			wantContains: []string{"461: network error"},
			// Must NOT start with "0 download failure(s)" when only artist errors exist.
			wantAbsent: []string{"0 download failure(s)"},
		},
		{
			name:         "artist errors and download failures combined",
			downloaded:   0, failed: 1, errs: []string{"461: timeout"},
			wantCalled:   true,
			wantTitle:    "Nugs Watch Error",
			wantPriority: 7,
			wantContains: []string{"1 download failure(s)", "461: timeout"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fn, gotCalled, gotTitle, gotMsg, gotPriority := notifyCapture()
			sendWatchSummary(context.Background(), fn, tc.downloaded, tc.failed, tc.errs)

			if *gotCalled != tc.wantCalled {
				t.Fatalf("notify called = %v, want %v", *gotCalled, tc.wantCalled)
			}
			if !tc.wantCalled {
				return
			}
			if *gotTitle != tc.wantTitle {
				t.Errorf("title = %q, want %q", *gotTitle, tc.wantTitle)
			}
			if *gotPriority != tc.wantPriority {
				t.Errorf("priority = %d, want %d", *gotPriority, tc.wantPriority)
			}
			for _, sub := range tc.wantContains {
				if !strings.Contains(*gotMsg, sub) {
					t.Errorf("message %q does not contain %q", *gotMsg, sub)
				}
			}
			for _, sub := range tc.wantAbsent {
				if strings.Contains(*gotMsg, sub) {
					t.Errorf("message %q must not contain %q", *gotMsg, sub)
				}
			}
		})
	}
}

// TestWatchEnableUnitContent_MinutesConversion verifies that a minutes-based interval
// is correctly emitted as 'min' (not 'm') in the timer file.
func TestWatchEnableUnitContent_MinutesConversion(t *testing.T) {
	unitDir := t.TempDir()
	cfg := &model.Config{
		WatchedArtists: []string{"1125"},
		WatchInterval:  "30m",
	}

	if err := writeWatchUnitFiles(cfg, unitDir, "/usr/bin/nugs"); err != nil {
		if strings.Contains(err.Error(), "require Linux") {
			t.Skip("systemd unit files not supported on this platform")
		}
		t.Fatalf("writeWatchUnitFiles() error = %v", err)
	}

	timerContent, err := os.ReadFile(filepath.Join(unitDir, "nugs-watch.timer"))
	if err != nil {
		t.Fatalf("failed to read timer file: %v", err)
	}
	content := string(timerContent)

	// Must use 'min' (minutes), not 'm' (which systemd interprets as months).
	if strings.Contains(content, "OnUnitActiveSec=30m\n") || strings.Contains(content, "OnUnitActiveSec=30m ") {
		t.Error("timer file uses '30m' which systemd interprets as 30 months, not 30 minutes")
	}
	if !strings.Contains(content, "OnUnitActiveSec=30min") {
		t.Errorf("timer file should use '30min' for minutes:\n%s", content)
	}
}
