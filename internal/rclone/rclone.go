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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/jmagar/nugs-cli/internal/helpers"
	"github.com/jmagar/nugs-cli/internal/model"
	"github.com/jmagar/nugs-cli/internal/ui"
)

var (
	transferredSegmentPattern = regexp.MustCompile(`(?i)\btransferred:\s*(.+)$`)
	transferredPairPattern    = regexp.MustCompile(`^\s*([^,]+?)\s*/\s*([^,]+?)(?:\s*,|$)`)
	percentPattern            = regexp.MustCompile(`(\d{1,3})\s*%`)
	speedPattern              = regexp.MustCompile(`(?:^|,)\s*@?\s*([^,]*?/s)\s*(?:,|$)`)
)

var defaultStorageProvider model.StorageProvider = NewStorageAdapter()

// SetStorageProvider swaps the default storage provider used by rclone wrappers.
// Passing nil resets it to the standard rclone adapter.
func SetStorageProvider(provider model.StorageProvider) {
	if provider == nil {
		defaultStorageProvider = NewStorageAdapter()
		return
	}
	defaultStorageProvider = provider
}

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
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 3 {
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
	return defaultStorageProvider.Upload(context.Background(), cfg, model.UploadRequest{
		LocalPath:    localPath,
		ArtistFolder: artistFolder,
		IsVideo:      isVideo,
	}, model.StorageHooks{
		OnProgress: func(progress model.UploadProgress) {
			if progressFn == nil {
				return
			}
			progressFn(progress.Percent, progress.Speed, progress.Uploaded, progress.Total)
		},
		OnPreUpload:         onPreUpload,
		OnComplete:          onComplete,
		OnDeleteAfterUpload: onDeleteAfterUpload,
	})
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

	segmentMatch := transferredSegmentPattern.FindStringSubmatch(line)
	if len(segmentMatch) < 2 {
		return 0, "", "", "", false
	}

	segment := strings.TrimSpace(segmentMatch[1])
	pairMatch := transferredPairPattern.FindStringSubmatch(segment)
	if len(pairMatch) < 3 {
		return 0, "", "", "", false
	}

	uploaded := strings.Join(strings.Fields(strings.TrimSpace(pairMatch[1])), " ")
	total := strings.Join(strings.Fields(strings.TrimSpace(pairMatch[2])), " ")
	if uploaded == "" || total == "" {
		return 0, "", "", "", false
	}

	percent := -1
	percentMatch := percentPattern.FindStringSubmatch(segment)
	if len(percentMatch) > 1 {
		if parsed, err := strconv.Atoi(strings.TrimSpace(percentMatch[1])); err == nil {
			percent = parsed
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
	speedMatch := speedPattern.FindStringSubmatch(segment)
	if len(speedMatch) > 1 {
		speed = strings.TrimSpace(speedMatch[1])
	}
	speed = strings.TrimPrefix(speed, "@")
	speed = strings.TrimPrefix(speed, "@ ")
	speed = strings.TrimSpace(speed)
	if speed == "" {
		speed = "0 B"
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

	// Wait for pipe readers to finish first (per Go docs requirement)
	wg.Wait()

	// Then wait for process to exit
	waitErr := cmd.Wait()
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
	return defaultStorageProvider.PathExists(context.Background(), cfg, remotePath, isVideo)
}

// ListRemoteArtistFolders returns show folder names under one artist folder on remote storage.
func ListRemoteArtistFolders(artistFolder string, cfg *model.Config) (map[string]struct{}, error) {
	return defaultStorageProvider.ListArtistFolders(context.Background(), cfg, artistFolder, false)
}
