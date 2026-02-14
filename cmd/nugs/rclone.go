package main

// Rclone wrappers delegating to internal/rclone during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/jmagar/nugs-cli/internal/model"
	"github.com/jmagar/nugs-cli/internal/rclone"
)

const (
	// uploadCompleteVisibilityDelay is how long to pause after upload reaches 100%
	// before showing the completion summary. This ensures users see the final state.
	uploadCompleteVisibilityDelay = 500 * time.Millisecond
)

func checkRcloneAvailable(quiet bool) error    { return rclone.CheckRcloneAvailable(quiet) }
func checkRclonePathOnline(cfg *Config) string { return rclone.CheckRclonePathOnline(cfg) }

func uploadToRclone(localPath string, artistFolder string, cfg *Config, progressBox *ProgressBoxState, isVideo bool) error {
	if progressBox != nil {
		return uploadWithProgressBox(localPath, artistFolder, cfg, progressBox, isVideo)
	}
	return rclone.UploadToRclone(context.Background(), localPath, artistFolder, cfg, nil, isVideo, nil, nil, nil)
}

// uploadWithProgressBox handles upload with progress box updates and phase tracking.
// Separated from uploadToRclone for better testability and maintainability.
func uploadWithProgressBox(localPath string, artistFolder string, cfg *Config, progressBox *ProgressBoxState, isVideo bool) error {
	{
		// Set upload phase and start time
		progressBox.Mu.Lock()
		progressBox.SetPhaseLocked(model.PhaseUpload)
		progressBox.UploadStartTime = time.Now()
		progressBox.Mu.Unlock()
		renderProgressBox(progressBox)

		// Wire up progress callback to update the ProgressBoxState
		progressFn := func(percent int, speed, uploaded, total string) {
			progressBox.Mu.Lock()
			progressBox.UploadPercent = percent
			progressBox.UploadSpeed = speed
			progressBox.Uploaded = uploaded
			// Only update UploadTotal if not already set by onPreUpload (calculated from directory size)
			if !progressBox.UploadTotalSet && total != "" && total != "..." {
				progressBox.UploadTotal = total
			}

			if percent < 100 && percent > 0 {
				speedBytes := rclone.ParseHumanizedBytes(speed)
				if speedBytes > 0 {
					progressBox.UploadSpeedHistory = updateSpeedHistory(progressBox.UploadSpeedHistory, float64(speedBytes))
					totalBytes := rclone.ParseHumanizedBytes(total)
					uploadedBytes := rclone.ParseHumanizedBytes(uploaded)
					if totalBytes > 0 && uploadedBytes > 0 {
						remaining := totalBytes - uploadedBytes
						progressBox.UploadETA = calculateETA(progressBox.UploadSpeedHistory, remaining)
					}
				}
			} else {
				progressBox.UploadETA = ""
			}
			progressBox.ForceRender = true // Force render on upload progress updates
			progressBox.Mu.Unlock()

			// renderProgressBox locks internally, call outside our lock
			renderProgressBox(progressBox)
		}

		onPreUpload := func(totalBytes int64) {
			progressBox.Mu.Lock()
			progressBox.UploadTotal = humanize.Bytes(uint64(totalBytes))
			progressBox.UploadTotalSet = true // Prevent rclone from overwriting calculated total
			progressBox.Uploaded = "0 B"
			progressBox.UploadPercent = 0
			progressBox.ForceRender = true
			progressBox.Mu.Unlock()

			// SetMessage locks internally, so call it outside our lock
			progressBox.SetMessage(model.MessagePriorityStatus, "Uploading to remote...", 0)
			renderProgressBox(progressBox)
		}

		err := rclone.UploadToRclone(context.Background(), localPath, artistFolder, cfg, progressFn, isVideo, onPreUpload, nil, nil)

		// Set upload duration and final state (success or error)
		progressBox.Mu.Lock()
		if err == nil {
			progressBox.UploadDuration = time.Since(progressBox.UploadStartTime)
			progressBox.UploadPercent = 100
			progressBox.SetPhaseLocked(model.PhaseComplete)
		} else {
			// Calculate duration even on error for stats
			if !progressBox.UploadStartTime.IsZero() {
				progressBox.UploadDuration = time.Since(progressBox.UploadStartTime)
			}
			progressBox.SetPhaseLocked(model.PhaseError)
		}
		progressBox.ForceRender = true
		progressBox.Mu.Unlock()

		// Show error message if upload failed (SetMessage locks internally)
		if err != nil {
			progressBox.SetMessage(model.MessagePriorityError, fmt.Sprintf("Upload failed: %v", err), 5*time.Second)
		}

		// Force final render with pause for visibility (allows user to see 100% or error state)
		renderProgressBox(progressBox)
		time.Sleep(uploadCompleteVisibilityDelay)

		return err
	}
}

func buildRcloneUploadCommand(localPath, artistFolder string, cfg *Config, transfers int, isVideo bool) (*exec.Cmd, string, error) {
	return rclone.BuildRcloneUploadCommand(localPath, artistFolder, cfg, transfers, isVideo)
}

func parseHumanizedBytes(s string) int64 {
	return rclone.ParseHumanizedBytes(s)
}

func parseRcloneProgressLine(line string) (int, string, string, string, bool) {
	return rclone.ParseRcloneProgressLine(line)
}

func computeProgressPercent(uploaded, total string) (int, bool) {
	return rclone.ComputeProgressPercent(uploaded, total)
}

func runRcloneWithProgress(cmd *exec.Cmd, onProgress func(percent int, speed, uploaded, total string)) error {
	return rclone.RunRcloneWithProgress(cmd, onProgress)
}

func buildRcloneVerifyCommand(localPath, remoteFullPath string) (*exec.Cmd, error) {
	return rclone.BuildRcloneVerifyCommand(localPath, remoteFullPath)
}

func remotePathExists(ctx context.Context, remotePath string, cfg *Config, isVideo bool) (bool, error) {
	return rclone.RemotePathExists(ctx, remotePath, cfg, isVideo)
}

func listRemoteArtistFolders(ctx context.Context, artistFolder string, cfg *Config, isVideo bool) (map[string]struct{}, error) {
	return rclone.ListRemoteArtistFolders(ctx, artistFolder, cfg, isVideo)
}
