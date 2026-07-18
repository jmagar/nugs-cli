package main

// Command adapters for media-aware catalog analysis.

import "github.com/jmagar/nugs-cli/internal/catalog"

func matchesMediaFilter(showMedia, filter MediaType) bool {
	return catalog.MatchesMediaFilter(showMedia, filter)
}
