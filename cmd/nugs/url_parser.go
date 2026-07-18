package main

// Command adapters for URL parsing.

import "github.com/jmagar/nugs-cli/internal/api"

// URL pattern indices returned by checkURL as the second return value.
const (
	urlTypeAlbum           = 0  // /release/<id>
	urlTypePlaylist        = 1  // /#/playlists/playlist/<id>
	urlTypeLibraryPlaylist = 2  // /library/playlist/<id>
	urlTypeShortenedURL    = 3  // 2nu.gs short link
	urlTypeVideo           = 4  // /#/videos/artist/.../<id>
	urlTypeArtist          = 5  // /artist/<id>
	urlTypeLivestreamExcl  = 6  // /livestream/<id>/exclusive
	urlTypeLivestreamWatch = 7  // /watch/livestreams/exclusive/<id>
	urlTypeLivestreamArch  = 8  // /#/my-webcasts/...
	urlTypePaidLivestream  = 9  // demandware paid livestream
	urlTypeLibraryWebcast  = 10 // /library/webcast/<id>
	urlTypeNumericID       = 11 // bare numeric ID
)

func parsePaidLstreamShowID(query string) (string, error) {
	return api.ParsePaidLstreamShowID(query)
}

func isLikelyLivestreamSegments(segURLs []string) (bool, error) {
	return api.IsLikelyLivestreamSegments(segURLs)
}

func parseStreamParams(userID string, subInfo *SubInfo, isPromo bool) (*StreamParams, error) {
	return api.ParseStreamParams(userID, subInfo, isPromo)
}

func checkURL(url string) (string, int) {
	return api.CheckURL(url)
}
