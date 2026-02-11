package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jmagar/nugs-cli/internal/model"
)

const (
	// DevKey and ClientID are intentionally hardcoded. Do NOT move these to env vars,
	// config files, or build-time injection. They are public app-level identifiers
	// required by the Nugs.net API and are not secret credentials.
	DevKey   = "x7f54tgbdyc64y656thy47er4"
	ClientID = "Eg7HuH873H65r5rt325UytR5429"
	Layout        = "01/02/2006 15:04:05"
	UserAgent     = "NugsNet/3.26.724 (Android; 7.1.2; Asus; ASUS_Z01QD; Scale/2.0; en)"
	UserAgentTwo  = "nugsnetAndroid"
	AuthURL       = "https://id.nugs.net/connect/token"
	StreamAPIBase = "https://streamapi.nugs.net/"
	SubInfoURL    = "https://subscriptions.nugs.net/api/v1/me/subscriptions"
	UserInfoURL   = "https://id.nugs.net/connect/userinfo"
	PlayerURL     = "https://play.nugs.net/"
)

var (
	Jar    = mustCookieJar()
	Client = &http.Client{
		Jar:     Jar,
		Timeout: 30 * time.Second,
	}
)

func mustCookieJar() *cookiejar.Jar {
	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create cookie jar: %v", err))
	}
	return jar
}

// QualityMap maps URL path segments to quality info.
var QualityMap = map[string]model.Quality{
	".alac16/": {Specs: "16-bit / 44.1 kHz ALAC", Extension: ".m4a", Format: 1},
	".flac16/": {Specs: "16-bit / 44.1 kHz FLAC", Extension: ".flac", Format: 2},
	".mqa24/":  {Specs: "24-bit / 48 kHz MQA", Extension: ".flac", Format: 3},
	".flac?":   {Specs: "FLAC", Extension: ".flac", Format: 2},
	".s360/":   {Specs: "360 Reality Audio", Extension: ".mp4", Format: 4},
	".aac150/": {Specs: "150 Kbps AAC", Extension: ".m4a", Format: 5},
	".m4a?":    {Specs: "AAC", Extension: ".m4a", Format: 5},
	".m3u8?":   {Extension: ".m4a", Format: 6},
}

// Auth authenticates with email/password and returns an access token.
func Auth(ctx context.Context, email, pwd string) (string, error) {
	data := url.Values{}
	data.Set("client_id", ClientID)
	data.Set("grant_type", "password")
	data.Set("scope", "openid profile email nugsnet:api nugsnet:legacyapi offline_access")
	data.Set("username", email)
	data.Set("password", pwd)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, AuthURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Add("User-Agent", UserAgent)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	do, err := Client.Do(req)
	if err != nil {
		return "", err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return "", errors.New(do.Status)
	}
	var obj model.Auth
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return "", err
	}
	return obj.AccessToken, nil
}

// GetUserInfo retrieves user subscription ID.
func GetUserInfo(ctx context.Context, token string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, UserInfoURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("User-Agent", UserAgent)
	do, err := Client.Do(req)
	if err != nil {
		return "", err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return "", errors.New(do.Status)
	}
	var obj model.UserInfo
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return "", err
	}
	return obj.Sub, nil
}

// GetSubInfo retrieves subscription information.
func GetSubInfo(ctx context.Context, token string) (*model.SubInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, SubInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("User-Agent", UserAgent)
	do, err := Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return nil, errors.New(do.Status)
	}
	var obj model.SubInfo
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return nil, err
	}
	return &obj, nil
}

// GetPlan returns the plan description and whether it's a promo.
func GetPlan(subInfo *model.SubInfo) (string, bool) {
	if subInfo.Plan.ID != "" {
		return subInfo.Plan.Description, false
	}
	return subInfo.Promo.Plan.Description, true
}

// ExtractLegToken extracts legacy token and uguid from JWT.
func ExtractLegToken(tokenStr string) (string, string, error) {
	parts := strings.SplitN(tokenStr, ".", 3)
	if len(parts) < 2 {
		return "", "", errors.New("invalid JWT: expected at least 2 dot-separated parts")
	}
	payload := parts[1]
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return "", "", err
	}
	var obj model.Payload
	err = json.Unmarshal(decoded, &obj)
	if err != nil {
		return "", "", err
	}
	return obj.LegacyToken, obj.LegacyUguid, nil
}

// GetAlbumMeta retrieves album metadata by container ID.
func GetAlbumMeta(ctx context.Context, albumId string) (*model.AlbumMeta, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, StreamAPIBase+"api.aspx", nil)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("method", "catalog.container")
	query.Set("containerID", albumId)
	query.Set("vdisp", "1")
	req.URL.RawQuery = query.Encode()
	req.Header.Add("User-Agent", UserAgent)
	do, err := Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return nil, errors.New(do.Status)
	}
	var obj model.AlbumMeta
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return nil, err
	}
	return &obj, nil
}

// GetPlistMeta retrieves playlist metadata.
func GetPlistMeta(ctx context.Context, plistId, email, legacyToken string, cat bool) (*model.PlistMeta, error) {
	var path string
	if cat {
		path = "api.aspx"
	} else {
		path = "secureApi.aspx"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, StreamAPIBase+path, nil)
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
		query.Set("developerKey", DevKey)
		query.Set("user", email)
		query.Set("token", legacyToken)
	}
	req.URL.RawQuery = query.Encode()
	req.Header.Add("User-Agent", UserAgentTwo)
	do, err := Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return nil, errors.New(do.Status)
	}
	var obj model.PlistMeta
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return nil, err
	}
	return &obj, nil
}

// GetLatestCatalog retrieves the latest catalog from the API.
func GetLatestCatalog(ctx context.Context) (*model.LatestCatalogResp, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, StreamAPIBase+"api.aspx", nil)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("method", "catalog.latest")
	query.Set("vdisp", "1")
	req.URL.RawQuery = query.Encode()
	req.Header.Add("User-Agent", UserAgentTwo)
	do, err := Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return nil, errors.New(do.Status)
	}
	var obj model.LatestCatalogResp
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return nil, err
	}
	return &obj, nil
}

// GetArtistMeta retrieves all pages of artist metadata.
func GetArtistMeta(ctx context.Context, artistId string) ([]*model.ArtistMeta, error) {
	var allArtistMeta []*model.ArtistMeta
	offset := 1
	query := url.Values{}
	query.Set("method", "catalog.containersAll")
	query.Set("limit", "100")
	query.Set("artistList", artistId)
	query.Set("availType", "1")
	query.Set("vdisp", "1")
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, StreamAPIBase+"api.aspx", nil)
		if err != nil {
			return nil, err
		}
		query.Set("startOffset", strconv.Itoa(offset))
		req.URL.RawQuery = query.Encode()
		req.Header.Add("User-Agent", UserAgent)
		do, err := Client.Do(req)
		if err != nil {
			return nil, err
		}
		if do.StatusCode != http.StatusOK {
			do.Body.Close()
			return nil, errors.New(do.Status)
		}
		var obj model.ArtistMeta
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

// GetArtistList retrieves the list of all artists.
func GetArtistList(ctx context.Context) (*model.ArtistListResp, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, StreamAPIBase+"api.aspx", nil)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("method", "catalog.artists")
	query.Set("vdisp", "1")
	req.URL.RawQuery = query.Encode()
	req.Header.Add("User-Agent", UserAgentTwo)
	do, err := Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return nil, errors.New(do.Status)
	}
	var obj model.ArtistListResp
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return nil, err
	}
	return &obj, nil
}

// GetPurchasedManURL retrieves the purchased manifest URL.
func GetPurchasedManURL(ctx context.Context, skuID int, showID, userID, uguID string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, StreamAPIBase+"bigriver/vidPlayer.aspx", nil)
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
	req.Header.Add("User-Agent", UserAgentTwo)
	do, err := Client.Do(req)
	if err != nil {
		return "", err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return "", errors.New(do.Status)
	}
	var obj model.PurchasedManResp
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return "", err
	}
	return obj.FileURL, nil
}

// GetStreamMeta retrieves the stream URL for a track.
func GetStreamMeta(ctx context.Context, trackId, skuId, format int, streamParams *model.StreamParams) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, StreamAPIBase+"bigriver/subPlayer.aspx", nil)
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
	req.Header.Add("User-Agent", UserAgentTwo)
	do, err := Client.Do(req)
	if err != nil {
		return "", err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return "", errors.New(do.Status)
	}
	var obj model.StreamMeta
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return "", err
	}
	return obj.StreamLink, nil
}

// QueryQuality identifies the quality from a stream URL.
func QueryQuality(streamUrl string) *model.Quality {
	for k, v := range QualityMap {
		if strings.Contains(streamUrl, k) {
			v.URL = streamUrl
			return &v
		}
	}
	return nil
}

// GetTrackQual finds a quality matching the desired format.
func GetTrackQual(quals []*model.Quality, wantFmt int) *model.Quality {
	for _, quality := range quals {
		if quality.Format == wantFmt {
			return quality
		}
	}
	return nil
}
