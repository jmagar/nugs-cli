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

func TestParsePaidLstreamShowID(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		want      string
		wantError bool
	}{
		{name: "valid", query: "showID=12345&foo=bar", want: "12345"},
		{name: "missing showID", query: "foo=bar", wantError: true},
		{name: "blank showID", query: "showID=   ", wantError: true},
		{name: "invalid query", query: "%", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePaidLstreamShowID(tt.query)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error, got nil and value %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected showID: got %q want %q", got, tt.want)
			}
		})
	}
}

func TestIsLikelyLivestreamSegments(t *testing.T) {
	tests := []struct {
		name      string
		segments  []string
		want      bool
		wantError bool
	}{
		{name: "no segments", segments: nil, wantError: true},
		{name: "single segment", segments: []string{"seg0.ts"}, want: false},
		{name: "same first two", segments: []string{"seg0.ts", "seg0.ts"}, want: false},
		{name: "different first two", segments: []string{"seg0.ts", "seg1.ts"}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := isLikelyLivestreamSegments(tt.segments)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected value: got %v want %v", got, tt.want)
			}
		})
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
