// Package notify provides push notification helpers.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 5 * time.Second}

// Send posts a message to a Gotify server.
// Returns nil immediately if url or token are empty.
func Send(ctx context.Context, serverURL, token, title, message string, priority int) error {
	if serverURL == "" || token == "" {
		return nil
	}

	url := strings.TrimRight(serverURL, "/") + "/message"

	body, err := json.Marshal(map[string]any{
		"title":    title,
		"message":  message,
		"priority": priority,
	})
	if err != nil {
		return fmt.Errorf("gotify: marshal failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("gotify: create request failed: %w", err)
	}
	req.Header.Set("X-Gotify-Token", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gotify: send failed: %w", err)
	}
	defer resp.Body.Close()

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
