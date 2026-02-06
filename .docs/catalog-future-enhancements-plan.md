# Catalog Future Enhancements Plan

**Created**: 2026-02-05
**Status**: Planning Phase
**Priority**: Medium (Post-MVP Enhancements)

## Overview

This document outlines planned enhancements to the catalog caching system based on user needs and lessons learned from initial implementation. These features build on the existing foundation to provide more powerful search, analysis, and tracking capabilities.

---

## Enhancement 1: Historical Snapshots & Change Tracking

**Priority**: High
**Estimated Effort**: 3-4 hours

### Description

Track catalog changes over time by creating daily snapshots. This enables users to:
- See what shows were added/removed on specific dates
- Track pricing changes
- Monitor artist catalog growth
- Detect shows that became unavailable

### Implementation Plan

**1. Snapshot Storage Structure**
```
~/.cache/nugs/
├── snapshots/
│   ├── 2026-02-05.json           # Daily snapshot
│   ├── 2026-02-04.json
│   └── index.json                # Snapshot metadata index
├── catalog.json                   # Current catalog (unchanged)
└── catalog_meta.json
```

**2. Snapshot Data Model**
```go
type SnapshotMeta struct {
    Date           string    `json:"date"`           // "2026-02-05"
    TotalShows     int       `json:"totalShows"`
    TotalArtists   int       `json:"totalArtists"`
    FilePath       string    `json:"filePath"`
    FileSizeBytes  int64     `json:"fileSizeBytes"`
}

type SnapshotIndex struct {
    Snapshots []SnapshotMeta `json:"snapshots"`
}
```

**3. New Commands**

```bash
# Create snapshot manually
nugs catalog snapshot create

# List available snapshots
nugs catalog snapshot list

# Compare two snapshots
nugs catalog diff 2026-02-04 2026-02-05
# Output: Shows added (3), removed (1), price changes (0)

# Show changes for specific artist
nugs catalog diff 2026-02-04 2026-02-05 --artist 1125

# Cleanup old snapshots
nugs catalog snapshot cleanup --keep-days 30
```

**4. Auto-Snapshot on Update**

Modify `catalogUpdate()` to:
- Create snapshot before overwriting catalog.json
- Store with date-based filename
- Update snapshot index
- Auto-cleanup snapshots older than 30 days (configurable)

**5. Diff Algorithm**

```go
type CatalogDiff struct {
    Added      []ShowDiff   `json:"added"`
    Removed    []ShowDiff   `json:"removed"`
    Modified   []ShowDiff   `json:"modified"`
    Unchanged  int          `json:"unchanged"`
}

type ShowDiff struct {
    ContainerID   int    `json:"containerID"`
    ArtistName    string `json:"artistName"`
    ContainerInfo string `json:"containerInfo"`
    ChangeType    string `json:"changeType"`  // "price", "availability"
    OldValue      string `json:"oldValue,omitempty"`
    NewValue      string `json:"newValue,omitempty"`
}
```

**Benefits**:
- Historical analysis
- Track catalog growth
- Identify removed shows to download
- Monitor pricing trends

---

## Enhancement 2: Advanced Search & Filtering

**Priority**: High
**Estimated Effort**: 2-3 hours

### Description

Add powerful search capabilities to find shows without downloading entire artist catalogs.

### Implementation Plan

**1. Search Index Structure**

Extend existing indexes with search capabilities:
```go
type SearchIndex struct {
    ByArtist     map[string][]int       `json:"byArtist"`     // name → containerIDs
    ByVenue      map[string][]int       `json:"byVenue"`      // venue → containerIDs
    ByCity       map[string][]int       `json:"byCity"`       // city → containerIDs
    ByState      map[string][]int       `json:"byState"`      // state → containerIDs
    ByYear       map[int][]int          `json:"byYear"`       // year → containerIDs
    ByMonth      map[string][]int       `json:"byMonth"`      // "2026-02" → containerIDs
}
```

**2. New Commands**

```bash
# Search by artist name (fuzzy matching)
nugs catalog search "Billy Strings"
nugs catalog search "grateful dead" --limit 20

# Search by venue
nugs catalog search --venue "Red Rocks"
nugs catalog search --venue "Madison Square Garden"

# Search by location
nugs catalog search --city "Denver"
nugs catalog search --state "CO"
nugs catalog search --city "Denver" --state "CO"

# Search by date range
nugs catalog search --year 2024
nugs catalog search --month 2024-12
nugs catalog search --date-range 2024-01-01:2024-12-31

# Combine filters
nugs catalog search "Phish" --venue "Red Rocks" --year 2023

# JSON output for piping
nugs catalog search "Billy Strings" --json standard | jq '.shows[] | .containerID'
```

**3. Search Algorithm**

```go
type SearchQuery struct {
    Artist     string
    Venue      string
    City       string
    State      string
    Year       int
    Month      string
    DateRange  DateRange
    Limit      int
}

type SearchResults struct {
    Query       SearchQuery  `json:"query"`
    Results     []ShowOutput `json:"results"`
    TotalFound  int          `json:"totalFound"`
    TotalShown  int          `json:"totalShown"`
}
```

**4. Fuzzy Matching**

Use Levenshtein distance for artist/venue name matching:
- Exact match: Priority 1
- Partial match: Priority 2
- Fuzzy match (< 3 edits): Priority 3

**Benefits**:
- Quick show discovery
- No need to know artist ID
- Filter by multiple criteria
- Great for trip planning (by city/state)

---

## Enhancement 3: Enhanced Gap Detection

**Priority**: Medium
**Estimated Effort**: 1-2 hours

### Description

Improve gap detection with filtering and better output formats.

### Implementation Plan

**1. Filter Options**

```bash
# Filter gaps by year
nugs catalog gaps 1125 --year 2024
nugs catalog gaps 1125 --year 2023,2024,2025

# Filter by date range
nugs catalog gaps 1125 --date-range 2024-01-01:2024-12-31

# Filter by venue/location
nugs catalog gaps 1125 --state "CO"
nugs catalog gaps 1125 --venue "Red Rocks"

# Exclude specific years (e.g., skip old shows)
nugs catalog gaps 1125 --exclude-before 2020

# Only show recent gaps
nugs catalog gaps 1125 --recent 90  # Last 90 days

# Sort options
nugs catalog gaps 1125 --sort date-desc     # Newest first (default)
nugs catalog gaps 1125 --sort date-asc      # Oldest first
nugs catalog gaps 1125 --sort venue         # Group by venue
```

**2. Export Formats**

```bash
# Export to CSV
nugs catalog gaps 1125 --export gaps.csv

# Export with metadata (JSON)
nugs catalog gaps 1125 --export gaps.json --include-metadata

# Create download script
nugs catalog gaps 1125 --export download.sh --format script
# Creates: for id in 123 456 789; do nugs $id; done
```

**3. Gap Statistics**

```bash
nugs catalog gaps 1125 --stats

Output:
Gap Statistics: Billy Strings

  Coverage: 23/430 (5.3%)
  Missing: 407 shows

  By Year:
    2025: 15/82 shows (18.3%)
    2024: 8/150 shows (5.3%)
    2023: 0/120 shows (0%)

  By State:
    CO: 12/45 missing (26.7%)
    CA: 8/30 missing (26.7%)
    NY: 5/25 missing (20%)
```

**4. Smart Recommendations**

```bash
nugs catalog gaps 1125 --recommend

Output:
Recommended Downloads:

  Priority 1 (Recent Red Rocks):
    46385  12/14/25  Red Rocks, Morrison, CO

  Priority 2 (Complete 2024 Run):
    Would complete: Fall 2024 Tour (8/10 shows)
    Missing: 46380, 46375

  Priority 3 (Historical Value):
    First show at venue: 12345  06/15/19  Fillmore, Denver, CO
```

**Benefits**:
- Focused downloading
- Better organization
- Trip-based gap filling
- Completion tracking

---

## Enhancement 4: Cache Compression

**Priority**: Low
**Estimated Effort**: 1 hour

### Description

Compress catalog cache to reduce disk usage (currently 7.5 MB).

### Implementation Plan

**1. Compression Strategy**

Use gzip compression for catalog.json:
```go
func compressCatalog(data []byte) ([]byte, error) {
    var buf bytes.Buffer
    gw := gzip.NewWriter(&buf)
    _, err := gw.Write(data)
    if err != nil {
        return nil, err
    }
    gw.Close()
    return buf.Bytes(), nil
}
```

**2. File Structure**

```
~/.cache/nugs/
├── catalog.json.gz          # Compressed (reduces to ~1.5 MB)
├── catalog_meta.json        # Uncompressed (small)
├── artists_index.json       # Uncompressed (fast access)
└── containers_index.json.gz # Compressed (large file)
```

**3. Backward Compatibility**

- Check for both .json and .json.gz
- Auto-migrate on next update
- Transparent decompression on read

**Benefits**:
- 80% reduction in cache size
- Faster cloud sync (for rclone users)
- Lower disk I/O

---

## Enhancement 5: Advanced Statistics

**Priority**: Medium
**Estimated Effort**: 2 hours

### Description

Provide deeper insights into the catalog and collection.

### New Commands

```bash
# Stats by date range
nugs catalog stats --range 2024-01-01:2024-12-31

Output:
Catalog Statistics (2024):

  Total Shows:    1,250
  Total Artists:  85
  Date Range:     2024-01-01 to 2024-12-31

  Top 10 Artists (by show count):
    Billy Strings: 150 shows
    ...

  Venue Statistics:
    Most shows: Red Rocks (125)
    Most states: CO (250 shows)

  Growth:
    Average: 104 shows/month
    Peak month: July (180 shows)

# Compare your collection to catalog
nugs catalog stats --my-collection

Output:
Collection vs Catalog:

  Your Collection:
    Total shows: 23
    Total artists: 5
    Storage: 45 GB

  Catalog Coverage:
    5.3% of Billy Strings (23/430)
    12% of total catalog (23/13,253)

  Recommendations:
    Complete artist: Goose (17/20 shows, 85%)
    High-value gaps: 15 Red Rocks shows

# Artist-specific stats
nugs catalog stats --artist 1125

Output:
Billy Strings Statistics:

  Total Shows: 430
  First Show: 2016-03-15
  Latest Show: 2025-12-31
  Span: 9 years, 9 months

  Venues:
    Most played: Red Rocks (45 shows)
    States visited: 42
    Cities visited: 175

  By Year:
    2025: 82 shows
    2024: 150 shows
    2023: 120 shows

  Your Coverage: 23/430 (5.3%)
```

**Benefits**:
- Collection insights
- Download planning
- Track catalog growth
- Artist analysis

---

## Enhancement 6: Integration Features

**Priority**: Low
**Estimated Effort**: 2-3 hours

### Description

Integrate catalog with other workflows.

### Implementation Plan

**1. Batch Download**

```bash
# Download all gaps for artist
nugs catalog gaps 1125 --ids-only | xargs -n1 nugs

# Download with parallel processing
nugs catalog gaps 1125 --ids-only | xargs -P 3 -n1 nugs

# Download specific filters
nugs catalog search "Billy Strings" --state CO --ids-only | xargs -n1 nugs
```

**2. Playlist Generation**

```bash
# Create M3U playlist from search
nugs catalog search "Grateful Dead" --year 1977 --playlist dead77.m3u

# Create playlist from gaps (for streaming)
nugs catalog gaps 461 --playlist wishlist.m3u --streaming
```

**3. Export to Other Formats**

```bash
# Export catalog subset to spreadsheet
nugs catalog search "Billy Strings" --export billy.xlsx

# Export for Plex
nugs catalog gaps 1125 --export plex-watchlist.csv --format plex

# Export for Lidarr
nugs catalog search "Phish" --export lidarr-import.json --format lidarr
```

**4. API Mode**

```bash
# Start HTTP API server
nugs catalog serve --port 53010

# Endpoints:
# GET /api/catalog/search?artist=Billy%20Strings
# GET /api/catalog/gaps/:artistId
# GET /api/catalog/stats
# GET /api/catalog/diff?from=2026-02-04&to=2026-02-05
```

**Benefits**:
- Automation
- Integration with media servers
- Batch operations
- Remote access

---

## Technical Improvements

### 1. Performance Optimizations

**Current State**: All operations load full catalog into memory (7.5 MB)

**Improvements**:

a) **Lazy Loading**
   - Load only metadata for listing
   - Load full data on-demand

b) **Streaming JSON Parser**
   - Use json.Decoder for large files
   - Reduce memory footprint

c) **Cached Queries**
   - Cache frequently used searches
   - TTL-based invalidation

### 2. Testing Infrastructure

**Add**:
- Unit tests for search algorithms
- Integration tests for cache operations
- Benchmark tests for large catalogs
- Concurrent access tests

### 3. Error Recovery

**Add**:
- Automatic cache repair
- Corruption detection
- Backup/restore capabilities
- Version migration support

---

## Implementation Priority

### Phase 1 (Next Release - 2-3 weeks)
1. ✅ Historical Snapshots & Diff
2. ✅ Advanced Search
3. ✅ Enhanced Gap Filtering

### Phase 2 (Future - 1-2 months)
4. Advanced Statistics
5. Cache Compression
6. Integration Features

### Phase 3 (When Needed)
7. Performance Optimizations
8. API Mode
9. Testing Infrastructure

---

## Resource Requirements

**Development Time**: ~15-20 hours total
**Testing Time**: ~5 hours
**Documentation**: ~3 hours

**Dependencies**:
- No new external dependencies
- All features use Go standard library
- Optional: gzip for compression

**Disk Space**:
- Snapshots: ~7.5 MB per day (compressed: ~1.5 MB)
- 30-day retention: ~225 MB (compressed: ~45 MB)
- Search indexes: +2 MB

---

## Success Metrics

**Adoption**:
- 50%+ of users enable snapshots
- 75%+ use search vs manual browsing
- 90%+ use gap filtering

**Performance**:
- Search results < 100ms
- Diff computation < 500ms
- Cache updates < 2s

**Quality**:
- Zero data corruption incidents
- 95%+ test coverage
- No critical bugs

---

## Notes

- All features maintain backward compatibility
- Existing cache format unchanged
- Optional features (user can disable)
- JSON output for all new commands
- Follow existing code patterns

## Future Considerations

**Beyond This Plan**:
- SQLite storage for very large catalogs (>100k shows)
- Machine learning recommendations
- Social features (compare collections with friends)
- Price tracking and alerts
- Show quality ratings integration
- Artist tour date predictions

---

**Next Steps**:
1. Get user feedback on priorities
2. Create detailed design docs for Phase 1
3. Set up feature branch
4. Begin implementation

**Questions for User**:
1. Which features are most valuable to you?
2. What's your typical workflow for finding shows?
3. Any features not listed that you'd like?
4. Preferred timeline for Phase 1 delivery?
