package main

// Process alive wrapper delegating to internal/runtime during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import nugsrt "github.com/jmagar/nugs-cli/internal/runtime"

func isProcessAlive(pid int) bool { return nugsrt.IsProcessAlive(pid) }
