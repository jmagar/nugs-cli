//go:build !windows

package main

// Unix adapters for detached runtime execution.

import "github.com/jmagar/nugs-cli/internal/runtime"

func maybeDetachAndExit(args []string, urls []string) bool {
	return runtime.MaybeDetachAndExit(args, urls)
}
