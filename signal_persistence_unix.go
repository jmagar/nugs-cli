//go:build !windows

package main

import (
	"os/signal"
	"syscall"
)

func setupSessionPersistence() {
	signal.Ignore(syscall.SIGHUP, syscall.SIGPIPE)
}
