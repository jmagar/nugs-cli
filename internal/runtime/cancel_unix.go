//go:build !windows

package runtime

import (
	"fmt"
	"os"
	"syscall"
)

// CancelProcessByPID sends SIGTERM to the process with the given PID.
func CancelProcessByPID(pid int) error {
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
