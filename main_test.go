package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBuildAlbumFolderName_TruncatesByRunes(t *testing.T) {
	artist := "José González"
	container := strings.Repeat("漢", 200)

	name := buildAlbumFolderName(artist, container, 120)
	if !utf8.ValidString(name) {
		t.Fatalf("expected valid UTF-8 folder name")
	}
	if got := len([]rune(name)); got > 120 {
		t.Fatalf("expected <= 120 runes, got %d", got)
	}
}

func TestBuildRcloneUploadCommand_UsesCopyForDirectory(t *testing.T) {
	tmp := t.TempDir()
	localDir := filepath.Join(tmp, "album-dir")
	if err := os.MkdirAll(localDir, 0755); err != nil {
		t.Fatalf("failed to create local dir: %v", err)
	}

	cfg := &Config{
		RcloneRemote: "gdrive",
		RclonePath:   "/Music/Nugs",
	}

	cmd, remotePath, err := buildRcloneUploadCommand(localDir, "Artist", cfg, 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cmd.Args) < 2 || cmd.Args[1] != "copy" {
		t.Fatalf("expected rclone copy for directory, got args: %v", cmd.Args)
	}
	expectedRemote := "gdrive:/Music/Nugs/Artist/album-dir"
	if remotePath != expectedRemote {
		t.Fatalf("unexpected remote path: got %q want %q", remotePath, expectedRemote)
	}
}

func TestBuildRcloneUploadCommand_UsesCopytoForFile(t *testing.T) {
	tmp := t.TempDir()
	localFile := filepath.Join(tmp, "video.mp4")
	if err := os.WriteFile(localFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create local file: %v", err)
	}

	cfg := &Config{
		RcloneRemote: "gdrive",
		RclonePath:   "/Music/Nugs",
	}

	cmd, remotePath, err := buildRcloneUploadCommand(localFile, "Artist", cfg, 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cmd.Args) < 2 || cmd.Args[1] != "copyto" {
		t.Fatalf("expected rclone copyto for file, got args: %v", cmd.Args)
	}
	expectedRemote := "gdrive:/Music/Nugs/Artist/video.mp4"
	if remotePath != expectedRemote {
		t.Fatalf("unexpected remote path: got %q want %q", remotePath, expectedRemote)
	}
}

func TestBuildRcloneVerifyCommand_UsesDirectCheckForDirectory(t *testing.T) {
	tmp := t.TempDir()
	localDir := filepath.Join(tmp, "album-dir")
	if err := os.MkdirAll(localDir, 0755); err != nil {
		t.Fatalf("failed to create local dir: %v", err)
	}

	cmd, err := buildRcloneVerifyCommand(localDir, "gdrive:/Music/Nugs/Artist/album-dir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"rclone", "check", "--one-way", localDir, "gdrive:/Music/Nugs/Artist/album-dir"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("unexpected verify args: got %v want %v", cmd.Args, want)
	}
}

func TestBuildRcloneVerifyCommand_UsesIncludeForFile(t *testing.T) {
	tmp := t.TempDir()
	localFile := filepath.Join(tmp, "video.mp4")
	if err := os.WriteFile(localFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create local file: %v", err)
	}

	cmd, err := buildRcloneVerifyCommand(localFile, "gdrive:/Music/Nugs/Artist/video.mp4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{
		"rclone", "check", "--one-way", "--include", "video.mp4",
		filepath.Dir(localFile), "gdrive:/Music/Nugs/Artist",
	}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("unexpected verify args: got %v want %v", cmd.Args, want)
	}
}
