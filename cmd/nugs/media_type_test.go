package main

import "testing"

func TestGetShowMediaType_EmptyProducts(t *testing.T) {
	show := &AlbArtResp{Products: nil}
	got := getShowMediaType(show)
	if got != MediaTypeAudio {
		t.Fatalf("expected MediaTypeAudio for empty Products, got %v", got)
	}
}

func TestGetShowMediaType_AudioOnly(t *testing.T) {
	show := &AlbArtResp{
		Products: []Product{
			{FormatStr: "FLAC", SkuID: 1},
			{FormatStr: "ALAC", SkuID: 2},
		},
	}
	got := getShowMediaType(show)
	if got != MediaTypeAudio {
		t.Fatalf("expected MediaTypeAudio, got %v", got)
	}
}

func TestGetShowMediaType_VideoOnly(t *testing.T) {
	show := &AlbArtResp{
		Products: []Product{
			{FormatStr: "VIDEO ON DEMAND", SkuID: 10},
		},
	}
	got := getShowMediaType(show)
	if got != MediaTypeVideo {
		t.Fatalf("expected MediaTypeVideo for video-only products, got %v", got)
	}
}

func TestGetShowMediaType_LiveHDVideo(t *testing.T) {
	show := &AlbArtResp{
		Products: []Product{
			{FormatStr: "LIVE HD VIDEO", SkuID: 10},
		},
	}
	got := getShowMediaType(show)
	if got != MediaTypeVideo {
		t.Fatalf("expected MediaTypeVideo for LIVE HD VIDEO format, got %v", got)
	}
}

func TestGetShowMediaType_Both(t *testing.T) {
	show := &AlbArtResp{
		Products: []Product{
			{FormatStr: "FLAC", SkuID: 1},
			{FormatStr: "VIDEO ON DEMAND", SkuID: 10},
		},
	}
	got := getShowMediaType(show)
	if got != MediaTypeBoth {
		t.Fatalf("expected MediaTypeBoth, got %v", got)
	}
}

func TestGetShowMediaType_FromProductFormatList_VideoOnly(t *testing.T) {
	show := &AlbArtResp{
		ProductFormatList: []*ProductFormatList{
			{FormatStr: "LIVE HD VIDEO", SkuID: 10},
		},
	}
	got := getShowMediaType(show)
	if got != MediaTypeVideo {
		t.Fatalf("expected MediaTypeVideo from productFormatList, got %v", got)
	}
}

func TestGetShowMediaType_FromProductFormatList_Both(t *testing.T) {
	show := &AlbArtResp{
		ProductFormatList: []*ProductFormatList{
			{FormatStr: "ALAC", SkuID: 2},
			{FormatStr: "LIVE HD VIDEO", SkuID: 10},
		},
	}
	got := getShowMediaType(show)
	if got != MediaTypeBoth {
		t.Fatalf("expected MediaTypeBoth from productFormatList, got %v", got)
	}
}

func TestMatchesMediaFilter(t *testing.T) {
	tests := []struct {
		name      string
		showMedia MediaType
		filter    MediaType
		want      bool
	}{
		{"unknown filter passes all", MediaTypeAudio, MediaTypeUnknown, true},
		{"both filter passes all", MediaTypeAudio, MediaTypeBoth, true},
		{"audio filter matches audio", MediaTypeAudio, MediaTypeAudio, true},
		{"audio filter matches both", MediaTypeBoth, MediaTypeAudio, true},
		{"audio filter rejects video", MediaTypeVideo, MediaTypeAudio, false},
		{"video filter matches video", MediaTypeVideo, MediaTypeVideo, true},
		{"video filter matches both", MediaTypeBoth, MediaTypeVideo, true},
		{"video filter rejects audio", MediaTypeAudio, MediaTypeVideo, false},
		{"audio filter matches unknown-as-audio", MediaTypeAudio, MediaTypeAudio, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := matchesMediaFilter(tc.showMedia, tc.filter)
			if got != tc.want {
				t.Fatalf("matchesMediaFilter(%v, %v) = %v, want %v", tc.showMedia, tc.filter, got, tc.want)
			}
		})
	}
}

func TestParseMediaModifier(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantMediaType MediaType
		wantRemaining []string
	}{
		{
			name:          "audio modifier first position",
			args:          []string{"audio", "1125", "fill"},
			wantMediaType: MediaTypeAudio,
			wantRemaining: []string{"1125", "fill"},
		},
		{
			name:          "video modifier middle position",
			args:          []string{"1125", "video", "fill"},
			wantMediaType: MediaTypeVideo,
			wantRemaining: []string{"1125", "fill"},
		},
		{
			name:          "both modifier last position",
			args:          []string{"1125", "fill", "both"},
			wantMediaType: MediaTypeBoth,
			wantRemaining: []string{"1125", "fill"},
		},
		{
			name:          "no modifier returns unknown",
			args:          []string{"1125", "fill"},
			wantMediaType: MediaTypeUnknown,
			wantRemaining: []string{"1125", "fill"},
		},
		{
			name:          "empty args returns unknown",
			args:          []string{},
			wantMediaType: MediaTypeUnknown,
			wantRemaining: []string{},
		},
		{
			name:          "only modifier",
			args:          []string{"video"},
			wantMediaType: MediaTypeVideo,
			wantRemaining: []string{},
		},
		{
			name:          "case insensitive audio",
			args:          []string{"AUDIO", "1125"},
			wantMediaType: MediaTypeAudio,
			wantRemaining: []string{"1125"},
		},
		{
			name:          "non-modifier args unchanged",
			args:          []string{"1125", "something", "else"},
			wantMediaType: MediaTypeUnknown,
			wantRemaining: []string{"1125", "something", "else"},
		},
		{
			name:          "first modifier wins",
			args:          []string{"audio", "video", "1125"},
			wantMediaType: MediaTypeAudio,
			wantRemaining: []string{"video", "1125"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotMediaType, gotRemaining := parseMediaModifier(tc.args)
			if gotMediaType != tc.wantMediaType {
				t.Errorf("parseMediaModifier(%v) mediaType = %v, want %v", tc.args, gotMediaType, tc.wantMediaType)
			}
			if len(gotRemaining) != len(tc.wantRemaining) {
				t.Errorf("parseMediaModifier(%v) remaining length = %d, want %d", tc.args, len(gotRemaining), len(tc.wantRemaining))
			}
			for i, arg := range gotRemaining {
				if i >= len(tc.wantRemaining) || arg != tc.wantRemaining[i] {
					t.Errorf("parseMediaModifier(%v) remaining[%d] = %q, want %q", tc.args, i, arg, tc.wantRemaining[i])
				}
			}
		})
	}
}
