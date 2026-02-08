//go:build !windows

package main

// Cancel unix wrappers delegating to internal/runtime during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import "github.com/jmagar/nugs-cli/internal/runtime"

func cancelProcessByPID(pid int) error { return runtime.CancelProcessByPID(pid) }
