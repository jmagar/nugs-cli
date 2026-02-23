package catalog

import (
	"context"
	"encoding/json"
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
		if err != nil {
			t.Fatalf("CatalogCoverage() error = %v", err)
		}
	})

	var payload struct {
		Artists []map[string]any `json:"artists"`
		Total   int              `json:"total"`
		Message string           `json:"message"`
	}

	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v; output=%q", err, stdout)
	}
	if payload.Message != "No downloaded artists found" {
		t.Fatalf("message = %q, want %q", payload.Message, "No downloaded artists found")
	}
	if strings.Contains(stdout, "Warning:") {
		t.Fatalf("expected JSON-only stdout, found warning text in output=%q", stdout)
	}
}
