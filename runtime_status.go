package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// RuntimeStatus and RuntimeControl types are defined in internal/model
// and aliased in model_aliases.go. Only runtime functions remain here.

var (
	runtimeStatusMu        sync.Mutex
	runtimeStatusPath      string
	runtimeStatus          RuntimeStatus
	runtimeStatusLastWrite time.Time
	runtimeStatusWarnOnce  sync.Once
)

func getRuntimeStatusPath() (string, error) {
	cacheDir, err := getCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "runtime-status.json"), nil
}

func getRuntimeControlPath() (string, error) {
	cacheDir, err := getCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "runtime-control.json"), nil
}

func initRuntimeStatus() {
	statusPath, err := getRuntimeStatusPath()
	if err != nil {
		return
	}
	runtimeStatusPath = statusPath
	now := time.Now().UTC().Format(time.RFC3339)
	runtimeStatus = RuntimeStatus{
		PID:       os.Getpid(),
		State:     "running",
		StartedAt: now,
		UpdatedAt: now,
	}
	writeRuntimeStatus(true)
	_ = writeRuntimeControl(RuntimeControl{Pause: false, Cancel: false, UpdatedAt: now})
}

func updateRuntimeProgress(label string, percentage int, speed, current, total string) {
	runtimeStatusMu.Lock()
	defer runtimeStatusMu.Unlock()
	if runtimeStatusPath == "" {
		return
	}
	runtimeStatus.Label = label
	runtimeStatus.Percentage = percentage
	runtimeStatus.Speed = speed
	runtimeStatus.Current = current
	runtimeStatus.Total = total
	runtimeStatus.Errors = runErrorCount
	runtimeStatus.Warnings = runWarningCount
	runtimeStatus.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	writeRuntimeStatus(false)
}

func finalizeRuntimeStatus(state string) {
	runtimeStatusMu.Lock()
	defer runtimeStatusMu.Unlock()
	if runtimeStatusPath == "" {
		return
	}
	runtimeStatus.State = state
	runtimeStatus.Errors = runErrorCount
	runtimeStatus.Warnings = runWarningCount
	runtimeStatus.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	writeRuntimeStatus(true)
}

func writeRuntimeStatus(force bool) {
	if runtimeStatusPath == "" {
		return
	}
	now := time.Now()
	if !force && now.Sub(runtimeStatusLastWrite) < 250*time.Millisecond {
		return
	}
	runtimeStatusLastWrite = now
	data, err := json.MarshalIndent(runtimeStatus, "", "  ")
	if err != nil {
		return
	}
	if err := writeFileAtomic(runtimeStatusPath, data, 0644); err != nil {
		runtimeStatusWarnOnce.Do(func() {
			fmt.Fprintf(os.Stderr, "warning: failed to write runtime status: %v\n", err)
		})
	}
}

func printRuntimeStatus() {
	status, err := readRuntimeStatus()
	if err != nil {
		statusPath, pathErr := getRuntimeStatusPath()
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
	printHeader("Nugs Runtime Status")
	stateColor := colorGreen
	if status.State == "stale" {
		stateColor = colorYellow
	}
	printKeyValue("State", status.State, stateColor)
	printKeyValue("PID", fmt.Sprintf("%d", status.PID), colorCyan)
	printKeyValue("Updated", status.UpdatedAt, colorCyan)
	printKeyValue("Progress", fmt.Sprintf("%s %d%%", status.Label, status.Percentage), colorYellow)
	printKeyValue("Rate", status.Speed, colorYellow)
	printKeyValue("Health", fmt.Sprintf("errors=%d warnings=%d", status.Errors, status.Warnings), colorYellow)
}

func readRuntimeStatus() (RuntimeStatus, error) {
	statusPath, err := getRuntimeStatusPath()
	if err != nil {
		return RuntimeStatus{}, err
	}
	data, err := os.ReadFile(statusPath)
	if err != nil {
		return RuntimeStatus{}, err
	}
	var status RuntimeStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return RuntimeStatus{}, err
	}
	if status.State == "running" && status.PID > 0 && !isProcessAlive(status.PID) {
		status.State = "stale"
		status.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		refreshed, marshalErr := json.MarshalIndent(status, "", "  ")
		if marshalErr == nil {
			_ = writeFileAtomic(statusPath, refreshed, 0644)
		}
	}
	return status, nil
}

func readRuntimeControl() (RuntimeControl, error) {
	controlPath, err := getRuntimeControlPath()
	if err != nil {
		return RuntimeControl{}, err
	}
	data, err := os.ReadFile(controlPath)
	if err != nil {
		if os.IsNotExist(err) {
			return RuntimeControl{}, nil
		}
		return RuntimeControl{}, err
	}
	var control RuntimeControl
	if err := json.Unmarshal(data, &control); err != nil {
		return RuntimeControl{}, err
	}
	return control, nil
}

func writeRuntimeControl(control RuntimeControl) error {
	controlPath, err := getRuntimeControlPath()
	if err != nil {
		return err
	}
	control.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(control, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(controlPath, data, 0644)
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
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

func requestRuntimeCancel() error {
	control, err := readRuntimeControl()
	if err != nil {
		return err
	}
	control.Cancel = true
	return writeRuntimeControl(control)
}

func requestRuntimePause(paused bool) error {
	control, err := readRuntimeControl()
	if err != nil {
		return err
	}
	control.Pause = paused
	return writeRuntimeControl(control)
}

func printActiveRuntimeHint(currentPID int, currentCommand []string) {
	if os.Getenv(detachedEnvVar) == "1" {
		return
	}
	if len(currentCommand) == 1 && (currentCommand[0] == "status" || currentCommand[0] == "cancel") {
		return
	}
	status, err := readRuntimeStatus()
	if err != nil {
		return
	}
	if status.State != "running" {
		return
	}
	if status.PID == currentPID || !isProcessAlive(status.PID) {
		return
	}
	printWarning(fmt.Sprintf("Active crawl detected (pid=%d, %s %d%%)", status.PID, status.Label, status.Percentage))
	printInfo("Use `nugs status` for progress, `nugs cancel` to stop it")
	printInfo("If attached in another terminal: Shift-P pause/resume, Shift-C cancel")
}
