//go:build windows

package runtime

import "github.com/jmagar/nugs-cli/internal/ui"

// MaybeDetachAndExit is a no-op on Windows; auto-detach is not supported.
func MaybeDetachAndExit(_args []string, urls []string) bool {
	if !ShouldAutoDetach(urls) {
		return false
	}
	ui.PrintWarning("Auto-detach is not enabled on this platform; running in foreground")
	return false
}
