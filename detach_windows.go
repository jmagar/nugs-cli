//go:build windows

package main

func maybeDetachAndExit(_args []string, urls []string) bool {
	if !shouldAutoDetach(urls) {
		return false
	}
	printWarning("Auto-detach is not enabled on this platform; running in foreground")
	return false
}
