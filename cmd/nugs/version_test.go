package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestIsVersionRequest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "command", args: []string{"version"}, want: true},
		{name: "long flag", args: []string{"--version"}, want: true},
		{name: "short flag", args: []string{"-v"}, want: true},
		{name: "empty", args: nil, want: false},
		{name: "extra argument", args: []string{"version", "extra"}, want: false},
		{name: "help", args: []string{"--help"}, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isVersionRequest(tc.args); got != tc.want {
				t.Fatalf("isVersionRequest(%q) = %v, want %v", tc.args, got, tc.want)
			}
		})
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
