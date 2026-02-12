package rclone

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jmagar/nugs-cli/internal/helpers"
	"github.com/jmagar/nugs-cli/internal/model"
	"github.com/jmagar/nugs-cli/internal/ui"
)

// StorageAdapter is a storage provider backed by the rclone CLI.
// Function fields are injected to keep the adapter unit-testable.
type StorageAdapter struct {
	validatePath      func(path string) error
	getRcloneBasePath func(cfg *model.Config, isVideo bool) string
	calculateLocal    func(path string) int64
	removeAll         func(path string) error

	buildUploadCommand func(localPath, artistFolder string, cfg *model.Config, transfers int, isVideo bool) (*exec.Cmd, string, error)
	buildVerifyCommand func(localPath, remoteFullPath string) (*exec.Cmd, error)
	runWithProgress    func(cmd *exec.Cmd, onProgress UploadProgressFunc) error
	runCommand         func(cmd *exec.Cmd) error
	outputCommand      func(cmd *exec.Cmd) ([]byte, error)
	command            func(name string, args ...string) *exec.Cmd
	commandContext     func(ctx context.Context, name string, args ...string) *exec.Cmd
	exitCode           func(err error) (int, bool)
}

// NewStorageAdapter creates an rclone-backed storage adapter.
func NewStorageAdapter() *StorageAdapter {
	return &StorageAdapter{
		validatePath:       helpers.ValidatePath,
		getRcloneBasePath:  helpers.GetRcloneBasePath,
		calculateLocal:     helpers.CalculateLocalSize,
		removeAll:          os.RemoveAll,
		buildUploadCommand: BuildRcloneUploadCommand,
		buildVerifyCommand: BuildRcloneVerifyCommand,
		runWithProgress: func(cmd *exec.Cmd, onProgress UploadProgressFunc) error {
			return RunRcloneWithProgress(cmd, onProgress)
		},
		runCommand:     func(cmd *exec.Cmd) error { return cmd.Run() },
		outputCommand:  func(cmd *exec.Cmd) ([]byte, error) { return cmd.Output() },
		command:        exec.Command,
		commandContext: exec.CommandContext,
		exitCode:       parseExitCode,
	}
}

func parseExitCode(err error) (int, bool) {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), true
	}
	return 0, false
}

// Upload implements model.StorageProvider.
func (a *StorageAdapter) Upload(ctx context.Context, cfg *model.Config, req model.UploadRequest, hooks model.StorageHooks) error {
	if !cfg.RcloneEnabled {
		return nil
	}
	if err := a.validatePath(req.LocalPath); err != nil {
		return fmt.Errorf("invalid local path: %w", err)
	}
	if req.ArtistFolder != "" {
		if err := a.validatePath(req.ArtistFolder); err != nil {
			return fmt.Errorf("invalid artist folder: %w", err)
		}
	}

	transfers := cfg.RcloneTransfers
	if transfers == 0 {
		transfers = 4
	}

	cmd, remoteFullPath, err := a.buildUploadCommand(req.LocalPath, req.ArtistFolder, cfg, transfers, req.IsVideo)
	if err != nil {
		return err
	}

	if hooks.OnPreUpload != nil {
		totalBytes := a.calculateLocal(req.LocalPath)
		if totalBytes > 0 {
			hooks.OnPreUpload(totalBytes)
		}
	}

	progressFn := UploadProgressFunc(nil)
	if hooks.OnProgress != nil {
		progressFn = func(percent int, speed, uploaded, total string) {
			hooks.OnProgress(model.UploadProgress{
				Percent:  percent,
				Speed:    speed,
				Uploaded: uploaded,
				Total:    total,
			})
		}
	}
	if progressFn != nil {
		err = a.runWithProgress(cmd, progressFn)
	} else {
		ui.PrintUpload(fmt.Sprintf("Uploading to %s%s%s...", ui.ColorBold, remoteFullPath, ui.ColorReset))
		err = a.runWithProgress(cmd, nil)
		fmt.Println("")
	}
	if err != nil {
		return fmt.Errorf("rclone upload failed: %w", err)
	}

	if hooks.OnComplete != nil {
		hooks.OnComplete()
	}
	if progressFn == nil {
		ui.PrintSuccess("Upload complete!")
	}

	if !cfg.DeleteAfterUpload {
		return nil
	}
	if progressFn == nil {
		ui.PrintInfo("Verifying upload integrity...")
	}

	verifyCmd, err := a.buildVerifyCommand(req.LocalPath, remoteFullPath)
	if err != nil {
		return fmt.Errorf("failed to build upload verification command: %w", err)
	}
	var verifyOut, verifyErr bytes.Buffer
	verifyCmd.Stdout = &verifyOut
	verifyCmd.Stderr = &verifyErr
	if err := a.runCommand(verifyCmd); err != nil {
		return fmt.Errorf("upload verification failed - NOT deleting local files: %w\nOutput: %s\nErrors: %s",
			err, verifyOut.String(), verifyErr.String())
	}

	if progressFn == nil {
		ui.PrintSuccess("Upload verified successfully")
		fmt.Printf("Deleting local files: %s\n", req.LocalPath)
	}
	if hooks.OnDeleteAfterUpload != nil {
		hooks.OnDeleteAfterUpload(req.LocalPath)
	}
	if err := a.removeAll(req.LocalPath); err != nil {
		return fmt.Errorf("failed to delete local files: %w", err)
	}
	if progressFn == nil {
		ui.PrintSuccess("Local files deleted")
	}
	return nil
}

// PathExists implements model.StorageProvider.
func (a *StorageAdapter) PathExists(ctx context.Context, cfg *model.Config, remotePath string, isVideo bool) (bool, error) {
	if !cfg.RcloneEnabled {
		return false, nil
	}
	if err := a.validatePath(remotePath); err != nil {
		return false, fmt.Errorf("invalid remote path: %w", err)
	}

	remoteDest := cfg.RcloneRemote + ":" + a.getRcloneBasePath(cfg, isVideo)
	fullPath := remoteDest + "/" + remotePath

	if ctx == nil {
		ctx = context.Background()
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := a.commandContext(timeoutCtx, "rclone", "lsf", fullPath)
	err := a.runCommand(cmd)
	if err == nil {
		return true, nil
	}
	if code, ok := a.exitCode(err); ok {
		if code == 3 {
			return false, nil
		}
		return false, fmt.Errorf("rclone error checking remote path (exit %d): %w", code, err)
	}
	return false, fmt.Errorf("failed to execute rclone: %w", err)
}

// ListArtistFolders implements model.StorageProvider.
func (a *StorageAdapter) ListArtistFolders(ctx context.Context, cfg *model.Config, artistFolder string, isVideo bool) (map[string]struct{}, error) {
	folders := make(map[string]struct{})
	if !cfg.RcloneEnabled {
		return folders, nil
	}
	if err := a.validatePath(artistFolder); err != nil {
		return nil, fmt.Errorf("invalid artist folder: %w", err)
	}

	remoteDest := cfg.RcloneRemote + ":" + a.getRcloneBasePath(cfg, isVideo)
	fullPath := remoteDest + "/" + artistFolder
	cmd := a.command("rclone", "lsf", fullPath, "--dirs-only")
	output, err := a.outputCommand(cmd)
	if err != nil {
		if code, ok := a.exitCode(err); ok && code == 3 {
			return folders, nil
		}
		return nil, fmt.Errorf("failed to list remote artist folders: %w", err)
	}

	for _, line := range strings.Split(string(output), "\n") {
		trimmed := strings.TrimSuffix(strings.TrimSpace(line), "/")
		if trimmed == "" {
			continue
		}
		folders[trimmed] = struct{}{}
	}
	return folders, nil
}
