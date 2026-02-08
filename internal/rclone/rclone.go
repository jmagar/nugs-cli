package rclone

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
	"github.com/jmagar/nugs-cli/internal/helpers"
	"github.com/jmagar/nugs-cli/internal/model"
	"github.com/jmagar/nugs-cli/internal/ui"
)

// CheckRcloneAvailable verifies rclone is installed and available in PATH.
func CheckRcloneAvailable(quiet bool) error {
	cmd := exec.Command("rclone", "version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("rclone is not installed or not available in PATH: %w\n"+
			"Please install rclone from https://rclone.org/downloads/ or disable rclone in config.json", err)
	}

	if !quiet {
		lines := strings.Split(string(output), "\n")
		if len(lines) > 0 {
			ui.PrintSuccess(fmt.Sprintf("Rclone is available: %s", strings.TrimSpace(lines[0])))
		}
	}

	return nil
}

// CheckRclonePathOnline checks if the configured rclone remote is reachable.
func CheckRclonePathOnline(cfg *model.Config) string {
	if !cfg.RcloneEnabled {
		return "Disabled"
	}
	if strings.TrimSpace(cfg.RcloneRemote) == "" {
		return "Offline (remote not configured)"
	}
	target := cfg.RcloneRemote + ":" + helpers.GetRcloneBasePath(cfg, false)
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
		return "Online (path missing)"
	}
	return "Offline"
}

// UploadProgressFunc is a callback for upload progress updates.
type UploadProgressFunc func(percent int, speed, uploaded, total string)

// UploadToRclone uploads the local path to the configured rclone remote.
// If progressFn is non-nil, it is called with progress updates.
// If fallbackPrintFn is non-nil, it is used for non-progress-box output.
func UploadToRclone(localPath string, artistFolder string, cfg *model.Config, progressFn UploadProgressFunc, isVideo bool, onPreUpload func(totalBytes int64), onComplete func(), onDeleteAfterUpload func(localPath string)) error {
	if !cfg.RcloneEnabled {
		return nil
	}

	if err := helpers.ValidatePath(localPath); err != nil {
		return fmt.Errorf("invalid local path: %w", err)
	}

	if artistFolder != "" {
		if err := helpers.ValidatePath(artistFolder); err != nil {
			return fmt.Errorf("invalid artist folder: %w", err)
		}
	}

	transfers := cfg.RcloneTransfers
	if transfers == 0 {
		transfers = 4
	}

	cmd, remoteFullPath, err := BuildRcloneUploadCommand(localPath, artistFolder, cfg, transfers, isVideo)
	if err != nil {
		return err
	}

	// Calculate local directory size for upload progress
	if onPreUpload != nil {
		totalBytes := helpers.CalculateLocalSize(localPath)
		if totalBytes > 0 {
			onPreUpload(totalBytes)
		}
	}

	if progressFn != nil {
		err = RunRcloneWithProgress(cmd, progressFn)
	} else {
		ui.PrintUpload(fmt.Sprintf("Uploading to %s%s%s...", ui.ColorBold, remoteFullPath, ui.ColorReset))
		err = RunRcloneWithProgress(cmd, nil)
		fmt.Println("")
	}

	if err != nil {
		return fmt.Errorf("rclone upload failed: %w", err)
	}

	if progressFn == nil {
		ui.PrintSuccess("Upload complete!")
	}

	if cfg.DeleteAfterUpload {
		if progressFn == nil {
			ui.PrintInfo("Verifying upload integrity...")
		}
		verifyCmd, err := BuildRcloneVerifyCommand(localPath, remoteFullPath)
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

		if progressFn == nil {
			ui.PrintSuccess("Upload verified successfully")
			fmt.Printf("Deleting local files: %s\n", localPath)
		}
		err = os.RemoveAll(localPath)
		if err != nil {
			return fmt.Errorf("failed to delete local files: %w", err)
		}
		if progressFn == nil {
			ui.PrintSuccess("Local files deleted")
		}
	}

	return nil
}

// BuildRcloneUploadCommand constructs the rclone copy/copyto command.
func BuildRcloneUploadCommand(localPath, artistFolder string, cfg *model.Config, transfers int, isVideo bool) (*exec.Cmd, string, error) {
	remoteDest := cfg.RcloneRemote + ":" + helpers.GetRcloneBasePath(cfg, isVideo)

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

// ParseHumanizedBytes converts a human-readable byte string back to bytes.
func ParseHumanizedBytes(s string) int64 {
	s = strings.TrimSuffix(s, "/s")
	s = strings.TrimSpace(s)

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

// ParseRcloneProgressLine parses a line of rclone --progress output.
func ParseRcloneProgressLine(line string) (int, string, string, string, bool) {
	line = strings.TrimSpace(ui.StripAnsiCodes(line))
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
		if computed, ok := ComputeProgressPercent(uploaded, total); ok {
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

// ComputeProgressPercent computes percent from uploaded/total human-readable strings.
func ComputeProgressPercent(uploaded, total string) (int, bool) {
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

// RunRcloneWithProgress runs an rclone command and reports progress.
func RunRcloneWithProgress(cmd *exec.Cmd, onProgress func(percent int, speed, uploaded, total string)) error {
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
			percent, speed, uploaded, total, ok := ParseRcloneProgressLine(line)
			if ok {
				if onProgress != nil {
					onProgress(percent, speed, uploaded, total)
				} else {
					ui.PrintUploadProgress(percent, speed, uploaded, total)
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
		ui.PrintUploadProgress(0, "0 B", "0", "...")
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

// BuildRcloneVerifyCommand constructs the rclone check command for upload verification.
func BuildRcloneVerifyCommand(localPath, remoteFullPath string) (*exec.Cmd, error) {
	localInfo, err := os.Stat(localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat local path for verification: %w", err)
	}

	if localInfo.IsDir() {
		return exec.Command("rclone", "check", "--one-way", localPath, remoteFullPath), nil
	}

	localDir := filepath.Dir(localPath)
	remoteDir := path.Dir(remoteFullPath)
	fileName := filepath.Base(localPath)
	return exec.Command("rclone", "check", "--one-way", "--include", fileName, localDir, remoteDir), nil
}

// RemotePathExists checks if a path exists on the configured rclone remote.
func RemotePathExists(remotePath string, cfg *model.Config, isVideo bool) (bool, error) {
	if !cfg.RcloneEnabled {
		return false, nil
	}

	if err := helpers.ValidatePath(remotePath); err != nil {
		return false, fmt.Errorf("invalid remote path: %w", err)
	}

	remoteDest := cfg.RcloneRemote + ":" + helpers.GetRcloneBasePath(cfg, isVideo)
	fullPath := remoteDest + "/" + remotePath

	cmd := exec.Command("rclone", "lsf", fullPath)
	err := cmd.Run()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 3 {
				return false, nil
			}
			return false, fmt.Errorf("rclone error checking remote path (exit %d): %w", exitErr.ExitCode(), err)
		}
		return false, fmt.Errorf("failed to execute rclone: %w", err)
	}

	return true, nil
}

// ListRemoteArtistFolders returns show folder names under one artist folder on remote storage.
func ListRemoteArtistFolders(artistFolder string, cfg *model.Config) (map[string]struct{}, error) {
	folders := make(map[string]struct{})
	if !cfg.RcloneEnabled {
		return folders, nil
	}

	if err := helpers.ValidatePath(artistFolder); err != nil {
		return nil, fmt.Errorf("invalid artist folder: %w", err)
	}

	remoteDest := cfg.RcloneRemote + ":" + helpers.GetRcloneBasePath(cfg, false)
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
