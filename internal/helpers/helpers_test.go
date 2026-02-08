package helpers

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/jmagar/nugs-cli/internal/model"
)

func TestBuildAlbumFolderName_TruncatesByRunes(t *testing.T) {
	artist := "José González"
	container := strings.Repeat("漢", 200)

	name := BuildAlbumFolderName(artist, container, 120)
	if !utf8.ValidString(name) {
		t.Fatalf("expected valid UTF-8 folder name")
	}
	if got := len([]rune(name)); got > 120 {
		t.Fatalf("expected <= 120 runes, got %d", got)
	}
}

func TestGetVideoOutPath_DefaultsToOutPath(t *testing.T) {
	cfg := &model.Config{OutPath: "/music/audio"}
	if got := GetVideoOutPath(cfg); got != "/music/audio" {
		t.Fatalf("expected video path to default to outPath, got %q", got)
	}

	cfg.VideoOutPath = "/music/video"
	if got := GetVideoOutPath(cfg); got != "/music/video" {
		t.Fatalf("expected configured videoOutPath, got %q", got)
	}
}

func TestGetRcloneBasePath_UsesVideoPathWhenRequested(t *testing.T) {
	cfg := &model.Config{
		RclonePath:      "/remote/audio",
		RcloneVideoPath: "/remote/video",
	}

	if got := GetRcloneBasePath(cfg, false); got != "/remote/audio" {
		t.Fatalf("expected audio rclone path, got %q", got)
	}
	if got := GetRcloneBasePath(cfg, true); got != "/remote/video" {
		t.Fatalf("expected video rclone path, got %q", got)
	}

	cfg.RcloneVideoPath = ""
	if got := GetRcloneBasePath(cfg, true); got != "/remote/audio" {
		t.Fatalf("expected video path fallback to audio path, got %q", got)
	}
}
