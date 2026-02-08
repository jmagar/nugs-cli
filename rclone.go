package main

// Rclone wrappers delegating to internal/rclone during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import (
	"os/exec"

	"github.com/dustin/go-humanize"
	"github.com/jmagar/nugs-cli/internal/rclone"
)

func checkRcloneAvailable(quiet bool) error { return rclone.CheckRcloneAvailable(quiet) }
func checkRclonePathOnline(cfg *Config) string { return rclone.CheckRclonePathOnline(cfg) }

func uploadToRclone(localPath string, artistFolder string, cfg *Config, progressBox *ProgressBoxState, isVideo bool) error {
	if progressBox != nil {
		// Wire up progress callback to update the ProgressBoxState
		progressFn := func(percent int, speed, uploaded, total string) {
			progressBox.mu.Lock()
			progressBox.UploadPercent = percent
			progressBox.UploadSpeed = speed
			progressBox.Uploaded = uploaded
			if progressBox.UploadTotal == "" || (total != "" && total != "...") {
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
			progressBox.mu.Unlock()
			renderProgressBox(progressBox)
		}

		onPreUpload := func(totalBytes int64) {
			progressBox.UploadTotal = humanize.Bytes(uint64(totalBytes))
			progressBox.Uploaded = "0 B"
			progressBox.UploadPercent = 0
		}

		return rclone.UploadToRclone(localPath, artistFolder, cfg, progressFn, isVideo, onPreUpload, nil, nil)
	}

	return rclone.UploadToRclone(localPath, artistFolder, cfg, nil, isVideo, nil, nil, nil)
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

func remotePathExists(remotePath string, cfg *Config, isVideo bool) (bool, error) {
	return rclone.RemotePathExists(remotePath, cfg, isVideo)
}

func listRemoteArtistFolders(artistFolder string, cfg *Config) (map[string]struct{}, error) {
	return rclone.ListRemoteArtistFolders(artistFolder, cfg)
}

