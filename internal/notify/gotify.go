// Package notify provides push notification helpers.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var httpClient = &http.Client{
	Timeout: 5 * time.Second,
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		// The token is a credential. Never replay it to a redirect target.
		return http.ErrUseLastResponse
	},
}

func validateServerURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return nil, fmt.Errorf("gotify: invalid server URL")
	}
	if u.User != nil {
		return nil, fmt.Errorf("gotify: server URL must not contain userinfo")
	}
	if u.Scheme == "https" {
		return u, nil
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme == "http" && (u.Hostname() == "localhost" || ip != nil && ip.IsLoopback()) {
		return u, nil
	}
	return nil, fmt.Errorf("gotify: HTTPS is required for non-loopback servers")
}

// Send posts a message to a Gotify server.
// Returns nil immediately if url or token are empty.
func Send(ctx context.Context, serverURL, token, title, message string, priority int) error {
	if serverURL == "" || token == "" {
		return nil
	}

	base, err := validateServerURL(serverURL)
	if err != nil {
		return err
	}
	requestURL := strings.TrimRight(base.String(), "/") + "/message"

	body, err := json.Marshal(map[string]any{
		"title":    title,
		"message":  message,
		"priority": priority,
	})
	if err != nil {
		return fmt.Errorf("gotify: marshal failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("gotify: create request failed: %w", err)
	}
	req.Header.Set("X-Gotify-Token", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gotify: send failed: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gotify: server returned %d", resp.StatusCode)
	}
	return nil
}

// BuildNotifier returns a Notify function wired to the given Gotify server.
// Returns nil (disabling notifications) if url or token are empty.
func BuildNotifier(serverURL, token string) func(ctx context.Context, title, message string, priority int) error {
	if serverURL == "" || token == "" {
		return nil
	}
	return func(ctx context.Context, title, message string, priority int) error {
		return Send(ctx, serverURL, token, title, message, priority)
	}
}
