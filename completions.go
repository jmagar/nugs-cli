package main

// Shell completion wrappers delegating to internal/completion during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import "github.com/jmagar/nugs-cli/internal/completion"

func completionCommand(args []string) error { return completion.Command(args) }
