# Known nugs API endpoints (from current codebase)

This document enumerates endpoints we can **prove** exist based on references in:
- `nugs-client` (Python)
- `nugs` (Go reference)

It intentionally avoids undocumented speculation.

## Conventions used by the current client
### User-Agents
Two user agents are used (matches the Go reference):
- **App UA** (`USER_AGENT`)
  - `NugsNet/3.26.724 (Android; 7.1.2; Asus; ASUS_Z01QD; Scale/2.0; en)`
- **Legacy UA** (`USER_AGENT_LEGACY`)
  - `nugsnetAndroid`

### Authentication
- OAuth2 access token (JWT) acquired via `id.nugs.net`.
- `Authorization: Bearer <token>` is used for `userinfo` and `subscriptions`.
- Legacy token is extracted from the JWT payload (`legacy_token`) and used for `secureApi.aspx`.

## Identity / OAuth domain (`id.nugs.net`)
### `POST https://id.nugs.net/connect/token`
- **Purpose**
  - OAuth2 password grant to get an access token.
- **Headers**
  - `User-Agent: <App UA>`
  - `Content-Type: application/x-www-form-urlencoded`
- **Body** (form)
  - `client_id=<CLIENT_ID>`
  - `grant_type=password`
  - `scope=openid profile email nugsnet:api nugsnet:legacyapi offline_access`
  - `username=<email>`
  - `password=<password>`

### `GET https://id.nugs.net/connect/userinfo`
- **Purpose**
  - Get user identifier used as `nn_userID` downstream.
- **Headers**
  - `Authorization: Bearer <access_token>`
  - `User-Agent: <App UA>`

## Subscriptions domain (`subscriptions.nugs.net`)
### `GET https://subscriptions.nugs.net/api/v1/me/subscriptions`
- **Purpose**
  - Determine subscription eligibility and generate stream parameters.
  - Provides `legacySubscriptionId`, plan IDs, and `startedAt/endsAt`.
- **Headers**
  - `Authorization: Bearer <access_token>`
  - `User-Agent: <App UA>`

## Stream API domain (`streamapi.nugs.net`)
Base URL: `https://streamapi.nugs.net/`

### Catalog: `GET https://streamapi.nugs.net/api.aspx`
This is a method-driven endpoint; behavior changes based on query parameters.

#### `method=catalog.artists`
- **Purpose**
  - List artists (includes `artistID` and `artistName`) without authentication.
- **Query params**
  - `method=catalog.artists`
  - `limit=<page_size>`
  - `startOffset=<1-based offset>`
  - `vdisp=1`
- **Headers**
  - `User-Agent: <App UA>` (recommended; endpoint is unauthenticated)
- **Response (observed fields)**
  - `Response.artists[]` entries include:
    - `artistID`
    - `artistName`
    - `artistImage`
    - `numShows`
    - `numAlbums`
    - `pageURL`

#### Pagination behavior (observed)
- `limit` and `startOffset` appear to be ignored.
- The endpoint returns the full artist list in one response (currently `633` artists) regardless of `limit` and `startOffset`.

#### Notes
- The following method guesses were tested and returned `responseAvailabilityCodeStr=NOT_AVAILABLE`:
  - `catalog.artistsAll`
  - `catalog.artistAll`
  - `catalog.artistsList`
  - `catalog.artistList`

#### `method=catalog.container`
- **Purpose**
  - Fetch a container (album/show) by ID, including tracks.
- **Query params**
  - `method=catalog.container`
  - `containerID=<album_id>`
  - `vdisp=1`
- **Headers**
  - `User-Agent: <App UA>`

#### Artwork (observed)
- Container artwork is exposed via:
  - `Response.pics[]` (preferred)
    - Each pic has `url` like `/images/shows/<id>_01.jpg`
    - These relative paths resolve publicly via `https://secure.livedownloads.com`.
  - `Response.extImage` (fallback)
    - Often a JPEG filename like `<prefix><YYYYMMDD>_cover.jpg` that can sometimes be fetched from `https://secure.livedownloads.com/images/shows/<extImage>`.

#### `method=catalog.containersAll`
- **Purpose**
  - List containers for an artist (this corresponds to "list all shows with IDs from an artist").
- **Query params**
  - `method=catalog.containersAll`
  - `artistList=<artist_id>`
  - `availType=1`
  - `vdisp=1`
  - `limit=<page_size>`
  - `startOffset=<1-based offset>`
- **Headers**
  - `User-Agent: <App UA>`

This is implemented in the Python client as:
- `CatalogAPI.get_artist_metadata(artist_id, offset=1, limit=100)`

#### `method=catalog.latest`
- **Purpose**
  - List recently added shows across ALL artists (global catalog additions).
  - Returns ~13,000+ shows sorted by when they were added to the catalog.
- **Query params**
  - `method=catalog.latest`
  - `vdisp=1`
- **Headers**
  - `User-Agent: <App UA>` (recommended; endpoint is unauthenticated)
- **Response (observed fields)**
  - `Response.recentItems[]` entries include:
    - `containerID` - Show ID
    - `artistID` - Artist ID
    - `artistName` - Artist name
    - `containerInfo` - Show title/description
    - `showDateFormattedShort` - Performance date (MM/DD/YY format)
    - `performanceDateStr` - Performance date (full string)
    - `postedDate` - When show was added to catalog (MM/DD/YY format)
    - `venue` - Venue name
    - `venueCity` - Venue city
    - `venueState` - Venue state
    - `pageURL` - Relative URL path
    - `imageURL` - Relative image path
    - `categoryID` - Content category
- **Notes**
  - Returns the entire recent catalog (13,253+ shows as of Feb 2026)
  - Shows are sorted by `postedDate` (most recently added first)
  - This is NOT sorted by performance date
  - Useful for displaying "what's new" on Nugs.net

#### `method=catalog.playlist`
- **Purpose**
  - Fetch a catalog playlist by GUID.
- **Query params**
  - `method=catalog.playlist`
  - `plGUID=<playlist_guid>`
- **Headers**
  - `User-Agent: <Legacy UA>` (as implemented today)

### Legacy: `GET https://streamapi.nugs.net/secureApi.aspx`
#### `method=user.playlist`
- **Purpose**
  - Fetch a user playlist (requires legacy auth).
- **Query params**
  - `method=user.playlist`
  - `playlistID=<playlist_id>`
  - `developerKey=<DEV_KEY>`
  - `user=<email>`
  - `token=<legacy_token>`
- **Headers**
  - `User-Agent: <Legacy UA>`

### Stream metadata: `GET https://streamapi.nugs.net/bigriver/subPlayer.aspx`
This endpoint is used both for audio and subscription-gated video manifests.

#### Audio stream URL (per platform)
- **Purpose**
  - Return a `streamLink` for a track/format.
  - Called multiple times (platform IDs `1,4,7,10`) to discover available qualities.
- **Query params**
  - `platformID=<1|4|7|10>`
  - `trackID=<track_id>`
  - `app=1`
  - `subscriptionID=<legacySubscriptionId>`
  - `subCostplanIDAccessList=<planId>`
  - `nn_userID=<user_id>`
  - `startDateStamp=<unix_seconds>`
  - `endDateStamp=<unix_seconds>`
- **Headers**
  - `User-Agent: <Legacy UA>`

#### Video manifest URL (subscription video)
- **Purpose**
  - Return a manifest URL for video content tied to a container + SKU.
- **Query params**
  - `skuId=<sku_id>`
  - `containerID=<container_id>`
  - `chap=1`
  - plus the same subscription params as above.
- **Headers**
  - `User-Agent: <Legacy UA>`

### Purchased video manifest: `GET https://streamapi.nugs.net/bigriver/vidPlayer.aspx`
- **Purpose**
  - Return manifest URL for purchased videos.
- **Query params**
  - `skuId=<sku_id>`
  - `showId=<show_id>`
  - `uguid=<legacy_uguid>`
  - `nn_userID=<user_id>`
  - `app=1`
- **Headers**
  - `User-Agent: <Legacy UA>`

## Player web domain (`play.nugs.net`)
### `https://play.nugs.net/`
- Used as the `Referer` for direct file downloads.
- Also the URL base for parsing content types/IDs (album/video/artist URLs).

## What we do *not* currently have (from code)
### Unknown / incomplete endpoint coverage
- The API appears to expose additional `api.aspx?method=...` methods that are not yet enumerated here.
- If you want broader coverage, we should do method discovery from web traffic and/or systematic probing.

## Where this is implemented
- Python:
  - `src/nugs_client/auth.py`
  - `src/nugs_client/endpoints.py`
  - `src/nugs_client/downloader.py`
- Go:
  - `nugs/main.go`
  - `nugs/structs.go`
