package main

// List command wrappers delegating to internal/list during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import (
	"context"

	"github.com/jmagar/nugs-cli/internal/list"
)

// buildListDeps wires root-level callbacks into the internal/list package.
func buildListDeps() *list.Deps {
	return &list.Deps{
		GetShowMediaType:   getShowMediaType,
		MatchesMediaFilter: matchesMediaFilter,
		Playlist:           playlist,
	}
}

func parseShowFilter(filter string) (string, int, error) {
	return list.ParseShowFilter(filter)
}

func applyShowFilter(artists []Artist, operator string, value int) []Artist {
	return list.ApplyShowFilter(artists, operator, value)
}

func listArtists(ctx context.Context, jsonLevel string, showFilter string) error {
	return list.ListArtists(ctx, jsonLevel, showFilter)
}

func displayWelcome(ctx context.Context) error {
	return list.DisplayWelcome(ctx)
}

func listArtistShows(ctx context.Context, artistId string, jsonLevel string, mediaFilter ...MediaType) error {
	return list.ListArtistShows(ctx, artistId, jsonLevel, buildListDeps(), mediaFilter...)
}

func listArtistShowsByVenue(ctx context.Context, artistId string, venueFilter string, jsonLevel string) error {
	return list.ListArtistShowsByVenue(ctx, artistId, venueFilter, jsonLevel)
}

func listArtistLatestShows(ctx context.Context, artistId string, limit int, jsonLevel string) error {
	return list.ListArtistLatestShows(ctx, artistId, limit, jsonLevel)
}

func resolveCatPlistId(ctx context.Context, plistUrl string) (string, error) {
	return list.ResolveCatPlistID(ctx, plistUrl)
}

func catalogPlist(ctx context.Context, plistId, legacyToken string, cfg *Config, streamParams *StreamParams) error {
	return list.CatalogPlist(ctx, plistId, legacyToken, cfg, streamParams, buildListDeps())
}
