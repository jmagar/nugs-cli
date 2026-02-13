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

// compiledRegexes are pre-compiled versions of RegexStrings.
var compiledRegexes []*regexp.Regexp

func init() {
	compiledRegexes = make([]*regexp.Regexp, len(RegexStrings))
	for i, s := range RegexStrings {
		compiledRegexes[i] = regexp.MustCompile(s)
	}
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
// Returns an error if either timestamp cannot be parsed.
func ParseTimestamps(start, end string) (string, string, error) {
	startTime, err := time.Parse(Layout, start)
	if err != nil {
		return "", "", errors.New("failed to parse start timestamp: " + err.Error())
	}
	endTime, err := time.Parse(Layout, end)
	if err != nil {
		return "", "", errors.New("failed to parse end timestamp: " + err.Error())
	}
	parsedStart := strconv.FormatInt(startTime.Unix(), 10)
	parsedEnd := strconv.FormatInt(endTime.Unix(), 10)
	return parsedStart, parsedEnd, nil
}

// ParseStreamParams builds stream parameters from user and subscription info.
// Returns nil and an error if timestamp parsing fails.
func ParseStreamParams(userId string, subInfo *model.SubInfo, isPromo bool) (*model.StreamParams, error) {
	startStamp, endStamp, err := ParseTimestamps(subInfo.StartedAt, subInfo.EndsAt)
	if err != nil {
		return nil, err
	}
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
	return streamParams, nil
}

// CheckURL matches a URL against known Nugs.net patterns.
// Returns the extracted ID and the pattern index.
// Returns ("", -1) if no pattern matches the URL.
func CheckURL(_url string) (string, int) {
	for i, re := range compiledRegexes {
		match := re.FindStringSubmatch(_url)
		if match != nil {
			return match[1], i
		}
	}
	return "", -1
}
