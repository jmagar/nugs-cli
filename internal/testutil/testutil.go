// Package testutil provides shared test helpers used across internal packages.
package testutil

import (
	"bytes"
	"os"
	"testing"
)

// WithTempHome sets HOME to a temporary directory for the duration of the test.
func WithTempHome(t *testing.T) string {
	t.Helper()
	origHome := os.Getenv("HOME")
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Cleanup(func() {
		_ = os.Setenv("HOME", origHome)
	})
	return tempHome
}

// CaptureStdout captures stdout during fn() and returns the output as a string.
func CaptureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = orig
		_ = w.Close()
		_ = r.Close()
	}()

	fn()

	_ = w.Close()

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return buf.String()
}

// ChdirTemp changes to a temp directory and restores cwd on cleanup.
func ChdirTemp(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("failed to chdir temp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(orig)
	})
	return tmp
}

// WriteExecutable writes a minimal executable script to the given path.
func WriteExecutable(t *testing.T, path string) {
	t.Helper()
	content := []byte("#!/bin/sh\nexit 0\n")
	if err := os.WriteFile(path, content, 0755); err != nil {
		t.Fatalf("failed to write executable %s: %v", path, err)
	}
}
