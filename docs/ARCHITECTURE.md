# Nugs CLI Architecture

Complete architectural overview of the Nugs CLI codebase including package structure, dependency patterns, and design decisions.

## Table of Contents

- [Overview](#overview)
- [Package Structure](#package-structure)
- [Dependency Hierarchy](#dependency-hierarchy)
- [Package Responsibilities](#package-responsibilities)
- [Architectural Patterns](#architectural-patterns)
- [Deps Pattern](#deps-pattern-dependency-injection)
- [Where New Code Goes](#where-new-code-goes)
- [Cross-Cutting Concerns](#cross-cutting-concerns)
- [Platform Abstraction](#platform-abstraction)
- [Common Gotchas](#common-gotchas)

---

## Overview

Nugs CLI is a Go-based monorepo with a clean 4-tier architecture:

```text
Tier 0: Foundation (model, testutil)
  ↓
Tier 1: Core Utilities (helpers, ui, api, cache)
  ↓
Tier 2: Infrastructure (config, rclone, runtime)
  ↓
Tier 3: Business Logic (catalog, download, list)
  ↓
Root: Command Orchestration (cmd/nugs/main.go)
```

**Key Stats:**
- **50 Go source files** across 13 internal packages
- **Module:** `github.com/jmagar/nugs-cli`
- **Entry Point:** `cmd/nugs/main.go` (648 lines)
- **Largest Package:** `internal/download/` (1738 lines)
- **No Circular Dependencies** - Strict upward dependency flow

---

## Package Structure

```text
nugs/
├── cmd/
│   └── nugs/
│       └── main.go           # Entry point (648 lines)
├── internal/                 # Private packages (13 total)
│   ├── model/                # Core data types (no dependencies)
│   ├── testutil/             # Test utilities (no dependencies)
│   ├── helpers/              # Path manipulation utilities
│   ├── ui/                   # Display and formatting
│   ├── api/                  # Nugs.net API client
│   ├── cache/                # Local catalog caching
│   ├── config/               # Configuration management
│   ├── rclone/               # Cloud upload integration
│   ├── runtime/              # Process control & detach
│   ├── catalog/              # Catalog operations
│   ├── download/             # Download engine
│   ├── list/                 # List commands
│   └── completion/           # Shell completions
├── Makefile                  # Build targets
└── go.mod                    # Module definition
```

---

## Dependency Hierarchy

### Tier 0: Foundation (No Internal Dependencies)

**model/** - All data structures, types, constants
- `Config` - User configuration
- `Args` - CLI arguments
- `LatestCatalogResp`, `ArtistMeta`, `ShowMeta` - API response types
- `ProgressBoxState`, `BatchProgressState` - UI state
- `MediaType` enum (Audio, Video, Both, Unknown)
- JSON level constants, message priority constants

**testutil/** - Test helpers
- `WithTempHome(t)` - Isolate HOME directory
- `CaptureStdout(t, fn)` - Capture stdout
- `ChdirTemp(t)` - Change to temp directory
- `WriteExecutable(t, path)` - Create executable script

**Key Pattern:** Dependency inversion - defines types used by higher layers without importing them.

---

### Tier 1: Core Utilities (Depend on Tier 0 Only)

**helpers/** - Path manipulation, validation, sanitization
- **Depends on:** model
- **Exports:** `Sanitise()`, `BuildAlbumFolderName()`, `FileExists()`, `MakeDirs()`, `ValidatePath()`, `GetVideoOutPath()`, `GetRcloneBasePath()`, `GetOutPathForMedia()`, `GetRclonePathForMedia()`, `CalculateLocalSize()`

**ui/** - Display, formatting, progress rendering
- **Depends on:** model
- **Exports:** `PrintSuccess()`, `PrintError()`, `PrintInfo()`, `PrintWarning()`, `GetMediaTypeIndicator()`, `DescribeAudioFormat()`, `DescribeVideoFormat()`, `RenderProgress()`, `RenderProgressBox()`, theme constants, error/warning counters

**api/** - Nugs.net API client
- **Depends on:** model
- **Exports:** `Auth()`, `GetUserInfo()`, `GetSubInfo()`, `ExtractLegToken()`, `GetLatestCatalog()`, `GetArtistMeta()`, `GetArtistList()`, `GetAlbumMeta()`, `GetPlistMeta()`, `GetStreamMeta()`, `GetPurchasedManURL()`, `QueryQuality()`, `GetTrackQual()`

**cache/** - Local catalog caching with POSIX file locking
- **Depends on:** model
- **Exports:** `GetCacheDir()`, `ReadCacheMeta()`, `ReadCatalogCache()`, `WriteCatalogCache()`, `BuildArtistIndex()`, `BuildContainerIndex()`, `WithCacheLock()`, `AcquireLock()`, `Release()`, `CacheArtistMeta()`, `ReadCachedArtistMeta()`
- **Platform-specific:** `filelock_unix.go`, `filelock_windows.go`

---

### Tier 2: Infrastructure (Depend on Tiers 0-1)

**config/** - Configuration management and CLI parsing
- **Depends on:** helpers, model, ui
- **Exports:** `ReadConfig()`, `WriteConfig()`, `ParseCfg()`, `PromptForConfig()`, `ResolveFfmpegBinary()`, `NormalizeCliAliases()`, `IsShowCountFilterToken()`, `IsMediaModifier()`, `LoadedConfigPath`

**rclone/** - Cloud upload via rclone
- **Depends on:** helpers, model, ui
- **Exports:** `CheckRcloneAvailable()`, `CheckRclonePathOnline()`, `UploadToRclone()`, `BuildRcloneUploadCommand()`, `BuildRcloneVerifyCommand()`, `RunRcloneWithProgress()`, `RemotePathExists()`, `ListRemoteArtistFolders()`, `ParseRcloneProgressLine()`, `ComputeProgressPercent()`

**runtime/** - Process control, detach, crawl lifecycle
- **Depends on:** cache, model, ui
- **Exports:** `IsReadOnlyCommand()`, `ShouldAutoDetach()`, `Detach()`, `SaveRuntimeStatus()`, `LoadRuntimeStatus()`, `HotkeyInput()`, `IsProcessAlive()`, constants: `DetachedEnvVar`, `ControlFilePath`, `StatusFilePath`
- **Platform-specific:** 9 platform-specific files for detach, cancel, hotkey, process checks

---

### Tier 3: Business Logic (Depend on Tiers 0-2 + Use Deps Pattern)

**catalog/** - Catalog browsing, gap analysis, auto-refresh
- **Depends on:** api, cache, config, helpers, model, ui
- **Uses Deps pattern** for root callbacks
- **Exports:** `Update()`, `CacheStatus()`, `Stats()`, `Latest()`, `Gaps()`, `Coverage()`, `AutoRefreshConfig()`, `ShouldAutoRefresh()`, `AutoRefreshIfNeeded()`, `FilterShowsByMediaType()`, `MatchesMediaFilter()`, `GetShowMediaType()`, `AnalyzeArtistCatalog()`, `FormatCoverageBar()`, `FormatShowDateRange()`

**download/** - Core download engine for audio and video
- **Depends on:** api, helpers, model, ui
- **Uses Deps pattern** for root callbacks
- **Exports:** `DownloadAlbum()`, `DownloadAudioTrack()`, `DownloadVideoTrack()`, `DownloadBatch()`, progress tracking with `ProgressBoxState` integration
- **Files:** `audio.go` (781 lines), `video.go` (791 lines), `batch.go` (166 lines), `deps.go` (43 lines)

**list/** - List commands for artists, shows, playlists
- **Depends on:** api, model, ui
- **Uses Deps pattern** for root callbacks
- **Exports:** `ListArtists()`, `ListShows()`, `ListPlaylists()`, JSON output support

**completion/** - Shell completion script generation
- **Depends on:** ui
- **Exports:** `GenerateBashCompletion()`, `GenerateZshCompletion()`, `GenerateFishCompletion()`, `GeneratePowerShellCompletion()`, context-aware completions

---

### Root Package: Command Orchestration

**cmd/nugs/main.go** - CLI entry point, wires everything together
- **Imports:** All internal packages
- **Responsibilities:**
  - CLI argument parsing
  - Command dispatcher
  - Authentication orchestration
  - Download coordination
  - Progress box management
  - Rclone upload coordination
  - Crawl control (pause/cancel)
  - Deps struct wiring

---

## Dependency Hierarchy Diagram

```text
┌─────────────────────────────────────────────┐
│  Tier 3: Business Logic (with Deps pattern) │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
│  │ catalog/ │  │download/ │  │  list/   │  │
│  └─────┬────┘  └─────┬────┘  └─────┬────┘  │
└────────┼─────────────┼─────────────┼────────┘
         │             │             │
┌────────┼─────────────┼─────────────┼────────┐
│  Tier 2: Infrastructure                     │
│  ┌──────┴───┐  ┌────┴─────┐  ┌────┴─────┐  │
│  │ config/  │  │ rclone/  │  │ runtime/ │  │
│  └─────┬────┘  └─────┬────┘  └─────┬────┘  │
└────────┼─────────────┼─────────────┼────────┘
         │             │             │
┌────────┼─────────────┼─────────────┼────────┐
│  Tier 1: Core Utilities                     │
│  ┌──────┴───┐  ┌────┴─────┐  ┌────┴─────┐  │
│  │ helpers/ │  │   ui/    │  │  cache/  │  │
│  │   api/   │  │          │  │          │  │
│  └─────┬────┘  └─────┬────┘  └─────┬────┘  │
└────────┼─────────────┼─────────────┼────────┘
         │             │             │
┌────────┴─────────────┴─────────────┴────────┐
│  Tier 0: Foundation                         │
│  ┌─────────────┐      ┌──────────────┐      │
│  │   model/    │      │  testutil/   │      │
│  │ (types only)│      │(test helpers)│      │
│  └─────────────┘      └──────────────┘      │
└─────────────────────────────────────────────┘

Root Package (cmd/nugs/main.go)
      │
      └──> Wires up Deps callbacks for catalog, download, list
```

---

## Package Responsibilities

### Model Package (Foundation)

**Purpose:** Define all data structures used across packages

**Key Principle:** Zero internal imports - pure data layer

**What lives here:**
- Configuration structs (`Config`, `Args`)
- API response types (`LatestCatalogResp`, `ArtistMeta`, `ShowMeta`, `Track`)
- UI state (`ProgressBoxState`, `BatchProgressState`)
- Enums (`MediaType`, `Phase`, `MessagePriority`)
- Constants (JSON levels, format codes)

**What does NOT live here:**
- Business logic
- I/O operations
- External dependencies

---

### Catalog Package (Business Logic)

**Purpose:** All catalog-related operations

**Responsibilities:**
- Update catalog cache from API
- Display catalog statistics
- Find gaps in collection (local + remote)
- Calculate coverage percentages
- Auto-refresh scheduling
- Media type filtering (audio/video/both)

**Deps Callbacks (from root):**

```go
type Deps struct {
    RemotePathExists func(remotePath string, cfg *model.Config, isVideo bool) (bool, error)
    Album func(ctx context.Context, albumID string, ...) error
    GetShowMediaType func(show *model.AlbArtResp) model.MediaType
    GetArtistMetaCached func(artistID int) (*model.ArtistMeta, error)
    ListRemoteArtistFolders func(cfg *model.Config, isVideo bool) ([]string, error)
    FormatDuration func(seconds int) string
    Playlist func(ctx context.Context, plistId, legacyToken string, ...) error
    // ... 8 total callbacks
}
```

---

### Download Package (Business Logic)

**Purpose:** Core download engine

**Responsibilities:**
- Download albums (audio tracks)
- Download videos (single segments)
- Batch operations (artist/playlist)
- Format selection and fallback
- FFmpeg integration (decrypt, convert)
- Progress tracking
- Upload coordination

**Deps Callbacks (from root):**

```go
type Deps struct {
    WaitIfPausedOrCancelled func() error
    IsCrawlCancelledErr func(err error) bool
    SetCurrentProgressBox func(pb *model.ProgressBoxState)
    RenderProgressBox func(box *model.ProgressBoxState)
    RenderCompletionSummary func(...)
    UploadToRclone func(localPath, artistFolder string, ...) error
    RemotePathExists func(remotePath string, ...) (bool, error)
    PrintProgress func(...)
    UpdateSpeedHistory func(history []float64, speed float64) []float64
    CalculateETA func(speedHistory []float64, remaining int64) string
    // ... 10 total callbacks
}
```

---

### List Package (Business Logic)

**Purpose:** List commands (artists, shows, playlists)

**Responsibilities:**
- List all artists with filtering
- List artist's shows with media/venue filtering
- List user/catalog playlists
- JSON output support
- Table rendering with media indicators

**Deps Callbacks (from root):**

```go
type Deps struct {
    GetShowMediaType func(show *model.AlbArtResp) model.MediaType
    MatchesMediaFilter func(show *model.AlbArtResp, filterType model.MediaType) bool
    Playlist func(ctx context.Context, plistId, legacyToken string, ...) error
    // ... 3 total callbacks
}
```

---

## Architectural Patterns

### 1. Dependency Inversion (Model Package)

**Pattern:** All types defined in `model/`, used by higher layers

**Benefits:**
- ✅ No circular dependencies
- ✅ Clear data contracts
- ✅ Easy to mock for testing

**Example:**

```go
// model/types.go (Tier 0)
type Config struct {
    Email    string
    Password string
    Format   int
    // ...
}

// config/config.go (Tier 2) - imports model
func ReadConfig() (*model.Config, error) {
    // Uses model.Config
}

// download/audio.go (Tier 3) - imports model
func DownloadAlbum(cfg *model.Config) error {
    // Uses model.Config
}
```

---

### 2. Dependency Injection (Deps Pattern)

**Problem:** Tier 3 packages need functionality from root package, but can't import it (would create circular dependency).

**Solution:** Dependency injection via `Deps` struct with callbacks.

**Pattern:**

```go
// catalog/deps.go
type Deps struct {
    RemotePathExists func(remotePath string, cfg *model.Config, isVideo bool) (bool, error)
    Album func(ctx context.Context, albumID string, ...) error
    // ... more callbacks
}

// catalog/handlers.go
func Gaps(ctx context.Context, artistID int, cfg *model.Config, deps Deps) error {
    // Use callback from root
    exists, err := deps.RemotePathExists(remotePath, cfg, false)
}
```

**Root wiring:**

```go
// cmd/nugs/main.go
catalogDeps := catalog.Deps{
    RemotePathExists: rclone.RemotePathExists,  // Tier 2 function
    Album: downloadAlbum,                       // Root function
    GetShowMediaType: getShowMediaType,         // Root function
}

catalog.Gaps(ctx, artistID, cfg, catalogDeps)
```

**Benefits:**
- ✅ No circular imports
- ✅ Clear dependency boundaries
- ✅ Testable (Deps can be mocked)
- ✅ Explicit about what root functionality is needed

**Trade-offs:**
- ⚠️ Verbose (requires explicit wiring)
- ⚠️ Deps struct must be threaded through function calls

---

### 3. Platform Abstraction (Build Tags)

**Pattern:** Platform-specific implementations with build tags

**Example:**

```go
// filelock_unix.go
//go:build !windows

func AcquireLock(path string, retries int) (*FileLock, error) {
    // Unix implementation using syscall.Flock
}

// filelock_windows.go
//go:build windows

func AcquireLock(path string, retries int) (*FileLock, error) {
    // Windows implementation using LockFileEx
}
```

**Packages using platform abstraction:**
- `cache/` - File locking (Unix/Windows)
- `runtime/` - Process detach, hotkeys, signals (Unix/Windows/Linux)

**Build tags used:**
- `//go:build !windows` - Unix-like systems (Linux, macOS, BSD)
- `//go:build windows` - Windows
- `//go:build linux` - Linux-specific (e.g., hotkeys)

---

### 4. Atomic Operations

**File Locking (cache/):**

```go
// Atomic write pattern
err := cache.WithCacheLock(func() error {
    tmpPath := cachePath + ".tmp"
    os.WriteFile(tmpPath, data, 0644)
    return os.Rename(tmpPath, cachePath)  // Atomic!
})
```

**UI Counters (ui/):**

```go
// Thread-safe atomic counters
var RunErrorCount atomic.Int64
var RunWarningCount atomic.Int64

RunErrorCount.Add(1)  // Atomic increment
```

**Progress Box (model/):**

```go
// Mutex-protected state
progressBox.Mu.Lock()
progressBox.DownloadPercent = 50
progressBox.Mu.Unlock()
```

---

### 5. Context Propagation

**Pattern:** All API calls accept `context.Context` for cancellation

**Example:**

```go
// api/client.go
func GetLatestCatalog(ctx context.Context) (*model.LatestCatalogResp, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("creating request: %w", err)
    }
    resp, err := Client.Do(req)
    // ...
}

// Caller can cancel
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
catalog, err := api.GetLatestCatalog(ctx)
```

**Why:**
- ✅ Supports timeouts
- ✅ Supports cancellation (Shift-C hotkey)
- ✅ Prevents hanging requests
- ✅ Standard Go idiom

---

## Deps Pattern (Dependency Injection)

### Why Deps Pattern?

**Problem:** Tier 3 packages (`catalog`, `download`, `list`) need functionality from root package:
- Progress box rendering (root)
- Crawl control (root)
- Rclone upload (root)
- Download orchestration (root)

**Constraint:** Cannot import root package from internal packages (circular dependency)

**Solution:** Dependency injection via `Deps` struct

---

### How Deps Works

**Step 1: Define Deps struct in package**

```go
// catalog/deps.go
type Deps struct {
    RemotePathExists func(remotePath string, cfg *model.Config, isVideo bool) (bool, error)
    Album func(ctx context.Context, albumID string, ...) error
    GetShowMediaType func(show *model.AlbArtResp) model.MediaType
    // ... more callbacks
}
```

**Step 2: Accept Deps in functions**

```go
// catalog/handlers.go
func Gaps(ctx context.Context, artistID int, cfg *model.Config, deps Deps) error {
    // Use callback from root
    exists, err := deps.RemotePathExists(remotePath, cfg, false)
    if err != nil {
        return err
    }
    // ...
}
```

**Step 3: Wire Deps in root package**

```go
// cmd/nugs/main.go
catalogDeps := catalog.Deps{
    RemotePathExists: rclone.RemotePathExists,  // Tier 2 function
    Album: downloadAlbum,                       // Root function
    GetShowMediaType: getShowMediaType,         // Root function
    // ... wire all callbacks
}

// Pass Deps to function
err := catalog.Gaps(ctx, artistID, cfg, catalogDeps)
```

---

### Deps Pattern: All Packages

**catalog.Deps (8 callbacks):**
- `RemotePathExists` - Check if path exists on rclone remote
- `ListRemoteArtistFolders` - List folders on remote
- `Album` - Download album (root orchestration)
- `Playlist` - Download playlist (root orchestration)
- `GetShowMediaType` - Detect audio/video/both
- `GetArtistMetaCached` - Get cached artist metadata
- `FormatDuration` - Format seconds to human string

**download.Deps (10 callbacks):**
- `WaitIfPausedOrCancelled` - Check crawl state
- `IsCrawlCancelledErr` - Detect cancellation error
- `SetCurrentProgressBox` - Update global progress box
- `RenderProgressBox` - Render progress to terminal
- `RenderCompletionSummary` - Render final summary
- `UploadToRclone` - Upload to cloud
- `RemotePathExists` - Check remote path
- `PrintProgress` - Fallback progress output
- `UpdateSpeedHistory` - Update speed smoothing
- `CalculateETA` - Calculate ETA from speed

**list.Deps (3 callbacks):**
- `GetShowMediaType` - Detect audio/video/both
- `MatchesMediaFilter` - Check if show matches filter
- `Playlist` - Download playlist (root orchestration)

---

## Where New Code Goes

| Feature Type | Package | Reasoning |
|-------------|---------|-----------|
| **New API endpoint** | `api/` | Centralized API client |
| **New data type** | `model/` | Shared across packages, Tier 0 |
| **Path manipulation** | `helpers/` | Path utilities live here |
| **Display/formatting** | `ui/` | Centralized UI logic |
| **Catalog command** | `catalog/` | Catalog-specific operations |
| **Download logic** | `download/` | Download engine |
| **List command** | `list/` | Listing operations |
| **Configuration option** | `config/` | Config parsing and management |
| **Rclone operation** | `rclone/` | Cloud upload logic |
| **Process control** | `runtime/` | Detach, pause, cancel |
| **Caching logic** | `cache/` | Local storage and locking |

### Adding New Functionality: Step-by-Step

**1. Determine package ownership**
- Does it fetch from API? → `api/`
- Does it display to user? → `ui/`
- Does it download media? → `download/`
- Does it analyze catalog? → `catalog/`

**2. Add code to appropriate package**

```go
// internal/catalog/new_feature.go
func NewFeature(ctx context.Context, cfg *model.Config) error {
    // Implementation
}
```

**3. Import from root if user-facing**

```go
// cmd/nugs/main.go
case "new-command":
    err := catalog.NewFeature(ctx, cfg)
```

**4. Update Deps if needs root callbacks**

```go
// catalog/deps.go
type Deps struct {
    // ... existing
    NewCallback func(...) error
}

// cmd/nugs/main.go
catalogDeps := catalog.Deps{
    // ... existing
    NewCallback: myRootFunction,
}
```

---

## Cross-Cutting Concerns

### Error Handling

**Pattern:** Explicit error returns (Go convention)

```go
func ProcessTrack(...) error {
    if err := DownloadTrack(...); err != nil {
        return fmt.Errorf("failed to download track: %w", err)
    }
    return nil
}
```

**Centralized errors:** `helpers/errors.go` defines common errors

**Metrics:** `ui.RunErrorCount`, `ui.RunWarningCount` - atomic counters

---

### Logging/Output

**Centralized in ui/:**
- `PrintSuccess()` - Green success messages
- `PrintError()` - Red error messages
- `PrintInfo()` - Blue info messages
- `PrintWarning()` - Yellow warnings
- `RenderProgress()` - Progress bars
- `RenderProgressBox()` - Complex progress display

**Consistent symbols/colors:** `ui/theme.go`

---

### Config Access

**Pattern:** Pass `*model.Config` explicitly through parameters

```go
func DownloadAlbum(ctx context.Context, albumID string, cfg *model.Config) error {
    // Use cfg.Format, cfg.OutPath, etc.
}
```

**No global config** - config is read once and threaded through

**Write location:** Tracked via `config.LoadedConfigPath`

---

### Context Propagation

**Pattern:** All API calls accept `context.Context` as first parameter

```go
func GetLatestCatalog(ctx context.Context) (*model.LatestCatalogResp, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("creating request: %w", err)
    }
    // ...
}
```

**Supports:**
- Timeout: `ctx, cancel := context.WithTimeout(ctx, 30*time.Second)`
- Cancellation: User presses Shift-C

---

### Media Type Handling

**Enum:** `model.MediaType` (Audio, Video, Both, Unknown)

**Filtering:**

```go
filteredShows := catalog.FilterShowsByMediaType(shows, model.MediaTypeVideo)
matches := catalog.MatchesMediaFilter(show, model.MediaTypeAudio)
```

**Path resolution:**

```go
outPath := helpers.GetOutPathForMedia(cfg, mediaType)
rclonePath := helpers.GetRclonePathForMedia(cfg, mediaType)
```

**UI indicators:**

```go
indicator := ui.GetMediaTypeIndicator(mediaType)  // 🎵/🎬/📹
```

---

## Platform Abstraction

### Build Tags

**Purpose:** Platform-specific implementations with shared interface

**Packages using platform abstraction:**
- `cache/` - File locking (2 files: Unix, Windows)
- `runtime/` - Process control (9 files: Unix, Windows, Linux)

**Build tag patterns:**

```go
// Unix-like (Linux, macOS, BSD, etc.)
//go:build !windows

// Windows
//go:build windows

// Linux-specific
//go:build linux
```

---

### Cache Package Platform Variants

**Unix (POSIX flock):**

```go
// filelock_unix.go
//go:build !windows

func AcquireLock(path string, retries int) (*FileLock, error) {
    fd, err := syscall.Open(path, syscall.O_CREAT|syscall.O_RDWR, 0644)
    for i := 0; i < retries; i++ {
        err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
        if err == nil {
            return &FileLock{fd: fd, path: path}, nil
        }
        time.Sleep(100 * time.Millisecond)
    }
    return nil, fmt.Errorf("failed to acquire lock after %d retries", retries)
}
```

**Windows (LockFileEx):**

```go
// filelock_windows.go
//go:build windows

func AcquireLock(path string, retries int) (*FileLock, error) {
    handle, err := syscall.CreateFile(...)
    for i := 0; i < retries; i++ {
        overlapped := &syscall.Overlapped{}
        err = syscall.LockFileEx(handle, syscall.LOCKFILE_EXCLUSIVE_LOCK|syscall.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, overlapped)
        if err == nil {
            return &FileLock{handle: handle, path: path}, nil
        }
        time.Sleep(100 * time.Millisecond)
    }
    return nil, fmt.Errorf("failed to acquire lock after %d retries", retries)
}
```

---

### Runtime Package Platform Variants

**9 platform-specific files:**
- `detach_unix.go`, `detach_windows.go` - Background process creation
- `cancel_unix.go`, `cancel_windows.go` - Crawl cancellation
- `hotkey_input_unix.go`, `hotkey_input_windows.go`, `hotkey_input_other.go` - Interactive pause/cancel
- `process_alive_unix.go`, `process_alive_windows.go` - PID checking
- `signal_persistence_unix.go`, `signal_persistence_windows.go` - Signal handling

**Why so many variants?**
- Unix uses fork/exec, Windows uses CreateProcess
- Signal handling differs (SIGTERM vs WM_CLOSE)
- Terminal control differs (termios vs Windows Console API)

---

## Common Gotchas

### 1. Never Import Root Package from Internal

```go
// ❌ BAD - Circular import
// internal/catalog/handlers.go
import "github.com/jmagar/nugs-cli/cmd/nugs"

// ✅ GOOD - Use Deps pattern
type Deps struct {
    Album func(...) error  // Callback to root
}
```

---

### 2. Model Package Must Have Zero Internal Imports

```go
// ❌ BAD - Breaks Tier 0 foundation
// internal/model/types.go
import "github.com/jmagar/nugs-cli/internal/helpers"

// ✅ GOOD - No internal imports
package model

type Config struct {
    Email string
    // ...
}
```

---

### 3. Platform-Specific Code Requires Build Tags

```go
// ❌ BAD - Compiles for all platforms
func AcquireLock(path string) error {
    // Unix-specific syscall
}

// ✅ GOOD - Platform-specific with build tag
//go:build !windows

func AcquireLock(path string) error {
    // Unix-specific syscall
}
```

---

### 4. Cache Writes Need File Locking

```go
// ❌ BAD - Race condition with concurrent access
os.WriteFile(cachePath, data, 0644)

// ✅ GOOD - Atomic write with file locking
err := cache.WithCacheLock(func() error {
    tmpPath := cachePath + ".tmp"
    os.WriteFile(tmpPath, data, 0644)
    return os.Rename(tmpPath, cachePath)  // Atomic!
})
```

---

### 5. Progress Box Must Be Locked

```go
// ❌ BAD - Race condition
progressBox.DownloadPercent = 50

// ✅ GOOD - Thread-safe
progressBox.Mu.Lock()
progressBox.DownloadPercent = 50
progressBox.Mu.Unlock()
```

---

### 6. Don't Hardcode Paths

```go
// ❌ BAD - Hardcoded path
outPath := cfg.OutPath + "/" + artistFolder

// ✅ GOOD - Use helper
outPath := helpers.GetOutPathForMedia(cfg, mediaType)
```

---

### 7. API Calls Need Context

```go
// ❌ BAD - No context
func FetchCatalog() (*Catalog, error) {
    resp, err := http.Get(url)  // Can't cancel
}

// ✅ GOOD - Context-aware
func FetchCatalog(ctx context.Context) (*Catalog, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("creating request: %w", err)
    }
    resp, err := Client.Do(req)  // Can cancel
}
```

---

### 8. Deps Must Be Threaded Through

```go
// ❌ BAD - Missing Deps parameter
func Gaps(ctx context.Context, artistID int, cfg *model.Config) error {
    // Can't call root functions!
}

// ✅ GOOD - Accept Deps
func Gaps(ctx context.Context, artistID int, cfg *model.Config, deps Deps) error {
    err := deps.Album(ctx, albumID, ...)  // Root callback
}
```

---

## Package Size & Complexity

### Largest Packages (by lines)

1. **download/** - 1738 lines (4 files: audio, video, batch, deps)
2. **catalog/** - 1527 lines (5 files: handlers, autorefresh, media_filter, helpers, deps)
3. **list/** - 787 lines (1 file)
4. **api/** - 430 lines (1 file)
5. **config/** - 506 lines (1 file)

### Most Dependencies (incoming)

1. **model/** - Used by all 12 other packages
2. **ui/** - Used by 7 packages
3. **helpers/** - Used by 5 packages
4. **api/** - Used by 3 packages
5. **cache/** - Used by 3 packages

---

## See Also

- [CLAUDE.md](../CLAUDE.md) - Development guide and quick start
- [CONFIG.md](./CONFIG.md) - Configuration reference
- [README.md](../README.md) - User documentation
