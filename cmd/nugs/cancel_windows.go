//go:build windows

package main

// Windows cancellation adapters for internal runtime control.

import "github.com/jmagar/nugs-cli/internal/runtime"

func cancelProcessByPID(pid int) error { return runtime.CancelProcessByPID(pid) }
