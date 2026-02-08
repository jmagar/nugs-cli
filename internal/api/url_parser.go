package api

import (
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jmagar/nugs-cli/internal/model"
)

// RegexStrings are patterns for recognizing Nugs.net URLs.
var RegexStrings = []string{
	`^https://play.nugs.net/release/(\d+)$`,
	`^https://play.nugs.net/#/playlists/playlist/(\d+)$`,
	`^https://play.nugs.net/library/playlist/(\d+)$`,
	`(^https://2nu.gs/[a-zA-Z\d]+$)`,
	`^https://play.nugs.net/#/videos/artist/\d+/.+/(\d+)$`,
	`^https://play.nugs.net/artist/(\d+)(?:/albums|/latest|)$`,
	`^https://play.nugs.net/livestream/(\d+)/exclusive$`,
	`^https://play.nugs.net/watch/livestreams/exclusive/(\d+)$`,
	`^https://play.nugs.net/#/my-webcasts/\d+-(\d+)-\d+-\d+$`,
	`^https://www.nugs.net/on/demandware.store/Sites-NugsNet-Site/d` +
		`efault/(?:Stash-QueueVideo|NugsVideo-GetStashVideo)\?([a-zA-Z0-9=%&-]+$)`,
	`^https://play.nugs.net/library/webcast/(\d+)$`,
	`^(\d+)$`,
}

// ParsePaidLstreamShowID extracts the showID parameter from a query string.
func ParsePaidLstreamShowID(query string) (string, error) {
	q, err := url.ParseQuery(query)
	if err != nil {
		return "", err
	}
	showIDs := q["showID"]
	if len(showIDs) == 0 {
		return "", errors.New("url didn't contain a show id parameter")
	}
	showID := strings.TrimSpace(showIDs[0])
	if showID == "" {
		return "", errors.New("url didn't contain a show id parameter")
	}
	return showID, nil
}

// IsLikelyLivestreamSegments checks if segment URLs indicate a livestream.
func IsLikelyLivestreamSegments(segURLs []string) (bool, error) {
	if len(segURLs) == 0 {
		return false, errors.New("video manifest returned no segments")
	}
	return len(segURLs) > 1 && segURLs[0] != segURLs[1], nil
}

// ParseTimestamps converts date strings to Unix timestamps.
func ParseTimestamps(start, end string) (string, string) {
	startTime, _ := time.Parse(Layout, start)
	endTime, _ := time.Parse(Layout, end)
	parsedStart := strconv.FormatInt(startTime.Unix(), 10)
	parsedEnd := strconv.FormatInt(endTime.Unix(), 10)
	return parsedStart, parsedEnd
}

// ParseStreamParams builds stream parameters from user and subscription info.
func ParseStreamParams(userId string, subInfo *model.SubInfo, isPromo bool) *model.StreamParams {
	startStamp, endStamp := ParseTimestamps(subInfo.StartedAt, subInfo.EndsAt)
	streamParams := &model.StreamParams{
		SubscriptionID:          subInfo.LegacySubscriptionID,
		SubCostplanIDAccessList: subInfo.Plan.PlanID,
		UserID:                  userId,
		StartStamp:              startStamp,
		EndStamp:                endStamp,
	}
	if isPromo {
		streamParams.SubCostplanIDAccessList = subInfo.Promo.Plan.PlanID
	} else {
		streamParams.SubCostplanIDAccessList = subInfo.Plan.PlanID
	}
	return streamParams
}

// CheckURL matches a URL against known Nugs.net patterns.
func CheckURL(_url string) (string, int) {
	for i, regexStr := range RegexStrings {
		regex := regexp.MustCompile(regexStr)
		match := regex.FindStringSubmatch(_url)
		if match != nil {
			return match[1], i
		}
	}
	return "", 0
}
