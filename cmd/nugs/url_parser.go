package main

// URL parser wrappers delegating to internal/api during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import "github.com/jmagar/nugs-cli/internal/api"

// URL pattern indices returned by checkURL as the second return value.
// Each constant corresponds to a regex pattern position in regexStrings.
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

var regexStrings = api.GetRegexStrings()

func parsePaidLstreamShowID(query string) (string, error) {
	return api.ParsePaidLstreamShowID(query)
}

func isLikelyLivestreamSegments(segURLs []string) (bool, error) {
	return api.IsLikelyLivestreamSegments(segURLs)
}

func parseTimestamps(start, end string) (string, string, error) {
	return api.ParseTimestamps(start, end)
}

func parseStreamParams(userId string, subInfo *SubInfo, isPromo bool) (*StreamParams, error) {
	return api.ParseStreamParams(userId, subInfo, isPromo)
}

func checkURL(url string) (string, int) {
	return api.CheckURL(url)
}
