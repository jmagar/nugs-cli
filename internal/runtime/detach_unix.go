//go:build !windows

package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/jmagar/nugs-cli/internal/cache"
	"github.com/jmagar/nugs-cli/internal/ui"
)

// SpawnDetached launches a new detached process with the given args.
// Returns the PID of the new process and the log file path.
func SpawnDetached(args []string) (int, string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return 0, "", err
	}
	cacheDir, err := cache.GetCacheDir()
	if err != nil {
		return 0, "", err
	}
	logPath := filepath.Join(cacheDir, "runtime.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return 0, "", err
	}
	defer logFile.Close()

	cmd := exec.Command(exePath, args...)
	cmd.Env = append(os.Environ(), DetachedEnvVar+"=1")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, "", err
	}
	return cmd.Process.Pid, logPath, nil
}

// MaybeDetachAndExit checks if the process should detach and, if so, spawns
// a background process and returns true. The caller should exit if true.
func MaybeDetachAndExit(args []string, urls []string) bool {
	if !ShouldAutoDetach(urls) {
		return false
	}
	pid, logPath, err := SpawnDetached(args)
	if err != nil {
		ui.PrintWarning(fmt.Sprintf("Background detach failed (%v); continuing in foreground", err))
		return false
	}
	ui.PrintSuccess(fmt.Sprintf("Detached background session started (pid=%d)", pid))
	ui.PrintInfo("Use `nugs status` to check progress and health")
	ui.PrintInfo(fmt.Sprintf("Log file: %s", logPath))
	return true
}
