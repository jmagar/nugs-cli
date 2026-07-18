package api

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAPILoggerInitCanRetryAfterFailure(t *testing.T) {
	defer CloseAPILogger()
	root := t.TempDir()
	blocker := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := InitAPILogger(filepath.Join(blocker, "api.log")); err == nil {
		t.Fatal("initialization unexpectedly succeeded")
	}
	valid := filepath.Join(root, "logs", "api.log")
	if err := InitAPILogger(valid); err != nil {
		t.Fatalf("retry initialization: %v", err)
	}
	LogRequest("test", 200, time.Millisecond, 0, "closed", nil)
	if err := CloseAPILogger(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(valid)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("log mode = %o, want 600", info.Mode().Perm())
	}
}

func TestAPILoggerRotates(t *testing.T) {
	defer CloseAPILogger()
	oldMax := apiLogMaxBytes
	apiLogMaxBytes = 1
	defer func() { apiLogMaxBytes = oldMax }()
	logPath := filepath.Join(t.TempDir(), "api.log")
	if err := InitAPILogger(logPath); err != nil {
		t.Fatal(err)
	}
	LogRequest("first", 200, 0, 0, "closed", nil)
	LogRequest("second", 200, 0, 0, "closed", nil)
	if _, err := os.Stat(logPath + ".1"); err != nil {
		t.Fatalf("rotated log missing: %v", err)
	}
}

func TestAPILoggerContinuesAfterRotationRenameFailure(t *testing.T) {
	defer CloseAPILogger()
	oldMax := apiLogMaxBytes
	apiLogMaxBytes = 1
	defer func() { apiLogMaxBytes = oldMax }()

	logPath := filepath.Join(t.TempDir(), "api.log")
	if err := InitAPILogger(logPath); err != nil {
		t.Fatal(err)
	}
	LogRequest("first", 200, 0, 0, "closed", nil)

	errRotate := errors.New("injected rotation rename failure")
	oldRename := apiLogRename
	apiLogRename = func(oldPath, newPath string) error {
		if oldPath == logPath && newPath == logPath+".1" {
			return errRotate
		}
		return oldRename(oldPath, newPath)
	}
	defer func() { apiLogRename = oldRename }()

	LogRequest("second", 200, 0, 0, "closed", nil)
	contents, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{`"label":"first"`, `"label":"second"`} {
		if !strings.Contains(string(contents), label) {
			t.Fatalf("active log after failed rotation does not contain %s: %s", label, contents)
		}
	}
}
