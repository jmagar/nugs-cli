package main

// Command adapters for runtime status publication.

import (
	"github.com/jmagar/nugs-cli/internal/runtime"
	"github.com/jmagar/nugs-cli/internal/ui"
)

func initRuntimeStatus()                          { runtime.InitRuntimeStatus() }
func printRuntimeStatus()                         { runtime.PrintRuntimeStatus() }
func readRuntimeStatus() (RuntimeStatus, error)   { return runtime.ReadRuntimeStatus() }
func readRuntimeControl() (RuntimeControl, error) { return runtime.ReadRuntimeControl() }
func requestRuntimeCancel() error                 { return runtime.RequestRuntimeCancel() }
func requestRuntimePause(paused bool) error       { return runtime.RequestRuntimePause(paused) }
func printActiveRuntimeHint(currentPID int, currentCommand []string) {
	runtime.PrintActiveRuntimeHint(currentPID, currentCommand)
}

func updateRuntimeProgress(label string, percentage int, speed, current, total string) {
	runtime.UpdateRuntimeProgress(label, percentage, speed, current, total, int(ui.RunErrorCount.Load()), int(ui.RunWarningCount.Load()))
}

func finalizeRuntimeStatus(state string) {
	runtime.FinalizeRuntimeStatus(state, int(ui.RunErrorCount.Load()), int(ui.RunWarningCount.Load()))
}
