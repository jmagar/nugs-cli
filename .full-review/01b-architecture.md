# Phase 1B: Architecture & Design Review

## Review Target

**Nugs CLI** -- A Go-based command-line tool for downloading and managing music from Nugs.net.

**Review Date:** 2026-02-06

**Files Analyzed:**
- `main.go` (~4500 lines)
- `structs.go` (~881 lines)
- `catalog_handlers.go` (~1073 lines)
- `catalog_autorefresh.go` (~248 lines)
- `filelock.go` (~107 lines)
- `completions.go` (~387 lines)
- `format.go` (~936 lines)
- `crawl_control.go` (~187 lines)
- `runtime_status.go` (~258 lines)
- `progress_box_global.go` (~25 lines)
- `tier3_methods.go` (~163 lines)
- `detach_common.go` (~55 lines)
- `detach_unix.go` (~55 lines)
- `cancel_unix.go`, `process_alive_unix.go`, `hotkey_input_unix.go`, `signal_persistence_unix.go`
- `main_test.go` (~230 lines), `main_alias_test.go` (~36 lines), `catalog_handlers_test.go` (~191 lines)
- `go.mod`

---

## 1. Component Boundaries & Separation of Concerns

### Finding 1.1: Monolithic main.go -- God File Anti-Pattern

**Severity:** Critical
**Architectural Impact:** The single largest structural problem in the codebase. `main.go` is approximately 4,500 lines and contains at least 8 distinct responsibilities: CLI argument parsing and command dispatch, API client (authentication, metadata fetching), download orchestration (audio, video, batch), cache I/O (read/write catalog JSON files), list commands (artists, shows, venues, latest), URL parsing and validation, configuration management, and rclone upload integration. This makes the file nearly impossible to navigate, understand as a unit, or safely modify without unintended side effects.

**Improvement Recommendation:**
Extract `main.go` into domain-oriented packages or, at minimum, separate files within `package main`:

```
nugs/
  main.go            (~200 lines)  -- Entry point and command dispatch only
  api_client.go      (~400 lines)  -- HTTP client, auth, API request functions
  download.go        (~600 lines)  -- Download orchestration (audio, video, batch)
  cache.go           (~300 lines)  -- Catalog cache read/write/index operations
  list_commands.go   (~500 lines)  -- All list/display commands
  url_parser.go      (~200 lines)  -- URL parsing and validation
  config.go          (~200 lines)  -- Configuration reading/writing
  rclone.go          (~300 lines)  -- Rclone upload integration
```

For a more robust architecture, introduce sub-packages:

```
nugs/
  cmd/               -- CLI dispatch
  internal/
    api/             -- API client and data types
    download/        -- Download engine
    catalog/         -- Cache management
    rclone/          -- Upload integration
    ui/              -- Terminal formatting and progress
```

---

### Finding 1.2: Everything in `package main` -- No Sub-Packages

**Severity:** Critical
**Architectural Impact:** The entire application lives in `package main` with zero sub-packages. This eliminates Go's package-level encapsulation -- every function, type, and variable is accessible to every other function. There is no enforced boundary between the API client, the download engine, the catalog system, or the UI layer. This also prevents any form of reuse; no other Go program can import Nugs CLI functionality as a library.

**Improvement Recommendation:**
Adopt an `internal/` package structure. At minimum:

```go
// internal/api/client.go
package api

type Client struct {
    httpClient *http.Client
    devKey     string
    clientID   string
    token      string
}

func NewClient(token string) *Client { ... }
func (c *Client) GetAlbumMeta(id int) (*AlbumMetadata, error) { ... }
func (c *Client) Auth(email, password string) (string, error) { ... }
```

This provides clear API boundaries, makes dependencies explicit through imports, and enables independent testing of each package.

---

### Finding 1.3: UI/Formatting Tightly Coupled to Business Logic

**Severity:** High
**Architectural Impact:** Functions in `main.go` and `catalog_handlers.go` intermix business logic with terminal output formatting. For example, `catalogStats()` computes statistics and directly prints ANSI-colored tables. `catalogGaps()` performs gap analysis while simultaneously rendering colored output. This makes it impossible to use the business logic in a non-terminal context (e.g., a web API, a CI pipeline, or programmatic access).

**Examples:**
- `catalogStats()` calls `printHeader()`, `printTableRow()`, and `fmt.Printf()` with ANSI codes directly within the statistics computation flow.
- `catalogLatest()` mixes API data fetching with `colorGreen`, `colorCyan`, `symbolCheck` formatting.
- `listArtists()` and `listArtistShows()` in `main.go` contain hundreds of lines of terminal formatting alongside data retrieval.

**Improvement Recommendation:**
Separate computation from presentation. Each command handler should return a data structure, and a separate rendering layer should handle presentation:

```go
// Business logic returns data
func analyzeGaps(artistID int, catalog []ShowMeta, config *Config) (*GapAnalysis, error) {
    // Pure computation, no printing
    return &GapAnalysis{
        TotalShows:      total,
        DownloadedShows: downloaded,
        MissingShows:    missing,
    }, nil
}

// Rendering layer handles display
func renderGapAnalysis(analysis *GapAnalysis, format string) {
    switch format {
    case "json":
        json.NewEncoder(os.Stdout).Encode(analysis)
    case "table":
        printGapTable(analysis)
    }
}
```

---

### Finding 1.4: Platform Abstraction is Well-Structured

**Severity:** Low (Positive Finding)
**Architectural Impact:** The codebase correctly uses Go build tags to separate platform-specific code:

- `cancel_unix.go` / `cancel_windows.go`
- `detach_unix.go` / `detach_windows.go`
- `hotkey_input_unix.go` / `hotkey_input_windows.go`
- `process_alive_unix.go` / `process_alive_windows.go`
- `signal_persistence_unix.go` / `signal_persistence_windows.go`

Each file properly uses `//go:build` constraints and provides platform-specific implementations of shared function signatures. `detach_common.go` contains shared logic without build tags. This is idiomatic Go and well-executed.

**Note:** `filelock.go` uses `syscall.Flock` which is Unix-only but does not have a build tag. It relies on `golang.org/x/sys` being available. A Windows-compatible alternative (or a build-tagged stub) would be needed for full cross-platform support.

---

## 2. Dependency Management

### Finding 2.1: Global Mutable State -- Shared HTTP Client and Credentials

**Severity:** Critical
**Architectural Impact:** The application uses package-level global variables for core dependencies:

```go
var jar, _ = cookiejar.New(nil)
var client = &http.Client{Jar: jar}
var loadedConfigPath string
var runErrorCount int
```

Multiple functions across the codebase directly reference these globals. This creates invisible coupling: any function can modify the cookie jar or increment `runErrorCount`, and the effects propagate unpredictably. It also makes unit testing effectively impossible -- there is no way to inject a mock HTTP client, no way to isolate test scenarios, and no way to run tests in parallel.

The ignored error from `cookiejar.New(nil)` (which cannot actually fail with nil options) is a minor issue, but the pattern of ignoring errors in global initialization is concerning as a habit.

**Improvement Recommendation:**
Introduce an application context struct that holds all dependencies:

```go
type App struct {
    client     *http.Client
    config     *Config
    configPath string
    errCount   int
}

func NewApp(cfg *Config, configPath string) *App {
    jar, _ := cookiejar.New(nil)
    return &App{
        client:     &http.Client{Jar: jar},
        config:     cfg,
        configPath: configPath,
    }
}

// All methods become receivers on App
func (a *App) downloadTrack(track Track, dir string) error { ... }
func (a *App) auth(email, password string) (string, error) { ... }
```

This makes dependencies explicit, enables testing with mock clients, and eliminates hidden global state.

---

### Finding 2.2: No Dependency Injection or Interfaces

**Severity:** High
**Architectural Impact:** The codebase defines zero interfaces. All external interactions (HTTP requests, filesystem operations, process execution, terminal I/O) are performed through concrete implementations. Functions directly call `http.Get()`, `os.ReadFile()`, `exec.Command()`, and `fmt.Printf()` with no abstraction layer.

This means:
- **Untestable:** There is no way to mock the Nugs.net API, the filesystem, rclone, or FFmpeg for testing.
- **Inflexible:** Switching from `http.Client` to a rate-limited client, or from local filesystem to a different storage backend, requires modifying every call site.
- **No seams:** There are no natural points where behavior can be substituted.

**Improvement Recommendation:**
Define interfaces at the consumer side (Go idiom) for key boundaries:

```go
// At the boundary where API calls are made
type CatalogFetcher interface {
    FetchLatestCatalog() (*LatestCatalogResp, error)
}

// At the boundary where files are stored
type CacheStore interface {
    ReadCatalog() ([]ShowMeta, error)
    WriteCatalog(shows []ShowMeta, duration time.Duration) error
}

// At the boundary where rclone is invoked
type Uploader interface {
    Upload(localPath, remotePath string) error
    PathExists(remotePath string) (bool, error)
}
```

---

### Finding 2.3: Hard-Coded API Credentials in Source Code

**Severity:** High
**Architectural Impact:** API credentials are hard-coded as constants:

```go
devKey   = "x7f54tgbdyc64y656thy47er4"
clientId = "Eg7HuH873H65r5rt325UytR5429"
```

While these appear to be application-level API keys (not user credentials), embedding them in source code means they are visible in the Git history, cannot be rotated without a code change, and cannot differ between environments (development vs production).

**Improvement Recommendation:**
Move API keys to configuration or environment variables:

```go
func getDevKey(cfg *Config) string {
    if key := os.Getenv("NUGS_DEV_KEY"); key != "" {
        return key
    }
    if cfg.DevKey != "" {
        return cfg.DevKey
    }
    return defaultDevKey // Fallback constant
}
```

---

### Finding 2.4: Minimal External Dependencies

**Severity:** Low (Positive Finding)
**Architectural Impact:** The `go.mod` file shows remarkably few dependencies:

```
require (
    github.com/alexflint/go-arg v1.5.1
    github.com/dustin/go-humanize v1.0.1
    github.com/grafov/m3u8 v0.12.1
    golang.org/x/sys v0.29.0
    golang.org/x/term v0.28.0
)
```

This is a strong architectural decision. Minimal dependencies reduce supply chain risk, simplify builds, and avoid dependency hell. The chosen dependencies are well-maintained and focused: argument parsing, human-readable formatting, M3U8 playlist parsing, and platform syscall access.

---

## 3. API Design

### Finding 3.1: CLI Command Dispatch is a Massive If/Else Chain

**Severity:** High
**Architectural Impact:** The `main()` function uses a ~400-line if/else/switch chain to dispatch commands. There is no command registry, no command pattern, and no subcommand framework. Adding a new command requires modifying the monolithic `main()` function, understanding the existing dispatch flow to find the right insertion point, and potentially breaking existing commands if the insertion order is wrong.

**Example of the pattern:**
```go
if args.Catalog != "" {
    switch args.Catalog {
    case "update":
        catalogUpdate(jsonLevel)
    case "cache":
        catalogCacheStatus(jsonLevel)
    // ... more cases
    }
} else if args.List != nil {
    if args.List.Artist != 0 {
        // ... nested dispatch
    }
} else if /* ... more conditions */ {
}
```

**Improvement Recommendation:**
Implement a command registry pattern:

```go
type Command struct {
    Name    string
    Aliases []string
    Handler func(args *Args, cfg *Config) error
}

var commands = []Command{
    {Name: "catalog", Handler: catalogCommand},
    {Name: "list", Handler: listCommand},
    {Name: "status", Handler: statusCommand},
    {Name: "cancel", Handler: cancelCommand},
    {Name: "completion", Handler: completionCommand},
    {Name: "help", Handler: helpCommand},
}

func dispatch(args *Args, cfg *Config) error {
    for _, cmd := range commands {
        if matches(cmd, args) {
            return cmd.Handler(args, cfg)
        }
    }
    return handleDownload(args, cfg) // Default action
}
```

---

### Finding 3.2: No Structured Error Returns from Commands

**Severity:** Medium
**Architectural Impact:** Command handlers use inconsistent error handling patterns. Some return errors, some call `log.Fatal()` or `os.Exit()` directly, and some print errors and continue. This inconsistency means the caller cannot reliably determine whether a command succeeded or failed, and error recovery is impossible.

**Examples:**
- `catalogUpdate()` returns an `error` (good pattern).
- Some download functions call `handleErr()` which does `log.Fatal(err)`, terminating the entire process.
- Some list functions print error messages with `fmt.Println()` and return without an error value.

**Improvement Recommendation:**
Standardize all command handlers to return `error`. Centralize error presentation in `main()`:

```go
func main() {
    if err := run(os.Args[1:]); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}

func run(args []string) error {
    // Parse args, dispatch commands
    // All handlers return error
}
```

---

### Finding 3.3: JSON Output is an Afterthought, Not a First-Class API

**Severity:** Medium
**Architectural Impact:** The `--json` flag is supported for catalog commands with multiple verbosity levels (`minimal`, `standard`, `extended`, `raw`). However, the JSON output is constructed ad-hoc in each handler by building `map[string]any` values rather than using typed structs. This means JSON output is:
- Not type-checked at compile time.
- Inconsistent across commands (different key names, different structures).
- Fragile -- adding a field in one command does not propagate to others.

**Improvement Recommendation:**
Define response types for JSON output:

```go
type CatalogStatsResponse struct {
    TotalShows    int       `json:"total_shows"`
    TotalArtists  int       `json:"total_artists"`
    CacheAge      string    `json:"cache_age"`
    LastUpdated   time.Time `json:"last_updated"`
}

func catalogStats(jsonLevel string) error {
    stats := computeStats(catalog)
    if jsonLevel != "" {
        return json.NewEncoder(os.Stdout).Encode(stats)
    }
    renderStatsTable(stats)
    return nil
}
```

---

## 4. Data Model

### Finding 4.1: Struct Duplication -- `containerWithDate` Defined Three Times

**Severity:** High
**Architectural Impact:** The `containerWithDate` struct is defined as a local type inside three separate functions in `main.go`:

```go
// In listArtistShows()
type containerWithDate struct {
    ContainerID int
    Date        string
    // ... fields
}

// In listArtistLatestShows()
type containerWithDate struct {
    ContainerID int
    Date        string
    // ... fields
}

// In a third location
type containerWithDate struct { ... }
```

Each definition has slightly different fields, creating a maintenance trap where a bug fix in one is not reflected in the others. This is a direct violation of DRY (Don't Repeat Yourself) and indicates that these functions evolved through copy-paste rather than through abstraction.

**Improvement Recommendation:**
Define a single `ShowWithDate` type in `structs.go`:

```go
type ShowWithDate struct {
    ContainerID int
    Date        string
    Title       string
    ArtistName  string
    VenueName   string
    // All needed fields
}
```

---

### Finding 4.2: `ProgressBoxState` is Overly Large and Multi-Responsibility

**Severity:** High
**Architectural Impact:** The `ProgressBoxState` struct in `format.go` has 30+ fields covering download progress, upload progress, batch state, speed tracking, message display, pause/cancel state, and completion tracking. It also has a mutex for thread safety, making it effectively a concurrent state machine.

This violates the Single Responsibility Principle: a single struct manages download state, upload state, UI messaging, batch context, and user interaction state. Changes to any one concern risk breaking the others.

**Improvement Recommendation:**
Decompose into focused structs:

```go
type DownloadProgress struct {
    Percent      float64
    Speed        string
    Downloaded   string
    Total        string
    ETA          string
    SpeedHistory []float64
}

type UploadProgress struct {
    Percent      float64
    Speed        string
    Uploaded     string
    Total        string
    ETA          string
    SpeedHistory []float64
}

type BatchContext struct {
    AlbumCurrent int
    AlbumTotal   int
    StartTime    time.Time
}

type ProgressBox struct {
    mu       sync.Mutex
    download DownloadProgress
    upload   UploadProgress
    batch    BatchContext
    message  MessageState
    control  ControlState
    track    TrackInfo
}
```

---

### Finding 4.3: `Config` Struct Mixes Concerns

**Severity:** Medium
**Architectural Impact:** The `Config` struct in `structs.go` combines authentication credentials, download preferences, rclone configuration, catalog settings, and UI preferences in a single flat structure. This means every function that receives a `Config` has access to credentials even when it only needs download format settings.

**Improvement Recommendation:**
Group config fields into logical sub-structs:

```go
type Config struct {
    Auth     AuthConfig     `json:"auth"`
    Download DownloadConfig `json:"download"`
    Rclone   RcloneConfig   `json:"rclone"`
    Catalog  CatalogConfig  `json:"catalog"`
    UI       UIConfig       `json:"ui"`
}

type AuthConfig struct {
    Email    string `json:"email"`
    Password string `json:"password"`
    Token    string `json:"token"`
}

type DownloadConfig struct {
    Format int    `json:"format"`
    OutPath string `json:"outPath"`
    // ...
}
```

Note: This would be a breaking change to `config.json` format and would need migration support.

---

### Finding 4.4: Extensive Use of `any` Type in API Response Structs

**Severity:** Medium
**Architectural Impact:** Several API response structs use `any` (Go's type alias for `interface{}`) for fields that have known shapes:

```go
type AlbArtResp struct {
    // ...
    Tracks     any `json:"tracks"`
    Categories any `json:"categories"`
    // ...
}
```

This defeats Go's static type system and pushes type errors from compile time to runtime. Every access to these fields requires type assertions, which can panic if the assertion is wrong.

**Improvement Recommendation:**
Define concrete types for known API response shapes. For fields with genuinely variable structure, use `json.RawMessage` to defer parsing:

```go
type AlbArtResp struct {
    Tracks     []Track          `json:"tracks"`
    Categories []Category       `json:"categories"`
    RawExtra   json.RawMessage  `json:"extra,omitempty"` // Truly variable
}
```

---

## 5. Design Patterns

### Finding 5.1: No Use of the Repository Pattern for Data Access

**Severity:** Medium
**Architectural Impact:** Cache operations (read/write catalog, read/write indexes, read/write config) are implemented as standalone functions scattered across `main.go` and `catalog_handlers.go`. Functions like `readCatalogCache()`, `writeCatalogCache()`, `readConfig()`, and `writeConfig()` each independently handle file paths, JSON marshaling, and error handling.

This scattering means:
- Cache path logic is duplicated across multiple functions.
- Error handling for I/O operations is inconsistent.
- There is no single place to add cross-cutting concerns like logging, metrics, or retry logic.

**Improvement Recommendation:**
Introduce a repository pattern for data access:

```go
type CatalogRepository struct {
    cacheDir string
    lockPath string
}

func NewCatalogRepository(cacheDir string) *CatalogRepository {
    return &CatalogRepository{
        cacheDir: cacheDir,
        lockPath: filepath.Join(cacheDir, ".catalog.lock"),
    }
}

func (r *CatalogRepository) ReadCatalog() ([]ShowMeta, error) { ... }
func (r *CatalogRepository) WriteCatalog(shows []ShowMeta) error { ... }
func (r *CatalogRepository) ReadIndex(name string) (map[string][]int, error) { ... }
func (r *CatalogRepository) ReadMeta() (*CacheMeta, error) { ... }
```

---

### Finding 5.2: Sorting Logic Repeated Across Multiple Functions

**Severity:** Medium
**Architectural Impact:** Show sorting by date is implemented in at least 4 separate locations with nearly identical code:

```go
// Pattern repeated in listArtistShows, listArtistLatestShows, catalogLatest, catalogGaps
sort.Slice(shows, func(i, j int) bool {
    ti, _ := time.Parse("2006-01-02", shows[i].Date)
    tj, _ := time.Parse("2006-01-02", shows[j].Date)
    return ti.After(tj)
})
```

Each occurrence parses dates from strings, ignores parse errors, and sorts descending. This is a classic DRY violation.

**Improvement Recommendation:**
Extract a reusable sort function:

```go
func sortShowsByDateDesc(shows []ShowMeta) {
    sort.Slice(shows, func(i, j int) bool {
        ti, _ := time.Parse("2006-01-02", shows[i].Date)
        tj, _ := time.Parse("2006-01-02", shows[j].Date)
        return ti.After(tj)
    })
}
```

Better yet, store parsed dates in the struct to avoid repeated parsing.

---

### Finding 5.3: Regex Recompilation on Every Function Call

**Severity:** Medium
**Architectural Impact:** The `sanitise()` function, which is called for every file and directory name during downloads, compiles a regex on every invocation:

```go
func sanitise(filename string) string {
    return regexp.MustCompile(sanRegexStr).ReplaceAllString(filename, "_")
}
```

Similarly, `checkUrl()` compiles multiple regexes inside a loop that processes every URL. Regex compilation is expensive and allocates memory. In a batch download of an artist's entire catalog (hundreds of albums, thousands of tracks), this results in thousands of unnecessary compilations.

**Improvement Recommendation:**
Compile regexes once at package level:

```go
var sanRegex = regexp.MustCompile(sanRegexStr)

func sanitise(filename string) string {
    return sanRegex.ReplaceAllString(filename, "_")
}
```

---

### Finding 5.4: No Retry or Resilience Patterns for Network Calls

**Severity:** Medium
**Architectural Impact:** All HTTP requests to the Nugs.net API are single-attempt with no retry logic, no exponential backoff, and no circuit breaker pattern. Network failures, rate limiting (HTTP 429), or transient server errors (HTTP 503) result in immediate failure. For a tool that performs long-running batch downloads (potentially hours), a single network hiccup can abort the entire operation.

**Improvement Recommendation:**
Implement a retry wrapper with exponential backoff:

```go
func (c *Client) doWithRetry(req *http.Request, maxRetries int) (*http.Response, error) {
    var lastErr error
    for attempt := 0; attempt <= maxRetries; attempt++ {
        resp, err := c.httpClient.Do(req)
        if err == nil && resp.StatusCode < 500 && resp.StatusCode != 429 {
            return resp, nil
        }
        if err != nil {
            lastErr = err
        } else {
            lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
            resp.Body.Close()
        }
        if attempt < maxRetries {
            backoff := time.Duration(1<<uint(attempt)) * time.Second
            time.Sleep(backoff)
        }
    }
    return nil, fmt.Errorf("after %d retries: %w", maxRetries, lastErr)
}
```

---

## 6. Architectural Consistency

### Finding 6.1: Inconsistent Error Handling Strategies

**Severity:** High
**Architectural Impact:** The codebase uses at least four different error handling strategies:

1. **Return error:** `catalogUpdate()` returns `error` to the caller.
2. **Fatal exit:** `handleErr()` calls `log.Fatal(err)`, immediately terminating the process.
3. **Print and continue:** Some functions print an error message with `fmt.Println()` and return without propagating the error.
4. **Ignore:** Some error returns are silently discarded (e.g., `json.Unmarshal` errors in certain paths).

This inconsistency means callers cannot predict whether calling a function will return an error, terminate the process, or silently fail. It makes error recovery impossible at higher levels and complicates debugging.

**Improvement Recommendation:**
Adopt a single error handling convention:
- All functions return `error` (never call `os.Exit` or `log.Fatal` except in `main()`).
- Use `fmt.Errorf("context: %w", err)` to wrap errors with context.
- Handle error presentation in one place (the `main()` function or a top-level error handler).

---

### Finding 6.2: File Locking is Unix-Only Without Build Tags

**Severity:** Medium
**Architectural Impact:** `filelock.go` uses `syscall.Flock()` which is a POSIX-only API. However, the file does not have a `//go:build` tag restricting it to Unix platforms. On Windows, this would fail to compile. The application has Windows-specific files for other features (`detach_windows.go`, `cancel_windows.go`), but there is no `filelock_windows.go` providing an alternative implementation.

**Improvement Recommendation:**
Either add `//go:build unix` to `filelock.go` and create a `filelock_windows.go` with `LockFileEx`/`UnlockFileEx` equivalents, or use a cross-platform file locking library.

---

### Finding 6.3: Atomic File Writes are Inconsistently Applied

**Severity:** Medium
**Architectural Impact:** The catalog cache write operations use atomic file writes (write to temp file, then `os.Rename`), which is the correct pattern for preventing corruption from concurrent reads or process crashes. However, not all file write operations follow this pattern. `writeConfig()` and some runtime status writes use direct `os.WriteFile()`, which can produce partially-written files on crash.

**Improvement Recommendation:**
Create a shared utility function for atomic file writes and use it consistently:

```go
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
    tmp := path + ".tmp"
    if err := os.WriteFile(tmp, data, perm); err != nil {
        return fmt.Errorf("write temp file: %w", err)
    }
    if err := os.Rename(tmp, path); err != nil {
        os.Remove(tmp) // Clean up on rename failure
        return fmt.Errorf("rename: %w", err)
    }
    return nil
}
```

Note: `runtime_status.go` already has a `writeFileAtomic()` helper, but it is not used consistently across the codebase.

---

### Finding 6.4: Progress Box Uses Both Internal Mutex and External Global Mutex

**Severity:** Medium
**Architectural Impact:** `ProgressBoxState` has its own `sync.Mutex` field (`mu`) for thread-safe field access. Separately, `progress_box_global.go` wraps the global `currentProgressBox` pointer in a `sync.RWMutex`. Some methods on `ProgressBoxState` document that the caller must hold `s.mu` (e.g., `GetDisplayMessage` says "REQUIRES: Caller must hold s.mu lock"), while `SetMessage()` acquires the lock internally.

This dual-locking strategy creates potential for:
- **Deadlocks:** If a caller holds the global RWMutex and then a method acquires `s.mu` (or vice versa), lock ordering violations can cause deadlocks.
- **Confusion:** Developers must understand two different locking protocols to safely modify progress state.

**Improvement Recommendation:**
Choose one locking strategy and document it clearly. Either the struct manages its own synchronization internally (all methods acquire/release the lock), or the caller manages synchronization externally (no lock inside the struct). Do not mix both.

---

## Summary of Findings by Severity

### Critical (2 findings)

| ID | Finding | Impact |
|----|---------|--------|
| 1.1 | Monolithic main.go (~4500 lines, 8+ responsibilities) | Unmaintainable, unsafe to modify, cannot onboard new contributors |
| 2.1 | Global mutable state (HTTP client, config path, error counter) | Untestable, invisible coupling, no isolation |

### High (5 findings)

| ID | Finding | Impact |
|----|---------|--------|
| 1.2 | Everything in `package main`, no sub-packages | No encapsulation, no reusability, no import boundaries |
| 1.3 | UI/formatting tightly coupled to business logic | Cannot use logic outside terminal context |
| 2.2 | No interfaces or dependency injection | Testing impossible without real network/filesystem |
| 2.3 | Hard-coded API credentials in source code | Cannot rotate keys, visible in Git history |
| 4.1 | `containerWithDate` struct defined 3 times | Maintenance trap, DRY violation |
| 4.2 | `ProgressBoxState` has 30+ fields, 5+ responsibilities | SRP violation, fragile state machine |
| 6.1 | Four different error handling strategies | Unpredictable behavior, impossible error recovery |

### Medium (8 findings)

| ID | Finding | Impact |
|----|---------|--------|
| 3.1 | CLI dispatch is ~400-line if/else chain | Hard to extend, error-prone insertion |
| 3.2 | No structured error returns from commands | Inconsistent success/failure signaling |
| 3.3 | JSON output built ad-hoc with `map[string]any` | Not type-safe, inconsistent across commands |
| 4.3 | `Config` struct mixes authentication, download, rclone, catalog concerns | Over-exposed credentials |
| 4.4 | Extensive use of `any` type in API response structs | Runtime type errors instead of compile-time |
| 5.2 | Sorting logic duplicated in 4+ locations | DRY violation, inconsistent bug fixes |
| 5.3 | Regex compiled on every call to `sanitise()` and `checkUrl()` | Unnecessary CPU and memory allocation |
| 5.4 | No retry/resilience patterns for HTTP calls | Fragile in face of network issues |
| 6.2 | File locking Unix-only without build tags | Windows build will fail |
| 6.3 | Atomic file writes inconsistently applied | Potential file corruption on crash |
| 6.4 | Dual locking strategy (internal mutex + external global mutex) | Deadlock risk, developer confusion |

### Low (2 findings)

| ID | Finding | Impact |
|----|---------|--------|
| 1.4 | Platform-specific code well-structured with build tags (positive) | Good practice, maintain it |
| 2.4 | Minimal external dependencies (positive) | Low supply chain risk |

---

## Critical Issues for Phase 2 Context

The following findings from this architectural review should inform the Security and Performance phases:

1. **Security:** Hard-coded API credentials (Finding 2.3) should be evaluated for exposure risk and credential rotation feasibility.
2. **Security:** Global HTTP client with shared cookie jar (Finding 2.1) could lead to credential leakage across requests if sessions are not properly isolated.
3. **Security:** Inconsistent error handling (Finding 6.1) may lead to sensitive information disclosure in error messages (e.g., tokens, file paths, API responses printed to terminal).
4. **Performance:** Regex recompilation on every call (Finding 5.3) is a direct performance issue, especially during batch downloads with thousands of file operations.
5. **Performance:** No retry logic (Finding 5.4) means batch downloads are fragile; a single transient error can abort hours of work.
6. **Performance:** `ProgressBoxState` with 30+ fields and mutex (Finding 4.2) may cause lock contention under heavy concurrent update scenarios.
7. **Testing:** The absence of interfaces (Finding 2.2) and global state (Finding 2.1) make the codebase nearly untestable, which impacts security and performance validation.

---

## Recommended Architectural Roadmap

### Phase 1: Stabilize (Low Risk, High Value)
1. Extract `containerWithDate` to a single shared type (Finding 4.1)
2. Compile regexes at package level (Finding 5.3)
3. Extract sorting utility functions (Finding 5.2)
4. Add `//go:build unix` to `filelock.go` (Finding 6.2)
5. Standardize error handling to always return `error` (Finding 6.1)

### Phase 2: Decompose (Medium Risk, High Value)
1. Split `main.go` into separate files by responsibility (Finding 1.1)
2. Introduce `App` context struct to replace globals (Finding 2.1)
3. Separate business logic from UI rendering (Finding 1.3)
4. Decompose `ProgressBoxState` into focused structs (Finding 4.2)

### Phase 3: Abstract (Higher Risk, Long-Term Value)
1. Introduce interfaces for key boundaries (Finding 2.2)
2. Move to sub-packages under `internal/` (Finding 1.2)
3. Add retry/resilience patterns for HTTP calls (Finding 5.4)
4. Implement command registry pattern (Finding 3.1)
5. Move API credentials to configuration (Finding 2.3)
