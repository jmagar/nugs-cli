//go:build linux

package catalog

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	backup := snapshotUnitFiles(servicePath, timerPath)
	rollback := func() {
		_ = systemctlUser("disable", "--now", "nugs-watch.timer")
		restoreUnitFiles(backup)
		_ = systemctlUser("daemon-reload")
	}
	if err := writeWatchUnitFiles(cfg, unitDir, binPath); err != nil {
		restoreUnitFiles(backup)
		return err
	}

	if err := systemctlUser("daemon-reload"); err != nil {
		rollback()
		return fmt.Errorf("daemon-reload failed: %w", err)
	}
	if err := systemctlUser("enable", "--now", "nugs-watch.timer"); err != nil {
		rollback()
		return fmt.Errorf("failed to enable timer: %w", err)
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
	// A failed disable leaves the unit files intact so the operation can be
	// retried without losing the installed configuration.
	if err := systemctlUser("disable", "--now", "nugs-watch.timer"); err != nil {
		return fmt.Errorf("failed to disable timer: %w", err)
	}
	_ = systemctlUser("stop", "nugs-watch.service")

	unitDir, err := systemdUserDir()
	if err != nil {
		return err
	}

	servicePath := filepath.Join(unitDir, "nugs-watch.service")
	timerPath := filepath.Join(unitDir, "nugs-watch.timer")
	backup := snapshotUnitFiles(servicePath, timerPath)

	removed := 0
	for _, p := range []string{servicePath, timerPath} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			restoreUnitFiles(backup)
			return fmt.Errorf("failed to remove %s: %w", p, err)
		} else if err == nil {
			removed++
		}
	}

	if err := systemctlUser("daemon-reload"); err != nil {
		restoreUnitFiles(backup)
		_ = systemctlUser("daemon-reload")
		return fmt.Errorf("daemon-reload failed after removing unit files: %w", err)
	}

	if removed == 0 {
		ui.PrintInfo("No nugs-watch unit files found (already disabled)")
	} else {
		ui.PrintSuccess("Nugs watch timer disabled and unit files removed")
	}
	return nil
}

type unitSnapshot map[string][]byte

func snapshotUnitFiles(paths ...string) unitSnapshot {
	snapshot := make(unitSnapshot, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil {
			snapshot[path] = data
		} else {
			snapshot[path] = nil
		}
	}
	return snapshot
}

func restoreUnitFiles(snapshot unitSnapshot) {
	for path, data := range snapshot {
		if data == nil {
			_ = os.Remove(path)
			continue
		}
		_ = cache.WriteFileAtomic(path, data, 0644)
	}
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
	cmdArgs := append([]string{"--user"}, args...)
	cmd := exec.Command("systemctl", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
