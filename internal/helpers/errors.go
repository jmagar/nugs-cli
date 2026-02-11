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

// HandleErr prints an error to stderr. The _panic parameter is deprecated and
// retained only for API compatibility — it now calls os.Exit(1) instead of panic.
func HandleErr(errText string, err error, _panic bool) {
	errString := errText + "\n" + err.Error()
	if _panic {
		fmt.Fprintln(os.Stderr, errString)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, errString)
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
			return "", errors.New("failed to get script filename")
		}
	} else {
		fname, err = os.Executable()
		if err != nil {
			return "", err
		}
	}
	return filepath.Dir(fname), nil
}

// ReadTxtFile reads non-empty lines from a text file.
func ReadTxtFile(path string) ([]string, error) {
	var lines []string
	f, err := os.OpenFile(path, os.O_RDONLY, 0755)
	if err != nil {
		return nil, err
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
		return nil, scanner.Err()
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
				if !Contains(processed, txtLine) {
					txtLine = strings.TrimSuffix(txtLine, "/")
					processed = append(processed, txtLine)
				}
			}
			txtPaths = append(txtPaths, _url)
		} else {
			if !Contains(processed, _url) {
				_url = strings.TrimSuffix(_url, "/")
				processed = append(processed, _url)
			}
		}
	}
	return processed, nil
}
