package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jmagar/nugs-cli/internal/testutil"
)

func TestResolveFfmpegBinary_LocalPreferred(t *testing.T) {
	tmp := testutil.ChdirTemp(t)
	local := filepath.Join(tmp, "ffmpeg")
	testutil.WriteExecutable(t, local)

	t.Setenv("PATH", "")

	cfg := &Config{UseFfmpegEnvVar: false}
	got, err := resolveFfmpegBinary(cfg)
	if err != nil {
		t.Fatalf("resolveFfmpegBinary returned error: %v", err)
	}
	if got != "./ffmpeg" {
		t.Fatalf("expected ./ffmpeg, got %q", got)
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
