package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeExecutable(t *testing.T, path string) {
	t.Helper()
	content := []byte("#!/bin/sh\nexit 0\n")
	if runtime.GOOS == "windows" {
		content = []byte("@echo off\r\nexit /b 0\r\n")
	}
	if err := os.WriteFile(path, content, 0755); err != nil {
		t.Fatalf("failed to write executable %s: %v", path, err)
	}
}

func chdirTemp(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("failed to chdir temp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(orig)
	})
	return tmp
}

func TestResolveFfmpegBinary_LocalPreferred(t *testing.T) {
	tmp := chdirTemp(t)
	local := filepath.Join(tmp, "ffmpeg")
	writeExecutable(t, local)

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
	tmp := chdirTemp(t)
	binDir := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	ffmpegPath := filepath.Join(binDir, "ffmpeg")
	writeExecutable(t, ffmpegPath)

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
	_ = chdirTemp(t)
	t.Setenv("PATH", "")

	cfg := &Config{UseFfmpegEnvVar: false}
	_, err := resolveFfmpegBinary(cfg)
	if err == nil {
		t.Fatal("expected error when ffmpeg missing, got nil")
	}
}
