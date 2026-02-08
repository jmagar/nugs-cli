package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const (
	devKey        = "x7f54tgbdyc64y656thy47er4"
	clientId      = "Eg7HuH873H65r5rt325UytR5429"
	layout        = "01/02/2006 15:04:05"
	userAgent     = "NugsNet/3.26.724 (Android; 7.1.2; Asus; ASUS_Z01QD; Scale/2.0; en)"
	userAgentTwo  = "nugsnetAndroid"
	authUrl       = "https://id.nugs.net/connect/token"
	streamApiBase = "https://streamapi.nugs.net/"
	subInfoUrl    = "https://subscriptions.nugs.net/api/v1/me/subscriptions"
	userInfoUrl   = "https://id.nugs.net/connect/userinfo"
	playerUrl     = "https://play.nugs.net/"
)

var (
	jar, _ = cookiejar.New(nil)
	client = &http.Client{Jar: jar}
)

var qualityMap = map[string]Quality{
	".alac16/": {Specs: "16-bit / 44.1 kHz ALAC", Extension: ".m4a", Format: 1},
	".flac16/": {Specs: "16-bit / 44.1 kHz FLAC", Extension: ".flac", Format: 2},
	// .mqa24/ must be above .flac?
	".mqa24/":  {Specs: "24-bit / 48 kHz MQA", Extension: ".flac", Format: 3},
	".flac?":   {Specs: "FLAC", Extension: ".flac", Format: 2},
	".s360/":   {Specs: "360 Reality Audio", Extension: ".mp4", Format: 4},
	".aac150/": {Specs: "150 Kbps AAC", Extension: ".m4a", Format: 5},
	".m4a?":    {Specs: "AAC", Extension: ".m4a", Format: 5},
	".m3u8?":   {Extension: ".m4a", Format: 6},
}

func auth(email, pwd string) (string, error) {
	data := url.Values{}
	data.Set("client_id", clientId)
	data.Set("grant_type", "password")
	data.Set("scope", "openid profile email nugsnet:api nugsnet:legacyapi offline_access")
	data.Set("username", email)
	data.Set("password", pwd)
	req, err := http.NewRequest(http.MethodPost, authUrl, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Add("User-Agent", userAgent)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	do, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return "", errors.New(do.Status)
	}
	var obj Auth
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return "", err
	}
	return obj.AccessToken, nil
}

func getUserInfo(token string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, userInfoUrl, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("User-Agent", userAgent)
	do, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return "", errors.New(do.Status)
	}
	var obj UserInfo
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return "", err
	}
	return obj.Sub, nil
}

func getSubInfo(token string) (*SubInfo, error) {
	req, err := http.NewRequest(http.MethodGet, subInfoUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("User-Agent", userAgent)
	do, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return nil, errors.New(do.Status)
	}
	var obj SubInfo
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return nil, err
	}
	return &obj, nil
}

func getPlan(subInfo *SubInfo) (string, bool) {
	if !reflect.ValueOf(subInfo.Plan).IsZero() {
		return subInfo.Plan.Description, false
	} else {
		return subInfo.Promo.Plan.Description, true
	}
}

func extractLegToken(tokenStr string) (string, string, error) {
	payload := strings.SplitN(tokenStr, ".", 3)[1]
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return "", "", err
	}
	var obj Payload
	err = json.Unmarshal(decoded, &obj)
	if err != nil {
		return "", "", err
	}
	return obj.LegacyToken, obj.LegacyUguid, nil
}

func getAlbumMeta(albumId string) (*AlbumMeta, error) {
	req, err := http.NewRequest(http.MethodGet, streamApiBase+"api.aspx", nil)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("method", "catalog.container")
	query.Set("containerID", albumId)
	query.Set("vdisp", "1")
	req.URL.RawQuery = query.Encode()
	req.Header.Add("User-Agent", userAgent)
	do, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return nil, errors.New(do.Status)
	}
	var obj AlbumMeta
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return nil, err
	}
	return &obj, nil
}

func getPlistMeta(plistId, email, legacyToken string, cat bool) (*PlistMeta, error) {
	var path string
	if cat {
		path = "api.aspx"
	} else {
		path = "secureApi.aspx"
	}
	req, err := http.NewRequest(http.MethodGet, streamApiBase+path, nil)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	if cat {
		query.Set("method", "catalog.playlist")
		query.Set("plGUID", plistId)
	} else {
		query.Set("method", "user.playlist")
		query.Set("playlistID", plistId)
		query.Set("developerKey", devKey)
		query.Set("user", email)
		query.Set("token", legacyToken)
	}
	req.URL.RawQuery = query.Encode()
	req.Header.Add("User-Agent", userAgentTwo)
	do, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return nil, errors.New(do.Status)
	}
	var obj PlistMeta
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return nil, err
	}
	return &obj, nil
}

func getLatestCatalog() (*LatestCatalogResp, error) {
	req, err := http.NewRequest(http.MethodGet, streamApiBase+"api.aspx", nil)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("method", "catalog.latest")
	query.Set("vdisp", "1")
	req.URL.RawQuery = query.Encode()
	req.Header.Add("User-Agent", userAgentTwo)
	do, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return nil, errors.New(do.Status)
	}
	var obj LatestCatalogResp
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return nil, err
	}
	return &obj, nil
}

func getArtistMeta(artistId string) ([]*ArtistMeta, error) {
	var allArtistMeta []*ArtistMeta
	offset := 1
	query := url.Values{}
	query.Set("method", "catalog.containersAll")
	query.Set("limit", "100")
	query.Set("artistList", artistId)
	query.Set("availType", "1")
	query.Set("vdisp", "1")
	for {
		req, err := http.NewRequest(http.MethodGet, streamApiBase+"api.aspx", nil)
		if err != nil {
			return nil, err
		}
		query.Set("startOffset", strconv.Itoa(offset))
		req.URL.RawQuery = query.Encode()
		req.Header.Add("User-Agent", userAgent)
		do, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		if do.StatusCode != http.StatusOK {
			do.Body.Close()
			return nil, errors.New(do.Status)
		}
		var obj ArtistMeta
		err = json.NewDecoder(do.Body).Decode(&obj)
		do.Body.Close()
		if err != nil {
			return nil, err
		}
		retLen := len(obj.Response.Containers)
		if retLen == 0 {
			break
		}
		allArtistMeta = append(allArtistMeta, &obj)
		offset += retLen
	}
	return allArtistMeta, nil
}

// getArtistMetaCached returns artist metadata from a local cache when fresh.
// If cache is stale or missing, it refreshes from API and rewrites cache.
// If refresh fails and stale cache exists, stale cache is returned.
func getArtistMetaCached(artistID string, ttl time.Duration) (pages []*ArtistMeta, cacheUsed bool, cacheStaleUse bool, err error) {
	cachedPages, cachedAt, readErr := readArtistMetaCache(artistID)
	if readErr == nil && len(cachedPages) > 0 {
		if time.Since(cachedAt) <= ttl {
			return cachedPages, true, false, nil
		}
	}

	freshPages, fetchErr := getArtistMeta(artistID)
	if fetchErr == nil {
		_ = writeArtistMetaCache(artistID, freshPages)
		return freshPages, false, false, nil
	}

	if readErr == nil && len(cachedPages) > 0 {
		return cachedPages, true, true, nil
	}

	return nil, false, false, fetchErr
}

func getArtistList() (*ArtistListResp, error) {
	req, err := http.NewRequest(http.MethodGet, streamApiBase+"api.aspx", nil)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("method", "catalog.artists")
	query.Set("vdisp", "1")
	req.URL.RawQuery = query.Encode()
	req.Header.Add("User-Agent", userAgentTwo)
	do, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return nil, errors.New(do.Status)
	}
	var obj ArtistListResp
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return nil, err
	}
	return &obj, nil
}

func getPurchasedManUrl(skuID int, showID, userID, uguID string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, streamApiBase+"bigriver/vidPlayer.aspx", nil)
	if err != nil {
		return "", err
	}
	query := url.Values{}
	query.Set("skuId", strconv.Itoa(skuID))
	query.Set("showId", showID)
	query.Set("uguid", uguID)
	query.Set("nn_userID", userID)
	query.Set("app", "1")
	req.URL.RawQuery = query.Encode()
	req.Header.Add("User-Agent", userAgentTwo)
	do, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return "", errors.New(do.Status)
	}
	var obj PurchasedManResp
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return "", err
	}
	return obj.FileURL, nil
}

func getStreamMeta(trackId, skuId, format int, streamParams *StreamParams) (string, error) {
	req, err := http.NewRequest(http.MethodGet, streamApiBase+"bigriver/subPlayer.aspx", nil)
	if err != nil {
		return "", err
	}
	query := url.Values{}
	if format == 0 {
		query.Set("skuId", strconv.Itoa(skuId))
		query.Set("containerID", strconv.Itoa(trackId))
		query.Set("chap", "1")
	} else {
		query.Set("platformID", strconv.Itoa(format))
		query.Set("trackID", strconv.Itoa(trackId))
	}
	query.Set("app", "1")
	query.Set("subscriptionID", streamParams.SubscriptionID)
	query.Set("subCostplanIDAccessList", streamParams.SubCostplanIDAccessList)
	query.Set("nn_userID", streamParams.UserID)
	query.Set("startDateStamp", streamParams.StartStamp)
	query.Set("endDateStamp", streamParams.EndStamp)
	req.URL.RawQuery = query.Encode()
	req.Header.Add("User-Agent", userAgentTwo)
	do, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return "", errors.New(do.Status)
	}
	var obj StreamMeta
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return "", err
	}
	return obj.StreamLink, nil
}

func queryQuality(streamUrl string) *Quality {
	for k, v := range qualityMap {
		if strings.Contains(streamUrl, k) {
			v.URL = streamUrl
			return &v
		}
	}
	return nil
}

func getTrackQual(quals []*Quality, wantFmt int) *Quality {
	for _, quality := range quals {
		if quality.Format == wantFmt {
			return quality
		}
	}
	return nil
}
