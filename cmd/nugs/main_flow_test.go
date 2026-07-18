package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jmagar/nugs-cli/internal/testutil"
)

func TestRealMainVersionDoesNotRequireConfig(t *testing.T) {
	testutil.WithTempHome(t)
	testutil.ChdirTemp(t)
	oldArgs := os.Args
	os.Args = []string{"nugs", "version"}
	t.Cleanup(func() { os.Args = oldArgs })

	output := testutil.CaptureStdout(t, func() {
		if code := realMain(); code != 0 {
			t.Fatalf("realMain() exit code = %d, want 0", code)
		}
	})
	if !strings.Contains(output, "nugs ") {
		t.Fatalf("version output = %q", output)
	}
}

func TestRunFinalizesFailedRuntimeStatus(t *testing.T) {
	testutil.WithTempHome(t)
	t.Setenv("PATH", "")
	t.Setenv("NUGS_DETACHED", "1")
	cfg := &Config{
		Urls:          []string{"23329"},
		RcloneEnabled: true,
	}

	err := run(cfg, "")
	if err == nil {
		t.Fatal("run() error = nil, want missing rclone error")
	}
	status, statusErr := readRuntimeStatus()
	if statusErr != nil {
		t.Fatalf("readRuntimeStatus() error = %v", statusErr)
	}
	if status.State != "failed" {
		t.Fatalf("runtime state = %q, want failed", status.State)
	}
}

func TestDispatchReturnsAggregateErrorForInvalidItem(t *testing.T) {
	cfg := &Config{Urls: []string{"not-a-nugs-url"}}
	cancelled, err := dispatch(context.Background(), cfg, nil, "", "")
	if cancelled {
		t.Fatal("dispatch() cancelled = true, want false")
	}
	if err == nil || !strings.Contains(err.Error(), "invalid URL") {
		t.Fatalf("dispatch() error = %v, want invalid URL", err)
	}
}

func TestArtistShorthandReturnsErrorInsteadOfExiting(t *testing.T) {
	cfg := &Config{Urls: []string{"1125", "gaps"}}
	handled, err := handleArtistShorthand(cfg)
	if !handled || err == nil {
		t.Fatalf("handleArtistShorthand() = (%v, %v), want handled error", handled, err)
	}
}

func TestCommandValidationReturnsErrors(t *testing.T) {
	tests := []struct {
		name string
		urls []string
		call func(*Config) (bool, error)
	}{
		{name: "invalid list limit", urls: []string{"list", "1125", "latest", "0"}, call: func(cfg *Config) (bool, error) { return handleListCommand(context.Background(), cfg, "") }},
		{name: "invalid catalog limit", urls: []string{"catalog", "latest", "0"}, call: func(cfg *Config) (bool, error) { return handleCatalogCommand(context.Background(), cfg, "") }},
		{name: "missing gaps artist", urls: []string{"catalog", "gaps", "--ids-only"}, call: func(cfg *Config) (bool, error) { return handleCatalogCommand(context.Background(), cfg, "") }},
		{name: "unknown catalog command", urls: []string{"catalog", "bogus"}, call: func(cfg *Config) (bool, error) { return handleCatalogCommand(context.Background(), cfg, "") }},
		{name: "missing watch add artist", urls: []string{"watch", "add"}, call: func(cfg *Config) (bool, error) { return handleWatchCommand(context.Background(), cfg, "") }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handled, err := tc.call(&Config{Urls: tc.urls})
			if !handled || err == nil {
				t.Fatalf("command = handled %v error %v, want handled error", handled, err)
			}
		})
	}
}
