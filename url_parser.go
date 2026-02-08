package main

import (
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var regexStrings = []string{
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

func parsePaidLstreamShowID(query string) (string, error) {
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

func isLikelyLivestreamSegments(segURLs []string) (bool, error) {
	if len(segURLs) == 0 {
		return false, errors.New("video manifest returned no segments")
	}
	return len(segURLs) > 1 && segURLs[0] != segURLs[1], nil
}

func parseTimestamps(start, end string) (string, string) {
	startTime, _ := time.Parse(layout, start)
	endTime, _ := time.Parse(layout, end)
	parsedStart := strconv.FormatInt(startTime.Unix(), 10)
	parsedEnd := strconv.FormatInt(endTime.Unix(), 10)
	return parsedStart, parsedEnd
}

func parseStreamParams(userId string, subInfo *SubInfo, isPromo bool) *StreamParams {
	startStamp, endStamp := parseTimestamps(subInfo.StartedAt, subInfo.EndsAt)
	streamParams := &StreamParams{
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

func checkUrl(_url string) (string, int) {
	for i, regexStr := range regexStrings {
		regex := regexp.MustCompile(regexStr)
		match := regex.FindStringSubmatch(_url)
		if match != nil {
			return match[1], i
		}
	}
	return "", 0
}
