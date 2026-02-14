package helpers

import (
	"errors"
	"os"
	"path/filepath"
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

func TestValidatePath_ReturnsSentinelErrors(t *testing.T) {
	invalidCharErr := ValidatePath("bad\npath")
	if invalidCharErr == nil {
		t.Fatal("expected invalid char error")
	}
	if !errors.Is(invalidCharErr, ErrInvalidPathCharacters) {
		t.Fatalf("expected ErrInvalidPathCharacters, got %v", invalidCharErr)
	}

	traversalErr := ValidatePath("../escape")
	if traversalErr == nil {
		t.Fatal("expected traversal error")
	}
	if !errors.Is(traversalErr, ErrPathTraversalDetected) {
		t.Fatalf("expected ErrPathTraversalDetected, got %v", traversalErr)
	}
}

func TestReadTxtFile_WrapsOpenErrorWithSentinel(t *testing.T) {
	_, err := ReadTxtFile(filepath.Join(t.TempDir(), "missing.txt"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !errors.Is(err, ErrOpenTextFile) {
		t.Fatalf("expected ErrOpenTextFile, got %v", err)
	}
}

func TestConfigPathResolver_LocalBasesForFilter(t *testing.T) {
	cfg := &model.Config{
		OutPath:      "/music/audio",
		VideoOutPath: "/music/video",
	}
	resolver := NewConfigPathResolver(cfg)

	if got := resolver.LocalBasesForFilter(model.MediaTypeAudio); len(got) != 1 || got[0] != "/music/audio" {
		t.Fatalf("unexpected audio bases: %#v", got)
	}
	if got := resolver.LocalBasesForFilter(model.MediaTypeVideo); len(got) != 1 || got[0] != "/music/video" {
		t.Fatalf("unexpected video bases: %#v", got)
	}
	if got := resolver.LocalBasesForFilter(model.MediaTypeBoth); len(got) != 2 {
		t.Fatalf("expected two base paths for both, got %#v", got)
	}

	cfg.VideoOutPath = "/music/audio"
	got := NewConfigPathResolver(cfg).LocalBasesForFilter(model.MediaTypeBoth)
	if len(got) != 1 || got[0] != "/music/audio" {
		t.Fatalf("expected deduped paths, got %#v", got)
	}
}

func TestConfigPathResolver_ShowPaths(t *testing.T) {
	cfg := &model.Config{OutPath: "/music/audio", VideoOutPath: "/music/video"}
	resolver := NewConfigPathResolver(cfg)
	show := &model.AlbArtResp{
		ArtistName:    "Artist/Name",
		ContainerInfo: "Live at Place",
	}

	audioPath := resolver.LocalShowPath(show, model.MediaTypeAudio)
	if !strings.HasPrefix(audioPath, filepath.Join("/music/audio", "Artist_Name")+string(os.PathSeparator)) {
		t.Fatalf("unexpected audio path %q", audioPath)
	}
	videoPath := resolver.LocalShowPath(show, model.MediaTypeVideo)
	if !strings.HasPrefix(videoPath, filepath.Join("/music/video", "Artist_Name")+string(os.PathSeparator)) {
		t.Fatalf("unexpected video path %q", videoPath)
	}
	remotePath := resolver.RemoteShowPath(show)
	if !strings.HasPrefix(remotePath, "Artist_Name"+string(os.PathSeparator)) {
		t.Fatalf("unexpected remote path %q", remotePath)
	}
}
