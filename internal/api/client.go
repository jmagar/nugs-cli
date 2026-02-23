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
	DevKey        = "x7f54tgbdyc64y656thy47er4"
	ClientID      = "Eg7HuH873H65r5rt325UytR5429"
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

	// RateLimiter enforces a courtesy rate limit on all outbound API calls.
	// 5 requests/second with a burst of 10 — prevents hammering the server
	// during batch operations while allowing normal interactive use to feel instant.
	RateLimiter = newRateLimiter(5.0, 10)

	// CircuitBreaker trips after 5 consecutive API-level failures (HTTP 429 or 5xx)
	// and stays open for 60 seconds before probing recovery.
	CircuitBreaker = newCircuitBreaker(5, 60*time.Second)
)

func mustCookieJar() *cookiejar.Jar {
	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create cookie jar: %v", err))
	}
	return jar
}

// retryDo is the single gateway for every outbound API call.
//
// It enforces, in order:
//  1. Rate limiting  — token-bucket, 5 req/s, burst 10
//  2. Circuit breaker — rejects immediately when open; logs state transitions
//  3. HTTP execution  — with context cancellation
//  4. Retry on 429 / 5xx — exponential backoff (500ms → 30s), Retry-After respected
//  5. Structured logging — every attempt, wait, rejection, and state change logged
//
// label is a short human-readable endpoint name used in log entries (e.g. "catalog.container").
// Caller is responsible for closing the returned response body.
func retryDo(ctx context.Context, label string, makeReq func() (*http.Request, error)) (*http.Response, error) {
	const maxRetries = 4
	backoff := 500 * time.Millisecond

	for attempt := 0; ; attempt++ {
		// 1. Rate limiter — block until a token is available.
		waited, err := RateLimiter.Wait(ctx)
		if err != nil {
			return nil, fmt.Errorf("rate limiter cancelled for %s: %w", label, err)
		}
		// Only log if we actually waited (> 1ms threshold avoids noise).
		if waited > time.Millisecond {
			LogRateLimitWait(label, waited)
		}

		// 2. Circuit breaker — fail fast when the API is known-down.
		cbState, allowed := CircuitBreaker.Allow()
		if !allowed {
			LogCircuitRejected(label)
			return nil, fmt.Errorf("%w (label: %s)", ErrCircuitOpen, label)
		}

		// 3. Build and execute the request.
		req, err := makeReq()
		if err != nil {
			return nil, err
		}
		start := time.Now()
		resp, err := Client.Do(req)
		duration := time.Since(start)

		if err != nil {
			// Network-level error: log but do NOT trip the circuit breaker.
			// Network hiccups are distinct from the API being overloaded.
			LogRequest(label, 0, duration, attempt, cbState.String(), err)
			return nil, err
		}

		isAPIError := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		if !isAPIError {
			// Success (2xx, 3xx, or client-error 4xx — server is healthy).
			prev := CircuitBreaker.RecordSuccess()
			if prev != circuitClosed {
				LogCircuitStateChange("circuit_closed", label, prev.String(), circuitClosed.String())
			}
			LogRequest(label, resp.StatusCode, duration, attempt, circuitClosed.String(), nil)
			return resp, nil
		}

		// API-level error: trip circuit breaker, log, then retry or give up.
		resp.Body.Close()
		newState := CircuitBreaker.RecordFailure()
		if newState == circuitOpen && cbState != circuitOpen {
			LogCircuitStateChange("circuit_opened", label, cbState.String(), newState.String())
		}
		apiErr := fmt.Errorf("HTTP %s", resp.Status)
		LogRequest(label, resp.StatusCode, duration, attempt, newState.String(), apiErr)

		if attempt >= maxRetries {
			return nil, fmt.Errorf("API %s failed after %d attempts: %w", label, attempt+1, apiErr)
		}

		wait := backoff
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if secs, e := strconv.Atoi(ra); e == nil {
				wait = time.Duration(secs) * time.Second
			}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}

// qualityPattern pairs a URL substring with its quality info.
type qualityPattern struct {
	Pattern string
	Quality model.Quality
}

// qualityPatterns is checked in order; more specific patterns first.
var qualityPatterns = []qualityPattern{
	{".alac16/", model.Quality{Specs: "16-bit / 44.1 kHz ALAC", Extension: ".m4a", Format: 1}},
	{".flac16/", model.Quality{Specs: "16-bit / 44.1 kHz FLAC", Extension: ".flac", Format: 2}},
	{".mqa24/", model.Quality{Specs: "24-bit / 48 kHz MQA", Extension: ".flac", Format: 3}},
	{".s360/", model.Quality{Specs: "360 Reality Audio", Extension: ".mp4", Format: 4}},
	{".aac150/", model.Quality{Specs: "150 Kbps AAC", Extension: ".m4a", Format: 5}},
	{".flac?", model.Quality{Specs: "FLAC", Extension: ".flac", Format: 2}},
	{".m4a?", model.Quality{Specs: "AAC", Extension: ".m4a", Format: 5}},
	{".m3u8?", model.Quality{Extension: ".m4a", Format: 6}},
}

// QualityMap maps URL path segments to quality info.
// Deprecated: Use qualityPatterns for deterministic iteration order.
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
	encoded := data.Encode()
	do, err := retryDo(ctx, "auth", func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, AuthURL, strings.NewReader(encoded))
		if err != nil {
			return nil, err
		}
		req.Header.Add("User-Agent", UserAgent)
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		return req, nil
	})
	if err != nil {
		return "", err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API authentication failed: %s", do.Status)
	}
	var obj model.Auth
	if err = json.NewDecoder(do.Body).Decode(&obj); err != nil {
		return "", err
	}
	return obj.AccessToken, nil
}

// GetUserInfo retrieves user subscription ID.
func GetUserInfo(ctx context.Context, token string) (string, error) {
	do, err := retryDo(ctx, "userinfo", func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, UserInfoURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Add("Authorization", "Bearer "+token)
		req.Header.Add("User-Agent", UserAgent)
		return req, nil
	})
	if err != nil {
		return "", err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API GetUserInfo failed: %s", do.Status)
	}
	var obj model.UserInfo
	if err = json.NewDecoder(do.Body).Decode(&obj); err != nil {
		return "", err
	}
	return obj.Sub, nil
}

// GetSubInfo retrieves subscription information.
func GetSubInfo(ctx context.Context, token string) (*model.SubInfo, error) {
	do, err := retryDo(ctx, "subscriptions", func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, SubInfoURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Add("Authorization", "Bearer "+token)
		req.Header.Add("User-Agent", UserAgent)
		return req, nil
	})
	if err != nil {
		return nil, err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API GetSubInfo failed: %s", do.Status)
	}
	var obj model.SubInfo
	if err = json.NewDecoder(do.Body).Decode(&obj); err != nil {
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
	query := url.Values{}
	query.Set("method", "catalog.container")
	query.Set("containerID", albumId)
	query.Set("vdisp", "1")
	rawQuery := query.Encode()
	do, err := retryDo(ctx, "catalog.container", func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, StreamAPIBase+"api.aspx", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = rawQuery
		req.Header.Add("User-Agent", UserAgent)
		return req, nil
	})
	if err != nil {
		return nil, err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API GetAlbumMeta failed: %s", do.Status)
	}
	var obj model.AlbumMeta
	if err = json.NewDecoder(do.Body).Decode(&obj); err != nil {
		return nil, err
	}
	return &obj, nil
}

// GetPlistMeta retrieves playlist metadata.
func GetPlistMeta(ctx context.Context, plistId, email, legacyToken string, cat bool) (*model.PlistMeta, error) {
	var apiPath string
	if cat {
		apiPath = "api.aspx"
	} else {
		apiPath = "secureApi.aspx"
	}
	query := url.Values{}
	label := "user.playlist"
	if cat {
		label = "catalog.playlist"
		query.Set("method", "catalog.playlist")
		query.Set("plGUID", plistId)
	} else {
		query.Set("method", "user.playlist")
		query.Set("playlistID", plistId)
		query.Set("developerKey", DevKey)
		query.Set("user", email)
		query.Set("token", legacyToken)
	}
	rawQuery := query.Encode()
	do, err := retryDo(ctx, label, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, StreamAPIBase+apiPath, nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = rawQuery
		req.Header.Add("User-Agent", UserAgentTwo)
		return req, nil
	})
	if err != nil {
		return nil, err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API GetPlistMeta failed: %s", do.Status)
	}
	var obj model.PlistMeta
	if err = json.NewDecoder(do.Body).Decode(&obj); err != nil {
		return nil, err
	}
	return &obj, nil
}

// GetLatestCatalog retrieves the latest catalog from the API.
func GetLatestCatalog(ctx context.Context) (*model.LatestCatalogResp, error) {
	query := url.Values{}
	query.Set("method", "catalog.latest")
	query.Set("vdisp", "1")
	rawQuery := query.Encode()
	do, err := retryDo(ctx, "catalog.latest", func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, StreamAPIBase+"api.aspx", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = rawQuery
		req.Header.Add("User-Agent", UserAgentTwo)
		return req, nil
	})
	if err != nil {
		return nil, err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API GetLatestCatalog failed: %s", do.Status)
	}
	var obj model.LatestCatalogResp
	if err = json.NewDecoder(do.Body).Decode(&obj); err != nil {
		return nil, err
	}
	return &obj, nil
}

func getArtistMetaByAvailType(ctx context.Context, artistId string, availType int) ([]*model.ArtistMeta, error) {
	var allArtistMeta []*model.ArtistMeta
	offset := 1
	baseQuery := url.Values{}
	baseQuery.Set("method", "catalog.containersAll")
	baseQuery.Set("limit", "100")
	baseQuery.Set("artistList", artistId)
	baseQuery.Set("availType", strconv.Itoa(availType))
	baseQuery.Set("vdisp", "1")
	for {
		currentOffset := offset // capture for closure
		do, err := retryDo(ctx, "catalog.containersAll", func() (*http.Request, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, StreamAPIBase+"api.aspx", nil)
			if err != nil {
				return nil, err
			}
			q := baseQuery
			q.Set("startOffset", strconv.Itoa(currentOffset))
			req.URL.RawQuery = q.Encode()
			req.Header.Add("User-Agent", UserAgent)
			return req, nil
		})
		if err != nil {
			return nil, err
		}
		if do.StatusCode != http.StatusOK {
			do.Body.Close()
			return nil, fmt.Errorf("API getArtistMetaByAvailType failed: %s", do.Status)
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

// GetArtistMeta retrieves all pages of artist metadata (audio catalog view).
func GetArtistMeta(ctx context.Context, artistId string) ([]*model.ArtistMeta, error) {
	return getArtistMetaByAvailType(ctx, artistId, 1)
}

// GetArtistMetaWithAvailType retrieves all pages of artist metadata for a specific availability type.
func GetArtistMetaWithAvailType(ctx context.Context, artistId string, availType int) ([]*model.ArtistMeta, error) {
	return getArtistMetaByAvailType(ctx, artistId, availType)
}

// GetArtistList retrieves the list of all artists.
func GetArtistList(ctx context.Context) (*model.ArtistListResp, error) {
	query := url.Values{}
	query.Set("method", "catalog.artists")
	query.Set("vdisp", "1")
	rawQuery := query.Encode()
	do, err := retryDo(ctx, "catalog.artists", func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, StreamAPIBase+"api.aspx", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = rawQuery
		req.Header.Add("User-Agent", UserAgentTwo)
		return req, nil
	})
	if err != nil {
		return nil, err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API GetArtistList failed: %s", do.Status)
	}
	var obj model.ArtistListResp
	if err = json.NewDecoder(do.Body).Decode(&obj); err != nil {
		return nil, err
	}
	return &obj, nil
}

// GetPurchasedManURL retrieves the purchased manifest URL.
func GetPurchasedManURL(ctx context.Context, skuID int, showID, userID, uguID string) (string, error) {
	query := url.Values{}
	query.Set("skuId", strconv.Itoa(skuID))
	query.Set("showId", showID)
	query.Set("uguid", uguID)
	query.Set("nn_userID", userID)
	query.Set("app", "1")
	rawQuery := query.Encode()
	do, err := retryDo(ctx, "vidPlayer", func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, StreamAPIBase+"bigriver/vidPlayer.aspx", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = rawQuery
		req.Header.Add("User-Agent", UserAgentTwo)
		return req, nil
	})
	if err != nil {
		return "", err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API GetPurchasedManURL failed: %s", do.Status)
	}
	var obj model.PurchasedManResp
	if err = json.NewDecoder(do.Body).Decode(&obj); err != nil {
		return "", err
	}
	return obj.FileURL, nil
}

// GetStreamMeta retrieves the stream URL for a track.
// Retries automatically on 429 / 5xx with exponential backoff.
func GetStreamMeta(ctx context.Context, trackId, skuId, format int, streamParams *model.StreamParams) (string, error) {
	buildQuery := func() string {
		q := url.Values{}
		if format == 0 {
			q.Set("skuId", strconv.Itoa(skuId))
			q.Set("containerID", strconv.Itoa(trackId))
			q.Set("chap", "1")
		} else {
			q.Set("platformID", strconv.Itoa(format))
			q.Set("trackID", strconv.Itoa(trackId))
		}
		q.Set("app", "1")
		q.Set("subscriptionID", streamParams.SubscriptionID)
		q.Set("subCostplanIDAccessList", streamParams.SubCostplanIDAccessList)
		q.Set("nn_userID", streamParams.UserID)
		q.Set("startDateStamp", streamParams.StartStamp)
		q.Set("endDateStamp", streamParams.EndStamp)
		return q.Encode()
	}
	rawQuery := buildQuery()
	do, err := retryDo(ctx, "subPlayer", func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, StreamAPIBase+"bigriver/subPlayer.aspx", nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = rawQuery
		req.Header.Add("User-Agent", UserAgentTwo)
		return req, nil
	})
	if err != nil {
		return "", err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API GetStreamMeta failed: %s", do.Status)
	}
	var obj model.StreamMeta
	if err = json.NewDecoder(do.Body).Decode(&obj); err != nil {
		return "", err
	}
	return obj.StreamLink, nil
}

// QueryQuality identifies the quality from a stream URL.
// Patterns are checked in deterministic order with more specific patterns first.
func QueryQuality(streamUrl string) *model.Quality {
	for _, entry := range qualityPatterns {
		if strings.Contains(streamUrl, entry.Pattern) {
			q := entry.Quality
			q.URL = streamUrl
			return &q
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
