package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/jmagar/nugs-cli/internal/helpers"
	"github.com/jmagar/nugs-cli/internal/model"
	"github.com/jmagar/nugs-cli/internal/ui"
)

// ArtistPresenceIndex holds pre-built local/remote folder lookups for an artist.
type ArtistPresenceIndex struct {
	ArtistFolder  string
	LocalFolders  map[string]struct{}
	RemoteFolders map[string]struct{}
	RemoteListErr error
}

var remoteCheckWarnOnce sync.Once

var (
	ErrRemoteArtistFolderListFailed = errors.New("failed to list remote artist folders")
)

// WarnRemoteCheckError prints a one-time warning about remote check failures.
func WarnRemoteCheckError(err error) {
	remoteCheckWarnOnce.Do(func() {
		ui.PrintWarning(fmt.Sprintf("Remote existence checks failed; treating as not found. First error: %v", err))
	})
}

// ListAllRemoteArtistFolders lists all artist folders on the remote.
func ListAllRemoteArtistFolders(cfg *model.Config) (map[string]struct{}, error) {
	folders := make(map[string]struct{})
	if !cfg.RcloneEnabled {
		return folders, nil
	}

	remoteDest := cfg.RcloneRemote + ":" + cfg.RclonePath
	cmd := exec.Command("rclone", "lsf", remoteDest, "--dirs-only")
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 3 {
			return folders, nil
		}
		return nil, fmt.Errorf("%w: %w", ErrRemoteArtistFolderListFailed, err)
	}

	for _, line := range strings.Split(string(output), "\n") {
		trimmed := strings.TrimSuffix(strings.TrimSpace(line), "/")
		if trimmed == "" {
			continue
		}
		folders[trimmed] = struct{}{}
	}
	return folders, nil
}

// NormalizeArtistFolderKey normalises an artist folder name for matching.
func NormalizeArtistFolderKey(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ShowExists checks if a show has been downloaded locally or exists on remote storage.
func ShowExists(show *model.AlbArtResp, cfg *model.Config, deps *Deps) bool {
	resolver := helpers.NewConfigPathResolver(cfg)
	albumPath := resolver.LocalShowPath(show, model.MediaTypeAudio)

	// Check local existence
	_, err := os.Stat(albumPath)
	if err == nil {
		return true
	}

	// Check remote if rclone enabled
	if cfg.RcloneEnabled && deps.RemotePathExists != nil {
		remotePath := resolver.RemoteShowPath(show)
		remoteExists, err := deps.RemotePathExists(remotePath, cfg, false)
		if err != nil {
			WarnRemoteCheckError(err)
			return false
		}
		if remoteExists {
			return true
		}
	}

	return false
}

// CollectArtistShows gathers all shows and artist name from artist metadata responses.
func CollectArtistShows(artistMetas []*model.ArtistMeta) (allShows []*model.AlbArtResp, artistName string) {
	for _, meta := range artistMetas {
		allShows = append(allShows, meta.Response.Containers...)
		if artistName == "" && len(meta.Response.Containers) > 0 {
			artistName = meta.Response.Containers[0].ArtistName
		}
	}
	return allShows, artistName
}

// BuildArtistPresenceIndex builds a pre-computed index of local and remote
// show folders for an artist. The mediaFilter determines which local paths
// are scanned:
//   - Audio: checks OutPath only
//   - Video: checks VideoOutPath (or OutPath if VideoOutPath not set)
//   - Both/Unknown: checks both OutPath and VideoOutPath (if different)
func BuildArtistPresenceIndex(artistName string, cfg *model.Config, deps *Deps, mediaFilter model.MediaType) ArtistPresenceIndex {
	idx := ArtistPresenceIndex{
		ArtistFolder:  helpers.Sanitise(artistName),
		LocalFolders:  make(map[string]struct{}),
		RemoteFolders: make(map[string]struct{}),
	}

	resolver := helpers.NewConfigPathResolver(cfg)
	pathsToCheck := resolver.LocalBasesForFilter(mediaFilter)
	if len(pathsToCheck) == 0 {
		pathsToCheck = append(pathsToCheck, cfg.OutPath)
	}

	// Build combined index from all relevant paths
	for _, basePath := range pathsToCheck {
		localArtistPath := filepath.Join(basePath, idx.ArtistFolder)
		if entries, err := os.ReadDir(localArtistPath); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				idx.LocalFolders[entry.Name()] = struct{}{}
			}
		}
	}

	if cfg.RcloneEnabled && deps.ListRemoteArtistFolders != nil {
		remoteFolders, err := deps.ListRemoteArtistFolders(idx.ArtistFolder, cfg)
		if err != nil {
			idx.RemoteListErr = err
		} else {
			idx.RemoteFolders = remoteFolders
		}
	}

	return idx
}

// IsShowDownloaded checks if a show is downloaded using the pre-built index.
func IsShowDownloaded(show *model.AlbArtResp, idx ArtistPresenceIndex, cfg *model.Config, deps *Deps) bool {
	albumFolder := helpers.BuildAlbumFolderName(show.ArtistName, show.ContainerInfo)

	if _, ok := idx.LocalFolders[albumFolder]; ok {
		return true
	}
	if _, ok := idx.RemoteFolders[albumFolder]; ok {
		return true
	}

	// Fallback for remote-list failures to preserve correctness.
	if cfg.RcloneEnabled && idx.RemoteListErr != nil && deps.RemotePathExists != nil {
		remotePath := filepath.Join(idx.ArtistFolder, albumFolder)
		remoteExists, err := deps.RemotePathExists(remotePath, cfg, false)
		if err != nil {
			WarnRemoteCheckError(err)
			return false
		}
		return remoteExists
	}

	return false
}

// PrintJSON marshals data to JSON and prints it.
func PrintJSON(data any) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Println(string(jsonData))
	return nil
}
