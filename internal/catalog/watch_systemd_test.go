//go:build linux

package catalog

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmagar/nugs-cli/internal/model"
)

func stubSystemctl(t *testing.T, fn func(args ...string) (string, error)) {
	t.Helper()
	old := runSystemctlUser
	runSystemctlUser = fn
	t.Cleanup(func() { runSystemctlUser = old })
}

func TestRestoreUnitFilesRollsBackCreatedAndReplacedFiles(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing.service")
	created := filepath.Join(dir, "created.timer")
	if err := os.WriteFile(existing, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	snapshot, err := snapshotUnitFiles(existing, created)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(created, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := restoreUnitFiles(snapshot); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(existing)
	if err != nil || string(data) != "old" {
		t.Fatalf("existing unit not restored: %q, %v", data, err)
	}
	if _, err := os.Stat(created); !os.IsNotExist(err) {
		t.Fatalf("new unit not removed: %v", err)
	}
}

func TestWatchEnableRollbackRestoresFilesAndActivation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	unitDir, err := systemdUserDir()
	if err != nil {
		t.Fatal(err)
	}
	servicePath := filepath.Join(unitDir, "nugs-watch.service")
	timerPath := filepath.Join(unitDir, "nugs-watch.timer")
	if err := os.WriteFile(servicePath, []byte("old service"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(timerPath, []byte("old timer"), 0644); err != nil {
		t.Fatal(err)
	}

	var calls []string
	stubSystemctl(t, func(args ...string) (string, error) {
		call := strings.Join(args, " ")
		calls = append(calls, call)
		switch call {
		case "is-enabled nugs-watch.timer":
			return "enabled\n", nil
		case "is-active nugs-watch.timer":
			return "active\n", nil
		case "daemon-reload":
			return "bus unavailable", errors.New("exit status 1")
		default:
			return "", nil
		}
	})
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	err = WatchEnable(&model.Config{WatchedArtists: []string{"1125"}, FfmpegNameStr: executable})
	if err == nil || !strings.Contains(err.Error(), "rollback failed") {
		t.Fatalf("WatchEnable() error = %v, want surfaced rollback failure", err)
	}
	for path, want := range map[string]string{servicePath: "old service", timerPath: "old timer"} {
		got, readErr := os.ReadFile(path)
		if readErr != nil || string(got) != want {
			t.Fatalf("restored %s = %q, err %v; want %q", path, got, readErr, want)
		}
	}
	joined := strings.Join(calls, "\n")
	for _, want := range []string{"disable --now nugs-watch.timer", "enable nugs-watch.timer", "start nugs-watch.timer"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("systemctl calls missing %q:\n%s", want, joined)
		}
	}
}

func TestWatchDisableTreatsAbsentUnitsAsAlreadyDisabled(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	stubSystemctl(t, func(args ...string) (string, error) {
		switch args[0] {
		case "is-enabled":
			return "not-found\n", errors.New("exit status 4")
		case "is-active":
			return "unknown\n", errors.New("exit status 3")
		case "disable", "stop":
			return "Unit nugs-watch.timer does not exist", errors.New("exit status 1")
		default:
			return "", nil
		}
	})
	if err := WatchDisable(); err != nil {
		t.Fatalf("WatchDisable() error = %v, want idempotent success", err)
	}
}

func TestSnapshotUnitFilesReportsReadFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unit.service")
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := snapshotUnitFiles(path); err == nil {
		t.Fatal("snapshotUnitFiles() error = nil for unreadable unit path")
	}
}

func TestRestoreUnitFilesReportsRemoveAndWriteFailures(t *testing.T) {
	t.Run("remove", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "unit.service")
		if err := os.Mkdir(path, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "child"), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := restoreUnitFiles(unitSnapshot{path: {}}); err == nil {
			t.Fatal("restoreUnitFiles() error = nil for failed removal")
		}
	})

	t.Run("write", func(t *testing.T) {
		parent := filepath.Join(t.TempDir(), "not-a-directory")
		if err := os.WriteFile(parent, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(parent, "unit.service")
		snapshot := unitSnapshot{path: {exists: true, data: []byte("unit")}}
		if err := restoreUnitFiles(snapshot); err == nil {
			t.Fatal("restoreUnitFiles() error = nil for failed write")
		}
	})
}
