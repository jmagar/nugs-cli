package main

import (
	"os"

	"golang.org/x/term"
)

const detachedEnvVar = "NUGS_DETACHED"

func isReadOnlyCommand(urls []string) bool {
	if len(urls) == 0 {
		return true
	}
	switch urls[0] {
	case "help", "--help", "status", "cancel", "completion":
		return true
	case "list":
		return true
	case "catalog":
		if len(urls) < 2 {
			return true
		}
		switch urls[1] {
		case "update", "cache", "stats", "latest", "list", "coverage", "config":
			return true
		case "gaps":
			if len(urls) >= 4 && urls[len(urls)-1] == "fill" {
				return false
			}
			for _, arg := range urls[2:] {
				if arg == "fill" {
					return false
				}
			}
			return true
		default:
			return true
		}
	default:
		return false
	}
}

func shouldAutoDetach(urls []string) bool {
	if os.Getenv(detachedEnvVar) == "1" {
		return false
	}
	// Keep interactive sessions attached so users can see live progress and use hotkeys.
	if term.IsTerminal(int(os.Stdin.Fd())) {
		return false
	}
	return !isReadOnlyCommand(urls)
}
