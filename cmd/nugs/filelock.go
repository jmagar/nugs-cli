package main

// Command aliases for cache locking.

import "github.com/jmagar/nugs-cli/internal/cache"

// FileLock is a type alias for the cache package's FileLock.
type FileLock = cache.FileLock

// AcquireLock delegates to cache.AcquireLock.
func AcquireLock(lockPath string, maxRetries int) (*FileLock, error) {
	return cache.AcquireLock(lockPath, maxRetries)
}

// WithCacheLock delegates to cache.WithCacheLock.
func WithCacheLock(fn func() error) error {
	return cache.WithCacheLock(fn)
}
