//go:build windows

package runtime

import (
	"fmt"
	"os"
)

// CancelProcessByPID kills the process with the given PID on Windows.
func CancelProcessByPID(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid: %d", pid)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}
