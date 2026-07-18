package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jmagar/nugs-cli/internal/testutil"
)

func TestResolveFfmpegBinary_DoesNotImplicitlyUseCurrentDirectory(t *testing.T) {
	tmp := testutil.ChdirTemp(t)
	local := filepath.Join(tmp, "ffmpeg")
	testutil.WriteExecutable(t, local)

	t.Setenv("PATH", "")

	cfg := &Config{UseFfmpegEnvVar: false}
	if _, err := resolveFfmpegBinary(cfg); err == nil {
		t.Fatal("resolveFfmpegBinary implicitly trusted ./ffmpeg")
	}
}

func TestResolveFfmpegBinary_ExplicitAbsolutePath(t *testing.T) {
	tmp := t.TempDir()
	local := filepath.Join(tmp, "ffmpeg")
	testutil.WriteExecutable(t, local)
	t.Setenv("PATH", "")

	got, err := resolveFfmpegBinary(&Config{FfmpegNameStr: local})
	if err != nil {
		t.Fatalf("resolveFfmpegBinary returned error: %v", err)
	}
	if got != local {
		t.Fatalf("got %q, want %q", got, local)
	}
}

func TestResolveFfmpegBinary_PathFallback(t *testing.T) {
	tmp := testutil.ChdirTemp(t)
	binDir := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	ffmpegPath := filepath.Join(binDir, "ffmpeg")
	testutil.WriteExecutable(t, ffmpegPath)

	t.Setenv("PATH", binDir)

	cfg := &Config{UseFfmpegEnvVar: false}
	got, err := resolveFfmpegBinary(cfg)
	if err != nil {
		t.Fatalf("resolveFfmpegBinary returned error: %v", err)
	}
	if got != ffmpegPath {
		t.Fatalf("expected %q, got %q", ffmpegPath, got)
	}
}

func TestResolveFfmpegBinary_MissingReturnsError(t *testing.T) {
	_ = testutil.ChdirTemp(t)
	t.Setenv("PATH", "")

	cfg := &Config{UseFfmpegEnvVar: false}
	_, err := resolveFfmpegBinary(cfg)
	if err == nil {
		t.Fatal("expected error when ffmpeg missing, got nil")
	}
}
