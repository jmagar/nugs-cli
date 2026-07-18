package main

// Command adapters for detached runtime execution.

import "github.com/jmagar/nugs-cli/internal/runtime"

const detachedEnvVar = runtime.DetachedEnvVar

func isReadOnlyCommand(urls []string) bool { return runtime.IsReadOnlyCommand(urls) }
