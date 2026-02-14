// Package list implements list commands for browsing artists, shows, and playlists.
package list

import (
	"context"

	"github.com/jmagar/nugs-cli/internal/model"
)

// Deps holds callbacks to functions that live outside this package.
type Deps struct {
	// GetShowMediaType determines what media types a show offers.
	GetShowMediaType func(show *model.AlbArtResp) model.MediaType

	// MatchesMediaFilter returns true if the show's media type matches the filter.
	MatchesMediaFilter func(showMedia, filter model.MediaType) bool

	// Playlist downloads a catalog playlist by GUID.
	Playlist func(ctx context.Context, plistId, legacyToken string, cfg *model.Config, streamParams *model.StreamParams, cat bool) error
}
