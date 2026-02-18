package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"
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

var (
	ErrRemoteArtistFolderListFailed = errors.New("failed to list remote artist folders")
)

// remoteCheckErrCount tracks how many remote check warnings have been emitted
// so large batch runs don't flood the log.
var remoteCheckErrCount atomic.Int64

// remoteCheckErrLimit is the number of individual warnings to show before
// switching to a single suppression notice.
const remoteCheckErrLimit = 3

// WarnRemoteCheckError logs a warning about a remote existence check failure.
// The first remoteCheckErrLimit failures are logged individually; subsequent
// failures emit a single suppression notice to avoid flooding the log during
// large catalog scans with persistent network issues.
func WarnRemoteCheckError(err error) {
	n := remoteCheckErrCount.Add(1)
	switch {
	case n <= remoteCheckErrLimit:
		ui.PrintWarning(fmt.Sprintf("Remote existence check failed: %v", err))
	case n == remoteCheckErrLimit+1:
		ui.PrintWarning("Remote existence checks keep failing; suppressing further warnings to reduce noise")
	}
}

// ListAllRemoteArtistFolders lists all artist folders on the remote.
func ListAllRemoteArtistFolders(ctx context.Context, cfg *model.Config) (map[string]struct{}, error) {
	folders := make(map[string]struct{})
	if !cfg.RcloneEnabled {
		return folders, nil
	}

	remoteDest := cfg.RcloneRemote + ":" + cfg.RclonePath
	cmd := exec.CommandContext(ctx, "rclone", "lsf", remoteDest, "--dirs-only")
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
// It checks the path appropriate for the given mediaType, falling back to audio if unknown.
func ShowExists(ctx context.Context, show *model.AlbArtResp, cfg *model.Config, mediaType model.MediaType, deps *Deps) bool {
	if mediaType == model.MediaTypeUnknown {
		mediaType = model.MediaTypeAudio
	}
	resolver := helpers.NewConfigPathResolver(cfg)
	albumPath := resolver.LocalShowPath(show, mediaType)

	// Check local existence
	_, err := os.Stat(albumPath)
	if err == nil {
		return true
	}

	// Check remote if rclone enabled
	isVideo := mediaType.HasVideo()
	if cfg.RcloneEnabled && deps.RemotePathExists != nil {
		remotePath := resolver.RemoteShowPath(show)
		remoteExists, err := deps.RemotePathExists(ctx, remotePath, cfg, isVideo)
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
func BuildArtistPresenceIndex(ctx context.Context, artistName string, cfg *model.Config, deps *Deps, mediaFilter model.MediaType) ArtistPresenceIndex {
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
		remoteTargets := []bool{false}
		if mediaFilter == model.MediaTypeVideo {
			remoteTargets = []bool{true}
		} else if mediaFilter == model.MediaTypeBoth || mediaFilter == model.MediaTypeUnknown {
			remoteTargets = []bool{false, true}
		}

		for _, isVideo := range remoteTargets {
			remoteFolders, err := deps.ListRemoteArtistFolders(ctx, idx.ArtistFolder, cfg, isVideo)
			if err != nil {
				mediaType := "audio"
				if isVideo {
					mediaType = "video"
				}
				ui.PrintWarning(fmt.Sprintf("Remote %s folder check failed for %s: %v",
					mediaType, idx.ArtistFolder, err))
				idx.RemoteListErr = err
				break
			}
			for name := range remoteFolders {
				idx.RemoteFolders[name] = struct{}{}
			}
		}
	}

	return idx
}

// IsShowDownloaded checks if a show is downloaded using the pre-built index.
func IsShowDownloaded(ctx context.Context, show *model.AlbArtResp, idx ArtistPresenceIndex, cfg *model.Config, deps *Deps) bool {
	albumFolder := helpers.BuildAlbumFolderName(show.ArtistName, show.ContainerInfo)

	if _, ok := idx.LocalFolders[albumFolder]; ok {
		return true
	}
	if _, ok := idx.RemoteFolders[albumFolder]; ok {
		return true
	}

	// Fallback for remote-list failures to preserve correctness.
	if cfg.RcloneEnabled && idx.RemoteListErr != nil && deps.RemotePathExists != nil {
		remotePath := path.Join(idx.ArtistFolder, albumFolder)
		remoteExists, err := deps.RemotePathExists(ctx, remotePath, cfg, false)
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
