//go:build !windows

package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenRuntimeLogSecuresExistingFile(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "runtime.log")
	if err := os.WriteFile(logPath, []byte("old log\n"), 0644); err != nil {
		t.Fatal(err)
	}
	logFile, err := openRuntimeLog(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := logFile.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("runtime.log mode = %o, want 600", got)
	}
}
