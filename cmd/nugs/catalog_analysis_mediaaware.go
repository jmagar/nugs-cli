package main

// Media-aware catalog analysis wrappers delegating to internal/catalog during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import "github.com/jmagar/nugs-cli/internal/catalog"

func matchesMediaFilter(showMedia, filter MediaType) bool {
	return catalog.MatchesMediaFilter(showMedia, filter)
}
