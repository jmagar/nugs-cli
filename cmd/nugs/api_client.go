package main

// API client wrappers delegating to internal/api during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import (
	"context"
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

func auth(ctx context.Context, email, pwd string) (string, error) {
	return api.Auth(ctx, email, pwd)
}
func getUserInfo(ctx context.Context, token string) (string, error) {
	return api.GetUserInfo(ctx, token)
}
func getSubInfo(ctx context.Context, token string) (*SubInfo, error) {
	return api.GetSubInfo(ctx, token)
}
func getPlan(subInfo *SubInfo) (string, bool) { return api.GetPlan(subInfo) }
func extractLegToken(tokenStr string) (string, string, error) {
	return api.ExtractLegToken(tokenStr)
}
func getAlbumMeta(ctx context.Context, albumId string) (*AlbumMeta, error) {
	return api.GetAlbumMeta(ctx, albumId)
}
func getPlistMeta(ctx context.Context, plistId, email, legacyToken string, cat bool) (*PlistMeta, error) {
	return api.GetPlistMeta(ctx, plistId, email, legacyToken, cat)
}
func getLatestCatalog(ctx context.Context) (*LatestCatalogResp, error) {
	return api.GetLatestCatalog(ctx)
}
func getArtistMeta(ctx context.Context, artistId string) ([]*ArtistMeta, error) {
	return api.GetArtistMeta(ctx, artistId)
}
func getArtistList(ctx context.Context) (*ArtistListResp, error) {
	return api.GetArtistList(ctx)
}
func getPurchasedManUrl(ctx context.Context, skuID int, showID, userID, uguID string) (string, error) {
	return api.GetPurchasedManURL(ctx, skuID, showID, userID, uguID)
}
func getStreamMeta(ctx context.Context, trackId, skuId, format int, streamParams *StreamParams) (string, error) {
	return api.GetStreamMeta(ctx, trackId, skuId, format, streamParams)
}
func queryQuality(streamUrl string) *Quality { return api.QueryQuality(streamUrl) }
func getTrackQual(quals []*Quality, wantFmt int) *Quality {
	return api.GetTrackQual(quals, wantFmt)
}

// getArtistMetaCached bridges api and cache packages.
// It stays in root because it imports both internal/api and internal/cache.
func getArtistMetaCached(ctx context.Context, artistID string, ttl time.Duration) (pages []*ArtistMeta, cacheUsed bool, cacheStaleUse bool, err error) {
	// Catalog commands need ALL shows (both audio and video) for accurate analysis.
	// Using availType=2 ensures we get the full show list from the API.
	const availType = 2 // Fetch with video availability to get complete catalog

	cachedPages, cachedAt, readErr := readArtistMetaCache(artistID)
	if readErr == nil && len(cachedPages) > 0 {
		if time.Since(cachedAt) <= ttl {
			return cachedPages, true, false, nil
		}
	}

	freshPages, fetchErr := api.GetArtistMetaWithAvailType(ctx, artistID, availType)
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
