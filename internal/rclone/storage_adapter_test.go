package rclone

import (
	"context"
	"errors"
	"os/exec"
	"testing"

	"github.com/jmagar/nugs-cli/internal/model"
)

func TestStorageAdapterUploadWithHooksAndDelete(t *testing.T) {
	adapter := NewStorageAdapter()
	adapter.validatePath = func(string) error { return nil }
	adapter.calculateLocal = func(string) int64 { return 4096 }
	adapter.buildUploadCommand = func(localPath, artistFolder string, cfg *model.Config, transfers int, isVideo bool) (*exec.Cmd, string, error) {
		if localPath != "/tmp/local" {
			t.Fatalf("local path = %q, want /tmp/local", localPath)
		}
		if artistFolder != "artist" {
			t.Fatalf("artist folder = %q, want artist", artistFolder)
		}
		if transfers != 4 {
			t.Fatalf("transfers = %d, want 4 default", transfers)
		}
		if !isVideo {
			t.Fatal("expected video upload request")
		}
		return exec.Command("echo"), "remote:path/artist/local", nil
	}
	adapter.runWithProgress = func(cmd *exec.Cmd, onProgress UploadProgressFunc) error {
		if onProgress == nil {
			t.Fatal("expected progress callback")
		}
		onProgress(55, "8 MiB/s", "440 MiB", "800 MiB")
		return nil
	}
	adapter.buildVerifyCommand = func(localPath, remoteFullPath string) (*exec.Cmd, error) {
		if remoteFullPath != "remote:path/artist/local" {
			t.Fatalf("verify remote path = %q", remoteFullPath)
		}
		return exec.Command("echo"), nil
	}
	adapter.runCommand = func(*exec.Cmd) error { return nil }
	deletedPath := ""
	adapter.removeAll = func(path string) error {
		deletedPath = path
		return nil
	}

	preBytes := int64(0)
	completeCalled := false
	deleteHookPath := ""
	progressSeen := model.UploadProgress{}
	err := adapter.Upload(context.Background(), &model.Config{
		RcloneEnabled:     true,
		DeleteAfterUpload: true,
	}, model.UploadRequest{
		LocalPath:    "/tmp/local",
		ArtistFolder: "artist",
		IsVideo:      true,
	}, model.StorageHooks{
		OnPreUpload: func(totalBytes int64) {
			preBytes = totalBytes
		},
		OnProgress: func(progress model.UploadProgress) {
			progressSeen = progress
		},
		OnComplete: func() {
			completeCalled = true
		},
		OnDeleteAfterUpload: func(localPath string) {
			deleteHookPath = localPath
		},
	})
	if err != nil {
		t.Fatalf("upload returned error: %v", err)
	}
	if preBytes != 4096 {
		t.Fatalf("pre-upload bytes = %d, want 4096", preBytes)
	}
	if progressSeen.Percent != 55 || progressSeen.Speed != "8 MiB/s" {
		t.Fatalf("unexpected progress update: %+v", progressSeen)
	}
	if !completeCalled {
		t.Fatal("expected completion hook")
	}
	if deleteHookPath != "/tmp/local" {
		t.Fatalf("delete hook path = %q, want /tmp/local", deleteHookPath)
	}
	if deletedPath != "/tmp/local" {
		t.Fatalf("deleted path = %q, want /tmp/local", deletedPath)
	}
}

func TestStorageAdapterPathExistsHandlesExitCodeThree(t *testing.T) {
	adapter := NewStorageAdapter()
	adapter.validatePath = func(string) error { return nil }
	adapter.commandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("echo")
	}
	notFoundErr := errors.New("missing")
	adapter.runCommand = func(*exec.Cmd) error { return notFoundErr }
	adapter.exitCode = func(err error) (int, bool) {
		if err == notFoundErr {
			return 3, true
		}
		return 0, false
	}

	exists, err := adapter.PathExists(context.Background(), &model.Config{
		RcloneEnabled: true,
		RcloneRemote:  "remote",
		RclonePath:    "Music",
	}, "artist/show", false)
	if err != nil {
		t.Fatalf("path exists returned error: %v", err)
	}
	if exists {
		t.Fatal("expected path to be absent")
	}
}

func TestStorageAdapterListArtistFoldersParsesOutput(t *testing.T) {
	adapter := NewStorageAdapter()
	adapter.validatePath = func(string) error { return nil }
	adapter.command = func(name string, args ...string) *exec.Cmd {
		return exec.Command("echo")
	}
	adapter.outputCommand = func(*exec.Cmd) ([]byte, error) {
		return []byte("2026-01-01/\n2026-01-02/\n\n"), nil
	}

	folders, err := adapter.ListArtistFolders(context.Background(), &model.Config{
		RcloneEnabled: true,
		RcloneRemote:  "remote",
		RclonePath:    "Music",
	}, "artist", false)
	if err != nil {
		t.Fatalf("list artist folders returned error: %v", err)
	}
	if len(folders) != 2 {
		t.Fatalf("folders len = %d, want 2", len(folders))
	}
	if _, ok := folders["2026-01-01"]; !ok {
		t.Fatal("expected folder 2026-01-01")
	}
	if _, ok := folders["2026-01-02"]; !ok {
		t.Fatal("expected folder 2026-01-02")
	}
}
