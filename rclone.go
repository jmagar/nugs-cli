package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
)

func checkRcloneAvailable(quiet bool) error {
	cmd := exec.Command("rclone", "version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("rclone is not installed or not available in PATH: %w\n"+
			"Please install rclone from https://rclone.org/downloads/ or disable rclone in config.json", err)
	}

	// Extract and display version (first line of output)
	if !quiet {
		lines := strings.Split(string(output), "\n")
		if len(lines) > 0 {
			printSuccess(fmt.Sprintf("Rclone is available: %s", strings.TrimSpace(lines[0])))
		}
	}

	return nil
}

func checkRclonePathOnline(cfg *Config) string {
	if !cfg.RcloneEnabled {
		return "Disabled"
	}
	if strings.TrimSpace(cfg.RcloneRemote) == "" {
		return "Offline (remote not configured)"
	}
	target := cfg.RcloneRemote + ":" + cfg.RclonePath
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "rclone", "lsf", target)
	err := cmd.Run()
	if err == nil {
		return "Online"
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "Offline (timeout)"
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 3 {
		// Path missing, but remote is reachable/authenticated.
		return "Online (path missing)"
	}
	return "Offline"
}

// uploadToRclone uploads the local directory at localPath to the configured rclone remote.
// If cfg.RcloneEnabled is false, the function returns immediately without error.
// The function uses cfg.RcloneTransfers (default 4) for parallel transfers and can
// optionally delete local files after successful upload verification if cfg.DeleteAfterUpload is true.
// Returns an error if:
//   - Path validation fails
//   - rclone copy command fails
//   - Upload verification fails (requires rclone check command)
//   - Local file deletion fails after successful upload
func uploadToRclone(localPath string, artistFolder string, cfg *Config, progressBox *ProgressBoxState) error {
	if !cfg.RcloneEnabled {
		return nil
	}

	// Validate paths before executing rclone command
	if err := validatePath(localPath); err != nil {
		return fmt.Errorf("invalid local path: %w", err)
	}

	// Validate artist folder path if provided
	if artistFolder != "" {
		if err := validatePath(artistFolder); err != nil {
			return fmt.Errorf("invalid artist folder: %w", err)
		}
	}

	// Default to 4 transfers if not set
	transfers := cfg.RcloneTransfers
	if transfers == 0 {
		transfers = 4
	}

	cmd, remoteFullPath, err := buildRcloneUploadCommand(localPath, artistFolder, cfg, transfers)
	if err != nil {
		return err
	}

	// Calculate local directory size for upload progress (Tier 1 enhancement)
	if progressBox != nil {
		totalBytes := calculateLocalSize(localPath)
		if totalBytes > 0 {
			// Convert bytes to human-readable format (GB, MB, etc.)
			progressBox.UploadTotal = humanize.Bytes(uint64(totalBytes))
			// Initialize upload progress to show "0/X GB" instead of "0/.../0%"
			progressBox.Uploaded = "0 B"
			progressBox.UploadPercent = 0
		}
	}

	if progressBox != nil {
		// Use progress box for upload progress with ETA
		uploadProgress := func(percent int, speed, uploaded, total string) {
			// Lock for atomic update of upload progress fields
			progressBox.mu.Lock()
			progressBox.UploadPercent = percent
			progressBox.UploadSpeed = speed
			progressBox.Uploaded = uploaded
			// Only update UploadTotal if pre-calculated value isn't set or rclone provides a real value
			if progressBox.UploadTotal == "" || (total != "" && total != "...") {
				progressBox.UploadTotal = total
			}

			// Calculate upload ETA
			if percent < 100 && percent > 0 {
				// Parse speed string (e.g., "8.2 MB" -> bytes per second)
				speedBytes := parseHumanizedBytes(speed)
				if speedBytes > 0 {
					// Thread-safe: now stored in ProgressBoxState (protected by mutex)
					progressBox.UploadSpeedHistory = updateSpeedHistory(progressBox.UploadSpeedHistory, float64(speedBytes))

					// Calculate remaining bytes
					totalBytes := parseHumanizedBytes(total)
					uploadedBytes := parseHumanizedBytes(uploaded)
					if totalBytes > 0 && uploadedBytes > 0 {
						remaining := totalBytes - uploadedBytes
						progressBox.UploadETA = calculateETA(progressBox.UploadSpeedHistory, remaining)
					}
				}
			} else {
				progressBox.UploadETA = ""
			}
			progressBox.mu.Unlock()

			// Render outside lock to avoid holding during I/O
			renderProgressBox(progressBox)
		}
		err = runRcloneWithProgress(cmd, uploadProgress)
	} else {
		// Fallback to old style progress
		printUpload(fmt.Sprintf("Uploading to %s%s%s...", colorBold, remoteFullPath, colorReset))
		err = runRcloneWithProgress(cmd, nil)
		fmt.Println("")
	}

	if err != nil {
		return fmt.Errorf("rclone upload failed: %w", err)
	}

	if progressBox == nil {
		printSuccess("Upload complete!")
	}

	if cfg.DeleteAfterUpload {
		// Verify upload before deleting local files
		if progressBox == nil {
			printInfo("Verifying upload integrity...")
		}
		verifyCmd, err := buildRcloneVerifyCommand(localPath, remoteFullPath)
		if err != nil {
			return fmt.Errorf("failed to build upload verification command: %w", err)
		}
		var verifyOut, verifyErr bytes.Buffer
		verifyCmd.Stdout = &verifyOut
		verifyCmd.Stderr = &verifyErr

		err = verifyCmd.Run()
		if err != nil {
			return fmt.Errorf("upload verification failed - NOT deleting local files: %w\nOutput: %s\nErrors: %s",
				err, verifyOut.String(), verifyErr.String())
		}

		if progressBox == nil {
			printSuccess("Upload verified successfully")
			fmt.Printf("Deleting local files: %s\n", localPath)
		}
		err = os.RemoveAll(localPath)
		if err != nil {
			return fmt.Errorf("failed to delete local files: %w", err)
		}
		if progressBox == nil {
			printSuccess("Local files deleted")
		}
	}

	return nil
}

func buildRcloneUploadCommand(localPath, artistFolder string, cfg *Config, transfers int) (*exec.Cmd, string, error) {
	remoteDest := cfg.RcloneRemote + ":" + cfg.RclonePath

	localInfo, err := os.Stat(localPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to stat local path: %w", err)
	}

	remoteParentPath := remoteDest
	if artistFolder != "" {
		remoteParentPath += "/" + artistFolder
	}
	remoteFullPath := remoteParentPath + "/" + filepath.Base(localPath)

	transfersFlag := fmt.Sprintf("--transfers=%d", transfers)
	statsFlags := []string{"--progress", "--stats=1s", "--stats-one-line"}
	if localInfo.IsDir() {
		args := []string{"copy", localPath, remoteFullPath, transfersFlag}
		args = append(args, statsFlags...)
		return exec.Command("rclone", args...), remoteFullPath, nil
	}
	args := []string{"copyto", localPath, remoteFullPath, transfersFlag}
	args = append(args, statsFlags...)
	return exec.Command("rclone", args...), remoteFullPath, nil
}

// parseHumanizedBytes converts a human-readable byte string (e.g., "8.2 MB") back to bytes
// Returns 0 if parsing fails
func parseHumanizedBytes(s string) int64 {
	// Remove "/s" suffix if present
	s = strings.TrimSuffix(s, "/s")
	s = strings.TrimSpace(s)

	// Try to parse using humanize library's reverse function
	// Note: go-humanize doesn't have a built-in parser, so we'll do it manually
	parts := strings.Fields(s)
	if len(parts) != 2 {
		return 0
	}

	value := 0.0
	if _, err := fmt.Sscanf(parts[0], "%f", &value); err != nil {
		return 0
	}

	unit := strings.ToUpper(parts[1])
	multiplier := int64(1)

	switch unit {
	case "B":
		multiplier = 1
	case "KB", "KIB":
		multiplier = 1024
	case "MB", "MIB":
		multiplier = 1024 * 1024
	case "GB", "GIB":
		multiplier = 1024 * 1024 * 1024
	case "TB", "TIB":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0
	}

	return int64(value * float64(multiplier))
}

func parseRcloneProgressLine(line string) (int, string, string, string, bool) {
	line = strings.TrimSpace(stripAnsiCodes(line))
	if line == "" {
		return 0, "", "", "", false
	}
	idx := strings.Index(line, "Transferred:")
	if idx < 0 {
		return 0, "", "", "", false
	}
	segment := strings.TrimSpace(line[idx+len("Transferred:"):])
	fields := strings.Split(segment, ",")
	if len(fields) < 2 {
		return 0, "", "", "", false
	}
	transferredPart := strings.Join(strings.Fields(strings.TrimSpace(fields[0])), " ")
	var uploaded string
	var total string
	if slashIdx := strings.Index(transferredPart, " / "); slashIdx >= 0 {
		uploaded = strings.TrimSpace(transferredPart[:slashIdx])
		total = strings.TrimSpace(transferredPart[slashIdx+3:])
	} else {
		parts := strings.SplitN(transferredPart, "/", 2)
		if len(parts) != 2 {
			return 0, "", "", "", false
		}
		uploaded = strings.TrimSpace(parts[0])
		total = strings.TrimSpace(parts[1])
	}
	if uploaded == "" || total == "" {
		return 0, "", "", "", false
	}

	percent := -1
	for _, field := range fields[1:] {
		part := strings.TrimSpace(field)
		if strings.HasSuffix(part, "%") {
			num := strings.TrimSuffix(part, "%")
			if parsed, err := strconv.Atoi(strings.TrimSpace(num)); err == nil {
				percent = parsed
				break
			}
		}
	}
	if percent < 0 {
		if computed, ok := computeProgressPercent(uploaded, total); ok {
			percent = computed
		}
	}
	if percent < 0 {
		if strings.EqualFold(uploaded, total) {
			percent = 100
		} else {
			percent = 0
		}
	}

	speed := "0 B"
	for _, field := range fields[1:] {
		part := strings.TrimSpace(field)
		if strings.Contains(part, "/s") {
			speed = part
			break
		}
	}

	return percent, speed, uploaded, total, true
}

func computeProgressPercent(uploaded, total string) (int, bool) {
	upNorm := strings.ReplaceAll(strings.TrimSpace(uploaded), " ", "")
	totalNorm := strings.ReplaceAll(strings.TrimSpace(total), " ", "")
	upBytes, errUp := humanize.ParseBytes(upNorm)
	totalBytes, errTotal := humanize.ParseBytes(totalNorm)
	if errUp != nil || errTotal != nil || totalBytes == 0 {
		return 0, false
	}
	pct := int((float64(upBytes) / float64(totalBytes)) * 100)
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return pct, true
}

func runRcloneWithProgress(cmd *exec.Cmd, onProgress func(percent int, speed, uploaded, total string)) error {
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	var (
		parseMu     sync.Mutex
		diagnostics bytes.Buffer
	)

	consume := func(r io.Reader, wg *sync.WaitGroup) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, splitErr error) {
			for i, b := range data {
				if b == '\n' || b == '\r' {
					return i + 1, bytes.TrimSpace(data[:i]), nil
				}
			}
			if atEOF && len(data) > 0 {
				return len(data), bytes.TrimSpace(data), nil
			}
			return 0, nil, nil
		})
		for scanner.Scan() {
			line := scanner.Text()
			parseMu.Lock()
			percent, speed, uploaded, total, ok := parseRcloneProgressLine(line)
			if ok {
				if onProgress != nil {
					onProgress(percent, speed, uploaded, total)
				} else {
					printUploadProgress(percent, speed, uploaded, total)
				}
			} else if line != "" {
				diagnostics.WriteString(line)
				diagnostics.WriteString("\n")
			}
			parseMu.Unlock()
		}
		if scanErr := scanner.Err(); scanErr != nil {
			parseMu.Lock()
			diagnostics.WriteString(scanErr.Error())
			diagnostics.WriteString("\n")
			parseMu.Unlock()
		}
	}

	if err := cmd.Start(); err != nil {
		return err
	}
	if onProgress != nil {
		onProgress(0, "0 B", "0", "...")
	} else {
		printUploadProgress(0, "0 B", "0", "...")
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go consume(stdoutPipe, &wg)
	go consume(stderrPipe, &wg)

	waitErr := cmd.Wait()
	wg.Wait()
	if waitErr != nil && diagnostics.Len() > 0 {
		return fmt.Errorf("%w\n%s", waitErr, strings.TrimSpace(diagnostics.String()))
	}
	return waitErr
}

func buildRcloneVerifyCommand(localPath, remoteFullPath string) (*exec.Cmd, error) {
	localInfo, err := os.Stat(localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat local path for verification: %w", err)
	}

	if localInfo.IsDir() {
		return exec.Command("rclone", "check", "--one-way", localPath, remoteFullPath), nil
	}

	// rclone check operates on directories; for file uploads, compare parent dirs
	// and constrain comparison to the uploaded file only.
	localDir := filepath.Dir(localPath)
	remoteDir := path.Dir(remoteFullPath)
	fileName := filepath.Base(localPath)
	return exec.Command("rclone", "check", "--one-way", "--include", fileName, localDir, remoteDir), nil
}

// remotePathExists checks if a directory exists at the specified remotePath on the configured rclone remote.
// The remotePath is relative to cfg.RclonePath and should not include the remote name or base path.
// Returns false without error if cfg.RcloneEnabled is false.
// Returns true if the directory exists, false if it doesn't exist or on error.
func remotePathExists(remotePath string, cfg *Config) (bool, error) {
	if !cfg.RcloneEnabled {
		return false, nil
	}

	// Validate paths before executing rclone command
	if err := validatePath(remotePath); err != nil {
		return false, fmt.Errorf("invalid remote path: %w", err)
	}

	remoteDest := cfg.RcloneRemote + ":" + cfg.RclonePath
	fullPath := remoteDest + "/" + remotePath

	// A successful command means the directory exists, even if it's empty.
	// Using --dirs-only with output length checks can misclassify empty dirs.
	cmd := exec.Command("rclone", "lsf", fullPath)
	err := cmd.Run()

	if err != nil {
		// Check exit code to distinguish "doesn't exist" from other errors
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Exit code 3 means "directory not found" - this is expected
			if exitErr.ExitCode() == 3 {
				return false, nil
			}
			// Other exit codes indicate real errors
			return false, fmt.Errorf("rclone error checking remote path (exit %d): %w", exitErr.ExitCode(), err)
		}
		// Non-exit errors (e.g., rclone not found) are real errors
		return false, fmt.Errorf("failed to execute rclone: %w", err)
	}

	return true, nil
}

// listRemoteArtistFolders returns show folder names under one artist folder on remote storage.
func listRemoteArtistFolders(artistFolder string, cfg *Config) (map[string]struct{}, error) {
	folders := make(map[string]struct{})
	if !cfg.RcloneEnabled {
		return folders, nil
	}

	if err := validatePath(artistFolder); err != nil {
		return nil, fmt.Errorf("invalid artist folder: %w", err)
	}

	remoteDest := cfg.RcloneRemote + ":" + cfg.RclonePath
	fullPath := remoteDest + "/" + artistFolder

	cmd := exec.Command("rclone", "lsf", fullPath, "--dirs-only")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 3 {
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
