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
