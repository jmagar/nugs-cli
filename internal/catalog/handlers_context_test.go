package catalog

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jmagar/nugs-cli/internal/helpers"
	"github.com/jmagar/nugs-cli/internal/model"
)

func TestBuildGapFillErrorContext_IncludesLocalAndRemoteChecks(t *testing.T) {
	audioRoot := t.TempDir()
	videoRoot := t.TempDir()
	cfg := &model.Config{
		OutPath:         audioRoot,
		VideoOutPath:    videoRoot,
		RcloneEnabled:   true,
		RcloneRemote:    "tootie",
		RclonePath:      "/mnt/audio",
		RcloneVideoPath: "/mnt/video",
	}
	show := &model.AlbArtResp{
		ArtistName:          "Billy Strings",
		ContainerInfo:       "06/07/25 Allstate Arena, Rosemont, IL",
		AvailabilityTypeStr: "AVAILABLE",
		ActiveState:         "AVAILABLE",
	}

	audioPath := helpers.NewConfigPathResolver(cfg).LocalShowPath(show, model.MediaTypeAudio)
	if err := helpers.MakeDirs(audioPath); err != nil {
		t.Fatalf("failed to create audio path: %v", err)
	}

	deps := &Deps{
		RemotePathExists: func(_ context.Context, remotePath string, _ *model.Config, isVideo bool) (bool, error) {
			if isVideo {
				return false, nil
			}
			return true, nil
		},
	}

	info := buildGapFillErrorContext(context.Background(), show, cfg, deps)

	if !info.LocalAudioExists {
		t.Fatal("expected LocalAudioExists=true")
	}
	if info.LocalVideoExists {
		t.Fatal("expected LocalVideoExists=false")
	}
	if !info.RemoteAudioExists {
		t.Fatal("expected RemoteAudioExists=true")
	}
	if info.RemoteVideoExists {
		t.Fatal("expected RemoteVideoExists=false")
	}
	if info.RemoteRelativePath == "" {
		t.Fatal("expected RemoteRelativePath to be set")
	}
	if info.LocalAudioPath == "" {
		t.Fatal("expected LocalAudioPath to be set")
	}
}

func TestDeriveGapFillReasonHint(t *testing.T) {
	tests := []struct {
		name string
		err  error
		info gapFillErrorContext
		want string
	}{
		{
			name: "preorder placeholder",
			info: gapFillErrorContext{
				AvailabilityType: "PREORDER",
			},
			want: "Preorder/placeholder container (not released yet)",
		},
		{
			name: "remote exists",
			info: gapFillErrorContext{
				RemoteAudioExists: true,
			},
			want: "Already exists on remote (naming/path mismatch likely)",
		},
		{
			name: "local exists",
			info: gapFillErrorContext{
				LocalAudioExists: true,
			},
			want: "Already exists locally (naming/path mismatch likely)",
		},
		{
			name: "empty metadata error",
			err:  model.ErrReleaseHasNoContent,
			info: gapFillErrorContext{},
			want: "Metadata has no downloadable tracks/videos yet",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := deriveGapFillReasonHint(tc.err, tc.info)
			if got != tc.want {
				t.Fatalf("deriveGapFillReasonHint() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildGapFillErrorContext_CapturesRemoteErrors(t *testing.T) {
	cfg := &model.Config{
		OutPath:       t.TempDir(),
		VideoOutPath:  t.TempDir(),
		RcloneEnabled: true,
		RcloneRemote:  "gdrive",
		RclonePath:    "/music",
	}
	show := &model.AlbArtResp{
		ArtistName:    "Artist",
		ContainerInfo: "Show",
	}

	deps := &Deps{
		RemotePathExists: func(_ context.Context, _ string, _ *model.Config, isVideo bool) (bool, error) {
			if isVideo {
				return false, errors.New("video remote timeout")
			}
			return false, errors.New("audio remote permission denied")
		},
	}

	info := buildGapFillErrorContext(context.Background(), show, cfg, deps)

	if info.RemoteAudioError == "" {
		t.Fatal("expected RemoteAudioError to be set")
	}
	if !strings.Contains(info.RemoteAudioError, "permission denied") {
		t.Fatalf("expected audio error containing 'permission denied', got %q", info.RemoteAudioError)
	}
	if info.RemoteVideoError == "" {
		t.Fatal("expected RemoteVideoError to be set")
	}
	if !strings.Contains(info.RemoteVideoError, "timeout") {
		t.Fatalf("expected video error containing 'timeout', got %q", info.RemoteVideoError)
	}

	// When remote checks error, exists flags should remain false
	if info.RemoteAudioExists {
		t.Fatal("expected RemoteAudioExists=false when error occurred")
	}
	if info.RemoteVideoExists {
		t.Fatal("expected RemoteVideoExists=false when error occurred")
	}

	// Verify the hint derivation for remote errors
	hint := deriveGapFillReasonHint(nil, info)
	if hint != "Remote existence check failed" {
		t.Fatalf("expected 'Remote existence check failed' hint, got %q", hint)
	}
}

func TestBuildGapFillErrorContext_SkipsRemoteWhenRcloneDisabled(t *testing.T) {
	cfg := &model.Config{
		OutPath:       t.TempDir(),
		VideoOutPath:  t.TempDir(),
		RcloneEnabled: false,
	}
	show := &model.AlbArtResp{
		ArtistName:    "Artist",
		ContainerInfo: "Show",
	}

	remoteCalled := false
	deps := &Deps{
		RemotePathExists: func(_ context.Context, _ string, _ *model.Config, _ bool) (bool, error) {
			remoteCalled = true
			return false, nil
		},
	}

	info := buildGapFillErrorContext(context.Background(), show, cfg, deps)

	if remoteCalled {
		t.Fatal("RemotePathExists should not be called when rclone is disabled")
	}
	if info.RemoteRelativePath != "" {
		t.Fatal("expected empty RemoteRelativePath when rclone disabled")
	}
}

func TestBuildGapFillErrorContext_SkipsRemoteWhenDepsNil(t *testing.T) {
	cfg := &model.Config{
		OutPath:       t.TempDir(),
		VideoOutPath:  t.TempDir(),
		RcloneEnabled: true,
		RcloneRemote:  "gdrive",
	}
	show := &model.AlbArtResp{
		ArtistName:    "Artist",
		ContainerInfo: "Show",
	}

	// RemotePathExists is nil in deps
	deps := &Deps{}

	info := buildGapFillErrorContext(context.Background(), show, cfg, deps)

	if info.RemoteAudioError != "" {
		t.Fatalf("expected no RemoteAudioError with nil RemotePathExists, got %q", info.RemoteAudioError)
	}
	if info.RemoteVideoError != "" {
		t.Fatalf("expected no RemoteVideoError with nil RemotePathExists, got %q", info.RemoteVideoError)
	}
}

func TestBuildGapFillErrorContext_PopulatesMetadataFields(t *testing.T) {
	cfg := &model.Config{
		OutPath:      t.TempDir(),
		VideoOutPath: t.TempDir(),
	}
	show := &model.AlbArtResp{
		ArtistName:          "Billy Strings",
		ContainerInfo:       "Test Show",
		AvailabilityTypeStr: "PREORDER",
		ActiveState:         "PENDING",
		Tracks:              []model.Track{{TrackID: 1}, {TrackID: 2}},
		Products:            []model.Product{{}, {}},
		ProductFormatList:   []*model.ProductFormatList{{FormatStr: "FLAC"}},
	}

	info := buildGapFillErrorContext(context.Background(), show, cfg, &Deps{})

	if info.AvailabilityType != "PREORDER" {
		t.Fatalf("expected AvailabilityType='PREORDER', got %q", info.AvailabilityType)
	}
	if info.ActiveState != "PENDING" {
		t.Fatalf("expected ActiveState='PENDING', got %q", info.ActiveState)
	}
	if info.Tracks != 2 {
		t.Fatalf("expected Tracks=2, got %d", info.Tracks)
	}
	if info.Products != 2 {
		t.Fatalf("expected Products=2, got %d", info.Products)
	}
	if info.ProductFormats != 1 {
		t.Fatalf("expected ProductFormats=1, got %d", info.ProductFormats)
	}
	if info.LocalAudioPath == "" {
		t.Fatal("expected LocalAudioPath to be set")
	}
	if info.LocalVideoPath == "" {
		t.Fatal("expected LocalVideoPath to be set")
	}
}
