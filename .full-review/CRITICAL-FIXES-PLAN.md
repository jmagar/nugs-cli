# Critical Issues - Fix Plan

**Generated:** 2026-02-07
**Status:** Ready for implementation

This document outlines the 5 critical findings from the comprehensive code review and provides an actionable, prioritized fix plan.

---

## Critical Findings Summary

| ID | Issue | Severity | Impact | Effort |
|----|-------|----------|--------|--------|
| **CX-01** | `main()` function is 518 lines with cyclomatic complexity 80+ | Critical | Unmaintainable, untestable, high defect risk | Large |
| **MT-01** | Entire codebase in one `package main` (9,150 lines) | Critical | Zero encapsulation, cannot reuse code | Large |
| **ARCH-01** | Monolithic main.go (4,500 lines, 8+ responsibilities) | Critical | God file anti-pattern, cannot reason about code | Large |
| **ARCH-02** | Global mutable state prevents unit testing | Critical | Untestable, hidden coupling, race conditions | Medium |
| **TD-01** | API credentials hardcoded in source | Critical | Security risk, inflexible deployment | Small |

---

## Fix Strategy: Phased Approach

Given the interconnected nature of these issues, we'll use a **phased approach** to minimize risk:

### Phase A: Quick Wins (1-2 hours)
- Fix hardcoded credentials (TD-01)
- Document global state (prep for ARCH-02)

### Phase B: Structural Foundation (4-6 hours)
- Extract main.go into separate files within `package main` (ARCH-01)
- Decompose main() function (CX-01)

### Phase C: Architectural Refactoring (8-12 hours)
- Introduce sub-packages (MT-01)
- Eliminate global mutable state with dependency injection (ARCH-02)

---

## Phase A: Quick Wins

### A1. Fix Hardcoded Credentials (TD-01)

**File:** `main.go:47-48`
**Effort:** 30 minutes
**Risk:** Low

**Current Code:**
```go
const (
    devKey   = "x7f54tgbdyc64y656thy47er4"
    clientId = "Eg7HuH873H65r5rt325UytR5429"
)
```

**Fix:**
```go
// getDevKey returns the API dev key, allowing environment override
func getDevKey() string {
    if key := os.Getenv("NUGS_DEV_KEY"); key != "" {
        return key
    }
    // Public key extracted from Nugs.net Android APK (reverse-engineered)
    // Not a secret - this is distributed with the Android app
    return "x7f54tgbdyc64y656thy47er4"
}

// getClientID returns the API client ID, allowing environment override
func getClientID() string {
    if id := os.Getenv("NUGS_CLIENT_ID"); id != "" {
        return id
    }
    // Public client ID from Nugs.net Android APK
    return "Eg7HuH873H65r5rt325UytR5429"
}
```

**Changes Required:**
1. Replace the two `const` declarations with these functions
2. Update all references to `devKey` → `getDevKey()`
3. Update all references to `clientId` → `getClientID()`
4. Add documentation to README.md about environment variables
5. Add entry to `.env.example`

**Verification:**
```bash
# Test environment override works
export NUGS_DEV_KEY="test_key"
export NUGS_CLIENT_ID="test_id"
./nugs list artists --json minimal | jq .
```

---

### A2. Document Global State (ARCH-02 prep)

**File:** Create `docs/global-state-audit.md`
**Effort:** 1 hour
**Risk:** None (documentation only)

**Action:** Create a comprehensive inventory of all global mutable state:

```markdown
# Global State Audit

## Package-Level Variables

### HTTP Client State
- `var client *http.Client` (main.go:52) - shared HTTP client with cookie jar
- `var cookieJar http.CookieJar` (main.go:53) - session cookies
- **Risk:** Not thread-safe, invisible coupling, prevents isolated testing

### Configuration State
- `var configFilePath string` (main.go:56) - discovered config path
- **Risk:** Global dependency makes config testing difficult

### Error Counter
- `var dlErrorCount int` (main.go:58) - tracks download errors across batch
- **Risk:** Mutable global counter, race condition if concurrent downloads

### Progress State
- `var globalProgressBox *ProgressBoxState` (progress_box_global.go:10)
- `var progressMutex sync.RWMutex` (progress_box_global.go:11)
- **Risk:** Global singleton prevents multiple concurrent downloads in tests

## Recommended Changes

1. **Phase B:** Group into `App` context struct passed to functions
2. **Phase C:** Use dependency injection with interfaces
3. **Testing:** Mock interfaces for unit tests
```

---

## Phase B: Structural Foundation

### B1. Extract main.go into Separate Files (ARCH-01)

**Effort:** 4-5 hours
**Risk:** Medium (mechanical refactoring, high test coverage needed)

**Goal:** Split main.go (4,500 lines) into focused files within `package main`.

**File Structure:**
```
nugs/
  main.go            (~200 lines) - Entry, bootstrap, dispatch
  api_client.go      (~450 lines) - HTTP client, auth, API requests
  download.go        (~650 lines) - Download orchestration
  cache.go           (~350 lines) - Catalog cache I/O
  list_commands.go   (~550 lines) - Artists, shows, venues, latest
  url_parser.go      (~250 lines) - URL parsing and validation
  config.go          (~250 lines) - Config read/write
  rclone.go          (~350 lines) - Rclone upload integration
  batch.go           (~450 lines) - Batch processing logic
```

**Step-by-step Process:**

1. **Preparation**
   ```bash
   # Create feature branch
   git checkout -b refactor/split-main-go

   # Run existing tests to establish baseline
   go test -v -race -cover ./...
   ```

2. **Extract in Order** (least to most coupled):

   **Step 1:** Extract `url_parser.go`
   - Functions: `checkUrl()`, `validateURL()`, URL regex compilation
   - Dependencies: None (pure functions)
   - Test: Verify URL parsing still works

   **Step 2:** Extract `config.go`
   - Functions: `readConfig()`, `writeConfig()`, `promptForConfig()`, `configLocation()`
   - Dependencies: `structs.go` (Config type)
   - Test: Config read/write operations

   **Step 3:** Extract `cache.go`
   - Functions: `readCatalogCache()`, `writeCatalogCache()`, `fetchLatestCatalog()`
   - Dependencies: `filelock.go`, `structs.go`
   - Test: Cache operations with file locking

   **Step 4:** Extract `api_client.go`
   - Functions: `auth()`, `getAlbumMeta()`, `getArtistMeta()`, `getVenueMeta()`, `extractLegToken()`
   - Dependencies: Global `client`, `devKey`, `clientId`
   - Test: API client functions (use httptest.Server for mocking)

   **Step 5:** Extract `rclone.go`
   - Functions: `uploadToRclone()`, `deleteLocalAfterUpload()`, `remotePathExists()`
   - Dependencies: `Config` struct
   - Test: Rclone integration (mock exec.Command)

   **Step 6:** Extract `list_commands.go`
   - Functions: `listArtists()`, `listShows()`, `listVenues()`, `catalogLatest()`
   - Dependencies: Cache functions, API client
   - Test: List command outputs

   **Step 7:** Extract `download.go`
   - Functions: `album()`, `processTrack()`, `processVideo()`, `download()`
   - Dependencies: API client, rclone
   - Test: Download workflows (integration tests)

   **Step 8:** Extract `batch.go`
   - Functions: Batch processing logic
   - Dependencies: Download functions
   - Test: Batch operations

3. **Keep in main.go:**
   - `main()` function (to be decomposed in B2)
   - `bootstrap()` - config discovery, auth setup
   - `dispatch()` - command routing
   - `fatal()` - error exit helper

4. **After Each Extraction:**
   ```bash
   # Verify compilation
   go build -o nugs

   # Run tests
   go test -v -race ./...

   # Commit atomically
   git add <new_file>.go main.go
   git commit -m "refactor: extract <component> from main.go"
   ```

**Verification:**
```bash
# Full test suite
go test -v -race -cover ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html

# Functional smoke tests
./nugs list artists | head -5
./nugs catalog stats
./nugs 23329 -f 4  # Test download
```

---

### B2. Decompose main() Function (CX-01)

**File:** `main.go`
**Current:** 518 lines, complexity 80+
**Target:** <50 lines, complexity <10
**Effort:** 2 hours
**Risk:** Medium

**Current Structure:**
```go
func main() {
    // 1. Session persistence setup (20 lines)
    // 2. Script directory change (30 lines)
    // 3. Config file discovery (40 lines)
    // 4. First-run setup (60 lines)
    // 5. JSON flag rewriting (30 lines)
    // 6. Authentication (multiple paths, 80 lines)
    // 7. Command parsing (40 lines)
    // 8. Command dispatch (150 lines, 20+ commands)
    // 9. URL processing (30 lines)
    // 10. Batch orchestration (40 lines)
    // 11. Summary reporting (28 lines)
}
```

**Decomposed Structure:**
```go
func main() {
    app, err := bootstrap()
    if err != nil {
        log.Fatal(err)
    }

    exitCode := run(app, os.Args[1:])
    os.Exit(exitCode)
}

func bootstrap() (*App, error) {
    setupSessionPersistence()

    if err := changeToScriptDir(); err != nil {
        return nil, err
    }

    cfg, err := loadOrCreateConfig()
    if err != nil {
        return nil, err
    }

    token, err := authenticate(cfg)
    if err != nil {
        return nil, err
    }

    return &App{
        Config: cfg,
        Token:  token,
        Client: newHTTPClient(),
    }, nil
}

func run(app *App, args []string) int {
    args = preprocessJSONFlags(args)

    cmd, err := parseCommand(args)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        return 1
    }

    if err := dispatch(app, cmd); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        return 1
    }

    return 0
}
```

**App Context Struct** (eliminates global state):
```go
// App holds application-wide context
type App struct {
    Config *Config
    Token  string
    Client *http.Client

    // Progress tracking (replaces global progressMutex)
    ProgressBox *ProgressBoxState
    ProgressMu  sync.RWMutex

    // Download state (replaces global dlErrorCount)
    ErrorCount int
    ErrorMu    sync.Mutex
}
```

**Extracted Functions:**

1. **setupSessionPersistence()** - 20 lines
2. **changeToScriptDir()** - 30 lines, returns error
3. **loadOrCreateConfig()** - 80 lines, returns (Config, error)
4. **authenticate(cfg Config)** - 80 lines, returns (token, error)
5. **preprocessJSONFlags(args []string)** - 30 lines
6. **parseCommand(args []string)** - 50 lines, returns (Command, error)
7. **dispatch(app *App, cmd Command)** - 200 lines (switch statement calling handlers)

**Verification:**
```bash
# Run full test suite
go test -v -race ./...

# Verify all commands still work
./nugs list artists
./nugs catalog stats
./nugs status
./nugs 23329 -f 4
```

---

## Phase C: Architectural Refactoring

### C1. Introduce Sub-Packages (MT-01)

**Effort:** 8-10 hours
**Risk:** High (major structural change)

**Target Package Structure:**
```
nugs/
  cmd/
    nugs/
      main.go              - CLI entry point
  internal/
    api/
      client.go            - HTTP client and auth
      types.go             - API response types
      endpoints.go         - API endpoint functions
    download/
      engine.go            - Download orchestration
      track.go             - Track processing
      video.go             - Video processing
      batch.go             - Batch operations
    catalog/
      cache.go             - Cache I/O
      index.go             - Index builders
      stats.go             - Statistics
      gaps.go              - Gap detection
    rclone/
      uploader.go          - Upload logic
      remote.go            - Remote path checking
    ui/
      format.go            - Terminal formatting
      progress.go          - Progress bars
      tables.go            - Table rendering
    config/
      config.go            - Config management
      types.go             - Config struct
  pkg/
    filelock/
      lock.go              - POSIX file locking (reusable)
```

**Migration Steps:**

1. **Create Package Skeleton**
   ```bash
   mkdir -p cmd/nugs internal/{api,download,catalog,rclone,ui,config} pkg/filelock
   ```

2. **Move in Order** (least to most dependent):

   **Step 1:** `pkg/filelock`
   - Move `filelock.go` → `pkg/filelock/lock.go`
   - Make package `filelock` (not `main`)
   - Export `Lock`, `AcquireLock()`, `WithCacheLock()`

   **Step 2:** `internal/config`
   - Move config functions from `config.go` → `internal/config/config.go`
   - Move `Config` struct from `structs.go` → `internal/config/types.go`
   - Export package `config`

   **Step 3:** `internal/ui`
   - Move `format.go`, `progress_box_global.go`, `tier3_methods.go`
   - Move `ProgressBoxState` to `internal/ui/types.go`
   - Export package `ui`

   **Step 4:** `internal/catalog`
   - Move cache functions from `cache.go`
   - Move catalog handlers from `catalog_handlers.go`
   - Export package `catalog`

   **Step 5:** `internal/api`
   - Move API client from `api_client.go`
   - Move response structs from `structs.go`
   - Create `Client` struct with methods
   - Export package `api`

   **Step 6:** `internal/rclone`
   - Move rclone functions from `rclone.go`
   - Export package `rclone`

   **Step 7:** `internal/download`
   - Move download functions from `download.go`, `batch.go`
   - Export package `download`

   **Step 8:** `cmd/nugs`
   - Move `main.go` with only entry point
   - Import internal packages

3. **Dependency Injection Pattern:**
   ```go
   // internal/api/client.go
   package api

   type Client struct {
       httpClient *http.Client
       devKey     string
       clientID   string
       token      string
   }

   func NewClient(httpClient *http.Client, token string) *Client {
       return &Client{
           httpClient: httpClient,
           devKey:     getDevKey(),
           clientID:   getClientID(),
           token:      token,
       }
   }

   // cmd/nugs/main.go
   package main

   import "nugs/internal/api"

   func main() {
       client := api.NewClient(http.DefaultClient, token)
       // ...
   }
   ```

4. **Testing with Interfaces:**
   ```go
   // internal/api/client.go
   package api

   type APIClient interface {
       GetAlbumMeta(id int) (*AlbumMeta, error)
       Auth(email, password string) (string, error)
   }

   type Client struct { /* ... */ }

   func (c *Client) GetAlbumMeta(id int) (*AlbumMeta, error) { /* ... */ }

   // Tests can now use mock implementations
   type MockAPIClient struct {
       GetAlbumMetaFunc func(int) (*AlbumMeta, error)
   }

   func (m *MockAPIClient) GetAlbumMeta(id int) (*AlbumMeta, error) {
       return m.GetAlbumMetaFunc(id)
   }
   ```

**Verification:**
```bash
# Rebuild from new structure
cd cmd/nugs
go build -o ../../nugs

# Full test suite
go test -v -race ./...

# Integration tests
./nugs list artists
./nugs catalog update
./nugs 23329 -f 4
```

---

### C2. Eliminate Global Mutable State (ARCH-02)

**Effort:** 2-3 hours (done incrementally with C1)
**Risk:** Medium

**Current Global State:**
```go
var client *http.Client          // line 52
var cookieJar http.CookieJar     // line 53
var configFilePath string        // line 56
var dlErrorCount int             // line 58
var globalProgressBox *ProgressBoxState
var progressMutex sync.RWMutex
```

**Solution: App Context with Dependency Injection**

```go
// cmd/nugs/app.go
package main

import (
    "nugs/internal/api"
    "nugs/internal/config"
    "nugs/internal/download"
    "nugs/internal/ui"
)

type App struct {
    Config   *config.Config
    API      api.APIClient
    Download *download.Engine
    UI       *ui.Manager
}

func NewApp(cfg *config.Config, token string) *App {
    httpClient := &http.Client{Jar: cookiejar.New(nil)}

    return &App{
        Config:   cfg,
        API:      api.NewClient(httpClient, token),
        Download: download.NewEngine(httpClient, cfg),
        UI:       ui.NewManager(),
    }
}

// cmd/nugs/main.go
func main() {
    cfg, token, err := bootstrap()
    if err != nil {
        log.Fatal(err)
    }

    app := NewApp(cfg, token)
    os.Exit(run(app, os.Args[1:]))
}

func run(app *App, args []string) int {
    cmd := parseCommand(args)
    return dispatch(app, cmd)
}

func dispatch(app *App, cmd Command) int {
    switch cmd.Name {
    case "list":
        return handleList(app, cmd)
    // ...
    }
}
```

**Benefits:**
- No global mutable state
- Each component gets only what it needs (dependency injection)
- Easy to create isolated test instances
- Thread-safe by design (no shared globals)
- Clear dependency graph

---

## Testing Strategy

### Test Coverage Requirements

**Phase A:** Maintain existing coverage (current: ~40%)
**Phase B:** Increase to 60% (test new extracted functions)
**Phase C:** Increase to 85% (project standard)

### Test Types

1. **Unit Tests** - Test individual functions in isolation
   ```go
   func TestParseCommand(t *testing.T) {
       tests := []struct{
           args []string
           want Command
       }{
           {[]string{"list", "artists"}, Command{Name: "list", Type: "artists"}},
           // ...
       }
       // ...
   }
   ```

2. **Integration Tests** - Test component interactions
   ```go
   func TestDownloadWorkflow(t *testing.T) {
       // Use httptest.Server for API mocking
       // Use temp directory for file I/O
       // Verify download → rclone → cleanup
   }
   ```

3. **Smoke Tests** - End-to-end CLI tests
   ```bash
   #!/bin/bash
   # tests/smoke.sh

   ./nugs list artists | grep -q "Billy Strings"
   ./nugs catalog stats | grep -q "Total Shows"
   ./nugs status  # Should not error
   ```

---

## Risk Mitigation

1. **Feature Branch:** All work on `refactor/critical-fixes`
2. **Atomic Commits:** Each extraction is a single commit
3. **Test After Each Step:** Run `go test -race ./...` after every change
4. **Smoke Tests:** Manually verify core workflows after each phase
5. **Rollback Plan:** Keep `main` branch stable, can revert any commit
6. **Code Review:** Have another developer review before merge

---

## Timeline Estimate

| Phase | Tasks | Effort | Duration |
|-------|-------|--------|----------|
| **Phase A** | TD-01 (credentials), ARCH-02 (docs) | 1.5 hours | Day 1 |
| **Phase B** | ARCH-01 (split main.go), CX-01 (decompose main) | 6 hours | Day 2-3 |
| **Phase C** | MT-01 (sub-packages), ARCH-02 (DI) | 10 hours | Day 4-6 |
| **Total** | All critical fixes | 17.5 hours | ~1 week |

---

## Success Criteria

### Phase A Complete When:
- ✅ No hardcoded credentials in source (environment overrides work)
- ✅ Global state documented in `docs/global-state-audit.md`
- ✅ All existing tests pass

### Phase B Complete When:
- ✅ main.go <500 lines (currently 4,500)
- ✅ main() function <50 lines (currently 518)
- ✅ 8 separate files in `package main`
- ✅ Test coverage ≥60%
- ✅ All CLI commands work identically

### Phase C Complete When:
- ✅ Code organized in sub-packages (`internal/`, `pkg/`)
- ✅ Zero global mutable state
- ✅ App context with dependency injection
- ✅ Interfaces defined for mockable components
- ✅ Test coverage ≥85%
- ✅ Full smoke test suite passes

---

## Next Steps

1. **Review this plan** - Confirm approach and timeline
2. **Create feature branch** - `git checkout -b refactor/critical-fixes`
3. **Start Phase A** - Quick wins (1.5 hours)
4. **Checkpoint** - Review Phase A before proceeding to B
5. **Execute Phase B** - Structural foundation (6 hours)
6. **Checkpoint** - Review Phase B before proceeding to C
7. **Execute Phase C** - Architectural refactoring (10 hours)
8. **Final Review** - Code review, merge to main

---

**Ready to proceed with Phase A?**
