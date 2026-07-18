//go:build windows

package config

// Windows does not support opening a directory and calling Sync on it. The
// renamed file itself has already been flushed before this platform hook.
func syncDirectory(_ string) error {
	return nil
}
