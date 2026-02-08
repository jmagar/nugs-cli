package main

// API client wrappers delegating to internal/api during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import (
	"net/http"
	"time"

	"github.com/jmagar/nugs-cli/internal/api"
)

const (
	devKey        = api.DevKey
	clientId      = api.ClientID
	layout        = api.Layout
	userAgent     = api.UserAgent
	userAgentTwo  = api.UserAgentTwo
	authUrl       = api.AuthURL
	streamApiBase = api.StreamAPIBase
	subInfoUrl    = api.SubInfoURL
	userInfoUrl   = api.UserInfoURL
	playerUrl     = api.PlayerURL
)

var (
	jar    = api.Jar
	client = api.Client
)

var qualityMap = api.QualityMap

func auth(email, pwd string) (string, error)       { return api.Auth(email, pwd) }
func getUserInfo(token string) (string, error)      { return api.GetUserInfo(token) }
func getSubInfo(token string) (*SubInfo, error)     { return api.GetSubInfo(token) }
func getPlan(subInfo *SubInfo) (string, bool)        { return api.GetPlan(subInfo) }
func extractLegToken(tokenStr string) (string, string, error) {
	return api.ExtractLegToken(tokenStr)
}
func getAlbumMeta(albumId string) (*AlbumMeta, error) { return api.GetAlbumMeta(albumId) }
func getPlistMeta(plistId, email, legacyToken string, cat bool) (*PlistMeta, error) {
	return api.GetPlistMeta(plistId, email, legacyToken, cat)
}
func getLatestCatalog() (*LatestCatalogResp, error)  { return api.GetLatestCatalog() }
func getArtistMeta(artistId string) ([]*ArtistMeta, error) {
	return api.GetArtistMeta(artistId)
}
func getArtistList() (*ArtistListResp, error) { return api.GetArtistList() }
func getPurchasedManUrl(skuID int, showID, userID, uguID string) (string, error) {
	return api.GetPurchasedManURL(skuID, showID, userID, uguID)
}
func getStreamMeta(trackId, skuId, format int, streamParams *StreamParams) (string, error) {
	return api.GetStreamMeta(trackId, skuId, format, streamParams)
}
func queryQuality(streamUrl string) *Quality { return api.QueryQuality(streamUrl) }
func getTrackQual(quals []*Quality, wantFmt int) *Quality {
	return api.GetTrackQual(quals, wantFmt)
}

// getArtistMetaCached bridges api and cache packages.
// It stays in root because it imports both internal/api and internal/cache.
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

// Re-export for callers that reference the client directly.
func getHTTPClient() *http.Client { return api.Client }
