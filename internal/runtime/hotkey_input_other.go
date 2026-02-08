//go:build !linux && !windows

package runtime

import (
	"fmt"

	"golang.org/x/term"
)

// EnableHotkeyInput puts the terminal into raw mode for hotkey detection.
// Returns a restore function that should be deferred to reset the terminal.
func EnableHotkeyInput(fd int) (func(), error) {
	state, err := term.MakeRaw(fd)
	if err != nil {
		return nil, fmt.Errorf("failed to enable hotkey mode: %w", err)
	}
	return func() {
		_ = term.Restore(fd, state)
	}, nil
}
