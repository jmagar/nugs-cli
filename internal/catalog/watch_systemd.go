//go:build linux

package catalog

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/jmagar/nugs-cli/internal/cache"
	"github.com/jmagar/nugs-cli/internal/model"
	"github.com/jmagar/nugs-cli/internal/ui"
)

const serviceTemplate = `[Unit]
Description=Nugs Watch - Auto-download new shows
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
Environment=NUGS_DETACHED=1
Environment="PATH={{.Path}}"
ExecStart="{{.BinaryPath}}" watch check
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=default.target
`

const timerTemplate = `[Unit]
Description=Nugs Watch Timer
Requires=nugs-watch.service

[Timer]
OnBootSec=5min
OnUnitActiveSec={{.WatchInterval}}

[Install]
WantedBy=timers.target
`

// WatchEnable writes systemd user unit files for the watch timer and enables them.
// Requires at least one artist in the watch list.
func WatchEnable(cfg *model.Config) error {
	if len(cfg.WatchedArtists) == 0 {
		return fmt.Errorf("no artists in watch list — add at least one with: nugs watch add <artistID>")
	}

	binPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve binary path: %w", err)
	}
	binPath, err = filepath.Abs(binPath)
	if err != nil {
		return fmt.Errorf("failed to make binary path absolute: %w", err)
	}
	if info, statErr := os.Stat(binPath); statErr != nil || info.IsDir() || info.Mode()&0111 == 0 {
		return fmt.Errorf("nugs binary is not an executable regular file: %s", binPath)
	}
	ffmpegName := cfg.FfmpegNameStr
	if ffmpegName == "" {
		ffmpegName = "ffmpeg"
	}
	if _, err := exec.LookPath(ffmpegName); err != nil {
		return fmt.Errorf("watch requires ffmpeg in the service PATH: %w", err)
	}
	if cfg.RcloneEnabled {
		if _, err := exec.LookPath("rclone"); err != nil {
			return fmt.Errorf("watch has rclone enabled but rclone is not in the service PATH: %w", err)
		}
	}

	unitDir, err := systemdUserDir()
	if err != nil {
		return err
	}

	servicePath := filepath.Join(unitDir, "nugs-watch.service")
	timerPath := filepath.Join(unitDir, "nugs-watch.timer")
	backup, err := snapshotUnitFiles(servicePath, timerPath)
	if err != nil {
		return fmt.Errorf("snapshot existing watch units: %w", err)
	}
	activation, err := snapshotUnitActivation("nugs-watch.timer")
	if err != nil {
		return fmt.Errorf("snapshot watch timer state: %w", err)
	}
	rollback := func(cause error) error {
		if rollbackErr := restoreWatchInstallation(backup, activation); rollbackErr != nil {
			return fmt.Errorf("%w; rollback failed: %w", cause, rollbackErr)
		}
		return cause
	}
	if err := writeWatchUnitFiles(cfg, unitDir, binPath); err != nil {
		if restoreErr := restoreUnitFiles(backup); restoreErr != nil {
			return fmt.Errorf("%w; restore unit files: %w", err, restoreErr)
		}
		return err
	}

	if err := systemctlUser("daemon-reload"); err != nil {
		return rollback(fmt.Errorf("daemon-reload failed: %w", err))
	}
	if err := systemctlUser("enable", "--now", "nugs-watch.timer"); err != nil {
		return rollback(fmt.Errorf("failed to enable timer: %w", err))
	}

	systemdInterval, _ := toSystemdDuration(watchIntervalOrDefault(cfg))

	ui.PrintSuccess("Nugs watch timer enabled")
	ui.PrintKeyValue("Binary", binPath, ui.ColorCyan)
	ui.PrintKeyValue("Interval", systemdInterval, ui.ColorCyan)
	ui.PrintKeyValue("Service", servicePath, ui.ColorCyan)
	ui.PrintKeyValue("Timer", timerPath, ui.ColorCyan)
	fmt.Println()
	fmt.Printf("  View logs: journalctl --user -u nugs-watch.service\n")
	fmt.Printf("  List timers: systemctl --user list-timers\n")
	return nil
}

// writeWatchUnitFiles renders and writes the service and timer unit files to unitDir.
// Separated from systemctl invocations so unit file content can be tested independently.
// The interval in cfg.WatchInterval is validated and converted to systemd time(7) format.
func writeWatchUnitFiles(cfg *model.Config, unitDir, binPath string) error {
	rawInterval := watchIntervalOrDefault(cfg)
	systemdInterval, err := toSystemdDuration(rawInterval)
	if err != nil {
		return err
	}

	servicePath := filepath.Join(unitDir, "nugs-watch.service")
	if !filepath.IsAbs(binPath) {
		return fmt.Errorf("binary path must be absolute: %s", binPath)
	}
	templateData := struct {
		BinaryPath string
		Path       string
	}{BinaryPath: binPath, Path: os.Getenv("PATH")}
	if err := writeUnitFile(servicePath, serviceTemplate, templateData); err != nil {
		return fmt.Errorf("failed to write service unit: %w", err)
	}

	timerPath := filepath.Join(unitDir, "nugs-watch.timer")
	if err := writeUnitFile(timerPath, timerTemplate, struct{ WatchInterval string }{WatchInterval: systemdInterval}); err != nil {
		return fmt.Errorf("failed to write timer unit: %w", err)
	}
	return nil
}

// toSystemdDuration parses a Go duration string and converts it to systemd time(7) format.
// In Go, "m" means minutes. In systemd, "m" means months — "min" is the correct unit for minutes.
// This conversion eliminates the ambiguity. Returns an error for unparseable input.
func toSystemdDuration(s string) (string, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return "", fmt.Errorf("watchInterval %q is not a valid duration (use Go syntax: 30m, 1h, 6h): %w", s, err)
	}
	if d <= 0 {
		return "", fmt.Errorf("watchInterval %q must be positive", s)
	}

	// Build systemd duration from largest unit down to avoid the 'm'/'min' ambiguity.
	// systemd accepts: us, ms, s, min, h, d, w, M, y — we use h, min, s.
	total := d
	hours := int(total.Hours())
	total -= time.Duration(hours) * time.Hour
	minutes := int(total.Minutes())
	total -= time.Duration(minutes) * time.Minute
	seconds := int(total.Seconds())

	var out string
	if hours > 0 {
		out += fmt.Sprintf("%dh", hours)
	}
	if minutes > 0 {
		out += fmt.Sprintf("%dmin", minutes) // "min" not "m" — avoids systemd's months interpretation
	}
	if seconds > 0 || out == "" {
		out += fmt.Sprintf("%ds", seconds)
	}
	return out, nil
}

// WatchDisable stops and disables the nugs-watch systemd timer and removes unit files.
func WatchDisable() error {
	unitDir, err := systemdUserDir()
	if err != nil {
		return err
	}

	servicePath := filepath.Join(unitDir, "nugs-watch.service")
	timerPath := filepath.Join(unitDir, "nugs-watch.timer")
	backup, err := snapshotUnitFiles(servicePath, timerPath)
	if err != nil {
		return fmt.Errorf("snapshot watch units: %w", err)
	}
	activation, err := snapshotUnitActivation("nugs-watch.timer")
	if err != nil {
		return fmt.Errorf("snapshot watch timer state: %w", err)
	}
	if err := systemctlUser("disable", "--now", "nugs-watch.timer"); err != nil && !isUnitAbsentError(err) {
		return fmt.Errorf("failed to disable timer: %w", err)
	}
	if err := systemctlUser("stop", "nugs-watch.service"); err != nil && !isUnitAbsentError(err) {
		cause := fmt.Errorf("failed to stop watch service: %w", err)
		if rollbackErr := restoreWatchInstallation(backup, activation); rollbackErr != nil {
			return fmt.Errorf("%w; rollback failed: %w", cause, rollbackErr)
		}
		return cause
	}

	removed := 0
	for _, p := range []string{servicePath, timerPath} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			cause := fmt.Errorf("failed to remove %s: %w", p, err)
			if rollbackErr := restoreWatchInstallation(backup, activation); rollbackErr != nil {
				return fmt.Errorf("%w; rollback failed: %w", cause, rollbackErr)
			}
			return cause
		} else if err == nil {
			removed++
		}
	}

	if err := systemctlUser("daemon-reload"); err != nil {
		cause := fmt.Errorf("daemon-reload failed after removing unit files: %w", err)
		if rollbackErr := restoreWatchInstallation(backup, activation); rollbackErr != nil {
			return fmt.Errorf("%w; rollback failed: %w", cause, rollbackErr)
		}
		return cause
	}

	if removed == 0 {
		ui.PrintInfo("No nugs-watch unit files found (already disabled)")
	} else {
		ui.PrintSuccess("Nugs watch timer disabled and unit files removed")
	}
	return nil
}

type unitFileSnapshot struct {
	exists bool
	data   []byte
}

type unitSnapshot map[string]unitFileSnapshot

func snapshotUnitFiles(paths ...string) (unitSnapshot, error) {
	snapshot := make(unitSnapshot, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				snapshot[path] = unitFileSnapshot{}
				continue
			}
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		snapshot[path] = unitFileSnapshot{exists: true, data: data}
	}
	return snapshot, nil
}

func restoreUnitFiles(snapshot unitSnapshot) error {
	var restoreErrs []error
	for path, file := range snapshot {
		if !file.exists {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				restoreErrs = append(restoreErrs, fmt.Errorf("remove %s: %w", path, err))
			}
			continue
		}
		if err := cache.WriteFileAtomic(path, file.data, 0644); err != nil {
			restoreErrs = append(restoreErrs, fmt.Errorf("write %s: %w", path, err))
		}
	}
	return errors.Join(restoreErrs...)
}

type unitActivation struct {
	enabled bool
	active  bool
}

func snapshotUnitActivation(unit string) (unitActivation, error) {
	enabled, err := systemctlUnitState("is-enabled", unit)
	if err != nil {
		return unitActivation{}, err
	}
	active, err := systemctlUnitState("is-active", unit)
	if err != nil {
		return unitActivation{}, err
	}
	return unitActivation{enabled: enabled, active: active}, nil
}

func systemctlUnitState(action, unit string) (bool, error) {
	output, err := runSystemctlUser(action, unit)
	state := strings.ToLower(strings.TrimSpace(output))
	if err == nil {
		return true, nil
	}
	for _, benign := range []string{"disabled", "inactive", "failed", "unknown", "not-found", "static", "masked", "no such file or directory", "does not exist", "not loaded"} {
		if strings.Contains(state, benign) {
			return false, nil
		}
	}
	return false, fmt.Errorf("systemctl --user %s %s: %w: %s", action, unit, err, strings.TrimSpace(output))
}

func restoreWatchInstallation(files unitSnapshot, activation unitActivation) error {
	var rollbackErrs []error
	if err := systemctlUser("disable", "--now", "nugs-watch.timer"); err != nil && !isUnitAbsentError(err) {
		rollbackErrs = append(rollbackErrs, err)
	}
	if err := restoreUnitFiles(files); err != nil {
		rollbackErrs = append(rollbackErrs, err)
	}
	if err := systemctlUser("daemon-reload"); err != nil {
		rollbackErrs = append(rollbackErrs, err)
	}
	if activation.enabled {
		if err := systemctlUser("enable", "nugs-watch.timer"); err != nil {
			rollbackErrs = append(rollbackErrs, err)
		}
	}
	if activation.active {
		if err := systemctlUser("start", "nugs-watch.timer"); err != nil {
			rollbackErrs = append(rollbackErrs, err)
		}
	}
	return errors.Join(rollbackErrs...)
}

func isUnitAbsentError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{"not found", "not-found", "not loaded", "does not exist", "no such file or directory"} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

// systemdUserDir returns ~/.config/systemd/user, creating it if needed.
func systemdUserDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	unitDir := filepath.Join(homeDir, ".config", "systemd", "user")
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create systemd user directory: %w", err)
	}
	return unitDir, nil
}

// writeUnitFile renders a template and writes it atomically to path.
// Uses temp-file + rename to match the codebase's established atomic-write invariant.
func writeUnitFile(path, tmplStr string, data any) error {
	tmpl, err := template.New("unit").Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to render template for %s: %w", path, err)
	}

	if err := cache.WriteFileAtomic(path, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to atomically write unit file %s: %w", path, err)
	}
	return nil
}

// systemctlUser runs systemctl --user with the given arguments.
func systemctlUser(args ...string) error {
	output, err := runSystemctlUser(args...)
	if err != nil {
		return fmt.Errorf("systemctl --user %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(output))
	}
	return nil
}

var runSystemctlUser = func(args ...string) (string, error) {
	cmdArgs := append([]string{"--user"}, args...)
	cmd := exec.Command("systemctl", cmdArgs...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}
