package main

// Root orchestration adapters that combine API and cache behavior.

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jmagar/nugs-cli/internal/api"
	"github.com/jmagar/nugs-cli/internal/cache"
	"github.com/jmagar/nugs-cli/internal/model"
)

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

// preorderCacheTTL is the reduced TTL applied when cached metadata contains any
// non-AVAILABLE shows (e.g. PREORDER). Keeps the window short so a show that
// transitions PREORDER → AVAILABLE is detected within the hour rather than
// waiting the full 24 h window.
const preorderCacheTTL = time.Hour

// hasPendingShows returns true if any page contains a show whose
// availabilityTypeStr is explicitly set to something other than AVAILABLE.
func hasPendingShows(pages []*ArtistMeta) bool {
	for _, page := range pages {
		for _, show := range page.Response.Containers {
			if show == nil {
				continue
			}
			if show.AvailabilityTypeStr != "" &&
				!strings.EqualFold(show.AvailabilityTypeStr, model.AvailableAvailabilityType) {
				return true
			}
		}
	}
	return false
}

// getArtistMetaCached bridges api and cache packages.
// It stays in root because it imports both internal/api and internal/cache.
func getArtistMetaCached(ctx context.Context, artistID string, ttl time.Duration) (pages []*ArtistMeta, cacheUsed bool, cacheStaleUse bool, err error) {
	// availType=1 returns all AVAILABLE shows (the full downloadable catalog).
	// availType=2 returns only PREORDER shows (upcoming, not yet downloadable).
	// Using availType=2 causes IsShowDownloadable to filter everything out (all shows
	// are PREORDER), making gaps detection always report "no missing shows."
	const availType = model.AvailableCatalogView // AVAILABLE is the only downloadable catalog view.

	cachedPages, cachedAt, readErr := readArtistMetaCache(artistID)
	if readErr == nil && len(cachedPages) > 0 {
		effectiveTTL := ttl
		if hasPendingShows(cachedPages) {
			effectiveTTL = preorderCacheTTL
		}
		if time.Since(cachedAt) <= effectiveTTL {
			return cachedPages, true, false, nil
		}
	}

	freshPages, fetchErr := api.GetArtistMetaWithAvailType(ctx, artistID, availType)
	if fetchErr == nil {
		if cacheErr := writeArtistMetaCache(artistID, freshPages); cacheErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to write artist meta cache for %s: %v\n", artistID, cacheErr)
		}
		return freshPages, false, false, nil
	}

	if readErr == nil && len(cachedPages) > 0 {
		return cachedPages, true, true, nil
	}

	return nil, false, false, fetchErr
}

// getArtistListCached fetches the catalog.artists list, using the on-disk cache
// when it is fresher than ttl. On fetch failure it falls back to stale cache
// so stats/list commands still work offline.
func getArtistListCached(ctx context.Context, ttl time.Duration) (*model.ArtistListResp, error) {
	cached, cachedAt, readErr := cache.ReadArtistListCache()
	if readErr == nil && time.Since(cachedAt) <= ttl {
		return cached, nil
	}

	fresh, fetchErr := api.GetArtistList(ctx)
	if fetchErr == nil {
		if cacheErr := cache.WriteArtistListCache(fresh); cacheErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to write artist list cache: %v\n", cacheErr)
		}
		return fresh, nil
	}

	// Stale cache is better than nothing on a network error.
	if readErr == nil && cached != nil {
		fmt.Fprintf(os.Stderr, "warning: using stale artist list cache (fetch failed: %v)\n", fetchErr)
		return cached, nil
	}
	return nil, fetchErr
}
