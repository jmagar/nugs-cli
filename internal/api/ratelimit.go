package api

import (
	"context"
	"sync"
	"time"
)

// rateLimiter is a token-bucket rate limiter.
// It allows up to burstSize requests immediately, then refills at ratePerSec tokens/second.
// All methods are safe for concurrent use.
type rateLimiter struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	ratePerSec float64
	lastRefill time.Time
}

func newRateLimiter(ratePerSec float64, burst int) *rateLimiter {
	return &rateLimiter{
		tokens:     float64(burst),
		maxTokens:  float64(burst),
		ratePerSec: ratePerSec,
		lastRefill: time.Now(),
	}
}

// Wait blocks until a token is available or ctx is cancelled.
// Returns ctx.Err() if the context is done before a token is acquired.
func (rl *rateLimiter) Wait(ctx context.Context) (waited time.Duration, err error) {
	start := time.Now()
	for {
		rl.mu.Lock()
		now := time.Now()
		elapsed := now.Sub(rl.lastRefill).Seconds()
		rl.tokens = min(rl.maxTokens, rl.tokens+elapsed*rl.ratePerSec)
		rl.lastRefill = now

		if rl.tokens >= 1 {
			rl.tokens--
			rl.mu.Unlock()
			return time.Since(start), nil
		}

		// How long until the next token arrives?
		waitDur := time.Duration((1.0-rl.tokens)/rl.ratePerSec*1000) * time.Millisecond
		rl.mu.Unlock()

		select {
		case <-ctx.Done():
			return time.Since(start), ctx.Err()
		case <-time.After(waitDur):
		}
	}
}
