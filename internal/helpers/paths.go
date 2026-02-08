package helpers

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jmagar/nugs-cli/internal/model"
)

const sanRegexStr = `[\/:*?"><|]`

// Sanitise cleans a filename by replacing invalid characters.
func Sanitise(filename string) string {
	san := regexp.MustCompile(sanRegexStr).ReplaceAllString(filename, "_")
	return strings.TrimSuffix(san, "	")
}

// BuildAlbumFolderName constructs a sanitized folder name for an album
// from artist name and container info. This ensures consistent naming
// across all download and gap detection logic.
// maxLen parameter allows customizing the length limit (default 120 for albums, 110 for videos).
func BuildAlbumFolderName(artistName, containerInfo string, maxLen ...int) string {
	limit := 120
	if len(maxLen) > 0 && maxLen[0] > 0 {
		limit = maxLen[0]
	}
	albumFolder := artistName + " - " + strings.TrimRight(containerInfo, " ")
	runes := []rune(albumFolder)
	if len(runes) > limit {
		albumFolder = string(runes[:limit])
	}
	return Sanitise(albumFolder)
}

// MakeDirs creates directories recursively.
func MakeDirs(path string) error {
	return os.MkdirAll(path, 0755)
}

// FileExists checks if a file (not directory) exists at the given path.
func FileExists(path string) (bool, error) {
	f, err := os.Stat(path)
	if err == nil {
		return !f.IsDir(), nil
	} else if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// ValidatePath checks that a path does not contain dangerous characters.
func ValidatePath(path string) error {
	if strings.ContainsAny(path, "\x00\n\r") {
		return fmt.Errorf("path contains invalid characters")
	}
	return nil
}

// GetVideoOutPath returns the video output path, falling back to OutPath.
func GetVideoOutPath(cfg *model.Config) string {
	if cfg == nil {
		return ""
	}
	if strings.TrimSpace(cfg.VideoOutPath) != "" {
		return cfg.VideoOutPath
	}
	return cfg.OutPath
}

// GetRcloneBasePath returns the rclone base path for audio or video.
func GetRcloneBasePath(cfg *model.Config, isVideo bool) string {
	if cfg == nil {
		return ""
	}
	if isVideo && strings.TrimSpace(cfg.RcloneVideoPath) != "" {
		return cfg.RcloneVideoPath
	}
	return cfg.RclonePath
}

// CalculateLocalSize walks the directory tree and calculates total size in bytes.
func CalculateLocalSize(localPath string) int64 {
	var totalSize int64

	err := filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
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

// GetOutPathForMedia returns the correct local output path based on media type.
func GetOutPathForMedia(cfg *model.Config, mediaType model.MediaType) string {
	if mediaType.HasVideo() && cfg.VideoOutPath != "" {
		return cfg.VideoOutPath
	}
	return cfg.OutPath
}

// GetRclonePathForMedia returns the correct remote path based on media type.
func GetRclonePathForMedia(cfg *model.Config, mediaType model.MediaType) string {
	if mediaType.HasVideo() && cfg.RcloneVideoPath != "" {
		return cfg.RcloneVideoPath
	}
	return cfg.RclonePath
}
