# Phase 1A: Code Quality Analysis

## Review Target

Nugs CLI -- a Go-based command-line tool for downloading and managing music from Nugs.net.

**Files Reviewed:** 14 Go source files, 9,150 total lines of code

| File | Lines | Description |
|------|-------|-------------|
| `main.go` | 4,576 | CLI dispatcher, download orchestration, API client, cache I/O |
| `structs.go` | 880 | Data structures, config, API response types |
| `catalog_handlers.go` | 1,072 | Catalog commands (update, cache, stats, latest, gaps, coverage) |
| `format.go` | 935 | Terminal UI formatting, progress bars, box drawing |
| `completions.go` | 386 | Shell completion script generators |
| `catalog_autorefresh.go` | 247 | Auto-refresh logic and configuration |
| `runtime_status.go` | 257 | Runtime status tracking via file-based IPC |
| `crawl_control.go` | 186 | Pause/resume/cancel with hotkey support |
| `tier3_methods.go` | 162 | ProgressBoxState methods (messages, quality names, reset) |
| `filelock.go` | 107 | POSIX file locking for concurrent safety |
| `detach_common.go` | 54 | Read-only command detection, auto-detach logic |
| `progress_box_global.go` | 24 | Global progress box reference with RWMutex |
| `main_test.go` | 229 | Unit tests for core functions |
| `main_alias_test.go` | 35 | Tests for CLI alias normalization |

---

## 1. Code Complexity

### CX-01: main() function is 518 lines with extreme cyclomatic complexity

- **Severity:** Critical
- **File:** `main.go:4059-4576`
- **Description:** The `main()` function spans 518 lines and serves as the application's monolithic entry point. It handles config file discovery, first-run setup, argument parsing, `--json` flag manipulation via `os.Args` rewriting, authentication (multiple paths), command dispatching (20+ commands), URL processing, batch download orchestration, and run summary reporting. The cyclomatic complexity is estimated at 80+, far exceeding the recommended maximum of 10. This makes the function nearly impossible to test, debug, or reason about in isolation.
- **Fix Recommendation:** Decompose `main()` into focused functions with single responsibilities.

```go
// Before (monolithic main):
func main() {
    // 518 lines of interleaved concerns
}

// After (decomposed):
func main() {
    cfg, token, err := bootstrap()
    if err != nil {
        fatal(err)
    }
    os.Exit(run(cfg, token, os.Args[1:]))
}

func bootstrap() (*Config, string, error) {
    setupSessionPersistence()
    scriptDir, err := getScriptDir()
    if err != nil {
        return nil, "", err
    }
    if err := os.Chdir(scriptDir); err != nil {
        return nil, "", err
    }
    // ... config discovery, first-run, authentication
    return cfg, token, nil
}

func run(cfg *Config, token string, args []string) int {
    cmd := parseCommand(args)
    return dispatch(cmd, cfg, token)
}

func dispatch(cmd Command, cfg *Config, token string) int {
    switch cmd.Name {
    case "list":
        return handleList(cmd, cfg, token)
    case "update":
        return handleUpdate(cmd, cfg)
    // ... each command in its own handler
    }
}
```

---

### CX-02: album() function is 168 lines with deep nesting

- **Severity:** High
- **File:** `main.go:2197-2365`
- **Description:** The `album()` function handles metadata fetching, folder creation, duplicate detection, track iteration with progress tracking, rclone upload orchestration, batch state management, and completion summaries. It contains 4-5 levels of nesting in several places and mixes download logic with UI presentation.
- **Fix Recommendation:** Extract track-processing loop, rclone upload, and completion summary into separate functions. The function should coordinate these sub-operations, not implement them.

---

### CX-03: processTrack() function is 186 lines with 7 parameters

- **Severity:** High
- **File:** `main.go:1933-2118`
- **Description:** `processTrack()` accepts 7 parameters and handles format selection, fallback logic (5 format codes with cascading fallbacks), file writing with progress callbacks, decryption, and post-processing. The nested switch within the format fallback loop creates cognitive complexity well above 15.
- **Fix Recommendation:** Group related parameters into a struct and extract format resolution into a dedicated function.

```go
// Before:
func processTrack(folPath string, trackNum, trackTotal int, cfg *Config,
    track *Track, streamParams *StreamParams, progressBox *ProgressBoxState) error

// After:
type TrackDownloadContext struct {
    FolderPath   string
    TrackNum     int
    TrackTotal   int
    Config       *Config
    StreamParams *StreamParams
    ProgressBox  *ProgressBoxState
}

func processTrack(ctx TrackDownloadContext, track *Track) error {
    format, err := resolveFormat(ctx.Config.Format, track)
    if err != nil {
        return err
    }
    return downloadAndDecrypt(ctx, track, format)
}
```

---

### CX-04: video() function is 148 lines with multiple nested conditionals

- **Severity:** High
- **File:** `main.go:3848-3996`
- **Description:** The `video()` function handles metadata fetching, manifest parsing, quality selection, HLS download, chapter processing, TS-to-MP4 conversion, and rclone upload. It has 4 levels of nesting in the manifest selection loop and mixes download, conversion, and upload concerns.
- **Fix Recommendation:** Extract manifest selection, HLS download, chapter processing, and file conversion into separate functions.

---

### CX-05: renderProgressBox() is 190 lines of dense formatting logic

- **Severity:** Medium
- **File:** `format.go:505-695`
- **Description:** The rendering function builds a complex terminal UI with box-drawing characters, multiple progress bars, conditional sections (batch info, ETA, messages), and ANSI color codes. The function handles 10+ conditional display sections, making it difficult to modify any single visual element without risk of breaking others.
- **Fix Recommendation:** Break into smaller rendering functions for each section (header, track info, download progress, upload progress, batch info, messages, footer).

---

### CX-06: catalogCoverage() is 215 lines handling two distinct code paths

- **Severity:** Medium
- **File:** `catalog_handlers.go:749-964`
- **Description:** This function handles both auto-discovery mode (scanning configured paths for all artist directories) and manual mode (specific artist IDs provided as arguments). These are effectively two separate features sharing some output formatting. The combined function has deeply nested loops for directory scanning, ID matching, and coverage calculation.
- **Fix Recommendation:** Split into `catalogCoverageAutoDiscover()` and `catalogCoverageByArtist()`, with a shared `renderCoverageReport()` for output.

---

### CX-07: listArtistShows(), listArtistShowsByVenue(), listArtistLatestShows() are each 170+ lines

- **Severity:** Medium
- **File:** `main.go:2682-2852`, `main.go:2856-3028`, `main.go:3030-3198`
- **Description:** Each of these three functions is 168-172 lines and follows nearly identical structure: paginated API fetching, container collection, date parsing, sorting, and table rendering. The structural duplication dramatically increases the total codebase size and maintenance burden. (See also DUP-01.)
- **Fix Recommendation:** Extract the common pattern into a single parameterized function. See DUP-01 for the detailed recommendation.

---

### CX-08: Args.Description() uses 60+ positional color arguments

- **Severity:** Medium
- **File:** `structs.go:173-263`
- **Description:** The `Description()` method is a single 90-line `fmt.Sprintf` call with 60+ positional `colorReset`/`colorBold`/`colorCyan`/etc. arguments. Any change to the help text requires carefully counting arguments to maintain alignment. A miscount silently produces corrupted terminal output.
- **Fix Recommendation:** Use a template approach or helper function instead of positional arguments.

```go
// Before: 60+ positional args in one Sprintf call
return fmt.Sprintf(`...%s...%s...%s...`, colorBold, colorReset, colorCyan, ...)

// After: use a colorize helper
func colorize(text, color string) string {
    return color + text + colorReset
}

func (Args) Description() string {
    var b strings.Builder
    b.WriteString(colorize("List Commands", colorBold) + "\n")
    b.WriteString("  " + colorize("*", colorGreen) + " " + colorize("list", colorCyan) + "\n")
    // ... much more maintainable
    return b.String()
}
```

---

## 2. Maintainability

### MT-01: Everything lives in package main -- no separation of concerns

- **Severity:** Critical
- **File:** All files
- **Description:** The entire application (9,150 lines) is a single `package main` with no subpackages. API client code, CLI parsing, download engine, configuration management, cache I/O, terminal formatting, file locking, process management, and shell completion generation all share the same namespace. This prevents independent testing, makes import by other tools impossible, and creates a flat namespace where `sanitise()`, `auth()`, `video()`, `album()`, and `handleErr()` all live side-by-side with no organizational hierarchy. Go's package system is designed to provide exactly this kind of separation.
- **Fix Recommendation:** Gradually extract into packages.

```
nugs/
    cmd/nugs/main.go         # CLI entry point only
    internal/
        api/client.go         # HTTP client, auth, API calls
        download/track.go     # Track download, decrypt, format selection
        download/video.go     # Video download, HLS, chapters
        catalog/cache.go      # Catalog caching, file locking
        catalog/handlers.go   # Catalog commands
        config/config.go      # Config read/write/search
        format/progress.go    # Progress bars, box drawing
        format/table.go       # Table rendering
        rclone/upload.go      # Rclone integration
```

---

### MT-02: Global mutable state creates hidden coupling

- **Severity:** High
- **File:** `main.go:63-71`
- **Description:** Four global variables create hidden dependencies across the entire codebase:
  - `jar, _ = cookiejar.New(nil)` -- ignored error at package init
  - `client = &http.Client{Jar: jar}` -- shared mutable HTTP client
  - `loadedConfigPath string` -- tracks which config was loaded
  - `runErrorCount int` / `runWarningCount int` -- global error/warning counters

  Any function can read or modify these at any time without the caller knowing. This prevents concurrent use, makes testing require global state setup/teardown, and creates race conditions if any goroutine touches `runErrorCount`.
- **Fix Recommendation:** Encapsulate in an application context struct passed explicitly.

```go
type App struct {
    client       *http.Client
    configPath   string
    errorCount   int
    warningCount int
}

func NewApp() (*App, error) {
    jar, err := cookiejar.New(nil)
    if err != nil {
        return nil, fmt.Errorf("creating cookie jar: %w", err)
    }
    return &App{
        client: &http.Client{Jar: jar},
    }, nil
}
```

---

### MT-03: Non-idiomatic Go naming conventions

- **Severity:** Medium
- **File:** `main.go` (throughout)
- **Description:** Several identifiers use naming patterns that violate Go conventions:
  - `_url` (line 4531), `_meta` (line 3848), `_panic` (line 147) -- underscore-prefixed parameters suggest unused variables in Go convention
  - `handleErr` with `_panic bool` -- parameter name suggests it should be unused
  - `errString` built from concatenation instead of `fmt.Errorf` wrapping
  - `sanRegexStr` (line 57) -- abbreviation-heavy naming
  - `chapsFileFname` (line 58) -- redundant "F" + "fname" ("file" + "filename")
  - `userAgentTwo` (line 51) -- numeric suffix instead of descriptive name (e.g., `userAgentLegacy`)
- **Fix Recommendation:** Rename to idiomatic Go names: `urlStr`, `existingMeta`, `shouldPanic` or restructure to avoid the boolean parameter pattern entirely.

---

### MT-04: main.go is 4,576 lines -- well beyond any maintainable file size

- **Severity:** High
- **File:** `main.go`
- **Description:** A single file containing 115+ functions and 4,576 lines requires significant scrolling and mental overhead to navigate. Finding a specific function requires searching rather than browsing. Related functions (e.g., all `list*` functions, all `get*Meta` functions) are not grouped in any discoverable way. The file exceeds reasonable limits by an order of magnitude (typical Go files are 200-500 lines).
- **Fix Recommendation:** Even without the full package restructuring of MT-01, the file should be split into logical groupings within `package main`:
  - `api.go` -- HTTP client functions (`auth`, `getUserInfo`, `getSubInfo`, `getAlbumMeta`, etc.)
  - `download.go` -- `album`, `processTrack`, `video`, `playlist`
  - `list.go` -- `listArtistShows`, `listArtistShowsByVenue`, `listArtistLatestShows`, `listAllArtists`
  - `config.go` -- `readConfig`, `writeConfig`, `getScriptDir`
  - `utils.go` -- `sanitise`, `checkUrl`, `parseTimestamps`, `handleErr`

---

### MT-05: ProgressBoxState struct has 30+ fields spanning multiple concerns

- **Severity:** Medium
- **File:** `format.go:402-459`
- **Description:** The `ProgressBoxState` struct combines download tracking (percent, speed, ETA), upload tracking (percent, speed, ETA), batch tracking (album count, total size), UI messages (status, warning, error with priority and expiry), control state (paused, cancelled), and album metadata (title, track name, format) into a single 30+ field struct. This "god struct" violates the Single Responsibility Principle and makes it unclear which fields are relevant for which operations.
- **Fix Recommendation:** Compose from smaller, focused structs.

```go
type DownloadProgress struct {
    Percent  float64
    Speed    string
    ETA      string
    Total    string
    Downloaded string
}

type UploadProgress struct {
    Percent  float64
    Speed    string
    ETA      string
    Total    string
    Uploaded string
}

type ProgressMessage struct {
    Priority int
    Text     string
    Expiry   time.Time
}

type ProgressBoxState struct {
    mu           sync.Mutex
    Download     DownloadProgress
    Upload       UploadProgress
    Message      ProgressMessage
    Batch        *BatchProgressState
    // ... only coordination fields remain here
}
```

---

## 3. Code Duplication

### DUP-01: listArtistShows/listArtistShowsByVenue/listArtistLatestShows share 90% identical code

- **Severity:** High
- **File:** `main.go:2682-3198` (516 lines total, ~430 lines duplicated)
- **Description:** These three functions follow the exact same structure:
  1. Paginated API fetching with `getArtistMeta()` calls
  2. Container collection with date parsing (`PerformanceDateShortYearFirst` fallback to `PerformanceDate`)
  3. Date-based sorting with `time.Parse`
  4. Table rendering with identical column structure

  The only differences are:
  - `listArtistShowsByVenue` adds a venue name filter (5 lines)
  - `listArtistLatestShows` limits the result count (3 lines)
  - `listArtistShows` shows all results

  The `containerWithDate` struct is identically defined in all three functions (lines 2704, 2885, 3054), which is a clear signal of copy-paste duplication.
- **Fix Recommendation:** Unify into a single function with filter options.

```go
type ListShowsOptions struct {
    ArtistID    string
    VenueFilter string    // empty = no filter
    Limit       int       // 0 = no limit
    JSONLevel   string
}

func listArtistShows(opts ListShowsOptions, cfg *Config, streamParams *StreamParams) error {
    allMeta, err := fetchPaginatedArtistMeta(opts.ArtistID, cfg, streamParams)
    if err != nil {
        return err
    }

    containers := collectContainers(allMeta)

    if opts.VenueFilter != "" {
        containers = filterByVenue(containers, opts.VenueFilter)
    }

    sortByDate(containers)

    if opts.Limit > 0 && len(containers) > opts.Limit {
        containers = containers[len(containers)-opts.Limit:]
    }

    return renderShowsTable(containers, opts.JSONLevel)
}
```

---

### DUP-02: HTTP request pattern repeated 8+ times without abstraction

- **Severity:** High
- **File:** `main.go` (throughout)
- **Description:** The same HTTP request-response pattern is repeated across `auth()`, `getUserInfo()`, `getSubInfo()`, `getAlbumMeta()`, `getPlistMeta()`, `getLatestCatalog()`, `getArtistMeta()`, `getArtistList()`, `getPurchasedManUrl()`, and `getStreamMeta()`. Each function independently:
  1. Creates an `http.NewRequest` or uses `client.Get/PostForm`
  2. Sets headers (User-Agent, Authorization)
  3. Checks `resp.StatusCode`
  4. Reads `resp.Body` and calls `json.NewDecoder().Decode()`
  5. Handles errors with slightly different patterns

  There is no shared HTTP middleware, no response validation helper, no centralized error handling.
- **Fix Recommendation:** Create an API client struct with shared request execution.

```go
type NugsClient struct {
    httpClient *http.Client
    baseURL    string
    userAgent  string
    token      string
}

func (c *NugsClient) get(ctx context.Context, path string, result interface{}) error {
    req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
    if err != nil {
        return fmt.Errorf("creating request: %w", err)
    }
    req.Header.Set("User-Agent", c.userAgent)
    if c.token != "" {
        req.Header.Set("Authorization", "Bearer "+c.token)
    }

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("executing request to %s: %w", path, err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("API returned status %d for %s", resp.StatusCode, path)
    }

    if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
        return fmt.Errorf("decoding response from %s: %w", path, err)
    }
    return nil
}
```

---

### DUP-03: Atomic file write pattern duplicated across multiple files

- **Severity:** Medium
- **File:** `main.go`, `runtime_status.go:207-217`, `catalog_handlers.go`
- **Description:** The temp-file-then-rename atomic write pattern is implemented independently in at least three locations:
  - `writeCatalogCache()` in `main.go`
  - `writeFileAtomic()` in `runtime_status.go`
  - Various write operations in `catalog_handlers.go`

  Each has slightly different error handling and cleanup behavior.
- **Fix Recommendation:** Use `runtime_status.go`'s `writeFileAtomic()` as the canonical implementation and call it from all write sites.

---

### DUP-04: JSON marshal-and-print pattern repeated across catalog commands

- **Severity:** Medium
- **File:** `catalog_handlers.go` (throughout)
- **Description:** Nearly every catalog command function contains this identical pattern:
  ```go
  if jsonLevel != "" {
      data := prepareJSONOutput(result, jsonLevel)
      jsonBytes, _ := json.MarshalIndent(data, "", "  ")
      fmt.Println(string(jsonBytes))
      return nil
  }
  ```
  This is repeated in `catalogUpdate`, `catalogCacheStatus`, `catalogStats`, `catalogLatest`, `catalogGaps`, and `catalogCoverage`.
- **Fix Recommendation:** Extract into a helper function.

```go
func outputJSON(data interface{}, jsonLevel string) error {
    prepared := prepareJSONOutput(data, jsonLevel)
    jsonBytes, err := json.MarshalIndent(prepared, "", "  ")
    if err != nil {
        return fmt.Errorf("marshaling JSON: %w", err)
    }
    fmt.Println(string(jsonBytes))
    return nil
}
```

---

### DUP-05: Date parsing with fallback duplicated in 3+ functions

- **Severity:** Low
- **File:** `main.go:2712-2718`, `main.go:2897-2903`, `main.go:3062-3068`
- **Description:** The pattern of preferring `PerformanceDateShortYearFirst` with fallback to `PerformanceDate` is repeated identically in all three `listArtist*` functions:
  ```go
  dateStr := container.PerformanceDateShortYearFirst
  if dateStr == "" {
      dateStr = container.PerformanceDate
  }
  ```
- **Fix Recommendation:** Extract into a method on the container type or a helper function.

```go
func (c *AlbArtResp) DisplayDate() string {
    if c.PerformanceDateShortYearFirst != "" {
        return c.PerformanceDateShortYearFirst
    }
    return c.PerformanceDate
}
```

---

## 4. Clean Code Principles

### CC-01: handleErr() uses boolean parameter to control panic vs. print (anti-pattern)

- **Severity:** High
- **File:** `main.go:147-153`
- **Description:** The `handleErr` function takes a `_panic bool` parameter to decide whether to `panic()` or `fmt.Println()`. Boolean parameters that control function behavior are a well-documented anti-pattern ("flag arguments") because they make call sites unreadable: `handleErr("failed", err, false)` -- what does `false` mean without reading the function? Additionally, mixing `panic` and print in the same function confuses error severity semantics.
- **Fix Recommendation:** Replace with two distinct functions.

```go
// Before:
handleErr("Item failed.", err, false)  // What does false mean?
handleErr("Fatal error.", err, true)   // What does true mean?

// After:
func printErr(msg string, err error) {
    fmt.Fprintf(os.Stderr, "%s: %s\n", msg, err)
    runErrorCount++
}

func fatalErr(msg string, err error) {
    fmt.Fprintf(os.Stderr, "FATAL: %s: %s\n", msg, err)
    os.Exit(1)
}
```

---

### CC-02: os.Args manipulation for --json flag is fragile and side-effect-laden

- **Severity:** High
- **File:** `main.go` (within `main()`)
- **Description:** The `main()` function directly modifies `os.Args` to strip the `--json` flag before passing arguments to the `go-arg` parser. This is done because `--json` is a "meta-flag" that affects output format but is not a standard arg-parser flag. Manipulating `os.Args` creates:
  - Invisible side effects (subsequent code sees different args than the user typed)
  - Ordering dependency (must happen before `arg.MustParse`)
  - Fragility (if `--json` appears as a value to another flag, it gets stripped incorrectly)
- **Fix Recommendation:** Add `--json` as a proper field in the `Args` struct and let `go-arg` handle it natively.

```go
type Args struct {
    Urls        []string `arg:"positional"`
    Format      int      `arg:"-f" default:"-1"`
    JSONLevel   string   `arg:"--json" help:"Output format: minimal, standard, extended, raw"`
    // ...
}
```

---

### CC-03: Magic numbers throughout media type switch

- **Severity:** High
- **File:** `main.go:4536-4553`
- **Description:** The `checkUrl` function returns an integer media type (0-11), which is then used in a switch statement with bare integer literals: `case 0:`, `case 1, 2:`, `case 4, 10:`, etc. The meaning of these numbers is completely opaque without reading the regex list. Any reordering of the regex list silently breaks the switch dispatch.
- **Fix Recommendation:** Define named constants.

```go
const (
    MediaTypeAlbum        = 0
    MediaTypePlaylist     = 1
    MediaTypeLibPlaylist  = 2
    MediaTypeCatalogPlist = 3
    MediaTypeVideo        = 4
    MediaTypeArtist       = 5
    MediaTypeLivestream1  = 6
    MediaTypeLivestream2  = 7
    MediaTypeLivestream3  = 8
    MediaTypePaidLstream  = 9
    MediaTypeVideoAlt     = 10
    MediaTypeRelease      = 11
)

switch mediaType {
case MediaTypeAlbum, MediaTypeRelease:
    itemErr = album(itemId, cfg, streamParams, nil, nil, nil)
case MediaTypePlaylist, MediaTypeLibPlaylist:
    itemErr = playlist(itemId, legacyToken, cfg, streamParams, false)
// ...
}
```

---

### CC-04: Format codes use magic integers (1-5) without named constants

- **Severity:** Medium
- **File:** `main.go` (processTrack, throughout), `tier3_methods.go:80-95`
- **Description:** Audio format codes (1=ALAC, 2=FLAC, 3=MQA, 4=360RA, 5=AAC) and video format codes (1=480p through 5=4K) are used as bare integers throughout the codebase. The `getQualityName()` function in `tier3_methods.go` maps these to human-readable names, but the code that selects and processes formats uses raw numbers.
- **Fix Recommendation:** Define typed constants.

```go
type AudioFormat int

const (
    FormatALAC AudioFormat = 1
    FormatFLAC AudioFormat = 2
    FormatMQA  AudioFormat = 3
    Format360  AudioFormat = 4
    FormatAAC  AudioFormat = 5
)

func (f AudioFormat) String() string {
    switch f {
    case FormatALAC: return "ALAC 16/44.1"
    case FormatFLAC: return "FLAC 16/44.1"
    case FormatMQA:  return "MQA 24/48"
    case Format360:  return "360 Reality Audio"
    case FormatAAC:  return "AAC 150kbps"
    default:         return fmt.Sprintf("Format %d", f)
    }
}
```

---

### CC-05: getPlan() uses reflect.ValueOf for zero-check instead of direct comparison

- **Severity:** Low
- **File:** `main.go:1326-1332`
- **Description:** `getPlan()` uses `reflect.ValueOf(subInfo.Plan).IsZero()` to check if a struct is empty. The `reflect` package is heavyweight (runtime type introspection) and obscures intent. For a simple struct zero-check, direct field comparison or a helper method is clearer and faster.
- **Fix Recommendation:**

```go
// Before:
func getPlan(subInfo *SubInfo) (string, bool) {
    if !reflect.ValueOf(subInfo.Plan).IsZero() {
        return subInfo.Plan.Description, false
    } else {
        return subInfo.Promo.Plan.Description, true
    }
}

// After:
func getPlan(subInfo *SubInfo) (string, bool) {
    if subInfo.Plan.Description != "" {
        return subInfo.Plan.Description, false
    }
    return subInfo.Promo.Plan.Description, true
}
```

---

### CC-06: Commented-out code block (33 lines) left in main.go

- **Severity:** Low
- **File:** `main.go:1808-1840`
- **Description:** A complete 33-line commented-out function (`decryptTrack` old implementation) is left in the source. This is dead code that adds visual noise, confuses readers about which implementation is current, and will never be compiled or tested. Version control (git) already preserves this history.
- **Fix Recommendation:** Delete the commented-out block entirely. It is preserved in git history at commit level.

---

## 5. Technical Debt

### TD-01: API credentials hardcoded in source code

- **Severity:** Critical
- **File:** `main.go:47-48`
- **Description:** The API key (`devKey`) and client ID (`clientId`) are hardcoded as string constants:
  ```go
  devKey   = "x7f54tgbdyc64y656thy47er4"
  clientId = "Eg7HuH873H65r5rt325UytR5429"
  ```
  These are compiled directly into the binary. Anyone can extract them with `strings nugs | grep -i key`. If these credentials are ever rotated, a new binary must be compiled and distributed. If they are confidential, they are now public in the git repository history.
- **Fix Recommendation:** Move to configuration or environment variables. Since these appear to be public API keys (common in mobile app reverse engineering), document them as such. If they are meant to be secret, they should be loaded from config.

```go
// If public API keys (acceptable to hardcode with documentation):
const (
    // devKey is the Nugs.net Android app API key (public, extracted from APK)
    devKey   = "x7f54tgbdyc64y656thy47er4"
    // clientId is the Nugs.net Android app OAuth client ID (public)
    clientId = "Eg7HuH873H65r5rt325UytR5429"
)

// If sensitive (should not be hardcoded):
func getDevKey(cfg *Config) string {
    if key := os.Getenv("NUGS_DEV_KEY"); key != "" {
        return key
    }
    return cfg.DevKey
}
```

---

### TD-02: sanitise() recompiles regex on every call

- **Severity:** High
- **File:** `main.go:1229-1232`
- **Description:** The `sanitise()` function calls `regexp.MustCompile(sanRegexStr)` every time it is invoked. Since `sanRegexStr` is a constant, the compiled regex never changes. Regex compilation is expensive (allocation + NFA construction), and `sanitise()` is called for every file and directory path during downloads -- potentially thousands of times per batch operation.
- **Fix Recommendation:** Pre-compile the regex at package level.

```go
// Before:
func sanitise(filename string) string {
    san := regexp.MustCompile(sanRegexStr).ReplaceAllString(filename, "_")
    return strings.TrimSuffix(san, "\t")
}

// After:
var sanRegex = regexp.MustCompile(sanRegexStr)

func sanitise(filename string) string {
    return strings.TrimSuffix(sanRegex.ReplaceAllString(filename, "_"), "\t")
}
```

---

### TD-03: checkUrl() recompiles 12 regexes on every call

- **Severity:** High
- **File:** `main.go:1359-1368`
- **Description:** The `checkUrl()` function iterates over `regexStrings` (a slice of 12 regex patterns) and calls `regexp.MustCompile()` for each one on every invocation. This means 12 regex compilations per URL check. In batch mode with 50+ URLs, this results in 600+ unnecessary regex compilations.
- **Fix Recommendation:** Pre-compile all regexes at init time.

```go
// Before:
var regexStrings = []string{...}

func checkUrl(_url string) (string, int) {
    for i, regexStr := range regexStrings {
        regex := regexp.MustCompile(regexStr)
        match := regex.FindStringSubmatch(_url)
        // ...
    }
}

// After:
var compiledRegexes []*regexp.Regexp

func init() {
    compiledRegexes = make([]*regexp.Regexp, len(regexStrings))
    for i, pattern := range regexStrings {
        compiledRegexes[i] = regexp.MustCompile(pattern)
    }
}

func checkUrl(urlStr string) (string, int) {
    for i, re := range compiledRegexes {
        match := re.FindStringSubmatch(urlStr)
        if match != nil {
            return match[1], i
        }
    }
    return "", 0
}
```

---

### TD-04: decryptTrack() uses hardcoded filename "temp_enc.ts"

- **Severity:** High
- **File:** `main.go:1847-1861`
- **Description:** The `decryptTrack()` function reads from a hardcoded filename `"temp_enc.ts"` in the current working directory:
  ```go
  encData, err := os.ReadFile("temp_enc.ts")
  ```
  This creates several problems:
  - No ability to run concurrent downloads (file name collision)
  - Depends on current working directory being set correctly
  - Temp file is not cleaned up on error
  - File name is not configurable or unique per track
- **Fix Recommendation:** Accept the path as a parameter and use `os.CreateTemp` for temp files.

```go
func decryptTrack(key, iv []byte, encPath string) ([]byte, error) {
    encData, err := os.ReadFile(encPath)
    if err != nil {
        return nil, fmt.Errorf("reading encrypted data from %s: %w", encPath, err)
    }
    // ... rest of decryption
}
```

---

### TD-05: pkcs5Trimming has no bounds checking -- potential panic on malformed input

- **Severity:** High
- **File:** `main.go:1842-1845`
- **Description:** The `pkcs5Trimming()` function performs PKCS5 padding removal without any validation:
  ```go
  func pkcs5Trimming(data []byte) []byte {
      padding := data[len(data)-1]
      return data[:len(data)-int(padding)]
  }
  ```
  If `data` is empty, this panics with an index-out-of-range error. If `padding` is larger than `len(data)`, this panics with a slice bounds error. If the decrypted data is corrupted, `padding` could be any value 0-255, causing silent data truncation or a panic.
- **Fix Recommendation:** Add bounds validation.

```go
func pkcs5Trimming(data []byte) ([]byte, error) {
    if len(data) == 0 {
        return nil, fmt.Errorf("pkcs5: empty data")
    }
    padding := int(data[len(data)-1])
    if padding == 0 || padding > len(data) || padding > aes.BlockSize {
        return nil, fmt.Errorf("pkcs5: invalid padding value %d", padding)
    }
    // Verify all padding bytes are consistent (PKCS5 requirement)
    for i := len(data) - padding; i < len(data); i++ {
        if data[i] != byte(padding) {
            return nil, fmt.Errorf("pkcs5: inconsistent padding at byte %d", i)
        }
    }
    return data[:len(data)-padding], nil
}
```

---

### TD-06: Config contains plaintext password with no encryption or secure storage

- **Severity:** Medium
- **File:** `structs.go` (Config struct)
- **Description:** The `Config` struct stores `Password` as a plaintext string, which is persisted to `config.json` on disk. While this is a CLI tool (not a web service), storing passwords in plaintext JSON files with default filesystem permissions is a security risk on shared systems.
- **Fix Recommendation:** At minimum, use OS keyring integration (e.g., `zalando/go-keyring`) or warn users about file permissions. The `readConfig()` function already warns about insecure permissions, which is a good start, but the password itself should not be stored in cleartext.

---

### TD-07: No context.Context propagation for cancellation and timeouts

- **Severity:** Medium
- **File:** `main.go` (throughout)
- **Description:** HTTP requests and long-running operations do not use `context.Context` for cancellation or timeout control. The `http.Client` has no timeout set. Downloads of large files have no timeout mechanism. If the Nugs.net API is slow or unresponsive, the application hangs indefinitely. The custom crawl control system (`crawl_control.go`) implements its own pause/cancel mechanism via file-based IPC instead of using Go's built-in context cancellation.
- **Fix Recommendation:** Add `context.Context` as the first parameter to all API and download functions. Set a default timeout on the HTTP client.

```go
client = &http.Client{
    Jar:     jar,
    Timeout: 30 * time.Second, // For API calls
}

func getAlbumMeta(ctx context.Context, albumID string, ...) (*AlbArtResp, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    // ...
}
```

---

## 6. Error Handling

### EH-01: Cookie jar error silently ignored at package init

- **Severity:** High
- **File:** `main.go:64`
- **Description:** The package-level initialization `jar, _ = cookiejar.New(nil)` discards the error from `cookiejar.New()`. While this function currently never returns an error in the standard library (it only returns an error if `Options.PublicSuffixList` validation fails, and `nil` options skip that), the underscore discard is a bad practice because:
  - Future Go versions could add error conditions
  - It sets a precedent for ignoring errors
  - Static analysis tools flag it as a warning
- **Fix Recommendation:**

```go
func init() {
    var err error
    jar, err = cookiejar.New(nil)
    if err != nil {
        panic(fmt.Sprintf("failed to create cookie jar: %v", err))
    }
    client = &http.Client{Jar: jar}
}
```

---

### EH-02: parseTimestamps() silently ignores time.Parse errors

- **Severity:** High
- **File:** `main.go:1334-1339`
- **Description:** The function discards both `time.Parse` errors:
  ```go
  func parseTimestamps(start, end string) (string, string) {
      startTime, _ := time.Parse(layout, start)
      endTime, _ := time.Parse(layout, end)
      parsedStart := strconv.FormatInt(startTime.Unix(), 10)
      parsedEnd := strconv.FormatInt(endTime.Unix(), 10)
      return parsedStart, parsedEnd
  }
  ```
  If the API returns a date in an unexpected format, `time.Parse` returns a zero `time.Time`, and `Unix()` on that returns `-62135596800` (January 1, year 1). This silently produces incorrect timestamps that will cause stream authentication to fail with an opaque error.
- **Fix Recommendation:**

```go
func parseTimestamps(start, end string) (string, string, error) {
    startTime, err := time.Parse(layout, start)
    if err != nil {
        return "", "", fmt.Errorf("parsing start time %q: %w", start, err)
    }
    endTime, err := time.Parse(layout, end)
    if err != nil {
        return "", "", fmt.Errorf("parsing end time %q: %w", end, err)
    }
    return strconv.FormatInt(startTime.Unix(), 10),
           strconv.FormatInt(endTime.Unix(), 10), nil
}
```

---

### EH-03: Error and print mixed in same flow -- double-reporting

- **Severity:** Medium
- **File:** `main.go` (multiple locations)
- **Description:** Several functions both print an error message to stdout AND return an error to the caller. The caller then also prints the error. This results in duplicate error messages that confuse users:
  ```
  Failed to fetch album metadata.
  Error: HTTP 404
  Item failed.
  HTTP 404
  ```
  Functions should either handle the error (print + continue) or propagate it (return to caller), never both.
- **Fix Recommendation:** Adopt a consistent pattern: functions return errors, and only the top-level dispatcher prints them.

---

### EH-04: resp.Body not deferred-closed in several HTTP functions

- **Severity:** Medium
- **File:** `main.go` (API client functions)
- **Description:** Several HTTP response handlers close `resp.Body` after reading, but do not use `defer`. If `json.NewDecoder().Decode()` or `io.ReadAll()` panics or an early return is added during maintenance, the body will leak. The idiomatic Go pattern is:
  ```go
  defer resp.Body.Close()
  ```
  immediately after checking for HTTP errors.
- **Fix Recommendation:** Add `defer resp.Body.Close()` immediately after the `client.Do(req)` success check in all HTTP functions.

---

### EH-05: resolveCatPlistId closes body before checking status code

- **Severity:** Medium
- **File:** `main.go:3998-4004`
- **Description:** The function closes `resp.Body` before checking the status code:
  ```go
  req, err := client.Get(plistUrl)
  // ...
  req.Body.Close()
  if req.StatusCode != http.StatusOK {
  ```
  This means if the status is not OK, the error response body has already been closed and cannot be read for diagnostic information.
- **Fix Recommendation:**

```go
resp, err := client.Get(plistUrl)
if err != nil {
    return "", err
}
defer resp.Body.Close()

if resp.StatusCode != http.StatusOK {
    body, _ := io.ReadAll(resp.Body)
    return "", fmt.Errorf("catalog playlist returned status %d: %s", resp.StatusCode, string(body))
}
```

---

### EH-06: Inconsistent error wrapping -- some use fmt.Errorf with %w, most do not

- **Severity:** Medium
- **File:** `main.go`, `catalog_handlers.go`, `filelock.go`
- **Description:** Error wrapping with `fmt.Errorf("context: %w", err)` is used inconsistently. Most functions use bare `err` returns or string concatenation. This prevents callers from using `errors.Is()` or `errors.As()` to make decisions based on error types. For example, distinguishing "network error" from "authentication error" from "file system error" is impossible at the call site.
- **Fix Recommendation:** Wrap all returned errors with context using `%w` verb consistently.

```go
// Before:
return nil, err

// After:
return nil, fmt.Errorf("fetching album %s: %w", albumID, err)
```

---

### EH-07: No error returned from checkUrl when no regex matches

- **Severity:** Low
- **File:** `main.go:1359-1368`
- **Description:** When `checkUrl()` finds no matching regex, it returns `("", 0)`. The empty string signals "no match," but the `0` return value is also a valid media type (MediaTypeAlbum). Callers must check the empty string, not the integer, to detect failure. This is error-prone.
- **Fix Recommendation:** Return `-1` or use a named constant for "no match," or better yet, return an error.

```go
func checkUrl(urlStr string) (string, int, error) {
    for i, re := range compiledRegexes {
        match := re.FindStringSubmatch(urlStr)
        if match != nil {
            return match[1], i, nil
        }
    }
    return "", -1, fmt.Errorf("URL does not match any known pattern: %s", urlStr)
}
```

---

## Summary

| Category | Critical | High | Medium | Low | Total |
|----------|----------|------|--------|-----|-------|
| Code Complexity | 1 | 4 | 3 | 0 | **8** |
| Maintainability | 1 | 2 | 2 | 0 | **5** |
| Code Duplication | 0 | 2 | 2 | 1 | **5** |
| Clean Code | 0 | 2 | 1 | 3 | **6** |
| Technical Debt | 1 | 4 | 2 | 0 | **7** |
| Error Handling | 0 | 2 | 4 | 1 | **7** |
| **Totals** | **3** | **16** | **14** | **5** | **38** |

### Critical Issues (Must Address)

1. **CX-01:** `main()` is 518 lines with ~80+ cyclomatic complexity
2. **MT-01:** Entire 9,150-line codebase is one `package main` with no separation
3. **TD-01:** API credentials hardcoded in source code

### Top 5 High-Priority Improvements (Highest Impact)

1. **DUP-01 + CX-07:** Unify the three `listArtist*` functions (saves ~340 lines, eliminates triple-defined struct)
2. **DUP-02:** Create shared API client to eliminate 8+ duplicated HTTP patterns
3. **TD-02 + TD-03:** Pre-compile all regexes (12+ regex compilations per URL check, sanitise called thousands of times)
4. **TD-05:** Add bounds checking to `pkcs5Trimming` (potential panic on malformed data)
5. **CC-01:** Replace `handleErr` boolean dispatch with explicit `printErr`/`fatalErr` functions

### Cross-Cutting Concerns for Phase 2

- **TD-01** (hardcoded credentials) should be evaluated in the Security phase
- **TD-05** (pkcs5Trimming panic) is both a code quality issue and a potential crash vulnerability
- **TD-07** (no context propagation) impacts both performance (timeouts) and reliability (cancellation)
- **EH-02** (silent timestamp parsing failures) could cause authentication failures in production
