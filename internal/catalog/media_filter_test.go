package catalog

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jmagar/nugs-cli/internal/helpers"
	"github.com/jmagar/nugs-cli/internal/model"
)

func TestShowExistsForMedia_RemoteFallbackUsesMediaFlag(t *testing.T) {
	cfg := &model.Config{
		OutPath:       t.TempDir(),
		VideoOutPath:  t.TempDir(),
		RcloneEnabled: true,
	}
	show := &model.AlbArtResp{
		ArtistName:    "Billy Strings",
		ContainerInfo: "2024-01-01 - Denver, CO",
	}

	var (
		gotRemotePath string
		gotIsVideo    bool
	)
	deps := &Deps{
		RemotePathExists: func(_ context.Context, remotePath string, _ *model.Config, isVideo bool) (bool, error) {
			gotRemotePath = remotePath
			gotIsVideo = isVideo
			return true, nil
		},
	}

	if !ShowExistsForMedia(context.Background(), show, cfg, model.MediaTypeVideo, deps) {
		t.Fatal("expected remote fallback to report show exists")
	}

	expectedRemote := helpers.NewConfigPathResolver(cfg).RemoteShowPath(show)
	if gotRemotePath != expectedRemote {
		t.Fatalf("expected remote path %q, got %q", expectedRemote, gotRemotePath)
	}
	if !gotIsVideo {
		t.Fatal("expected video flag to be true for video media type")
	}
}

func TestBuildArtistPresenceIndex_BothFilterReadsAudioAndVideoTrees(t *testing.T) {
	audioRoot := t.TempDir()
	videoRoot := t.TempDir()
	cfg := &model.Config{
		OutPath:      audioRoot,
		VideoOutPath: videoRoot,
	}
	artist := "Billy Strings"
	artistFolder := helpers.Sanitise(artist)

	audioShow := helpers.BuildAlbumFolderName(artist, "Audio Show")
	videoShow := helpers.BuildAlbumFolderName(artist, "Video Show")

	if err := os.MkdirAll(filepath.Join(audioRoot, artistFolder, audioShow), 0o755); err != nil {
		t.Fatalf("mkdir audio show: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(videoRoot, artistFolder, videoShow), 0o755); err != nil {
		t.Fatalf("mkdir video show: %v", err)
	}

	idx := BuildArtistPresenceIndex(context.Background(), artist, cfg, &Deps{}, model.MediaTypeBoth)

	if _, ok := idx.LocalFolders[audioShow]; !ok {
		t.Fatalf("expected audio show %q in local folder index", audioShow)
	}
	if _, ok := idx.LocalFolders[videoShow]; !ok {
		t.Fatalf("expected video show %q in local folder index", videoShow)
	}
}

func TestListAllRemoteArtistFolders_WrapsErrorWithSentinel(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses POSIX shell script, not portable to Windows")
	}

	binDir := t.TempDir()
	rclonePath := filepath.Join(binDir, "rclone")
	if err := os.WriteFile(rclonePath, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write mock rclone: %v", err)
	}

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cfg := &model.Config{
		RcloneEnabled: true,
		RcloneRemote:  "test",
		RclonePath:    "/music",
	}

	_, err := ListAllRemoteArtistFolders(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected non-nil error from failing rclone command")
	}
	if !errors.Is(err, ErrRemoteArtistFolderListFailed) {
		t.Fatalf("expected ErrRemoteArtistFolderListFailed, got %v", err)
	}
}
