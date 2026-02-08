//go:build windows

package main

// Hotkey input windows wrappers delegating to internal/runtime during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import "github.com/jmagar/nugs-cli/internal/runtime"

func enableHotkeyInput(fd int) (func(), error) { return runtime.EnableHotkeyInput(fd) }
