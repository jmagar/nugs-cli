package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const sanRegexStr = `[\/:*?"><|]`

func handleErr(errText string, err error, _panic bool) {
	errString := errText + "\n" + err.Error()
	if _panic {
		panic(errString)
	}
	fmt.Println(errString)
}

func wasRunFromSrc() bool {
	buildPath := filepath.Join(os.TempDir(), "go-build")
	return strings.HasPrefix(os.Args[0], buildPath)
}

func getScriptDir() (string, error) {
	var (
		ok    bool
		err   error
		fname string
	)
	runFromSrc := wasRunFromSrc()
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

func readTxtFile(path string) ([]string, error) {
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

func contains(lines []string, value string) bool {
	for _, line := range lines {
		if strings.EqualFold(line, value) {
			return true
		}
	}
	return false
}

func processUrls(urls []string) ([]string, error) {
	var (
		processed []string
		txtPaths  []string
	)
	for _, _url := range urls {
		if strings.HasSuffix(_url, ".txt") && !contains(txtPaths, _url) {
			txtLines, err := readTxtFile(_url)
			if err != nil {
				return nil, err
			}
			for _, txtLine := range txtLines {
				if !contains(processed, txtLine) {
					txtLine = strings.TrimSuffix(txtLine, "/")
					processed = append(processed, txtLine)
				}
			}
			txtPaths = append(txtPaths, _url)
		} else {
			if !contains(processed, _url) {
				_url = strings.TrimSuffix(_url, "/")
				processed = append(processed, _url)
			}
		}
	}
	return processed, nil
}

func makeDirs(path string) error {
	err := os.MkdirAll(path, 0755)
	return err
}

func fileExists(path string) (bool, error) {
	f, err := os.Stat(path)
	if err == nil {
		return !f.IsDir(), nil
	} else if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func sanitise(filename string) string {
	san := regexp.MustCompile(sanRegexStr).ReplaceAllString(filename, "_")
	return strings.TrimSuffix(san, "	")
}

// buildAlbumFolderName constructs a sanitized folder name for an album
// from artist name and container info. This ensures consistent naming
// across all download and gap detection logic.
// maxLen parameter allows customizing the length limit (default 120 for albums, 110 for videos).
func buildAlbumFolderName(artistName, containerInfo string, maxLen ...int) string {
	limit := 120
	if len(maxLen) > 0 && maxLen[0] > 0 {
		limit = maxLen[0]
	}
	albumFolder := artistName + " - " + strings.TrimRight(containerInfo, " ")
	runes := []rune(albumFolder)
	if len(runes) > limit {
		albumFolder = string(runes[:limit])
	}
	return sanitise(albumFolder)
}

func validatePath(path string) error {
	// Only block null bytes and newlines which can cause real issues
	// exec.Command handles shell metacharacters safely
	if strings.ContainsAny(path, "\x00\n\r") {
		return fmt.Errorf("path contains invalid characters")
	}
	return nil
}

// calculateLocalSize walks the directory tree and calculates total size in bytes (Tier 1 enhancement)
// Returns the total size of all files in the directory, or 0 if an error occurs
func calculateLocalSize(localPath string) int64 {
	var totalSize int64

	err := filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil {
		return 0
	}

	return totalSize
}
