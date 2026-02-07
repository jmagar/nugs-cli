//go:build !windows

package main

import (
	"fmt"
	"os"
	"syscall"
)

func cancelProcessByPID(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid: %d", pid)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	return nil
}
