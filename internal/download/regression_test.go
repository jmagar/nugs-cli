package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmagar/nugs-cli/internal/api"
	"github.com/jmagar/nugs-cli/internal/model"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func httpResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestProcessTrack_QualityFallback_NoHang(t *testing.T) {
	tests := []struct {
		name      string
		wantFmt   int
		streamURL string
		wantExt   string
	}{
		{
			name:      "aac request falls back to flac",
			wantFmt:   5,
			streamURL: "https://cdn.example.com/audio.flac16/track.flac?token=1",
			wantExt:   ".flac",
		},
		{
			name:      "mqa request falls back to flac",
			wantFmt:   3,
			streamURL: "https://cdn.example.com/audio.flac16/track.flac?token=1",
			wantExt:   ".flac",
		},
		{
			name:      "alac request stays alac",
			wantFmt:   1,
			streamURL: "https://cdn.example.com/audio.alac16/track.bin",
			wantExt:   ".m4a",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			oldClient := api.Client
			t.Cleanup(func() { api.Client = oldClient })

			api.Client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case strings.Contains(req.URL.Path, "/bigriver/subPlayer.aspx"):
					return httpResponse(http.StatusOK, fmt.Sprintf(`{"streamLink":%q}`, tc.streamURL)), nil
				case req.URL.Host == "cdn.example.com":
					return httpResponse(http.StatusOK, "audio-bytes"), nil
				default:
					return nil, fmt.Errorf("unexpected request: %s", req.URL.String())
				}
			})}

			dir := t.TempDir()
			cfg := &model.Config{Format: tc.wantFmt}
			track := &model.Track{TrackID: 1234, SongTitle: "Track Name"}
			streamParams := &model.StreamParams{}

			errCh := make(chan error, 1)
			go func() {
				errCh <- ProcessTrack(context.Background(), dir, 1, 1, cfg, track, streamParams, nil, &Deps{})
			}()

			select {
			case err := <-errCh:
				if err != nil {
					t.Fatalf("ProcessTrack returned error: %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("ProcessTrack timed out (possible fallback loop regression)")
			}

			matches, err := filepath.Glob(filepath.Join(dir, "*"+tc.wantExt))
			if err != nil {
				t.Fatalf("glob failed: %v", err)
			}
			if len(matches) != 1 {
				t.Fatalf("expected one downloaded file with extension %s, found %d", tc.wantExt, len(matches))
			}
		})
	}
}

func TestHlsOnly_ValidationErrors_NoPanic(t *testing.T) {
	tests := []struct {
		name        string
		playlist    string
		wantErrText string // empty means just check err != nil (library error)
	}{
		{
			name:        "rejects invalid playlist payload",
			playlist:    "#EXTM3U\n#EXT-X-NOT-A-REAL-TAG\n",
			wantErrText: "", // library error, don't assert on exact message
		},
		{
			name: "rejects playlist with no key",
			playlist: "#EXTM3U\n" +
				"#EXT-X-VERSION:3\n" +
				"#EXT-X-TARGETDURATION:10\n" +
				"#EXTINF:10.0,\n" +
				"segment.ts\n" +
				"#EXT-X-ENDLIST\n",
			wantErrText: "no encryption key",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			oldClient := api.Client
			t.Cleanup(func() { api.Client = oldClient })

			api.Client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if strings.HasSuffix(req.URL.Path, "/manifest.m3u8") {
					return httpResponse(http.StatusOK, tc.playlist), nil
				}
				return nil, fmt.Errorf("unexpected request: %s", req.URL.String())
			})}

			panicked := false
			var gotErr error
			func() {
				defer func() {
					if recover() != nil {
						panicked = true
					}
				}()
				gotErr = HlsOnly(context.Background(), filepath.Join(t.TempDir(), "out.m4a"), "https://stream.test/manifest.m3u8", "ffmpeg", nil, false, &Deps{})
			}()

			if panicked {
				t.Fatal("HlsOnly panicked")
			}
			if gotErr == nil {
				t.Fatal("expected error, got nil")
			}
			if tc.wantErrText != "" && !strings.Contains(gotErr.Error(), tc.wantErrText) {
				t.Fatalf("expected error containing %q, got %q", tc.wantErrText, gotErr.Error())
			}
		})
	}
}

func TestChooseVariant_InvalidResolution_NoPanic(t *testing.T) {
	oldClient := api.Client
	t.Cleanup(func() { api.Client = oldClient })

	api.Client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, "/master.m3u8") {
			playlist := "#EXTM3U\n" +
				"#EXT-X-STREAM-INF:BANDWIDTH=5000000,RESOLUTION=bad-resolution\n" +
				"video.m3u8\n"
			return httpResponse(http.StatusOK, playlist), nil
		}
		return nil, fmt.Errorf("unexpected request: %s", req.URL.String())
	})}

	panicked := false
	var gotErr error
	func() {
		defer func() {
			if recover() != nil {
				panicked = true
			}
		}()
		_, _, gotErr = ChooseVariant("https://video.test/master.m3u8", "2160")
	}()

	if panicked {
		t.Fatal("ChooseVariant panicked")
	}
	if gotErr == nil {
		t.Fatal("expected error for invalid resolution format")
	}
	if !strings.Contains(gotErr.Error(), "invalid resolution format") {
		t.Fatalf("unexpected error: %v", gotErr)
	}
}
