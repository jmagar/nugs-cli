// Package catalog implements catalog browsing, gap analysis, coverage,
// auto-refresh, and media-aware filtering.
package catalog

import (
	"context"
	"time"

	"github.com/jmagar/nugs-cli/internal/model"
)

// Deps holds callbacks to functions that live outside this package.
// These cannot be imported directly because they either live in the root
// (main) package or would cause circular imports.
type Deps struct {
	// RemotePathExists checks whether a path exists on the rclone remote.
	RemotePathExists func(remotePath string, cfg *model.Config, isVideo bool) (bool, error)

	// ListRemoteArtistFolders returns show folder names under an artist
	// folder on remote storage.
	ListRemoteArtistFolders func(artistFolder string, cfg *model.Config, isVideo bool) (map[string]struct{}, error)

	// Album downloads a single album/show by container ID.
	// Used by gap-fill to download missing shows.
	Album func(ctx context.Context, albumID string, cfg *model.Config, streamParams *model.StreamParams, artResp *model.AlbArtResp, batchState *model.BatchProgressState, progressBox *model.ProgressBoxState) error

	// Playlist downloads a catalog playlist by GUID.
	Playlist func(ctx context.Context, plistId, legacyToken string, cfg *model.Config, streamParams *model.StreamParams, cat bool) error

	// SetCurrentProgressBox registers (or clears) the global progress box.
	SetCurrentProgressBox func(box *model.ProgressBoxState)

	// GetShowMediaType determines what media types a show offers.
	GetShowMediaType func(show *model.AlbArtResp) model.MediaType

	// FormatDuration formats a time.Duration as a human-readable string.
	FormatDuration func(d time.Duration) string

	// GetArtistMetaCached retrieves artist metadata, using the cache when fresh.
	GetArtistMetaCached func(ctx context.Context, artistID string, ttl time.Duration) (pages []*model.ArtistMeta, cacheUsed bool, cacheStaleUse bool, err error)
}
