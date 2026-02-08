//go:build !windows

package main

// Detach unix wrappers delegating to internal/runtime during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import "github.com/jmagar/nugs-cli/internal/runtime"

func spawnDetached(args []string) (int, string, error) { return runtime.SpawnDetached(args) }

func maybeDetachAndExit(args []string, urls []string) bool {
	return runtime.MaybeDetachAndExit(args, urls)
}
