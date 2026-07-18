package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jmagar/nugs-cli/internal/model"
	"github.com/jmagar/nugs-cli/internal/testutil"
)

func TestCatalogCoverage_JSONMode_RemoteScanFailurePreservesJSONOutput(t *testing.T) {
	t.Setenv("PATH", "")

	cfg := &model.Config{
		OutPath:       t.TempDir(),
		RcloneEnabled: true,
		RcloneRemote:  "missing-remote",
		RclonePath:    "/music",
	}

	stdout := testutil.CaptureStdout(t, func() {
		err := CatalogCoverage(context.Background(), nil, cfg, "normal", model.MediaTypeAudio, &Deps{})
		var partial *CoveragePartialError
		if !errors.As(err, &partial) || partial.RemoteScanErr == nil {
			t.Fatalf("CatalogCoverage() error = %v, want structured partial error", err)
		}
	})

	var payload struct {
		Artists         []map[string]any `json:"artists"`
		Total           int              `json:"total"`
		Message         string           `json:"message"`
		Partial         bool             `json:"partial"`
		RemoteScanError string           `json:"remoteScanError"`
	}

	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v; output=%q", err, stdout)
	}
	if payload.Message != "No downloaded artists found" {
		t.Fatalf("message = %q, want %q", payload.Message, "No downloaded artists found")
	}
	if !payload.Partial || payload.RemoteScanError == "" {
		t.Fatalf("partial failure fields missing: %+v", payload)
	}
	if strings.Contains(stdout, "Warning:") {
		t.Fatalf("expected JSON-only stdout, found warning text in output=%q", stdout)
	}
}
