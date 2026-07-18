package cache

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var cacheProcessMu sync.Mutex

// WriteFileAtomic durably writes data to a unique temporary file in the target
// directory and then atomically replaces targetPath.
func WriteFileAtomic(targetPath string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(targetPath), ".nugs-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(mode); err != nil {
		return fmt.Errorf("failed to set temp file mode: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}
	ok = true
	if err := syncParentDirectory(filepath.Dir(targetPath)); err != nil {
		return fmt.Errorf("failed to sync target directory: %w", err)
	}
	return nil
}

func atomicWriteFile(targetPath string, data []byte) error {
	return WriteFileAtomic(targetPath, data, 0644)
}
