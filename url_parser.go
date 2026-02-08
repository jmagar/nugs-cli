package main

// URL parser wrappers delegating to internal/api during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import "github.com/jmagar/nugs-cli/internal/api"

var regexStrings = api.RegexStrings

func parsePaidLstreamShowID(query string) (string, error) {
	return api.ParsePaidLstreamShowID(query)
}

func isLikelyLivestreamSegments(segURLs []string) (bool, error) {
	return api.IsLikelyLivestreamSegments(segURLs)
}

func parseTimestamps(start, end string) (string, string) {
	return api.ParseTimestamps(start, end)
}

func parseStreamParams(userId string, subInfo *SubInfo, isPromo bool) *StreamParams {
	return api.ParseStreamParams(userId, subInfo, isPromo)
}

func checkUrl(_url string) (string, int) {
	return api.CheckURL(_url)
}
