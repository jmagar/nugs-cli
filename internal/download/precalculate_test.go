package download

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/jmagar/nugs-cli/internal/api"
	"github.com/jmagar/nugs-cli/internal/model"
)

func TestPreCalculateShowSizeReusesTrackURL(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != http.MethodHead {
			t.Errorf("method = %s, want HEAD", r.Method)
		}
		w.Header().Set("Content-Length", "123")
	}))
	defer server.Close()

	ctx := api.WithHTTPClient(context.Background(), server.Client())

	size, err := PreCalculateShowSize(ctx, []model.Track{{TrackID: 42, TrackURL: server.URL + "/track"}}, &model.StreamParams{}, &model.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if size != 123 {
		t.Fatalf("size = %d, want 123", size)
	}
	if requests.Load() != 1 {
		t.Fatalf("requests = %d, want one metadata URL HEAD", requests.Load())
	}
}
