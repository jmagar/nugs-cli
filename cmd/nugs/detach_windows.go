//go:build windows

package main

// Windows adapters for detached runtime execution.

import "github.com/jmagar/nugs-cli/internal/runtime"

func maybeDetachAndExit(_args []string, urls []string) bool {
	return runtime.MaybeDetachAndExit(_args, urls)
}
