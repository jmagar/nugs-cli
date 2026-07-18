package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// APILogEntry is a single structured record written to the API log file.
// Each field uses snake_case JSON keys for easy grep/jq consumption.
type APILogEntry struct {
	Timestamp     string `json:"ts"`
	Event         string `json:"event"`                     // "request", "retry", "rate_limit_wait", "circuit_open", "circuit_closed", "circuit_rejected"
	Label         string `json:"label,omitempty"`           // human-readable API endpoint name
	StatusCode    int    `json:"status_code,omitempty"`     // HTTP status (0 = network error)
	DurationMS    int64  `json:"duration_ms,omitempty"`     // round-trip time
	Attempt       int    `json:"attempt,omitempty"`         // retry attempt (0 = first try)
	RateLimitedMS int64  `json:"rate_limited_ms,omitempty"` // ms spent waiting for rate limiter
	CircuitState  string `json:"circuit_state,omitempty"`   // closed / open / half-open
	Error         string `json:"error,omitempty"`
}

// apiLogger writes structured JSON-line entries to a dedicated log file.
// All methods are safe for concurrent use.
type apiLogger struct {
	mu   sync.Mutex
	enc  *json.Encoder
	f    *os.File
	path string
}

// Logger is the package-level API logger. It is nil until InitAPILogger is called.
// All log functions are no-ops when Logger is nil.
var Logger *apiLogger
var loggerMu sync.RWMutex

var apiLogMaxBytes int64 = 5 << 20
var apiLogRename = os.Rename

const apiLogBackups = 3

// InitAPILogger opens (or creates) the dedicated API log file at logPath.
// It must be called once at startup before any API requests are made.
// The directory is created with mode 0700 if it does not exist.
// Returns a non-nil error only when the file cannot be opened; in that case
// logging is silently disabled (all other operations continue normally).
func InitAPILogger(logPath string) error {
	if err := os.MkdirAll(filepath.Dir(logPath), 0700); err != nil {
		return fmt.Errorf("api logger: mkdir %s: %w", filepath.Dir(logPath), err)
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("api logger: open %s: %w", logPath, err)
	}
	if err := f.Chmod(0600); err != nil {
		_ = f.Close()
		return fmt.Errorf("api logger: chmod %s: %w", logPath, err)
	}
	loggerMu.Lock()
	old := Logger
	Logger = &apiLogger{f: f, enc: json.NewEncoder(f), path: logPath}
	loggerMu.Unlock()
	if old != nil {
		return old.close()
	}
	return nil
}

// CloseAPILogger flushes and closes the logger. A later InitAPILogger may retry.
func CloseAPILogger() error {
	loggerMu.Lock()
	old := Logger
	Logger = nil
	loggerMu.Unlock()
	if old == nil {
		return nil
	}
	return old.close()
}

func currentLogger() *apiLogger {
	loggerMu.RLock()
	defer loggerMu.RUnlock()
	return Logger
}

func (l *apiLogger) close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return nil
	}
	err := l.f.Close()
	l.f = nil
	return err
}

func (l *apiLogger) rotateIfNeeded() error {
	info, err := l.f.Stat()
	if err != nil || info.Size() < apiLogMaxBytes {
		return err
	}
	if err := l.f.Close(); err != nil {
		return err
	}
	l.f = nil
	l.enc = nil
	for i := apiLogBackups - 1; i >= 1; i-- {
		_ = apiLogRename(fmt.Sprintf("%s.%d", l.path, i), fmt.Sprintf("%s.%d", l.path, i+1))
	}
	if err := apiLogRename(l.path, l.path+".1"); err != nil && !os.IsNotExist(err) {
		return errors.Join(err, l.openActive())
	}
	if err := l.openActive(); err != nil {
		rollbackErr := apiLogRename(l.path+".1", l.path)
		return errors.Join(err, rollbackErr, l.openActive())
	}
	return nil
}

func (l *apiLogger) openActive() error {
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	if err := f.Chmod(0600); err != nil {
		_ = f.Close()
		return err
	}
	l.f = f
	l.enc = json.NewEncoder(f)
	return nil
}

// write is the internal append function. Failures are silently ignored —
// a logging error must never abort a download.
func (l *apiLogger) write(e APILogEntry) {
	e.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return
	}
	// A failed rotation is non-fatal when rotateIfNeeded recovered the active
	// file; append the entry so transient filesystem errors do not disable logs.
	_ = l.rotateIfNeeded()
	if l.f == nil || l.enc == nil {
		return
	}
	_ = l.enc.Encode(e)
}

// LogRequest records a completed HTTP request (success or API-level error).
func LogRequest(label string, statusCode int, duration time.Duration, attempt int, circState string, reqErr error) {
	logger := currentLogger()
	if logger == nil {
		return
	}
	e := APILogEntry{
		Event:        "request",
		Label:        label,
		StatusCode:   statusCode,
		DurationMS:   duration.Milliseconds(),
		Attempt:      attempt,
		CircuitState: circState,
	}
	if reqErr != nil {
		e.Error = reqErr.Error()
	}
	if attempt > 0 {
		e.Event = "retry"
	}
	logger.write(e)
}

// LogRateLimitWait records that a request was delayed by the rate limiter.
func LogRateLimitWait(label string, waited time.Duration) {
	logger := currentLogger()
	if logger == nil {
		return
	}
	logger.write(APILogEntry{
		Event:         "rate_limit_wait",
		Label:         label,
		RateLimitedMS: waited.Milliseconds(),
	})
}

// LogCircuitStateChange records a circuit breaker state transition.
func LogCircuitStateChange(event, label, fromState, toState string) {
	logger := currentLogger()
	if logger == nil {
		return
	}
	logger.write(APILogEntry{
		Event:        event,
		Label:        label,
		CircuitState: toState,
		Error:        fmt.Sprintf("state transition: %s → %s", fromState, toState),
	})
}

// LogCircuitRejected records a request that was rejected immediately because
// the circuit breaker is open.
func LogCircuitRejected(label string) {
	logger := currentLogger()
	if logger == nil {
		return
	}
	logger.write(APILogEntry{
		Event:        "circuit_rejected",
		Label:        label,
		CircuitState: "open",
		Error:        ErrCircuitOpen.Error(),
	})
}
