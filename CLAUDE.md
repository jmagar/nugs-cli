# Nugs CLI Development Guide

This document contains development information for contributors working on Nugs CLI.

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Building from Source](#building-from-source)
- [Code Structure](#code-structure)
- [Development Patterns](#development-patterns)
- [Testing](#testing)
- [Contributing](#contributing)

---

## Architecture Overview

Nugs CLI is a Go-based command-line tool for downloading and managing music from Nugs.net.

### Core Components

**Authentication & API**
- OAuth-based authentication with Nugs.net API
- Token-based auth support for Apple/Google accounts
- Session management and credential storage

**Download Engine**
- Multi-format support (ALAC, FLAC, MQA, 360RA, AAC)
- Video download with FFmpeg integration
- Batch processing for multiple albums
- Rclone integration for cloud uploads

**Catalog System**
- Local caching of entire Nugs.net catalog (~13,000 shows)
- Four index files for fast lookups:
  - `catalog.json` - Full show metadata
  - `catalog-meta.json` - Cache statistics
  - `by-artist.json` - Shows grouped by artist
  - `by-date.json` - Shows sorted chronologically
- Cache location: `~/.cache/nugs/`
- File locking for concurrent safety

**Auto-Refresh**
- Configurable schedule (daily/weekly)
- Timezone-aware refresh times
- Automatic updates at startup if needed

---

## Building from Source

### Prerequisites

- Go 1.16 or later
- FFmpeg (for video downloads)
- Make (required for building)

### Build Commands

**⚠️ CRITICAL: ALWAYS use `make build` - NEVER use `go build` directly**

**Using Make (REQUIRED):**

```bash
make build
```

This builds the binary and installs it to `~/.local/bin/nugs`.

**Why `make build` is required:**
- Direct `go build` commands create binaries in the project root (nugs, nugs.exe, etc.)
- These clutter the workspace even though they're gitignored
- The Makefile handles the correct build target and output location
- `make build` ensures consistent builds across all environments

**Cross-compilation verification ONLY:**

These commands are for verification purposes only. For actual building, use `make build` above.

```bash
# Cross-compile check for macOS (does NOT create local binary)
GOOS=darwin go build ./cmd/nugs

# Cross-compile check for Windows (does NOT create local binary)
GOOS=windows go build ./cmd/nugs

# Cross-compile check for Linux ARM
GOOS=linux GOARCH=arm64 go build ./cmd/nugs
```

**Note:** These cross-compilation checks don't create local binaries when using a different target OS.

**Clean build artifacts:**

```bash
make clean
```

---

## Code Structure

**Monorepo Structure:**

```
nugs/
├── cmd/
│   └── nugs/
│       └── main.go           # Entry point (648 lines)
├── internal/                 # Private packages (13 total)
│   ├── api/                  # API client and authentication
│   ├── cache/                # File locking and cache utilities
│   ├── catalog/              # Catalog handlers, autorefresh, media filtering
│   ├── completion/           # Shell completion generators
│   ├── config/               # Configuration management
│   ├── download/             # Download engine and FFmpeg integration
│   ├── helpers/              # Shared utilities
│   ├── list/                 # List commands (artists, shows)
│   ├── model/                # Data structures (Config, API responses)
│   ├── rclone/               # Rclone integration
│   ├── runtime/              # Process management, signals, status
│   ├── testutil/             # Test utilities
│   └── ui/                   # UI rendering and progress
├── docs/                     # Technical documentation
├── .docs/                    # Session logs and planning docs
├── config.json               # User configuration (gitignored)
├── Makefile                  # Build targets
├── go.mod                    # Module definition
└── README.md                 # User documentation
```

**Module:** `github.com/jmagar/nugs-cli`

**Package Organization:**
- `cmd/nugs/` - CLI entry point only, imports internal packages
- `internal/` - All application logic (not importable by external projects)
- 50 Go source files across 13 packages

### Key Packages

**cmd/nugs/main.go** (648 lines)
- CLI argument parsing
- Command dispatcher
- Coordinates internal packages

**internal/model/** (Data structures)
- `Config` - User configuration
- `LatestCatalogResp`, `ArtistMeta`, `ShowMeta` - API responses
- `CacheMeta` - Cache statistics
- `Args` - CLI arguments

**internal/catalog/** (Catalog operations)
- `handlers.go` - Update, cache, stats, latest, gaps commands
- `autorefresh.go` - Auto-refresh logic and configuration
- `media_filter.go` - Audio/video filtering
- `helpers.go` - Catalog utilities

**internal/cache/** (Caching infrastructure)
- `filelock_unix.go` - POSIX file locking with retry logic
- Lock helpers for concurrent safety

**internal/download/** (Download engine)
- Download orchestration
- FFmpeg integration for video
- Progress tracking

**internal/api/** (API client)
- API client functions
- Authentication (OAuth, token-based)
- Session management

**internal/rclone/** (Cloud integration)
- Rclone upload coordination
- Remote path checking
- Transfer management

**internal/completion/** (Shell completions)
- Bash, Zsh, Fish, PowerShell completion generators

**internal/runtime/** (Process management)
- Signal handling and persistence
- Background process management
- Status tracking

### Package Import Guide

**Internal packages use fully qualified imports:**

```go
import (
    "github.com/jmagar/nugs-cli/internal/api"
    "github.com/jmagar/nugs-cli/internal/catalog"
    "github.com/jmagar/nugs-cli/internal/config"
    "github.com/jmagar/nugs-cli/internal/model"
)
```

**Package dependencies (high-level):**

```
cmd/nugs/main.go
  ↓
internal/catalog → internal/cache (file locking)
internal/download → internal/rclone (cloud uploads)
internal/api → internal/model (data structures)
internal/list → internal/catalog (show data)
internal/completion → (standalone)
```

**Adding new functionality:**
1. Determine which package owns the feature
2. Add code to appropriate `internal/` package
3. Import from `cmd/nugs/main.go` if user-facing
4. Update package `deps.go` if adding new dependencies

### Architecture Patterns

**Dependency Tiers (Strict Hierarchy):**
```
Tier 0: model, testutil (no dependencies)
  ↓
Tier 1: helpers, ui, api, cache (depend on Tier 0)
  ↓
Tier 2: config, rclone, runtime (depend on Tiers 0-1)
  ↓
Tier 3: catalog, download, list (depend on Tiers 0-2 + use Deps pattern)
  ↓
cmd/nugs (root package, wires everything together)
```

**Key Patterns:**
- **Dependency Inversion:** All types in `model/`, no imports from internal packages
- **Dependency Injection:** Tier 3 packages use `Deps` structs to avoid circular imports
- **Platform Abstraction:** Build tags for Unix/Windows variants (`//go:build !windows`)
- **Atomic Operations:** File locking (`cache/`), counters (`ui/`), progress (`model.ProgressBoxState`)
- **Context Propagation:** All API calls accept `context.Context` for cancellation

**Where New Code Goes:**

| Feature Type | Package | Example |
|-------------|---------|---------|
| API endpoint | `api/` | New Nugs.net API method |
| Data type | `model/` | New response struct |
| Path logic | `helpers/` | Filename sanitization |
| Display | `ui/` | Progress rendering |
| Catalog cmd | `catalog/` | New gap analysis |
| Download | `download/` | New format support |
| Config field | `config/` | New option parsing |
| Cloud upload | `rclone/` | Upload verification |
| Process ctrl | `runtime/` | New signal handler |
| Cache logic | `cache/` | Index optimization |

**Architectural Gotchas:**
- ⚠️ Never import root package (`cmd/nugs/`) from `internal/` - use Deps pattern
- ⚠️ `model/` must have zero internal imports (Tier 0 foundation)
- ⚠️ Platform-specific code requires build tags (`//go:build windows`)
- ⚠️ Cache writes need `cache.WithCacheLock()` wrapper (POSIX flock)
- ⚠️ Progress updates need `ProgressBoxState.Mu` lock
- ⚠️ API calls need `context.Context` (never `context.Background()` in libraries)

---

## Development Patterns

### ⚠️ Build Command Rule

**CRITICAL: Always use `make build` - Never use `go build` directly**

- ❌ `go build ./cmd/nugs` - Creates binaries in project root (nugs, nugs.exe, etc.)
- ✅ `make build` - Correctly builds to `~/.local/bin/nugs`

**Why:** Direct `go build` commands create binaries in the project root which clutter the workspace even though they're gitignored. The Makefile handles the correct build target and output location.

**Verification commands are OK:**
- `go test ./... -count=1` - Run tests
- `go vet ./...` - Linting
- `GOOS=darwin go build ./cmd/nugs` - Cross-compile check (doesn't create local binary)

**For actual building:**
```bash
make build          # Build and install
make clean          # Remove build artifacts
```

---

### Configuration Management

**Config Structure (23 fields):**

The `Config` struct in `internal/model/types.go` contains:

**Authentication (3 fields):**
- `email` (string) - Nugs.net account email
- `password` (string) - Account password (plain text)
- `token` (string) - Pre-existing auth token for Apple/Google accounts

**Download Quality (4 fields):**
- `format` (int 1-5) - Audio: 1=ALAC, 2=FLAC, 3=MQA, 4=360RA, 5=AAC (default: 4)
- `videoFormat` (int 1-5) - Video: 1=480p, 2=720p, 3=1080p, 4=1440p, 5=4K (default: 5)
- `defaultOutputs` (string) - Media preference: "audio", "video", "both" (default: "audio")
- `wantRes` (string) - Computed resolution string from videoFormat

**Output Paths (3 fields):**
- `outPath` (string) - Local download directory for audio (default: "Nugs downloads")
- `videoOutPath` (string) - Local download directory for videos (defaults to outPath)
- `urls` ([]string) - Parsed URLs from CLI arguments

**FFmpeg Integration (4 fields):**
- `useFfmpegEnvVar` (bool) - Use FFmpeg from system PATH vs local binary
- `ffmpegNameStr` (string) - Custom FFmpeg binary name/path
- `skipChapters` (bool) - Skip chapter embedding in videos
- **Deprecated:** `forceVideo`, `skipVideos` (use `defaultOutputs` instead)

**Rclone Cloud Uploads (6 fields):**
- `rcloneEnabled` (bool) - Enable cloud uploads via rclone
- `rcloneRemote` (string) - Rclone remote name (e.g., "gdrive")
- `rclonePath` (string) - Remote base path for audio (e.g., "/Music")
- `rcloneVideoPath` (string) - Remote base path for videos (defaults to rclonePath)
- `deleteAfterUpload` (bool) - Delete local files after successful upload (default: true)
- `rcloneTransfers` (int) - Number of parallel transfers (default: 4)

**Catalog Auto-Refresh (4 fields):**
- `catalogAutoRefresh` (bool) - Enable automatic catalog updates (default: true)
- `catalogRefreshTime` (string) - Time to refresh in HH:MM format (default: "05:00")
- `catalogRefreshTimezone` (string) - IANA timezone for refresh time (default: "America/New_York")
- `catalogRefreshInterval` (string) - Frequency: "daily" or "weekly" (default: "daily")

**Performance (1 field):**
- `skipSizePreCalculation` (bool) - Skip pre-calculation of download sizes

**Config File Search Order:** (First found wins)
1. `~/.nugs/config.json` (recommended)
2. `~/.config/nugs/config.json` (XDG standard)

**Config Validation Rules:**
- `format` must be 1-5 (fails: `track Format must be between 1 and 5`)
- `videoFormat` must be 1-5 (fails: `video format must be between 1 and 5`)
- `defaultOutputs` must be "audio", "video", or "both" (fails: `invalid defaultOutputs`)
- `catalogRefreshTimezone` must be valid IANA timezone (fails: `invalid timezone`)
- `catalogRefreshTime` must be HH:MM format (fails: `invalid refresh time format`)
- `catalogRefreshInterval` must be "daily" or "weekly" (fails: `invalid interval`)
- `rcloneTransfers` must be >= 1 if set (fails: `transfers must be a positive integer`)

**Security:**
- Config file created with `0600` permissions (owner read/write only)
- Auto-fix for insecure permissions (warns and fixes on read)
- Credentials stored in **plain text** (no encryption)
- Token prefix "Bearer " automatically stripped

**Common Misconfigurations:**
```json
{
  "format": 6,                          // ❌ Invalid (must be 1-5)
  "defaultOutputs": "videos",           // ❌ Typo ("videos" vs "video")
  "catalogRefreshTimezone": "EST",      // ❌ Use "America/New_York"
  "catalogRefreshTime": "5am",          // ❌ Use "05:00"
  "catalogRefreshInterval": "hourly",   // ❌ Use "daily" or "weekly"
  "rcloneEnabled": true,
  "rcloneRemote": "",                   // ❌ Missing required remote
  "rclonePath": ""                      // ❌ Missing required path
}
```

**Example Valid Config:**
```json
{
  "email": "user@example.com",
  "token": "your_auth_token_here",
  "format": 2,
  "videoFormat": 5,
  "defaultOutputs": "both",
  "outPath": "/home/user/Music/Nugs",
  "videoOutPath": "/home/user/Videos/Nugs",
  "useFfmpegEnvVar": true,
  "rcloneEnabled": true,
  "rcloneRemote": "gdrive",
  "rclonePath": "/Music/Nugs",
  "rcloneVideoPath": "/Videos/Nugs",
  "deleteAfterUpload": true,
  "rcloneTransfers": 8,
  "catalogAutoRefresh": true,
  "catalogRefreshTime": "03:00",
  "catalogRefreshTimezone": "America/Los_Angeles",
  "catalogRefreshInterval": "weekly"
}
```

### Cache Management

**Cache Structure:**

```
~/.cache/nugs/
├── catalog.json           # Full show metadata (7-8 MB)
├── catalog-meta.json      # Cache statistics
├── by-artist.json         # Shows grouped by artist ID
└── by-date.json           # Shows sorted by date
```

**Cache I/O:**

```go
// Read cache with automatic fallback
catalog, err := readCatalogCache()

// Write cache with file locking
err := writeCatalogCache(catalog, updateDuration)

// Atomic write pattern
tmpPath := cachePath + ".tmp"
os.WriteFile(tmpPath, data, 0644)
os.Rename(tmpPath, cachePath)  // Atomic!
```

### File Locking

**Why:** Prevents cache corruption when multiple `nugs` processes run concurrently.

**Implementation:**

```go
// Acquire lock (POSIX flock with retry)
lock, err := AcquireLock(lockPath, 50)  // 50 retries = 5s timeout
defer lock.Release()

// Helper for catalog operations
err := WithCacheLock(func() error {
    // Your cache operation here
})
```

**Lock Details:**
- Uses `syscall.Flock()` with `LOCK_EX | LOCK_NB`
- Retries 50 times with 100ms delay (5s total timeout)
- Lock file: `~/.cache/nugs/.catalog.lock`

### JSON Output

**Levels:**
- `minimal` - Essential fields only
- `standard` - Adds location details
- `extended` - All metadata
- `raw` - Unmodified API response

**Implementation:**

```go
if jsonLevel != "" {
    // Suppress banner/headers
    data := prepareJSONOutput(result, jsonLevel)
    jsonBytes, _ := json.MarshalIndent(data, "", "  ")
    fmt.Println(string(jsonBytes))
    return
}
// Normal table output
```

### API Integration

**Authentication:**

```go
// Login with email/password
token, err := auth(email, password)

// Or use pre-configured token
token := cfg.Token
```

**Catalog API:**

```go
// Fetch latest catalog
resp, err := http.Get("https://play.nugs.net/api/v1/catalog.latest")
var catalog LatestCatalogResp
json.NewDecoder(resp.Body).Decode(&catalog)
```

### Download Flow (Critical Path)

**End-to-End Flow:**
```
1. CLI Command
   ↓
2. bootstrap() → parse config/args
   ↓
3. run() → authenticate, parse URLs
   ↓
4. URL Parser → extract media type & ID
   ↓
5. Dispatch to handler (album/video/artist/playlist)
   ↓
6. Album/Video Handler
   ├─ Fetch metadata (API call)
   ├─ Check local/remote existence (skip if exists)
   ├─ Create directories (artist/album folders)
   ├─ Pre-calculate size (optional, 8 concurrent HEAD requests)
   ├─ Initialize/reuse progress box
   └─ Download loop (sequential)
      ├─ For each track/segment:
      │  ├─ Check pause/cancel state
      │  ├─ Query available formats (API calls)
      │  ├─ Select format (quality matching + fallback)
      │  ├─ Download file (HTTP GET with progress callback)
      │  ├─ Decrypt (if HLS-only) + FFmpeg convert
      │  └─ Update progress box (track + show level)
      └─ All tracks complete
         ↓
7. Upload Phase (if rcloneEnabled)
   ├─ Calculate upload size
   ├─ Build rclone command (copy/copyto)
   ├─ Execute with progress parsing
   ├─ Update progress box (upload percent/speed/ETA)
   ├─ Verify upload (if deleteAfterUpload)
   └─ Delete local files (if verified)
      ↓
8. Completion
   ├─ Set phase to Complete
   ├─ Calculate total duration
   ├─ Render completion summary
   └─ Return (next URL or exit)
```

**Format Selection:**

Audio formats (1-5):
1. 16-bit 44.1kHz ALAC
2. 16-bit 44.1kHz FLAC (most common)
3. 24-bit 48kHz MQA
4. 360 Reality Audio (default)
5. 150 Kbps AAC (HLS-only fallback)

Fallback chain: ALAC→FLAC→AAC, MQA→FLAC, 360RA→MQA

Video formats (1-5):
1. 480p
2. 720p
3. 1080p
4. 1440p
5. 4K/best available (default)

Fallback chain: 1440p→1080p, 1080p→720p, 720p→480p

**FFmpeg Integration:**

Audio (HLS-only streams):
```bash
# Decrypt AES-128 encrypted .ts → Convert to AAC
ffmpeg -i pipe: -c:a copy output.m4a
```

Video (TS to MP4 conversion):
```bash
# Without chapters:
ffmpeg -hide_banner -i input.ts -c copy output.mp4

# With chapters:
ffmpeg -hide_banner \
  -i input.ts \
  -f ffmetadata -i chapters.txt \
  -map_metadata 1 \
  -c copy \
  output.mp4
```

**Progress Tracking:**

Global progress box state (`model.ProgressBoxState`):
- Show-level: percent, downloaded/total, track count
- Track-level: track number, name, format, percent, speed, ETA
- Upload-level: percent, uploaded/total, speed, ETA, duration
- Phase transitions: Download → Upload → Complete
- Message priority: status < warning < error
- Thread-safe: Protected by `Mu` (sync.Mutex)

**Concurrent Operations:**

Sequential track downloads (NOT parallel) - prevents API rate limits
- **Exception:** Size pre-calculation uses 8 concurrent HEAD requests (60s timeout)

**Rclone Upload:**

```go
// Audio: rclone copy localPath remote:path/artistFolder/albumName --transfers=4
// Video: rclone copyto localPath remote:path/artistFolder/video.mp4 --transfers=4

if cfg.RcloneEnabled {
    remotePath := cfg.RclonePath + "/" + albumFolder
    cmd := exec.Command("rclone", "copy", localPath, remotePath,
        "--transfers", strconv.Itoa(cfg.RcloneTransfers),
        "--progress", "--stats=1s")

    RunRcloneWithProgress(cmd, progressFn)

    if cfg.DeleteAfterUpload {
        // Verify first: rclone check --one-way localPath remoteFullPath
        os.RemoveAll(localPath)
    }
}
```

**Gap Detection with Rclone:**

```go
// Check local path first
if sanitise(localPath) {
    downloaded = append(downloaded, show.ContainerID)
    continue
}

// Then check remote if rclone enabled
if cfg.RcloneEnabled && remotePathExists(remotePath, cfg) {
    downloaded = append(downloaded, show.ContainerID)
}
```

**Error Handling:**

Track-level errors (batch continues):
- Network timeouts → logged, continue to next track
- Format unavailable → fallback to next format
- Disk full → error, stop download

Album-level errors (track continues):
- Invalid show ID → error, skip to next album
- Missing FFmpeg → error, skip video conversion

Crawl-level errors (fatal):
- User cancellation (Shift-C) → stop all downloads
- Authentication failure → exit immediately

---

## Testing

### Current Test Coverage

**Overall Coverage:** 5.5% (37 tests across 10 test files)

**Test Files:**
- `cmd/nugs/catalog_handlers_test.go` (3 tests) - Catalog analysis
- `cmd/nugs/config_test.go` (1 test) - CLI alias normalization
- `cmd/nugs/config_ffmpeg_test.go` (3 tests) - FFmpeg binary resolution
- `cmd/nugs/helpers_test.go` (3 tests) - Path helpers
- `cmd/nugs/media_type_test.go` (9 tests) - Media type detection ✅ Comprehensive
- `cmd/nugs/url_parser_test.go` (2 tests) - URL parsing
- `cmd/nugs/rclone_test.go` (9 tests) - Upload progress ✅ Comprehensive
- `cmd/nugs/format_render_test.go` (3 tests) - Progress rendering
- `internal/helpers/helpers_test.go` (3 tests) - Helper functions
- `internal/rclone/rclone_test.go` (1 test) - Rclone progress parsing

**Test Utilities:** `internal/testutil/testutil.go`
- `WithTempHome(t)` - Isolate HOME directory
- `CaptureStdout(t, fn)` - Capture stdout during function
- `ChdirTemp(t)` - Change to temp directory with cleanup
- `WriteExecutable(t, path)` - Create minimal executable script

**Testing Patterns:**
- **Table-Driven Tests** (95% of tests) - Go idiomatic pattern
- **Test Isolation** - Use `t.TempDir()`, `t.Setenv()`, `t.Cleanup()`
- **Fast Unit Tests** - No network/API calls, completes in ~0.007s
- **No Mocking Framework** - Direct testing with real data structures

**Critical Coverage Gaps (0% tested):**
- ❌ `internal/api` (~600 lines) - Authentication, API calls
- ❌ `internal/download` (~2500 lines) - Download logic, FFmpeg
- ❌ `internal/cache` (~800 lines) - File locking, concurrency
- ❌ `internal/catalog` (~1200 lines) - Catalog operations
- ❌ `internal/config` (~300 lines) - Config loading
- ❌ `internal/list` (~400 lines) - List commands

**Next Testing Priorities:**
1. API client tests with `httptest.Server` (mock responses)
2. Cache concurrency tests (file locking, corruption recovery)
3. Config validation tests (all 23 fields)
4. Download flow tests with small test files (<1MB)

### Manual Testing

**Build and test:**

```bash
# Build (ALWAYS use make, never go build directly)
make build

# Run automated tests
make test                    # All tests
go test ./... -v             # Verbose
go test ./... -cover         # With coverage
go test ./... -race          # Race detector
go test -bench=. ./cmd/nugs  # Benchmarks

# Manual functional tests
nugs 23329                   # Test download
nugs update                  # Test catalog
nugs stats                   # Test stats
nugs gaps 1125               # Test gap detection
nugs refresh set             # Test auto-refresh
```

### Test Data

**Artist IDs for testing:**
- `1125` - Billy Strings (430 shows, primary test artist)
- `461` - Grateful Dead (large catalog)
- `1045` - Dead & Company
- `22` - Umphrey's McGee

**Show IDs for testing:**
- `23329` - Valid album
- `23790` - Valid album
- `24105` - Valid album

### Edge Cases

**Test these scenarios:**
- Empty config file
- Invalid credentials
- Missing FFmpeg (test error: "ffmpeg not found in PATH")
- Concurrent catalog updates (test file locking)
- Network interruptions (test download resume)
- Invalid artist IDs (test error handling)
- Invalid show IDs (test error handling)
- Rclone not installed when enabled (test error: "rclone not found")
- Cache corruption (test fallback to API)
- Disk full (test error: "no space left on device")
- Insecure config permissions (test auto-fix warning)
- Invalid config values (test validation errors)

---

## Contributing

### Code Style

**Go Conventions:**
- Follow standard Go formatting (`gofmt`)
- Use meaningful variable names
- Comment exported functions
- Keep functions under 50 lines where possible
- Handle errors explicitly

**Build Requirements:**
- **ALWAYS** use `make build` to build the project
- **NEVER** use `go build` directly (creates unwanted binaries in project root)
- Use `GOOS=<os> go build ./cmd/nugs` ONLY for cross-compilation verification
- Use `make clean` to remove build artifacts

**Example:**

```go
// catalogUpdate fetches the latest catalog from Nugs.net API
// and updates the local cache at ~/.cache/nugs/
func catalogUpdate(jsonLevel string) error {
    startTime := time.Now()

    // Fetch catalog
    catalog, err := fetchLatestCatalog()
    if err != nil {
        return fmt.Errorf("failed to fetch catalog: %w", err)
    }

    // Write cache with file locking
    updateDuration := time.Since(startTime)
    if err := writeCatalogCache(catalog, updateDuration); err != nil {
        return fmt.Errorf("failed to write cache: %w", err)
    }

    return nil
}
```

### Pull Request Process

1. **Fork the repository**
2. **Create a feature branch:** `git checkout -b feature/your-feature`
3. **Make your changes**
4. **Test thoroughly** (see Testing section above)
5. **Commit with clear messages:**

   ```
   Add gap detection for missing shows

   - Implement catalog gaps command
   - Check both local and remote paths
   - Support --ids-only flag for piping
   ```

6. **Push to your fork:** `git push origin feature/your-feature`
7. **Open a Pull Request** with:
   - Clear description of changes
   - Test results
   - Any breaking changes noted

### Commit Guidelines

**Format:**

```
<type>: <short summary>

<detailed description>

<optional footer>
```

**Types:**
- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation changes
- `refactor:` - Code refactoring
- `test:` - Test additions
- `chore:` - Build/tooling changes

**Examples:**

```
feat: Add catalog auto-refresh system

- Implement configurable schedule (daily/weekly)
- Add timezone-aware refresh times
- Auto-update at startup if needed
- Add config commands (enable/disable/set)

Closes #123
```

```
fix: Prevent catalog cache corruption

Use POSIX file locking to prevent concurrent writes
when multiple nugs processes run simultaneously.

- Implement filelock.go with retry logic
- Add WithCacheLock() helper for catalog operations
- Use atomic write pattern (temp file + rename)
```

---

## Recent Improvements

### Video First-Class Citizen (2026-02-08)

**Implemented:**
- `defaultOutputs` config field for media type preference
- Media type modifiers for all catalog commands
- Emoji indicators for media availability (🎵 audio, 🎬 video, 📹 both)
- Video-aware gap detection and coverage
- Both-format downloads

**Features:**
- **Media Preference:** `defaultOutputs` = "audio" (default), "video", or "both"
- **Command Modifiers:** All catalog commands accept `audio`, `video`, or `both` filters
- **Visual Indicators:** Emoji symbols show media availability in list tables
- **Download Control:** `nugs grab <id> both` downloads both formats
- **Gap Detection:** Media-aware gap analysis (e.g., `nugs gaps 1125 video`)
- **Coverage Stats:** Filter coverage by media type

**Command Examples:**
```bash
# List commands with media filters
nugs list video                  # Only video artists
nugs list 1125 video             # Billy Strings videos only
nugs list 1125 both              # Shows with both formats

# Latest commands
nugs latest video                # Latest video releases
nugs latest 50 audio             # Latest 50 audio shows

# Gap detection
nugs gaps 1125 video             # Video gaps only
nugs gaps 1125 both              # Shows missing either format
nugs gaps 1125 fill video        # Download all video gaps

# Coverage statistics
nugs coverage 1125 video         # Video coverage
nugs coverage both               # Both-format coverage
```

**Configuration:**
```json
{
  "defaultOutputs": "video",
  "format": 2,
  "videoFormat": 5
}
```

**Development Patterns:**

**MediaType System:**
```go
type MediaType int
const (
    MediaTypeAudio MediaType = iota
    MediaTypeVideo
    MediaTypeBoth
)

// Parse media type from command args
mediaType := parseMediaTypeFromArgs(args)

// Filter shows by media type
filteredShows := filterShowsByMediaType(shows, mediaType)
```

**Media-Aware Analysis:**
```go
// Gap detection with media type
func findGaps(artistID int, mediaType MediaType) []Show {
    // Check local paths based on media type
    // Return shows missing requested format
}

// Coverage calculation
func calculateCoverage(artistID int, mediaType MediaType) CoverageStats {
    // Count downloads vs total for specific media type
}
```

**Command Parsing:**
```go
// Extract media type modifier from args
// "nugs list 1125 video" → artistID=1125, mediaType=Video
// "nugs gaps 1125 audio fill" → artistID=1125, mediaType=Audio, action=fill
```

**Files Modified:**
- `internal/model/structs.go` - Added `defaultOutputs` to Config, MediaType enum
- `internal/catalog/handlers.go` - Media-aware analysis functions
- `internal/catalog/media_filter.go` - Media type detection and helpers
- `internal/helpers/` - Path detection for audio/video formats
- `cmd/nugs/main.go` - Media type command parsing and download integration
- `README.md` - Comprehensive documentation with video examples
- `CLAUDE.md` - This documentation

**Benefits:**
- Video is now a first-class citizen, not an afterthought
- Users can easily filter, browse, and download videos
- Clear visual feedback on media availability
- Flexible workflows for audio-only, video-only, or both
- Consistent media type handling across all commands

### Shell Completions (2026-02-06)

**Implemented:**
- `nugs completion <shell>` - Generate shell-specific completion scripts
- Support for bash, zsh, fish, and PowerShell
- Comprehensive command, flag, and argument completions

**Features:**
- Auto-complete all commands: `list`, `catalog`, `status`, `cancel`, `completion`, `help`
- Auto-complete catalog subcommands: `update`, `cache`, `stats`, `latest`, `gaps`, `coverage`, `config`
- Auto-complete flags: `-f`, `-F`, `-o`, `--json`, `--force-video`, etc.
- Context-aware completions (e.g., format values 1-5, JSON levels)
- Shell-specific installation instructions included in output

**Files Created:**
- `internal/completion/` - Completion script generators for all supported shells

**Files Modified:**
- `cmd/nugs/main.go` - Added completion command dispatcher
- `internal/runtime/detach_common.go` - Added "completion" to read-only commands list
- `README.md` - Added Shell Completions section with installation instructions
- `CLAUDE.md` - This documentation

**Installation:**
```bash
# Bash
nugs completion bash > ~/.bash_completion.d/nugs

# Zsh (vanilla)
nugs completion zsh > ~/.zsh/completion/_nugs

# Zsh (oh-my-zsh) - most common setup
mkdir -p ~/.oh-my-zsh/custom/completions
nugs completion zsh > ~/.oh-my-zsh/custom/completions/_nugs
# Add to .zshrc BEFORE oh-my-zsh.sh: fpath=($ZSH/custom/completions $fpath)

# Fish
nugs completion fish > ~/.config/fish/completions/nugs.fish

# PowerShell
nugs completion powershell >> $PROFILE
```

**Benefits:**
- Faster command discovery and reduced typos
- Tab-complete artist IDs, format codes, and flags
- Shell-native completion behavior
- Zero dependencies (pure Go string constants)

### Artist Catalog Shortcuts (2026-02-05)

**Implemented:**
- `nugs <artist_id> full` - Download entire artist catalog
- `nugs grab <artist_id> latest` - Download latest shows

**Improved UX:**
- Before: `nugs https://play.nugs.net/#/artist/461`
- After: `nugs 461 full`

**Implementation:**
- Added shorthand parser in `cmd/nugs/main.go`
- Constructs full artist URL: `https://play.nugs.net/#/artist/{id}`
- Displays message: "Downloading entire catalog from artist {id}"

### Catalog Caching System (2026-02-05)

**Implemented:**
- Local catalog caching at `~/.cache/nugs/`
- Four index files for fast lookups
- Five catalog commands (update, cache, stats, latest, gaps)
- Auto-refresh with configurable schedule
- Gap detection with --ids-only flag
- JSON output for all catalog commands
- File locking for concurrent safety

**Files Created:**
- `internal/catalog/handlers.go` - Catalog command implementations
- `internal/catalog/autorefresh.go` - Auto-refresh logic
- `internal/cache/filelock_unix.go` - POSIX file locking

**Files Modified:**
- `internal/model/structs.go` - Added cache structures and config fields
- `cmd/nugs/main.go` - Added catalog dispatcher and cache I/O
- `README.md` - Comprehensive catalog documentation

**Session Docs:**
- `.docs/sessions/2026-02-05-catalog-caching-implementation.md`
- `.docs/sessions/2026-02-05-improvements-summary.md`
- `.docs/catalog-future-enhancements-plan.md`

### Code Quality Improvements (2026-02-05)

**Replaced deprecated APIs:**
- `ioutil.ReadFile` → `os.ReadFile` (Go 1.16+)
- `ioutil.WriteFile` → `os.WriteFile` (Go 1.16+)
- 10 occurrences updated across codebase

**Added concurrent safety:**
- POSIX file locking with `syscall.Flock`
- Atomic write operations (temp file + rename)
- Retry logic for lock acquisition
- Grade improved from A- to A

### Rclone Configuration Clarification (2026-02-05)

**Important Behavioral Note:**
The `rclonePath` configuration field in `config.json` specifies the **remote storage path** only. It does NOT affect local download paths.

**Configuration Behavior:**
- **Local downloads:** Always go to `outPath` (e.g., `/home/user/Music`)
- **Remote uploads:** Go to `rcloneRemote:rclonePath` (e.g., `gdrive:/Music`)
- **Artist folder structure:** Preserved in both local and remote locations

**Example:**

```json
{
  "outPath": "/home/user/Music",
  "rclonePath": "/Music",
  "rcloneRemote": "gdrive"
}
```

Downloads create: `/home/user/Music/Artist Name/Album/`
Uploads to: `gdrive:/Music/Artist Name/Album/`

**Code Reference:**
- See `internal/model/structs.go` for field documentation
- See `internal/rclone/` for remote path construction and upload logic

---

### Breaking Changes & Migration Notes

#### rclonePath Behavior Change (2026-02-05)

**⚠️ BREAKING CHANGE:** The `rclonePath` field no longer affects local download paths.

**Previous Behavior (before 2026-02-05):**

```go
// rclonePath was used as a fallback for local base path
basePath := cfg.OutPath
if cfg.RclonePath != "" {
    basePath = cfg.RclonePath  // ❌ Confusing dual-purpose
}
```

**New Behavior (after 2026-02-05):**

```go
// rclonePath ONLY affects remote storage uploads
basePath := cfg.OutPath  // Always use outPath for local downloads
remotePath := cfg.RclonePath  // Only for remote storage
```

**Migration Guide:**

If you previously set `rclonePath` expecting it to control local download locations:

1. **Update your config:**
   - Move your desired local path to `outPath`
   - Keep `rclonePath` for the remote storage path only

2. **Example migration:**

   ```json
   // OLD CONFIG (relied on rclonePath for local paths)
   {
     "outPath": "/tmp/music",
     "rclonePath": "/mnt/user/data/media/music",
     "rcloneEnabled": false
   }

   // NEW CONFIG (explicit local path in outPath)
   {
     "outPath": "/mnt/user/data/media/music",
     "rclonePath": "/Music",
     "rcloneEnabled": true,
     "rcloneRemote": "gdrive"
   }
   ```

3. **Impact:**
   - Local-only users: Update `outPath` to your preferred download location
   - Rclone users: `outPath` for local, `rclonePath` for remote (clear separation)

**Rationale:**
This change eliminates a confusing "leaky abstraction" where a field named "rclone**Path**" (implying remote storage) was also controlling local filesystem behavior. The new design provides clear separation of concerns: `outPath` = local, `rclonePath` = remote.

---

## Common Gotchas & Non-Obvious Patterns

### Concurrency & State Management

**1. Progress Box Must Be Locked**
```go
// ❌ BAD - Race condition
progressBox.DownloadPercent = 50

// ✅ GOOD - Thread-safe
progressBox.Mu.Lock()
progressBox.DownloadPercent = 50
progressBox.Mu.Unlock()
```

**2. Cache Writes Require File Locking**
```go
// ❌ BAD - Can corrupt cache with concurrent access
os.WriteFile(cachePath, data, 0644)

// ✅ GOOD - Atomic write with file locking
err := cache.WithCacheLock(func() error {
    tmpPath := cachePath + ".tmp"
    os.WriteFile(tmpPath, data, 0644)
    return os.Rename(tmpPath, cachePath)  // Atomic!
})
```

**3. Single-Threaded Download Loop**
- Tracks download sequentially (NOT parallel)
- Prevents API rate limiting
- Progress box not designed for concurrent track updates
- **Exception:** Size pre-calculation uses 8 concurrent HEAD requests

### Configuration Quirks

**4. Config Search Order Matters**
```go
// Searches in order:
// 1. ~/.nugs/config.json
// 2. ~/.config/nugs/config.json
// First found wins - no merging!
```

**5. Security Auto-Fix**
- Config permissions checked on every read
- Unix: Auto-fixes to `0600` if insecure
- Windows: Permission check skipped (relies on NTFS ACLs)

**6. Token Prefix Stripping**
```go
// Automatically strips "Bearer " prefix from tokens
// This is intentional for API compatibility
cfg.Token = strings.TrimPrefix(cfg.Token, "Bearer ")
```

### API & Network

**7. Context Propagation Is Required**
```go
// ❌ BAD - Don't create context in libraries
func FetchCatalog() (*Catalog, error) {
    ctx := context.Background()  // ❌ Wrong
    // ...
}

// ✅ GOOD - Accept context from caller
func FetchCatalog(ctx context.Context) (*Catalog, error) {
    // Can respect cancellation/timeout
}
```

**8. Crawl Cancellation Pattern**
```go
// Check pause/cancel state periodically in loops:
if deps.WaitIfPausedOrCancelled != nil {
    if err := deps.WaitIfPausedOrCancelled(); err != nil {
        return err  // Returns ErrCrawlCancelled
    }
}

// Detect cancellation in caller:
if deps.IsCrawlCancelledErr != nil && deps.IsCrawlCancelledErr(err) {
    return err  // Stop processing, bubble up
}
```

**9. Format Fallback Chain**
- ALAC (1) → FLAC (2) → AAC (5)
- MQA (3) → FLAC (2)
- 360RA (4) → MQA (3)
- Max 10 fallback attempts (prevents infinite loops)

### File System

**10. Path Sanitization**
```go
// ❌ BAD - Unsafe filename
filename := track.SongTitle  // May contain / or other invalid chars

// ✅ GOOD - Sanitized
filename := helpers.Sanitise(track.SongTitle)  // Removes /, \, :, etc.
```

**11. Video Output Path Defaults**
```go
// If videoOutPath is empty, it defaults to outPath
// This is NOT a config error - it's intentional
if cfg.VideoOutPath == "" {
    cfg.VideoOutPath = cfg.OutPath  // Same directory as audio
}
```

**12. Rclone Path Confusion**
```go
// Local downloads: ALWAYS use outPath/videoOutPath
// Remote uploads: ONLY use rclonePath/rcloneVideoPath
// These are separate - rclonePath does NOT affect local downloads
```

### Progress & UI

**13. Phase Transition Validation**
```go
// Progress box validates phase transitions:
// Download → Upload ✅
// Upload → Download ❌ Invalid!
// Complete → Upload ❌ Invalid!
// Use SetPhase() for validation, not direct assignment
progressBox.SetPhase(model.PhaseUpload)
```

**14. Message Priority System**
```go
// Lower priority messages are overwritten by higher priority:
MessagePriorityStatus   = 0  // Normal status
MessagePriorityWarning  = 1  // Warnings (e.g., format fallback)
MessagePriorityError    = 2  // Errors (highest priority)

// Setting status won't overwrite existing warning/error
```

**15. Render Throttling**
```go
// Progress box throttles renders to 100ms by default
// Use ForceRender = true to bypass throttle:
progressBox.Mu.Lock()
progressBox.UploadPercent = 100
progressBox.ForceRender = true  // Force immediate render
progressBox.Mu.Unlock()
```

### Build & Platform

**16. Build Tags Are Critical**
```go
// ❌ BAD - Will compile for all platforms
func AcquireLock(path string) error

// ✅ GOOD - Platform-specific implementations
// filelock_unix.go:
//go:build !windows

// filelock_windows.go:
//go:build windows
```

**17. Go Build vs Make Build**
```bash
# ❌ BAD - Creates binary in project root
go build ./cmd/nugs

# ✅ GOOD - Installs to ~/.local/bin/nugs
make build
```

### Testing

**18. Use testutil Helpers**
```go
// ❌ BAD - Duplicating helpers in each test file
func withTempHome(t *testing.T) string { ... }

// ✅ GOOD - Use shared testutil package
import "github.com/jmagar/nugs-cli/internal/testutil"
tempHome := testutil.WithTempHome(t)
```

**19. t.Helper() for Test Helpers**
```go
func myHelper(t *testing.T) {
    t.Helper()  // ← IMPORTANT: Marks this as helper
    // Errors will show caller's line, not helper's line
}
```

### Deps Pattern

**20. Root Package Callbacks**
```go
// Tier 3 packages (catalog, download, list) cannot import cmd/nugs
// They define Deps structs with function callbacks:

type Deps struct {
    UploadToRclone func(localPath, artistFolder string, ...) error
    RenderProgressBox func(box *ProgressBoxState)
    // ... more callbacks
}

// Root package wires them up:
deps := &download.Deps{
    UploadToRclone: uploadToRclone,  // Root function
    RenderProgressBox: renderProgressBox,  // Root function
}

// Pass Deps to every function that needs root callbacks
download.Album(ctx, albumID, cfg, streamParams, deps)
```

---

## Future Enhancements

See `.docs/catalog-future-enhancements-plan.md` for detailed roadmap including:

**Phase 1 (Quick Wins)**
- Catalog snapshots and version management
- Advanced search and filtering
- Compression for faster updates

**Phase 2 (Power Features)**
- Catalog comparison (diff between dates)
- Batch operations and bulk downloads
- Enhanced statistics and analytics

**Phase 3 (Integrations)**
- Integration with music players (Plex, Subsonic)
- Webhook support for new releases
- Database backend option (PostgreSQL)

---

## Troubleshooting Development

### Build Issues

**"command not found: make"**

```bash
# Install make
sudo apt install make  # Linux
brew install make      # macOS
```

**"package not found"**

```bash
# Update Go modules
go mod tidy
go mod download
```

### Runtime Issues

**Config file location confusion:**
- Auto-refresh config commands write to `./config.json` (current directory)
- Main app reads from `./config.json` OR `~/.nugs/config.json`
- Keep config in current directory during development

**Cache permission errors:**

```bash
# Fix cache directory permissions
chmod -R 755 ~/.cache/nugs
```

**File lock timeout:**
- Default timeout: 5 seconds (50 retries × 100ms)
- Increase retries in `AcquireLock()` if needed
- Check for stale lock files: `~/.cache/nugs/.catalog.lock`

---

## Resources

- [Nugs.net API Documentation](https://play.nugs.net/api/v1/)
- [Go Documentation](https://golang.org/doc/)
- [Rclone Documentation](https://rclone.org/docs/)
- [FFmpeg Documentation](https://ffmpeg.org/documentation.html)

---

## License

See repository license for details.

## Contact

- GitHub: [@jmagar](https://github.com/jmagar)
- Issues: [GitHub Issues](https://github.com/jmagar/nugs-cli/issues)
