//go:build !windows

package runtime

import (
	"os/signal"
	"syscall"
)

// SetupSessionPersistence ignores signals that would terminate the process
// when a controlling terminal disconnects (SIGHUP) or a pipe breaks (SIGPIPE).
func SetupSessionPersistence() {
	signal.Ignore(syscall.SIGHUP, syscall.SIGPIPE)
}
