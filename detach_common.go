package main

// Detach wrappers delegating to internal/runtime during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import "github.com/jmagar/nugs-cli/internal/runtime"

const detachedEnvVar = runtime.DetachedEnvVar

func isReadOnlyCommand(urls []string) bool { return runtime.IsReadOnlyCommand(urls) }
func shouldAutoDetach(urls []string) bool  { return runtime.ShouldAutoDetach(urls) }
