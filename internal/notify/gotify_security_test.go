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

func TestSendBuildsMessageURLStructurally(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gotify/message" {
			t.Errorf("request path = %q, want /gotify/message", r.URL.Path)
		}
		if r.URL.RawQuery != "" || r.URL.Fragment != "" {
			t.Errorf("request URL retained query or fragment: %s", r.URL.String())
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if err := Send(context.Background(), server.URL+"/gotify/", "secret", "title", "body", 1); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
}

func TestSendRejectsServerURLQueryAndFragment(t *testing.T) {
	for _, suffix := range []string{"?redirect=evil", "#fragment"} {
		err := Send(context.Background(), "http://localhost/"+suffix, "secret", "title", "body", 1)
		if err == nil {
			t.Fatalf("Send accepted server URL suffix %q", suffix)
		}
	}
}
