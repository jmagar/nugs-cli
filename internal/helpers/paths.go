package helpers

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jmagar/nugs-cli/internal/model"
)

var sanRegex = regexp.MustCompile(`[\/:*?"><|]`)

var (
	// ErrInvalidPathCharacters indicates control characters were found in a path.
	ErrInvalidPathCharacters = errors.New("path contains invalid characters")
	// ErrPathTraversalDetected indicates path traversal tokens were found.
	ErrPathTraversalDetected = errors.New("path contains directory traversal sequence")
)

// PathResolver centralizes media-aware local/remote path resolution.
type PathResolver interface {
	LocalBaseForMedia(mediaType model.MediaType) string
	LocalBasesForFilter(mediaFilter model.MediaType) []string
	LocalShowPath(show *model.AlbArtResp, mediaType model.MediaType) string
	RemoteShowPath(show *model.AlbArtResp) string
}

// ConfigPathResolver resolves paths based on Config.
type ConfigPathResolver struct {
	cfg *model.Config
}

// NewConfigPathResolver returns a media-aware path resolver backed by cfg.
func NewConfigPathResolver(cfg *model.Config) PathResolver {
	return &ConfigPathResolver{cfg: cfg}
}

// Sanitise cleans a filename by replacing invalid characters.
func Sanitise(filename string) string {
	san := sanRegex.ReplaceAllString(filename, "_")
	return strings.TrimSpace(san)
}

// BuildAlbumFolderName constructs a sanitized folder name for an album
// from artist name and container info. This ensures consistent naming
// across all download and gap detection logic.
// maxLen parameter allows customizing the length limit (default 120 for albums, 110 for videos).
func BuildAlbumFolderName(artistName, containerInfo string, maxLen ...int) string {
	limit := model.AlbumFolderMaxRunes
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

// ValidatePath checks that a path does not contain dangerous characters or traversal sequences.
func ValidatePath(path string) error {
	if strings.ContainsAny(path, "\x00\n\r") {
		return fmt.Errorf("%w: %q", ErrInvalidPathCharacters, path)
	}
	if strings.Contains(path, "..") {
		return fmt.Errorf("%w: %q", ErrPathTraversalDetected, path)
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

	err := filepath.WalkDir(localPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			info, infoErr := d.Info()
			if infoErr == nil {
				totalSize += info.Size()
			}
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
	return NewConfigPathResolver(cfg).LocalBaseForMedia(mediaType)
}

// LocalBaseForMedia returns the correct local output path based on media type.
func (r *ConfigPathResolver) LocalBaseForMedia(mediaType model.MediaType) string {
	if r == nil || r.cfg == nil {
		return ""
	}
	if mediaType.HasVideo() && strings.TrimSpace(r.cfg.VideoOutPath) != "" {
		return r.cfg.VideoOutPath
	}
	return r.cfg.OutPath
}

// GetRclonePathForMedia returns the correct remote path based on media type.
func GetRclonePathForMedia(cfg *model.Config, mediaType model.MediaType) string {
	if cfg == nil {
		return ""
	}
	if mediaType.HasVideo() && cfg.RcloneVideoPath != "" {
		return cfg.RcloneVideoPath
	}
	return cfg.RclonePath
}

// LocalBasesForFilter returns the local base paths relevant for a media filter.
func (r *ConfigPathResolver) LocalBasesForFilter(mediaFilter model.MediaType) []string {
	if r == nil || r.cfg == nil {
		return nil
	}

	var paths []string
	switch mediaFilter {
	case model.MediaTypeAudio:
		paths = append(paths, r.cfg.OutPath)
	case model.MediaTypeVideo:
		if strings.TrimSpace(r.cfg.VideoOutPath) != "" {
			paths = append(paths, r.cfg.VideoOutPath)
		} else {
			paths = append(paths, r.cfg.OutPath)
		}
	default:
		paths = append(paths, r.cfg.OutPath)
		if strings.TrimSpace(r.cfg.VideoOutPath) != "" {
			paths = append(paths, r.cfg.VideoOutPath)
		}
	}
	return uniqueNonEmptyPaths(paths)
}

// LocalShowPath returns the artist/show local path for the given media type.
func (r *ConfigPathResolver) LocalShowPath(show *model.AlbArtResp, mediaType model.MediaType) string {
	if r == nil || show == nil {
		return ""
	}
	albumFolder := BuildAlbumFolderName(show.ArtistName, show.ContainerInfo)
	return filepath.Join(r.LocalBaseForMedia(mediaType), Sanitise(show.ArtistName), albumFolder)
}

// RemoteShowPath returns the artist/show remote-relative path.
func (r *ConfigPathResolver) RemoteShowPath(show *model.AlbArtResp) string {
	if show == nil {
		return ""
	}
	albumFolder := BuildAlbumFolderName(show.ArtistName, show.ContainerInfo)
	return path.Join(Sanitise(show.ArtistName), albumFolder)
}

func uniqueNonEmptyPaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	uniq := make([]string, 0, len(paths))
	for _, p := range paths {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		uniq = append(uniq, trimmed)
	}
	return uniq
}
