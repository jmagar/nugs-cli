//go:build windows

package main

import (
	"os"
	"syscall"
)

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Windows, Kill can be used as a probe only if it succeeds/fails with known errors.
	// We avoid killing and instead rely on process lookup semantics.
	// Best effort fallback:
	return proc.Signal(syscall.Signal(0)) == nil
}
