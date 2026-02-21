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

func TestShowExistsForMediaIndexed_BothMediaFallbackChecksAllRemotes(t *testing.T) {
	show := &model.AlbArtResp{
		ArtistName:    "Test Artist",
		ContainerInfo: "2024-01-01 Show",
	}
	cfg := &model.Config{
		OutPath:       t.TempDir(),
		VideoOutPath:  t.TempDir(),
		RcloneEnabled: true,
	}

	idx := &ArtistPresenceIndex{
		LocalFolders:  make(map[string]struct{}),
		RemoteFolders: make(map[string]struct{}),
		RemoteListErr: errors.New("index failed"), // Force fallback
	}

	var callOrder []bool
	deps := &Deps{
		RemotePathExists: func(_ context.Context, _ string, _ *model.Config, isVideo bool) (bool, error) {
			callOrder = append(callOrder, isVideo)
			if isVideo {
				return true, nil // Video exists
			}
			return false, errors.New("audio remote error") // Audio errors
		},
	}

	exists := ShowExistsForMediaIndexed(context.Background(), show, cfg, model.MediaTypeBoth, idx, deps)

	if !exists {
		t.Fatal("expected exists=true when video remote has show")
	}
	if len(callOrder) != 2 {
		t.Fatalf("expected 2 remote checks, got %d", len(callOrder))
	}
	if callOrder[0] != false || callOrder[1] != true {
		t.Fatalf("expected [false, true] call order (audio then video), got %v", callOrder)
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

func TestClassifyShows_IntegrationWithPresenceIndex(t *testing.T) {
	artist := "Billy Strings"
	artistFolder := helpers.Sanitise(artist)
	audioRoot := t.TempDir()
	videoRoot := t.TempDir()

	cfg := &model.Config{
		OutPath:      audioRoot,
		VideoOutPath: videoRoot,
	}

	// Create three shows:
	// 1. downloadedShow - exists locally (should be counted as downloaded)
	// 2. missingShow - downloadable but not found locally (should be in MissingShows)
	// 3. preorderShow - not downloadable (should be excluded entirely)
	downloadedShow := &model.AlbArtResp{
		ArtistName:          artist,
		ContainerInfo:       "2024-06-01 - Red Rocks, CO",
		AvailabilityTypeStr: model.AvailableAvailabilityType,
		Tracks:              []model.Track{{TrackID: 1}},
	}
	missingShow := &model.AlbArtResp{
		ArtistName:          artist,
		ContainerInfo:       "2024-07-04 - Chicago, IL",
		AvailabilityTypeStr: model.AvailableAvailabilityType,
		Tracks:              []model.Track{{TrackID: 2}},
	}
	preorderShow := &model.AlbArtResp{
		ArtistName:          artist,
		ContainerInfo:       "2025-01-01 - Nashville, TN",
		AvailabilityTypeStr: "PREORDER",
	}

	// Create the local directory for the downloaded show
	downloadedFolder := helpers.BuildAlbumFolderName(artist, downloadedShow.ContainerInfo)
	if err := os.MkdirAll(filepath.Join(audioRoot, artistFolder, downloadedFolder), 0o755); err != nil {
		t.Fatalf("mkdir downloaded show: %v", err)
	}

	// Build the presence index from the real directory structure
	presenceIdx := BuildArtistPresenceIndex(context.Background(), artist, cfg, &Deps{}, model.MediaTypeAudio)

	// Verify the index found the downloaded show
	if _, ok := presenceIdx.LocalFolders[downloadedFolder]; !ok {
		t.Fatalf("expected downloaded show %q in presence index", downloadedFolder)
	}

	deps := &Deps{
		GetShowMediaType: func(_ *model.AlbArtResp) model.MediaType {
			return model.MediaTypeAudio
		},
	}

	allShows := []*model.AlbArtResp{downloadedShow, missingShow, preorderShow}
	analysis := &model.ArtistCatalogAnalysis{}

	classifyShows(context.Background(), allShows, model.MediaTypeAudio, &presenceIdx, cfg, deps, analysis)

	// Preorder should be excluded entirely (not downloadable)
	if len(analysis.Shows) != 2 {
		t.Fatalf("expected 2 shows in analysis (preorder excluded), got %d", len(analysis.Shows))
	}

	// One downloaded, one missing
	if analysis.Downloaded != 1 {
		t.Fatalf("expected Downloaded=1, got %d", analysis.Downloaded)
	}
	if len(analysis.MissingShows) != 1 {
		t.Fatalf("expected 1 missing show, got %d", len(analysis.MissingShows))
	}

	// Verify the missing show is the correct one
	if analysis.MissingShows[0].Show.ContainerInfo != missingShow.ContainerInfo {
		t.Fatalf("expected missing show %q, got %q",
			missingShow.ContainerInfo, analysis.MissingShows[0].Show.ContainerInfo)
	}

	// Verify media type propagation
	for _, s := range analysis.Shows {
		if s.MediaType != model.MediaTypeAudio {
			t.Fatalf("expected MediaType=audio for all shows, got %v", s.MediaType)
		}
	}
}

func TestClassifyShows_VideoFilterExcludesAudioOnlyShows(t *testing.T) {
	artist := "Test Artist"
	cfg := &model.Config{
		OutPath:      t.TempDir(),
		VideoOutPath: t.TempDir(),
	}

	audioOnlyShow := &model.AlbArtResp{
		ArtistName:          artist,
		ContainerInfo:       "Audio Only Show",
		AvailabilityTypeStr: model.AvailableAvailabilityType,
		Tracks:              []model.Track{{TrackID: 1}},
	}
	videoShow := &model.AlbArtResp{
		ArtistName:          artist,
		ContainerInfo:       "Video Show",
		AvailabilityTypeStr: model.AvailableAvailabilityType,
		Tracks:              []model.Track{{TrackID: 2}},
	}

	deps := &Deps{
		GetShowMediaType: func(show *model.AlbArtResp) model.MediaType {
			if show.ContainerInfo == "Video Show" {
				return model.MediaTypeVideo
			}
			return model.MediaTypeAudio
		},
	}

	presenceIdx := &ArtistPresenceIndex{
		LocalFolders:  make(map[string]struct{}),
		RemoteFolders: make(map[string]struct{}),
	}

	allShows := []*model.AlbArtResp{audioOnlyShow, videoShow}
	analysis := &model.ArtistCatalogAnalysis{}

	classifyShows(context.Background(), allShows, model.MediaTypeVideo, presenceIdx, cfg, deps, analysis)

	// Only the video show should pass the filter
	if len(analysis.Shows) != 1 {
		t.Fatalf("expected 1 show with video filter, got %d", len(analysis.Shows))
	}
	if analysis.Shows[0].Show.ContainerInfo != "Video Show" {
		t.Fatalf("expected 'Video Show', got %q", analysis.Shows[0].Show.ContainerInfo)
	}
}

func TestClassifyShows_NilGetShowMediaTypeDefaultsToAudio(t *testing.T) {
	artist := "Test Artist"
	cfg := &model.Config{
		OutPath:      t.TempDir(),
		VideoOutPath: t.TempDir(),
	}

	show := &model.AlbArtResp{
		ArtistName:          artist,
		ContainerInfo:       "Test Show",
		AvailabilityTypeStr: model.AvailableAvailabilityType,
		Tracks:              []model.Track{{TrackID: 1}},
	}

	// No GetShowMediaType - should default to audio
	deps := &Deps{}

	presenceIdx := &ArtistPresenceIndex{
		LocalFolders:  make(map[string]struct{}),
		RemoteFolders: make(map[string]struct{}),
	}

	analysis := &model.ArtistCatalogAnalysis{}

	classifyShows(context.Background(), []*model.AlbArtResp{show}, model.MediaTypeAudio, presenceIdx, cfg, deps, analysis)

	if len(analysis.Shows) != 1 {
		t.Fatalf("expected 1 show, got %d", len(analysis.Shows))
	}
	if analysis.Shows[0].MediaType != model.MediaTypeAudio {
		t.Fatalf("expected audio media type when GetShowMediaType is nil, got %v", analysis.Shows[0].MediaType)
	}
}

func TestBuildArtistPresenceIndex_RemoteListErrorStopsEarly(t *testing.T) {
	cfg := &model.Config{
		OutPath:       t.TempDir(),
		VideoOutPath:  t.TempDir(),
		RcloneEnabled: true,
	}

	var callCount int
	deps := &Deps{
		ListRemoteArtistFolders: func(_ context.Context, _ string, _ *model.Config, isVideo bool) (map[string]struct{}, error) {
			callCount++
			if !isVideo {
				return nil, errors.New("audio remote failed")
			}
			// Should never reach here due to early break
			return map[string]struct{}{"video-show": {}}, nil
		},
	}

	idx := BuildArtistPresenceIndex(context.Background(), "Artist", cfg, deps, model.MediaTypeBoth)

	if idx.RemoteListErr == nil {
		t.Fatal("expected RemoteListErr to be set")
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call before error break, got %d", callCount)
	}
	if len(idx.RemoteFolders) != 0 {
		t.Fatalf("expected empty remote folders after error, got %d", len(idx.RemoteFolders))
	}
}
