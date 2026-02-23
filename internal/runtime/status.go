package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jmagar/nugs-cli/internal/cache"
	"github.com/jmagar/nugs-cli/internal/model"
	"github.com/jmagar/nugs-cli/internal/ui"
)

// DetachedEnvVar is the environment variable set when running in detached mode.
const DetachedEnvVar = "NUGS_DETACHED"

// Package-level state for runtime status tracking.
var (
	RuntimeStatusMu        sync.Mutex
	RuntimeStatusPath      string
	Status                 model.RuntimeStatus
	RuntimeStatusLastWrite time.Time
	// RuntimeStatusWarnOnce suppresses duplicate warnings about runtime status
	// write failures. Status writes occur up to 4 times/second during downloads
	// (throttled to every 250ms by WriteRuntimeStatus). If the cache directory
	// becomes read-only or fills up, repeated warnings would flood stderr and
	// obscure more critical errors. The first failure is sufficient to alert
	// the user; subsequent failures are silently ignored.
	RuntimeStatusWarnOnce sync.Once
)

// GetRuntimeStatusPath returns the path to the runtime status JSON file.
func GetRuntimeStatusPath() (string, error) {
	cacheDir, err := cache.GetCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "runtime-status.json"), nil
}

// GetRuntimeControlPath returns the path to the runtime control JSON file.
func GetRuntimeControlPath() (string, error) {
	cacheDir, err := cache.GetCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "runtime-control.json"), nil
}

// InitRuntimeStatus creates initial runtime status and control files.
func InitRuntimeStatus() {
	statusPath, err := GetRuntimeStatusPath()
	if err != nil {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	RuntimeStatusMu.Lock()
	RuntimeStatusPath = statusPath
	Status = model.RuntimeStatus{
		PID:       os.Getpid(),
		State:     "running",
		StartedAt: now,
		UpdatedAt: now,
	}
	RuntimeStatusMu.Unlock()
	WriteRuntimeStatus(true)
	_ = WriteRuntimeControl(model.RuntimeControl{Pause: false, Cancel: false, UpdatedAt: now})
}

// UpdateRuntimeProgress updates the runtime status with current progress info.
// errorCount and warningCount are passed in from the caller since they are
// tracked in the root package (output.go).
func UpdateRuntimeProgress(label string, percentage int, speed, current, total string, errorCount, warningCount int) {
	RuntimeStatusMu.Lock()
	if RuntimeStatusPath == "" {
		RuntimeStatusMu.Unlock()
		return
	}
	Status.Label = label
	Status.Percentage = percentage
	Status.Speed = speed
	Status.Current = current
	Status.Total = total
	Status.Errors = errorCount
	Status.Warnings = warningCount
	Status.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	RuntimeStatusMu.Unlock()
	WriteRuntimeStatus(false)
}

// FinalizeRuntimeStatus sets the final state and writes it.
// errorCount and warningCount are passed in from the caller.
func FinalizeRuntimeStatus(state string, errorCount, warningCount int) {
	RuntimeStatusMu.Lock()
	if RuntimeStatusPath == "" {
		RuntimeStatusMu.Unlock()
		return
	}
	Status.State = state
	Status.Errors = errorCount
	Status.Warnings = warningCount
	Status.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	RuntimeStatusMu.Unlock()
	WriteRuntimeStatus(true)
}

// WriteRuntimeStatus writes the current runtime status to disk.
// If force is false, writes are throttled to at most once per 250ms.
func WriteRuntimeStatus(force bool) {
	var (
		statusPath string
		statusSnap model.RuntimeStatus
	)

	RuntimeStatusMu.Lock()
	if RuntimeStatusPath == "" {
		RuntimeStatusMu.Unlock()
		return
	}
	now := time.Now()
	if !force && now.Sub(RuntimeStatusLastWrite) < 250*time.Millisecond {
		RuntimeStatusMu.Unlock()
		return
	}
	RuntimeStatusLastWrite = now
	statusPath = RuntimeStatusPath
	statusSnap = Status
	RuntimeStatusMu.Unlock()

	data, err := json.MarshalIndent(statusSnap, "", "  ")
	if err != nil {
		return
	}
	if err := WriteFileAtomic(statusPath, data, 0644); err != nil {
		RuntimeStatusWarnOnce.Do(func() {
			fmt.Fprintf(os.Stderr, "warning: failed to write runtime status: %v\n", err)
		})
	}
}

// PrintRuntimeStatus reads and displays the current runtime status.
func PrintRuntimeStatus() {
	status, err := ReadRuntimeStatus()
	if err != nil {
		statusPath, pathErr := GetRuntimeStatusPath()
		if pathErr != nil {
			fmt.Println("No runtime status available")
			return
		}
		if os.IsNotExist(err) {
			fmt.Printf("No runtime status found at %s\n", statusPath)
			return
		}
		fmt.Printf("Runtime status unavailable (%v)\n", err)
		return
	}
	ui.PrintHeader("Nugs Runtime Status")
	stateColor := ui.ColorGreen
	if status.State == "stale" {
		stateColor = ui.ColorYellow
	}
	ui.PrintKeyValue("State", status.State, stateColor)
	ui.PrintKeyValue("PID", fmt.Sprintf("%d", status.PID), ui.ColorCyan)
	ui.PrintKeyValue("Updated", status.UpdatedAt, ui.ColorCyan)
	ui.PrintKeyValue("Progress", fmt.Sprintf("%s %d%%", status.Label, status.Percentage), ui.ColorYellow)
	ui.PrintKeyValue("Rate", status.Speed, ui.ColorYellow)
	ui.PrintKeyValue("Health", fmt.Sprintf("errors=%d warnings=%d", status.Errors, status.Warnings), ui.ColorYellow)
}

// ReadRuntimeStatus reads the runtime status from disk and detects stale processes.
func ReadRuntimeStatus() (model.RuntimeStatus, error) {
	statusPath, err := GetRuntimeStatusPath()
	if err != nil {
		return model.RuntimeStatus{}, err
	}
	data, err := os.ReadFile(statusPath)
	if err != nil {
		return model.RuntimeStatus{}, err
	}
	var status model.RuntimeStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return model.RuntimeStatus{}, err
	}
	if status.State == "running" && status.PID > 0 && !IsProcessAlive(status.PID) {
		status.State = "stale"
		status.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		refreshed, marshalErr := json.MarshalIndent(status, "", "  ")
		if marshalErr == nil {
			_ = WriteFileAtomic(statusPath, refreshed, 0644)
		}
	}
	return status, nil
}

// ReadRuntimeControl reads the runtime control file from disk.
func ReadRuntimeControl() (model.RuntimeControl, error) {
	controlPath, err := GetRuntimeControlPath()
	if err != nil {
		return model.RuntimeControl{}, err
	}
	data, err := os.ReadFile(controlPath)
	if err != nil {
		if os.IsNotExist(err) {
			return model.RuntimeControl{}, nil
		}
		return model.RuntimeControl{}, err
	}
	var control model.RuntimeControl
	if err := json.Unmarshal(data, &control); err != nil {
		return model.RuntimeControl{}, err
	}
	return control, nil
}

// WriteRuntimeControl writes the runtime control file to disk.
func WriteRuntimeControl(control model.RuntimeControl) error {
	controlPath, err := GetRuntimeControlPath()
	if err != nil {
		return err
	}
	control.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(control, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(controlPath, data, 0644)
}

// WriteFileAtomic writes data to a file atomically using a temp file and rename.
func WriteFileAtomic(path string, data []byte, mode os.FileMode) error {
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, mode); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

// RequestRuntimeCancel sets the cancel flag in the runtime control file.
func RequestRuntimeCancel() error {
	control, err := ReadRuntimeControl()
	if err != nil {
		return err
	}
	control.Cancel = true
	return WriteRuntimeControl(control)
}

// RequestRuntimePause sets the pause flag in the runtime control file.
func RequestRuntimePause(paused bool) error {
	control, err := ReadRuntimeControl()
	if err != nil {
		return err
	}
	control.Pause = paused
	return WriteRuntimeControl(control)
}

// PrintActiveRuntimeHint warns the user if another crawl is already running.
func PrintActiveRuntimeHint(currentPID int, currentCommand []string) {
	if os.Getenv(DetachedEnvVar) == "1" {
		return
	}
	if len(currentCommand) == 1 && (currentCommand[0] == "status" || currentCommand[0] == "cancel") {
		return
	}
	status, err := ReadRuntimeStatus()
	if err != nil {
		return
	}
	if status.State != "running" {
		return
	}
	if status.PID == currentPID || !IsProcessAlive(status.PID) {
		return
	}
	ui.PrintWarning(fmt.Sprintf("Active crawl detected (pid=%d, %s %d%%)", status.PID, status.Label, status.Percentage))
	ui.PrintInfo("Use `nugs status` for progress, `nugs cancel` to stop it")
	ui.PrintInfo("If attached in another terminal: Shift-P pause/resume, Shift-C cancel")
}
