package api

import (
	"sync"
	"testing"
	"time"
)

func TestCircuitBreakerAllowsOnlyOneHalfOpenProbe(t *testing.T) {
	cb := newCircuitBreaker(1, time.Nanosecond)
	cb.RecordFailure()
	time.Sleep(time.Millisecond)

	const callers = 32
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok := cb.Allow()
			if ok {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if allowed != 1 {
		t.Fatalf("half-open allowed %d probes, want 1", allowed)
	}
}

func TestFailureDomainsAreScoped(t *testing.T) {
	if got := failureDomain("auth"); got != "identity" {
		t.Fatalf("auth domain = %q", got)
	}
	if got := failureDomain("subPlayer.aspx"); got != "stream" {
		t.Fatalf("player domain = %q", got)
	}
	if got := failureDomain("catalog.container"); got != "catalog" {
		t.Fatalf("catalog domain = %q", got)
	}
}
