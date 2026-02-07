//go:build !windows

package main

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func enableHotkeyInput(fd int) (func(), error) {
	orig, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return nil, fmt.Errorf("failed to get terminal attributes: %w", err)
	}
	newState := *orig
	newState.Lflag &^= unix.ICANON | unix.ECHO
	newState.Cc[unix.VMIN] = 1
	newState.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &newState); err != nil {
		return nil, fmt.Errorf("failed to set terminal attributes: %w", err)
	}
	return func() {
		_ = unix.IoctlSetTermios(fd, unix.TCSETS, orig)
	}, nil
}
