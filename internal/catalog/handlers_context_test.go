package catalog

import (
	"context"
	"errors"
	"path/filepath"
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
	if filepath.Base(info.LocalAudioPath) == "" {
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
			err:  errors.New("release has no tracks or videos"),
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
