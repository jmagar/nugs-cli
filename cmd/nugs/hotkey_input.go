package main

// Command adapter for runtime hotkey input.

import "github.com/jmagar/nugs-cli/internal/runtime"

func enableHotkeyInput(fd int) (func(), error) { return runtime.EnableHotkeyInput(fd) }
