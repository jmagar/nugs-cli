//go:build windows

package cache

// Windows cannot fsync an opened directory. WriteFileAtomic still flushes the
// file before rename and uses this no-op only for the unsupported directory step.
func syncParentDirectory(_ string) error {
	return nil
}
