package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestSendRejectsPlainHTTPOffLoopback(t *testing.T) {
	err := Send(context.Background(), "http://example.com", "secret", "title", "body", 1)
	if err == nil {
		t.Fatal("Send accepted plaintext HTTP for a non-loopback server")
	}
}

func TestSendDoesNotForwardTokenAcrossRedirect(t *testing.T) {
	var redirected atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirected.Store(true)
		if got := r.Header.Get("X-Gotify-Token"); got != "" {
			t.Errorf("redirect target received token %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()

	err := Send(context.Background(), source.URL, "secret", "title", "body", 1)
	if err == nil {
		t.Fatal("Send accepted a redirect response")
	}
	if redirected.Load() {
		t.Fatal("redirect target was contacted")
	}
}
