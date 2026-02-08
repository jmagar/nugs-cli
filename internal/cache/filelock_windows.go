//go:build windows

package cache

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// FileLock represents a file-based lock (Windows stub).
type FileLock struct {
	lockFile *os.File
	path     string
}

// AcquireLock acquires an exclusive lock on a lock file.
// On Windows this uses a simple file-existence check as a best-effort lock.
func AcquireLock(lockPath string, maxRetries int) (*FileLock, error) {
	lockDir := filepath.Dir(lockPath)
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create lock directory: %w", err)
	}

	var lastErr error
	for i := 0; i <= maxRetries; i++ {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0644)
		if err == nil {
			return &FileLock{lockFile: f, path: lockPath}, nil
		}
		lastErr = err
		if i < maxRetries {
			time.Sleep(100 * time.Millisecond)
		}
	}
	return nil, fmt.Errorf("failed to acquire lock after %d retries: %w", maxRetries, lastErr)
}

// Release releases the file lock.
func (fl *FileLock) Release() error {
	if fl.lockFile == nil {
		return nil
	}
	fl.lockFile.Close()
	fl.lockFile = nil
	_ = os.Remove(fl.path)
	return nil
}

// WithCacheLock executes a function with the catalog cache lock acquired.
func WithCacheLock(fn func() error) error {
	cacheDir, err := GetCacheDir()
	if err != nil {
		return err
	}

	lockPath := filepath.Join(cacheDir, ".catalog.lock")

	lock, err := AcquireLock(lockPath, 50)
	if err != nil {
		return fmt.Errorf("failed to acquire cache lock: %w", err)
	}
	defer func() {
		if releaseErr := lock.Release(); releaseErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to release lock: %v\n", releaseErr)
		}
	}()

	return fn()
}
