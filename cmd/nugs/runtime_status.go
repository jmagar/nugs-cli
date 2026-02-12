package main

// Runtime status wrappers delegating to internal/runtime during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import (
	"os"

	"github.com/jmagar/nugs-cli/internal/runtime"
)

func getRuntimeStatusPath() (string, error)            { return runtime.GetRuntimeStatusPath() }
func getRuntimeControlPath() (string, error)           { return runtime.GetRuntimeControlPath() }
func initRuntimeStatus()                               { runtime.InitRuntimeStatus() }
func writeRuntimeStatus(force bool)                    { runtime.WriteRuntimeStatus(force) }
func printRuntimeStatus()                              { runtime.PrintRuntimeStatus() }
func readRuntimeStatus() (RuntimeStatus, error)        { return runtime.ReadRuntimeStatus() }
func readRuntimeControl() (RuntimeControl, error)      { return runtime.ReadRuntimeControl() }
func writeRuntimeControl(control RuntimeControl) error { return runtime.WriteRuntimeControl(control) }
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	return runtime.WriteFileAtomic(path, data, mode)
}
func requestRuntimeCancel() error           { return runtime.RequestRuntimeCancel() }
func requestRuntimePause(paused bool) error { return runtime.RequestRuntimePause(paused) }
func printActiveRuntimeHint(currentPID int, currentCommand []string) {
	runtime.PrintActiveRuntimeHint(currentPID, currentCommand)
}

func updateRuntimeProgress(label string, percentage int, speed, current, total string) {
	runtime.UpdateRuntimeProgress(label, percentage, speed, current, total, int(runErrorCount.Load()), int(runWarningCount.Load()))
}

func finalizeRuntimeStatus(state string) {
	runtime.FinalizeRuntimeStatus(state, int(runErrorCount.Load()), int(runWarningCount.Load()))
}
