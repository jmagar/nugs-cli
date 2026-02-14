package helpers

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	// ErrScriptFilenameUnavailable indicates the runtime caller path could not be resolved.
	ErrScriptFilenameUnavailable = errors.New("failed to get script filename")
	// ErrOpenTextFile indicates opening a text file failed.
	ErrOpenTextFile = errors.New("failed to open text file")
	// ErrScanTextFile indicates scanner iteration over a text file failed.
	ErrScanTextFile = errors.New("failed to scan text file")
)

// HandleErr prints an error to stderr and optionally exits. When fatal is true,
// the process exits with code 1 after printing. When false, execution continues
// and callers are responsible for checking err themselves before calling.
//
// Deprecated: Prefer returning errors to callers instead of printing directly.
func HandleErr(errText string, err error, fatal bool) {
	if err == nil {
		return
	}
	errString := errText + "\n" + err.Error()
	fmt.Fprintln(os.Stderr, errString)
	if fatal {
		os.Exit(1)
	}
}

// WasRunFromSrc checks if the binary was run from a Go build temp directory.
func WasRunFromSrc() bool {
	buildPath := filepath.Join(os.TempDir(), "go-build")
	return strings.HasPrefix(os.Args[0], buildPath)
}

// GetScriptDir returns the directory of the running script or binary.
func GetScriptDir() (string, error) {
	var (
		ok    bool
		err   error
		fname string
	)
	runFromSrc := WasRunFromSrc()
	if runFromSrc {
		_, fname, _, ok = runtime.Caller(0)
		if !ok {
			return "", ErrScriptFilenameUnavailable
		}
	} else {
		fname, err = os.Executable()
		if err != nil {
			return "", fmt.Errorf("resolve executable path: %w", err)
		}
	}
	return filepath.Dir(fname), nil
}

// ReadTxtFile reads non-empty lines from a text file.
func ReadTxtFile(path string) ([]string, error) {
	var lines []string
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%w %q: %w", ErrOpenTextFile, path, err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	if scanner.Err() != nil {
		return nil, fmt.Errorf("%w %q: %w", ErrScanTextFile, path, scanner.Err())
	}
	return lines, nil
}

// Contains checks if a string slice contains a value (case-insensitive).
func Contains(lines []string, value string) bool {
	for _, line := range lines {
		if strings.EqualFold(line, value) {
			return true
		}
	}
	return false
}

// ProcessUrls expands .txt file URLs and deduplicates.
func ProcessUrls(urls []string) ([]string, error) {
	var (
		processed []string
		txtPaths  []string
	)
	for _, _url := range urls {
		if strings.HasSuffix(_url, ".txt") && !Contains(txtPaths, _url) {
			txtLines, err := ReadTxtFile(_url)
			if err != nil {
				return nil, err
			}
			for _, txtLine := range txtLines {
				txtLine = strings.TrimSuffix(txtLine, "/")
				if !Contains(processed, txtLine) {
					processed = append(processed, txtLine)
				}
			}
			txtPaths = append(txtPaths, _url)
		} else {
			_url = strings.TrimSuffix(_url, "/")
			if !Contains(processed, _url) {
				processed = append(processed, _url)
			}
		}
	}
	return processed, nil
}
