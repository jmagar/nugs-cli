 Plan: Full Catalog Crawl + Incremental Updates

 Context

 The Nugs.net catalog has 31,508 shows across 637
 artists. The current catalog.latest feed only
 surfaces ~13,253 of them (recent additions). This
 means:
 - catalog stats under-counts total shows relative
 to reality
 - catalog gaps requires ~1,500 on-demand API calls
 (per-artist, cached 24h) on first use
 - No single source of truth for "what exists on
 Nugs"

 Goal: One-time full crawl (~1,500 API calls, ~5
 min) stores all shows locally. Every subsequent
 catalog update needs exactly 1 API call
 (catalog.latest) to stay current — merging only new
  shows into the existing index. All downstream
 commands (stats, gaps, list) read from local cache.

 Architecture

 New cache files

 File: ~/.cache/nugs/full-catalog-index.json
 Contents: map[int]FullCatalogShow (containerID →
   show)
 Written by: Full crawl + incremental merge
 ────────────────────────────────────────
 File: ~/.cache/nugs/full-catalog-meta.json
 Contents: FullCatalogMeta (timestamps, counts)
 Written by: Both

 New model types (internal/model/full_catalog.go)

 // FullCatalogShow — lightweight show record for
 catalog index.
 // Populated from catalog.containersAll (full
 crawl) or catalog.latest (incremental).
 type FullCatalogShow struct {
     ContainerID                  int
 `json:"containerID"`
     ArtistID                     int
 `json:"artistID"`
     ArtistName                   string
 `json:"artistName"`
     ContainerInfo                string
 `json:"containerInfo"`
     PerformanceDateStr           string
 `json:"performanceDateStr"`
     PerformanceDateShortYearFirst string
 `json:"performanceDateShortYearFirst"`
     PostedDate                   string
 `json:"postedDate"`
     Venue                        string
 `json:"venue"`
     VenueCity                    string
 `json:"venueCity"`
     VenueState                   string
 `json:"venueState"`
     AvailabilityTypeStr          string
 `json:"availabilityTypeStr"` // empty = treat as
 "available"
 }

 // FullCatalogMeta — metadata tracking crawl
 timestamps.
 type FullCatalogMeta struct {
     FullCrawlAt       time.Time
 `json:"fullCrawlAt"`       // when last full crawl
 ran
     LastIncrementalAt time.Time
 `json:"lastIncrementalAt"` // when last
 catalog.latest merge ran
     TotalShows        int       `json:"totalShows"`
     TotalArtists      int
 `json:"totalArtists"`
 }

 New command surface

 nugs catalog update        → incremental (1 API
 call, catalog.latest merge)
 nugs catalog update full   → full crawl (~1,500 API
  calls, all artists)

 The word full as a positional modifier matches
 existing patterns (catalog gaps 1125 fill).

 TDD Implementation Sequence

 Each step: RED (write failing tests) → GREEN
 (minimal implementation) → REFACTOR.

 ---
 Step 1 — Model + Cache I/O (RED → GREEN → REFACTOR)

 New files:
 - internal/model/full_catalog.go — FullCatalogShow,
  FullCatalogMeta types
 - internal/cache/full_catalog.go — read/write
 functions
 - internal/cache/full_catalog_test.go — write tests
  first

 Tests to write (RED phase):
 TestWriteAndReadFullCatalogIndex_RoundTrip    //
 marshal → write → read → unmarshal
 TestReadFullCatalogIndex_MissingFile          //
 returns os.ErrNotExist, not panic
 TestReadFullCatalogIndex_CorruptJSON          //
 returns error, not panic
 TestWriteFullCatalogIndex_IsAtomic           //
 temp file + rename, no partial writes
 TestWriteAndReadFullCatalogMeta_RoundTrip
 TestMergeIntoFullCatalog_AddsNewShows        // new
  containerIDs are added
 TestMergeIntoFullCatalog_SkipsExisting       //
 existing containerIDs untouched
 TestMergeIntoFullCatalog_ReturnsMergeCount   //
 returns count of newly added shows

 Cache functions to implement:
 // internal/cache/full_catalog.go
 func ReadFullCatalogIndex()
 (map[int]model.FullCatalogShow, error)
 func WriteFullCatalogIndex(index
 map[int]model.FullCatalogShow) error
 func ReadFullCatalogMeta() (*model.FullCatalogMeta,
  error)
 func WriteFullCatalogMeta(meta
 *model.FullCatalogMeta) error
 // MergeIntoFullCatalog merges shows into existing
 index, returns count of new additions.
 func MergeIntoFullCatalog(newShows
 []model.FullCatalogShow) (added int, err error)

 All writes use existing atomicWriteFile pattern.
 WriteFullCatalogIndex uses WithCacheLock.

 ---
 Step 2 — Full Crawl Logic (RED → GREEN → REFACTOR)

 New files:
 - internal/catalog/full_crawl.go
 - internal/catalog/full_crawl_test.go — write tests
  first

 Tests to write (RED phase):
 TestFullCrawl_CallsAllArtists              //
 deps.FetchArtistList called once,
 GetArtistMetaCached called per artist
 TestFullCrawl_WritesFullCatalogIndex       //
 result stored correctly
 TestFullCrawl_WritesFullCatalogMeta        //
 fullCrawlAt timestamp set
 TestFullCrawl_RespectsContextCancellation  // stops
  cleanly on ctx.Done()
 TestFullCrawl_ContinuesOnPerArtistError   // one
 artist failure doesn't abort crawl
 TestFullCrawl_DeduplicatesContainerIDs    // no
 duplicate entries in index
 TestShowsFromArtistMeta_ConvertsFields    //
 AlbArtResp → FullCatalogShow field mapping

 Key design — mock deps (no network):
 deps := &Deps{
     FetchArtistList: func(_ context.Context)
 (*model.ArtistListResp, error) {
         return
 buildArtistListResp([]model.Artist{{ArtistID: 1125,
  ArtistName: "Billy Strings", NumShows: 2}}), nil
     },
     GetArtistMetaCached: func(_ context.Context, id
  string, _ time.Duration) ([]*model.ArtistMeta,
 bool, bool, error) {
         return buildArtistMetaPages(id, 2
 /*shows*/), false, false, nil
     },
 }

 Function to implement:
 // internal/catalog/full_crawl.go

 // CatalogFullCrawl crawls all 637 artists and
 builds a complete local catalog index.
 // Uses 5 concurrent workers; progress reported via
  progressFn (nil = silent).
 // Takes ~5 minutes at the 5 req/s rate limit.
 func CatalogFullCrawl(
     ctx context.Context,
     deps *Deps,
     progressFn func(done, total int, artistName
 string),
 ) (totalShows int, err error)

 Concurrency: 5 workers, each pulling artists from a
  channel. Rate limiting handled automatically by
 retryDo inside GetArtistMetaCached. After all
 workers complete, merge all collected shows into
 index atomically via cache.MergeIntoFullCatalog.

 Helper:
 // showsFromArtistMeta converts AlbArtResp pages to
  FullCatalogShow entries.
 func showsFromArtistMeta(pages []*model.ArtistMeta)
  []model.FullCatalogShow

 ---
 Step 3 — Incremental Update Integration (RED →
 GREEN → REFACTOR)

 Modify: internal/catalog/handlers.go —
 CatalogUpdate
 Modify: internal/catalog/handlers_test.go — write
 new tests first

 Tests to write (RED phase):
 TestCatalogUpdate_MergesNewShowsIntoFullCatalog  //
  new catalog.latest items added to index
 TestCatalogUpdate_ReportsNewShowCount            //
  output includes merged count
 TestCatalogUpdate_SkipsFullCatalogMergeIfMissing //
  graceful if full-catalog-index.json not present
 yet

 Change in CatalogUpdate: after writing catalog.json
  (the existing catalog.latest cache), also call
 cache.MergeIntoFullCatalog with the RecentItems
 converted to []FullCatalogShow. This is best-effort
  — failure logs to stderr but doesn't fail the
 update.

 No change to the existing "new shows" detection
 logic — that still compares against
 containers_index.json.

 ---
 Step 4 — Gap Detection Fast Path (RED → GREEN →
 REFACTOR)

 Modify: internal/catalog/media_filter.go —
 AnalyzeArtistCatalogMediaAware
 Modify existing test file — write new tests first

 Tests to write (RED phase):
 TestAnalyzeArtistCatalog_UsesFullCatalogWhenAvailab
 le  // GetArtistMetaCached NOT called if full
 catalog has artist
 TestAnalyzeArtistCatalog_FallsBackToAPIIfFullCatalo
 gMissing  // existing behavior preserved
 TestAnalyzeArtistCatalog_FallsBackToAPIIfArtistNotI
 nFullCatalog // partial crawl handled

 Change in AnalyzeArtistCatalogMediaAware:
 // Before calling deps.GetArtistMetaCached, check
 full catalog:
 if index, err := cache.ReadFullCatalogIndex(); err
 == nil {
     if shows := filterByArtist(index, artistID);
 len(shows) > 0 {
         // Use full catalog data — no API call
 needed
         return analyzeFromFullCatalogShows(shows,
 ...)
     }
 }
 // Fall back to existing per-artist API logic
 artistMetas, cacheUsed, cacheStaleUse, err :=
 deps.GetArtistMetaCached(...)

 Add analyzeFromFullCatalogShows helper that
 converts []FullCatalogShow into
 ArtistCatalogAnalysis using the same presence index
  logic.

 ---
 Step 5 — CLI Wiring (RED → GREEN → REFACTOR)

 Modify: cmd/nugs/main.go — detect catalog update
 full in handleCatalogCommand
 Modify: cmd/nugs/catalog_handlers.go — add
 catalogFullCrawl wrapper
 Add to: internal/catalog/deps.go — no changes
 needed (reuses FetchArtistList +
 GetArtistMetaCached)

 Routing change:
 // In handleCatalogCommand, before existing
 "update" case:
 case args[0] == "catalog" && args[1] == "update" &&
  len(args) > 2 && args[2] == "full":
     err = catalogFullCrawl(ctx, jsonLevel)
 case args[0] == "catalog" && args[1] == "update":
     err = catalogUpdate(ctx, jsonLevel)  //
 existing incremental

 Output for full crawl (plain text):
 Crawling full Nugs catalog...
   Progress: 350 / 637 artists (Widespread Panic)...
 ✓ Full crawl complete
   Total shows indexed: 31,508
   Artists crawled: 637
   Crawl time: 4m32s
   Cache: ~/.cache/nugs/full-catalog-index.json

 ---
 Step 6 — Stats Update (RED → GREEN → REFACTOR)

 Modify: internal/catalog/handlers.go — CatalogStats
 Write tests first.

 Tests to write (RED phase):
 TestCatalogStats_UsesFullCatalogAsNumeratorAndDenom
 inator  // when index present
 TestCatalogStats_FallsBackToArtistListAPIWhenNoFull
 Catalog  // existing behavior

 Change: If full catalog index exists, derive
 totalNugsShows and per-artist counts from it
 directly (no FetchArtistList call needed). The
 index has both ArtistID and ContainerID — group by
 artist to get per-artist show counts.

 ---
 Critical Files

 File: internal/model/full_catalog.go
 Action: CREATE
 Purpose: FullCatalogShow, FullCatalogMeta types
 ────────────────────────────────────────
 File: internal/cache/full_catalog.go
 Action: CREATE
 Purpose: Read/write/merge for full catalog index
 ────────────────────────────────────────
 File: internal/cache/full_catalog_test.go
 Action: CREATE
 Purpose: TDD tests for cache I/O
 ────────────────────────────────────────
 File: internal/catalog/full_crawl.go
 Action: CREATE
 Purpose: CatalogFullCrawl with worker pool
 ────────────────────────────────────────
 File: internal/catalog/full_crawl_test.go
 Action: CREATE
 Purpose: TDD tests for full crawl logic
 ────────────────────────────────────────
 File: internal/catalog/handlers.go
 Action: MODIFY
 Purpose: CatalogUpdate merge, CatalogStats index
   path
 ────────────────────────────────────────
 File: internal/catalog/handlers_test.go
 Action: MODIFY
 Purpose: New tests for updated handlers
 ────────────────────────────────────────
 File: internal/catalog/media_filter.go
 Action: MODIFY
 Purpose: AnalyzeArtistCatalogMediaAware fast path
 ────────────────────────────────────────
 File: cmd/nugs/main.go
 Action: MODIFY
 Purpose: Route catalog update full
 ────────────────────────────────────────
 File: cmd/nugs/catalog_handlers.go
 Action: MODIFY
 Purpose: catalogFullCrawl wrapper

 Reused patterns:
 - cache.atomicWriteFile
 (internal/cache/cache.go:223) — atomic writes
 - cache.WithCacheLock
 (internal/cache/filelock_unix.go) — concurrent
 safety
 - catalog.ArtistMetaCacheTTL
 (internal/catalog/handlers.go:23) — 24h TTL
 constant
 - cache.GetCacheDir (internal/cache/cache.go:15) —
 cache directory resolution
 - testutil.WithTempHome
 (internal/testutil/testutil.go) — test HOME
 isolation
 - Existing Deps struct fields — FetchArtistList,
 GetArtistMetaCached (no new deps needed)

 Verification

 # Step 1 — cache I/O tests pass
 go test ./internal/cache/... -run TestFullCatalog
 -v

 # Step 2 — full crawl tests pass
 go test ./internal/catalog/... -run TestFullCrawl
 -v

 # Step 3-4 — integration tests pass
 go test ./internal/catalog/... -v

 # Step 5-6 — full build + all tests
 make build && go test ./... -count=1 -race

 # Manual E2E
 nugs catalog update full   # first run: ~5 min,
 builds index
 nugs catalog stats          # instant: reads from
 index, no API call
 nugs catalog gaps 1125      # instant: reads from
 index, no API call
 nugs catalog update         # fast: 1 API call,
 merges new shows
 nugs catalog stats          # reflects any newly
 added shows

 # Verify index file created
 ls -lh ~/.cache/nugs/full-catalog-index.json  #
 should be ~6-8MB
