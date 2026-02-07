package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// FileLock represents a file-based lock
type FileLock struct {
	lockFile *os.File
	path     string
}

// AcquireLock acquires an exclusive lock on a lock file
// Returns a FileLock that must be released with Release()
// Retries for up to maxRetries times with 100ms delay between attempts
func AcquireLock(lockPath string, maxRetries int) (*FileLock, error) {
	// Ensure lock directory exists
	lockDir := filepath.Dir(lockPath)
	err := os.MkdirAll(lockDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create lock directory: %w", err)
	}

	var lockFile *os.File
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		// Open or create the lock file
		lockFile, err = os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open lock file: %w", err)
		}

		// Try to acquire exclusive lock (non-blocking)
		err = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			// Lock acquired successfully
			return &FileLock{
				lockFile: lockFile,
				path:     lockPath,
			}, nil
		}

		// Lock failed, close file and retry
		lockFile.Close()
		lastErr = err

		if i < maxRetries {
			// Wait before retrying
			time.Sleep(100 * time.Millisecond)
		}
	}

	return nil, fmt.Errorf("failed to acquire lock after %d retries: %w", maxRetries, lastErr)
}

// Release releases the file lock
func (fl *FileLock) Release() error {
	if fl.lockFile == nil {
		return nil
	}

	// Release the lock
	err := syscall.Flock(int(fl.lockFile.Fd()), syscall.LOCK_UN)
	if err != nil {
		fl.lockFile.Close()
		return fmt.Errorf("failed to release lock: %w", err)
	}

	// Close the file
	err = fl.lockFile.Close()
	if err != nil {
		return fmt.Errorf("failed to close lock file: %w", err)
	}

	fl.lockFile = nil
	return nil
}

// WithCacheLock executes a function with the catalog cache lock acquired
// This ensures only one process can write to the cache at a time
func WithCacheLock(fn func() error) error {
	cacheDir, err := getCacheDir()
	if err != nil {
		return err
	}

	lockPath := filepath.Join(cacheDir, ".catalog.lock")

	// Acquire lock with up to 50 retries (5 seconds total)
	lock, err := AcquireLock(lockPath, 50)
	if err != nil {
		return fmt.Errorf("failed to acquire cache lock: %w", err)
	}
	defer func() {
		if releaseErr := lock.Release(); releaseErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to release lock: %v\n", releaseErr)
		}
	}()

	// Execute the protected function
	return fn()
}
