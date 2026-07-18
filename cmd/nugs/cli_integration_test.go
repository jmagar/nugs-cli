package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

const cliHelperEnv = "NUGS_CLI_INTEGRATION_HELPER"

func TestCLIProcessHelper(t *testing.T) {
	if os.Getenv(cliHelperEnv) != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"nugs"}, os.Args[i+1:]...)
			os.Exit(realMain())
		}
	}
	os.Exit(2)
}

func runCLIProcess(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	cmdArgs := append([]string{"-test.run=^TestCLIProcessHelper$", "--"}, args...)
	cmd := exec.Command(os.Args[0], cmdArgs...)
	cmd.Env = append(os.Environ(), cliHelperEnv+"=1", "HOME="+t.TempDir())
	cmd.Dir = t.TempDir()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.String(), stderr.String(), 0
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("run CLI helper: %v", err)
	}
	return stdout.String(), stderr.String(), exitErr.ExitCode()
}

func TestCLIProcessContracts(t *testing.T) {
	t.Run("version bypasses configuration and authentication", func(t *testing.T) {
		t.Parallel()
		stdout, stderr, code := runCLIProcess(t, "version")
		if code != 0 || stderr != "" || !strings.Contains(stdout, "\nnugs ") {
			t.Fatalf("version = code %d stdout %q stderr %q", code, stdout, stderr)
		}
	})

	t.Run("unknown input fails without success output", func(t *testing.T) {
		t.Parallel()
		stdout, stderr, code := runCLIProcess(t, "not-a-nugs-url")
		if code == 0 {
			t.Fatalf("unknown input exit code = 0, stdout %q stderr %q", stdout, stderr)
		}
		if strings.Contains(strings.ToLower(stdout), "complete") {
			t.Fatalf("failure emitted completion output: %q", stdout)
		}
	})
}
