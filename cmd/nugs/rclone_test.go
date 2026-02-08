package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

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

	cmd, remotePath, err := buildRcloneUploadCommand(localDir, "Artist", cfg, 4, false)
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

	cmd, remotePath, err := buildRcloneUploadCommand(localFile, "Artist", cfg, 4, false)
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

func TestBuildRcloneUploadCommand_UsesVideoRemotePath(t *testing.T) {
	tmp := t.TempDir()
	localFile := filepath.Join(tmp, "video.mp4")
	if err := os.WriteFile(localFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create local file: %v", err)
	}

	cfg := &Config{
		RcloneRemote:    "gdrive",
		RclonePath:      "/Music/Nugs",
		RcloneVideoPath: "/Videos/Nugs",
	}

	_, remotePath, err := buildRcloneUploadCommand(localFile, "Artist", cfg, 4, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedRemote := "gdrive:/Videos/Nugs/Artist/video.mp4"
	if remotePath != expectedRemote {
		t.Fatalf("unexpected remote path: got %q want %q", remotePath, expectedRemote)
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

func TestParseRcloneProgressLine(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantOK    bool
		wantPct   int
		wantSpeed string
		wantDone  string
		wantTotal string
	}{
		{
			name:      "valid line",
			line:      "Transferred:   1.234 GiB / 3.210 GiB, 38%, 12.3 MiB/s, ETA 2m30s",
			wantOK:    true,
			wantPct:   38,
			wantSpeed: "12.3 MiB/s",
			wantDone:  "1.234 GiB",
			wantTotal: "3.210 GiB",
		},
		{
			name:   "invalid line",
			line:   "INFO  : copied 10 files",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pct, speed, done, total, ok := parseRcloneProgressLine(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("parseRcloneProgressLine ok=%v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if pct != tt.wantPct {
				t.Fatalf("pct=%d want=%d", pct, tt.wantPct)
			}
			if speed != tt.wantSpeed {
				t.Fatalf("speed=%q want=%q", speed, tt.wantSpeed)
			}
			if done != tt.wantDone {
				t.Fatalf("done=%q want=%q", done, tt.wantDone)
			}
			if total != tt.wantTotal {
				t.Fatalf("total=%q want=%q", total, tt.wantTotal)
			}
		})
	}
}
