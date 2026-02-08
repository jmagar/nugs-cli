package main

// List command wrappers delegating to internal/list during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import "github.com/jmagar/nugs-cli/internal/list"

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

func listArtists(jsonLevel string, showFilter string) error {
	return list.ListArtists(jsonLevel, showFilter)
}

func displayWelcome() error {
	return list.DisplayWelcome()
}

func listArtistShows(artistId string, jsonLevel string, mediaFilter ...MediaType) error {
	return list.ListArtistShows(artistId, jsonLevel, buildListDeps(), mediaFilter...)
}

func listArtistShowsByVenue(artistId string, venueFilter string, jsonLevel string) error {
	return list.ListArtistShowsByVenue(artistId, venueFilter, jsonLevel)
}

func listArtistLatestShows(artistId string, limit int, jsonLevel string) error {
	return list.ListArtistLatestShows(artistId, limit, jsonLevel)
}

func resolveCatPlistId(plistUrl string) (string, error) {
	return list.ResolveCatPlistId(plistUrl)
}

func catalogPlist(plistId, legacyToken string, cfg *Config, streamParams *StreamParams) error {
	return list.CatalogPlist(plistId, legacyToken, cfg, streamParams, buildListDeps())
}
