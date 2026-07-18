package download

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/jmagar/nugs-cli/internal/api"
	"github.com/jmagar/nugs-cli/internal/model"
)

func TestResolveAlbumDownloadModes(t *testing.T) {
	metaBoth := &model.AlbArtResp{Products: []model.Product{
		{FormatStr: model.VideoOnDemandFormatLabel, SkuID: 1},
		{FormatStr: "FLAC", SkuID: 2},
	}}
	metaAudioOnly := &model.AlbArtResp{Products: []model.Product{{FormatStr: "FLAC", SkuID: 2}}}

	tests := []struct {
		name      string
		cfg       *model.Config
		meta      *model.AlbArtResp
		wantAudio bool
		wantVideo bool
	}{
		{
			name:      "unknown preference defaults to audio",
			cfg:       &model.Config{DefaultOutputs: ""},
			meta:      metaBoth,
			wantAudio: true,
			wantVideo: false,
		},
		{
			name:      "both preference enables both when available",
			cfg:       &model.Config{DefaultOutputs: "both"},
			meta:      metaBoth,
			wantAudio: true,
			wantVideo: true,
		},
		{
			name:      "video preference only",
			cfg:       &model.Config{DefaultOutputs: "video"},
			meta:      metaBoth,
			wantAudio: false,
			wantVideo: true,
		},
		{
			name:      "skip videos override",
			cfg:       &model.Config{DefaultOutputs: "both", SkipVideos: true},
			meta:      metaBoth,
			wantAudio: true,
			wantVideo: false,
		},
		{
			name:      "force video override",
			cfg:       &model.Config{DefaultOutputs: "both", SkipVideos: true, ForceVideo: true},
			meta:      metaBoth,
			wantAudio: false,
			wantVideo: true,
		},
		{
			name:      "video preference without video availability",
			cfg:       &model.Config{DefaultOutputs: "video"},
			meta:      metaAudioOnly,
			wantAudio: false,
			wantVideo: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotAudio, gotVideo := resolveAlbumDownloadModes(tc.cfg, tc.meta)
			if gotAudio != tc.wantAudio || gotVideo != tc.wantVideo {
				t.Fatalf("resolveAlbumDownloadModes() = (%v, %v), want (%v, %v)", gotAudio, gotVideo, tc.wantAudio, tc.wantVideo)
			}
		})
	}
}

func TestDownloadAlbumAudioDoesNotUploadPartialAlbum(t *testing.T) {
	errDownload := errors.New("download failed")
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errDownload
	})}
	ctx := api.WithHTTPClient(context.Background(), client)
	uploads := 0
	deps := &Deps{UploadToRclone: func(context.Context, string, string, *model.Config, *model.ProgressBoxState, bool) error {
		uploads++
		return nil
	}}
	cfg := &model.Config{Format: 2, RcloneEnabled: true}
	tracks := []model.Track{{
		TrackID:   1,
		SongTitle: "failed track",
		TrackURL:  "https://cdn.example.test/audio.flac16/failed.flac",
	}}

	err := downloadAlbumAudio(ctx, tracks, t.TempDir(), "artist", cfg, &model.StreamParams{}, nil, false, deps)
	if !errors.Is(err, errDownload) {
		t.Fatalf("downloadAlbumAudio error = %v, want wrapped download failure", err)
	}
	if uploads != 0 {
		t.Fatalf("upload count = %d, want 0 for a partial album", uploads)
	}
}

func TestProcessArtistAlbumsReturnsJoinedFailures(t *testing.T) {
	artist := &model.ArtistMeta{}
	artist.Response.Containers = []*model.AlbArtResp{
		{ContainerID: 101, ContainerInfo: "First failed album"},
		{ContainerID: 202, ContainerInfo: "Second failed album"},
	}
	batchState := &model.BatchProgressState{TotalAlbums: 2}
	progressBox := &model.ProgressBoxState{BatchState: batchState}

	err := processArtistAlbums(
		context.Background(),
		[]*model.ArtistMeta{artist},
		&model.Config{SkipVideos: true},
		&model.StreamParams{},
		batchState,
		progressBox,
		&Deps{},
	)
	if !errors.Is(err, model.ErrReleaseHasNoContent) {
		t.Fatalf("processArtistAlbums error = %v, want joined ErrReleaseHasNoContent", err)
	}
	for _, want := range []string{"First failed album", "Second failed album"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("processArtistAlbums error %q does not include %q", err, want)
		}
	}
	if batchState.Failed != 2 || batchState.Complete != 0 {
		t.Fatalf("batch counts = failed %d, complete %d; want 2, 0", batchState.Failed, batchState.Complete)
	}
}

func TestBuildAlbumShowNumber(t *testing.T) {
	meta := &model.AlbArtResp{PerformanceDateShort: "2026-01-01"}

	gotSingle := buildAlbumShowNumber(meta, nil)
	if gotSingle != "2026-01-01" {
		t.Fatalf("buildAlbumShowNumber() single = %q, want %q", gotSingle, "2026-01-01")
	}

	batch := &model.BatchProgressState{CurrentAlbum: 2, TotalAlbums: 5}
	gotBatch := buildAlbumShowNumber(meta, batch)
	if gotBatch != "Show 2/5: 2026-01-01" {
		t.Fatalf("buildAlbumShowNumber() batch = %q, want %q", gotBatch, "Show 2/5: 2026-01-01")
	}
}

func TestHandleVideoOnlyAlbumSkipsWhenDisabled(t *testing.T) {
	handled, err := handleVideoOnlyAlbum(
		context.Background(),
		"123",
		&model.Config{SkipVideos: true},
		&model.StreamParams{},
		&model.AlbArtResp{},
		0,     // trackTotal - no tracks expected for this test
		999,   // skuID - arbitrary non-zero value
		false, // downloadVideo - disabled to test skip path
		&Deps{},
	)
	if err != nil {
		t.Fatalf("handleVideoOnlyAlbum() err = %v, want nil", err)
	}
	if !handled {
		t.Fatalf("handleVideoOnlyAlbum() handled = %v, want true", handled)
	}
}
