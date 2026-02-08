//go:build windows

package main

// Process alive windows wrappers delegating to internal/runtime during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import "github.com/jmagar/nugs-cli/internal/runtime"

func isProcessAlive(pid int) bool { return runtime.IsProcessAlive(pid) }
