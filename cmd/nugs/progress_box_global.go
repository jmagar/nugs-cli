package main

import "sync"

// Global progress box reference for thread-safe access from crawl_control.go
var (
	currentProgressBox   *ProgressBoxState
	currentProgressBoxMu sync.RWMutex
)

// setCurrentProgressBox sets the global progress box reference (thread-safe)
func setCurrentProgressBox(pb *ProgressBoxState) {
	currentProgressBoxMu.Lock()
	currentProgressBox = pb
	currentProgressBoxMu.Unlock()
}

// getCurrentProgressBox gets the global progress box reference (thread-safe)
// Returns nil if no progress box is currently active
func getCurrentProgressBox() *ProgressBoxState {
	currentProgressBoxMu.RLock()
	defer currentProgressBoxMu.RUnlock()
	return currentProgressBox
}
