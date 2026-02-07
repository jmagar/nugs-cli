//go:build windows

package main

import (
	"fmt"

	"golang.org/x/term"
)

func enableHotkeyInput(fd int) (func(), error) {
	state, err := term.MakeRaw(fd)
	if err != nil {
		return nil, fmt.Errorf("failed to enable hotkey mode: %w", err)
	}
	return func() {
		_ = term.Restore(fd, state)
	}, nil
}
