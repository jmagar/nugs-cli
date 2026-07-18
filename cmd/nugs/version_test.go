package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestIsVersionRequest(t *testing.T) {
	t.Parallel()
	for _, arg := range []string{"version", "--version", "-v"} {
		if !isVersionRequest([]string{arg}) {
			t.Fatalf("expected %q to be a version request", arg)
		}
	}
	for _, args := range [][]string{nil, {"version", "extra"}, {"--help"}} {
		if isVersionRequest(args) {
			t.Fatalf("did not expect %q to be a version request", args)
		}
	}
}

func TestPrintVersion(t *testing.T) {
	oldVersion, oldCommit, oldBuildDate := version, commit, buildDate
	version, commit, buildDate = "v1.2.3", "abc123", "2026-07-18T00:00:00Z"
	t.Cleanup(func() { version, commit, buildDate = oldVersion, oldCommit, oldBuildDate })

	var output bytes.Buffer
	printVersion(&output)
	for _, expected := range []string{"nugs v1.2.3", "commit: abc123", "built: 2026-07-18T00:00:00Z", "go:", "platform:"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("version output %q missing %q", output.String(), expected)
		}
	}
}
