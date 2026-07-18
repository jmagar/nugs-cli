package api

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"
)

type failingRoundTripper func(*http.Request) (*http.Response, error)

func (f failingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

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

func TestRetryDoReopensHalfOpenCircuitWhenProbeCannotRun(t *testing.T) {
	oldLimiter := RateLimiter
	RateLimiter = newRateLimiter(1000, 10)
	t.Cleanup(func() { RateLimiter = oldLimiter })

	tests := []struct {
		name    string
		makeReq func(context.Context) func() (*http.Request, error)
	}{
		{
			name: "request construction error",
			makeReq: func(context.Context) func() (*http.Request, error) {
				return func() (*http.Request, error) {
					return nil, errors.New("construct request")
				}
			},
		},
		{
			name: "transport error",
			makeReq: func(ctx context.Context) func() (*http.Request, error) {
				client := &http.Client{Transport: failingRoundTripper(func(*http.Request) (*http.Response, error) {
					return nil, errors.New("transport failed")
				})}
				ctx = WithHTTPClient(ctx, client)
				return func() (*http.Request, error) {
					return http.NewRequestWithContext(ctx, http.MethodGet, "https://example.test", nil)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cb := newCircuitBreaker(1, time.Hour)
			cb.RecordFailure()
			cb.mu.Lock()
			cb.openedAt = time.Now().Add(-2 * time.Hour)
			cb.mu.Unlock()

			circuitMu.Lock()
			oldCircuits := circuits
			circuits = map[string]*circuitBreaker{"catalog": cb}
			circuitMu.Unlock()
			t.Cleanup(func() {
				circuitMu.Lock()
				circuits = oldCircuits
				circuitMu.Unlock()
			})

			ctx := context.Background()
			if _, err := retryDo(ctx, "catalog.probe", tc.makeReq(ctx)); err == nil {
				t.Fatal("retryDo returned nil error")
			}
			if got := cb.State(); got != circuitOpen {
				t.Fatalf("circuit state = %s, want open", got)
			}
			if _, allowed := cb.Allow(); allowed {
				t.Fatal("failed half-open probe did not restart the open timeout")
			}
		})
	}
}
