//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func spawnDetached(args []string) (int, string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return 0, "", err
	}
	cacheDir, err := getCacheDir()
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
	cmd.Env = append(os.Environ(), detachedEnvVar+"=1")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, "", err
	}
	return cmd.Process.Pid, logPath, nil
}

func maybeDetachAndExit(args []string, urls []string) bool {
	if !shouldAutoDetach(urls) {
		return false
	}
	pid, logPath, err := spawnDetached(args)
	if err != nil {
		printWarning(fmt.Sprintf("Background detach failed (%v); continuing in foreground", err))
		return false
	}
	printSuccess(fmt.Sprintf("Detached background session started (pid=%d)", pid))
	printInfo("Use `nugs status` to check progress and health")
	printInfo(fmt.Sprintf("Log file: %s", logPath))
	return true
}
