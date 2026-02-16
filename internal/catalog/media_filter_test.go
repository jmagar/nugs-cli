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

func TestBuildArtistPresenceIndex_BothFilterReadsAudioAndVideoRemoteTrees(t *testing.T) {
	cfg := &model.Config{
		OutPath:       t.TempDir(),
		VideoOutPath:  t.TempDir(),
		RcloneEnabled: true,
	}

	audioShow := "Audio Remote Show"
	videoShow := "Video Remote Show"

	deps := &Deps{
		ListRemoteArtistFolders: func(_ context.Context, _ string, _ *model.Config, isVideo bool) (map[string]struct{}, error) {
			if isVideo {
				return map[string]struct{}{videoShow: {}}, nil
			}
			return map[string]struct{}{audioShow: {}}, nil
		},
	}

	idx := BuildArtistPresenceIndex(context.Background(), "Billy Strings", cfg, deps, model.MediaTypeBoth)

	if _, ok := idx.RemoteFolders[audioShow]; !ok {
		t.Fatalf("expected audio remote show %q in index", audioShow)
	}
	if _, ok := idx.RemoteFolders[videoShow]; !ok {
		t.Fatalf("expected video remote show %q in index", videoShow)
	}
}

func TestIsShowDownloadable(t *testing.T) {
	tests := []struct {
		name string
		show *model.AlbArtResp
		want bool
	}{
		{
			name: "nil show is not downloadable",
			show: nil,
			want: false,
		},
		{
			name: "empty availability with no content is not downloadable",
			show: &model.AlbArtResp{},
			want: false,
		},
		{
			name: "preorder empty metadata is not downloadable",
			show: &model.AlbArtResp{
				AvailabilityTypeStr: "PREORDER",
			},
			want: false,
		},
		{
			name: "available with product formats is downloadable",
			show: &model.AlbArtResp{
				AvailabilityTypeStr: model.AvailableAvailabilityType,
				ProductFormatList:   []*model.ProductFormatList{{FormatStr: "16-bit / 44.1 kHz FLAC"}},
			},
			want: true,
		},
		{
			name: "available with tracks is downloadable",
			show: &model.AlbArtResp{
				AvailabilityTypeStr: model.AvailableAvailabilityType,
				Tracks:              []model.Track{{TrackID: 1}},
			},
			want: true,
		},
		{
			name: "available with only songs is downloadable",
			show: &model.AlbArtResp{
				AvailabilityTypeStr: model.AvailableAvailabilityType,
				Songs:               []model.Track{{TrackID: 1}},
			},
			want: true,
		},
		{
			name: "available with only products is downloadable",
			show: &model.AlbArtResp{
				AvailabilityTypeStr: model.AvailableAvailabilityType,
				Products:            []model.Product{{SkuID: 1}},
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsShowDownloadable(tc.show)
			if got != tc.want {
				t.Fatalf("IsShowDownloadable() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMatchesMediaFilter(t *testing.T) {
	tests := []struct {
		name      string
		showMedia model.MediaType
		filter    model.MediaType
		want      bool
	}{
		{
			name:      "unknown filter matches all shows",
			showMedia: model.MediaTypeAudio,
			filter:    model.MediaTypeUnknown,
			want:      true,
		},
		{
			name:      "both filter matches all shows",
			showMedia: model.MediaTypeVideo,
			filter:    model.MediaTypeBoth,
			want:      true,
		},
		{
			name:      "audio filter matches audio-only show",
			showMedia: model.MediaTypeAudio,
			filter:    model.MediaTypeAudio,
			want:      true,
		},
		{
			name:      "audio filter matches both-media show",
			showMedia: model.MediaTypeBoth,
			filter:    model.MediaTypeAudio,
			want:      true,
		},
		{
			name:      "audio filter excludes video-only show",
			showMedia: model.MediaTypeVideo,
			filter:    model.MediaTypeAudio,
			want:      false,
		},
		{
			name:      "video filter matches both-media show",
			showMedia: model.MediaTypeBoth,
			filter:    model.MediaTypeVideo,
			want:      true,
		},
		{
			name:      "video filter excludes audio-only show",
			showMedia: model.MediaTypeAudio,
			filter:    model.MediaTypeVideo,
			want:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := MatchesMediaFilter(tc.showMedia, tc.filter)
			if got != tc.want {
				t.Fatalf("MatchesMediaFilter(%v, %v) = %v, want %v",
					tc.showMedia, tc.filter, got, tc.want)
			}
		})
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
