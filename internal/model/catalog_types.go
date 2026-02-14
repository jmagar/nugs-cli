package model

import "time"

// LatestCatalogResp represents the response from catalog.latest.
type LatestCatalogResp struct {
	MethodName                  string `json:"methodName"`
	ResponseAvailabilityCode    int    `json:"responseAvailabilityCode"`
	ResponseAvailabilityCodeStr string `json:"responseAvailabilityCodeStr"`
	Response                    struct {
		RecentItems []struct {
			ContainerInfo          string `json:"containerInfo"`
			ArtistName             string `json:"artistName"`
			ShowDateFormattedShort string `json:"showDateFormattedShort"`
			ArtistID               int    `json:"artistID"`
			ContainerID            int    `json:"containerID"`
			PerformanceDateStr     string `json:"performanceDateStr"`
			PostedDate             string `json:"postedDate"`
			VenueCity              string `json:"venueCity"`
			VenueState             string `json:"venueState"`
			Venue                  string `json:"venue"`
			PageURL                string `json:"pageURL"`
			CategoryID             int    `json:"categoryID"`
			ImageURL               string `json:"imageURL"`
		} `json:"recentItems"`
	} `json:"Response"`
}

// CacheMeta stores metadata about the cached catalog.
type CacheMeta struct {
	LastUpdated    time.Time `json:"lastUpdated"`
	CacheVersion   string    `json:"cacheVersion"`
	TotalShows     int       `json:"totalShows"`
	TotalArtists   int       `json:"totalArtists"`
	APIMethod      string    `json:"apiMethod"`
	UpdateDuration string    `json:"updateDuration"`
}

// ArtistsIndex provides fast artist name to ID lookup.
type ArtistsIndex struct {
	Index map[string]int `json:"index"`
}

// ContainersIndex maps container IDs to artist information.
type ContainersIndex struct {
	Containers map[int]ContainerIndexEntry `json:"containers"`
}

// ContainerIndexEntry stores basic show info for gap detection.
type ContainerIndexEntry struct {
	ArtistID        int    `json:"artistID"`
	ArtistName      string `json:"artistName"`
	ContainerInfo   string `json:"containerInfo"`
	PerformanceDate string `json:"performanceDate"`
}

// ArtistMetaCache stores cached artist metadata pages from catalog.containersAll.
type ArtistMetaCache struct {
	ArtistID string        `json:"artistID"`
	CachedAt time.Time     `json:"cachedAt"`
	Pages    []*ArtistMeta `json:"pages"`
}
