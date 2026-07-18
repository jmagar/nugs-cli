//go:build windows

package cache

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows"
)

// FileLock represents a file-based lock (Windows stub).
type FileLock struct {
	lockFile *os.File
}

// AcquireLock acquires an exclusive lock on a lock file.
// On Windows this uses LockFileEx so process death releases the kernel lock;
// the lock file itself is persistent and is never guessed to be stale.
func AcquireLock(lockPath string, maxRetries int) (*FileLock, error) {
	lockDir := filepath.Dir(lockPath)
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create lock directory: %w", err)
	}

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file: %w", err)
	}
	var lastErr error
	for i := 0; i <= maxRetries; i++ {
		var overlapped windows.Overlapped
		err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &overlapped)
		if err == nil {
			return &FileLock{lockFile: f}, nil
		}
		lastErr = err
		if !errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			_ = f.Close()
			return nil, fmt.Errorf("failed to lock cache file: %w", err)
		}
		if i < maxRetries {
			time.Sleep(100 * time.Millisecond)
		}
	}
	_ = f.Close()
	return nil, fmt.Errorf("failed to acquire lock after %d retries: %w", maxRetries, lastErr)
}

// Release releases the file lock.
func (fl *FileLock) Release() error {
	if fl.lockFile == nil {
		return nil
	}
	var overlapped windows.Overlapped
	unlockErr := windows.UnlockFileEx(windows.Handle(fl.lockFile.Fd()), 0, 1, 0, &overlapped)
	closeErr := fl.lockFile.Close()
	fl.lockFile = nil
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
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
