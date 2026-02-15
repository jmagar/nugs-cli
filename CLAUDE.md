# Nugs CLI Development Guide

This is the main development guide for contributors. For specialized topics, see the detailed guides below.

## 📚 Documentation Index

- **[ARCHITECTURE.md](./docs/ARCHITECTURE.md)** - Package structure, dependencies, design patterns
- **[CONFIG.md](./docs/CONFIG.md)** - Complete configuration reference (all 23 fields)
- **[README.md](./README.md)** - User documentation and command reference
- **[COMMANDS.md](./docs/COMMANDS.md)** - Comprehensive command examples

## Table of Contents

- [Quick Start](#quick-start)
- [Architecture Overview](#architecture-overview)
- [Building from Source](#building-from-source)
- [Development Workflow](#development-workflow)
- [Download Flow](#download-flow-critical-path)
- [Testing](#testing)
- [Common Gotchas](#common-gotchas)
- [Recent Improvements](#recent-improvements)
- [Troubleshooting](#troubleshooting)

---

## Quick Start

**Prerequisites:**
- Go 1.16 or later
- FFmpeg (for video downloads)
- Make (required for building)

**Build and test:**
```bash
# Build (ALWAYS use make, never go build directly)
make build

# Run tests
make test
go test ./... -count=1 -v

# Test download
nugs 23329

# Test catalog
nugs update
nugs stats
nugs gaps 1125
```

**Test Data:**
- **Artist IDs:** `1125` (Billy Strings), `461` (Grateful Dead), `1045` (Dead & Company)
- **Show IDs:** `23329`, `23790`, `24105`

---

## Architecture Overview

Nugs CLI is a Go-based monorepo with a clean 4-tier architecture:

### High-Level Structure

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
- **No Circular Dependencies** - Strict upward dependency flow

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
- Four index files for fast lookups
- Cache location: `~/.cache/nugs/`
- File locking for concurrent safety

**Auto-Refresh**
- Configurable schedule (daily/weekly)
- Timezone-aware refresh times
- Automatic updates at startup if needed

**For detailed architecture:** See [ARCHITECTURE.md](./docs/ARCHITECTURE.md)

**For configuration details:** See [CONFIG.md](./docs/CONFIG.md)

---

## Building from Source

### ⚠️ Build Command Rule

**CRITICAL: Always use `make build` - Never use `go build` directly**

- ❌ `go build ./cmd/nugs` - Creates binaries in project root (nugs, nugs.exe, etc.)
- ✅ `make build` - Correctly builds to `~/.local/bin/nugs`

**Why:** Direct `go build` commands create binaries in the project root which clutter the workspace even though they're gitignored. The Makefile handles the correct build target and output location.

### Build Commands

**Using Make (REQUIRED):**

```bash
make build          # Build and install to ~/.local/bin/nugs
make clean          # Remove build artifacts
make test           # Run all tests
```

This builds the binary and installs it to `~/.local/bin/nugs`.

**Verification commands (cross-compilation checks ONLY):**

```bash
# These commands are for verification only - use make build for actual building
GOOS=darwin go build ./cmd/nugs        # macOS cross-compile check
GOOS=windows go build ./cmd/nugs       # Windows cross-compile check
GOOS=linux GOARCH=arm64 go build ./cmd/nugs  # Linux ARM check
```

**Note:** Cross-compilation still creates a local binary in the current directory (or the `-o` path), but it will not be runnable on the current OS. Clean up with `make clean`.

---

## Development Workflow

### Where New Code Goes

| Feature Type | Package | Example |
|-------------|---------|---------|
| API endpoint | `internal/api/` | New Nugs.net API method |
| Data type | `internal/model/` | New response struct |
| Path logic | `internal/helpers/` | Filename sanitization |
| Display | `internal/ui/` | Progress rendering |
| Catalog cmd | `internal/catalog/` | New gap analysis |
| Download | `internal/download/` | New format support |
| Config field | `internal/config/` | New option parsing |
| Cloud upload | `internal/rclone/` | Upload verification |
| Process ctrl | `internal/runtime/` | New signal handler |
| Cache logic | `internal/cache/` | Index optimization |

### Adding New Functionality

1. Determine which package owns the feature
2. Add code to appropriate `internal/` package
3. Import from `cmd/nugs/main.go` if user-facing
4. Update package `deps.go` if adding new dependencies

**For detailed patterns:** See [ARCHITECTURE.md - Where New Code Goes](./ARCHITECTURE.md#where-new-code-goes)

---

## Download Flow (Critical Path)

### End-to-End Flow

```text
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

### Format Selection

**Audio formats (1-5):**
- 1 = 16-bit 44.1kHz ALAC
- 2 = 16-bit 44.1kHz FLAC (most common)
- 3 = 24-bit 48kHz MQA
- 4 = 360 Reality Audio (default)
- 5 = 150 Kbps AAC (HLS-only fallback)

**Fallback chain:** ALAC→FLAC→AAC, MQA→FLAC, 360RA→MQA

**Video formats (1-5):**
- 1 = 480p
- 2 = 720p
- 3 = 1080p
- 4 = 1440p
- 5 = 4K / best available (default)

**Fallback chain:** 1440p→1080p, 1080p→720p, 720p→480p

### FFmpeg Integration

**Audio (HLS-only streams):**
```bash
# Decrypt AES-128 encrypted .ts → Convert to AAC
ffmpeg -i pipe: -c:a copy output.m4a
```

**Video (TS to MP4 conversion):**
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

### Progress Tracking

**Global progress box state** (`model.ProgressBoxState`):
- **Show-level:** percent, downloaded/total, track count
- **Track-level:** track number, name, format, percent, speed, ETA
- **Upload-level:** percent, uploaded/total, speed, ETA, duration
- **Phase transitions:** Download → Upload → Complete
- **Thread-safe:** Protected by `Mu` (sync.Mutex)

### Concurrent Operations

**Sequential track downloads** (NOT parallel):
- Prevents API rate limits
- Single progress box per album
- **Exception:** Size pre-calculation uses 8 concurrent HEAD requests (60s timeout)

### Error Handling

**Track-level errors** (batch continues):
- Network timeouts → logged, continue to next track
- Format unavailable → fallback to next format

**Album-level errors** (track continues):
- Invalid show ID → error, skip to next album
- Missing FFmpeg → error, skip video conversion

**Crawl-level errors** (fatal):
- User cancellation (Shift-C) → stop all downloads
- Authentication failure → exit immediately

---

## Testing

### Current Test Coverage

Run `go test ./... -cover` to get the latest coverage report.

**Well-tested components:**
- Media type detection
- Upload progress tracking
- FFmpeg binary resolution

**Key areas needing coverage:**
- `internal/api` - Authentication, API calls
- `internal/download` - Download logic, FFmpeg
- `internal/cache` - File locking, concurrency
- `internal/catalog` - Catalog operations
- `internal/config` - Config loading

### Test Utilities

**Shared helpers** (`internal/testutil/testutil.go`):
- `WithTempHome(t)` - Isolate HOME directory
- `CaptureStdout(t, fn)` - Capture stdout
- `ChdirTemp(t)` - Change to temp directory with cleanup
- `WriteExecutable(t, path)` - Create executable script

### Testing Patterns

**Table-driven tests** (95% of tests):
```go
func TestParseMediaModifier(t *testing.T) {
    tests := []struct {
        name          string
        args          []string
        wantMediaType MediaType
        wantRemaining []string
    }{
        {
            name:          "audio modifier first position",
            args:          []string{"audio", "1125", "fill"},
            wantMediaType: MediaTypeAudio,
            wantRemaining: []string{"1125", "fill"},
        },
        // ... more test cases
    }

    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            gotMediaType, gotRemaining := parseMediaModifier(tc.args)
            // ... assertions
        })
    }
}
```

**Key principles:**
- Fast unit tests (no network/API calls, completes in ~0.007s)
- Test isolation (`t.TempDir()`, `t.Setenv()`, `t.Cleanup()`)
- No mocking framework - direct testing with real data structures

### Next Testing Priorities

1. **API client tests** with `httptest.Server` (mock responses)
2. **Cache concurrency tests** (file locking, corruption recovery)
3. **Config validation tests** (all 23 fields)
4. **Download flow tests** with small test files (<1MB)

### Manual Testing

```bash
# Automated tests
make test                    # All tests
go test ./... -v             # Verbose
go test ./... -cover         # With coverage
go test ./... -race          # Race detector
go test -bench=. ./cmd/nugs  # Benchmarks

# Functional tests
nugs 23329                   # Download
nugs update                  # Catalog
nugs stats                   # Statistics
nugs gaps 1125               # Gap detection
nugs refresh set             # Auto-refresh
```

### Edge Cases to Test

- Empty config file
- Invalid credentials
- Missing FFmpeg (error: "ffmpeg not found in PATH")
- Concurrent catalog updates (file locking)
- Network interruptions (download resume)
- Invalid artist/show IDs (error handling)
- Rclone not installed when enabled
- Cache corruption (fallback to API)
- Disk full (error: "no space left on device")
- Insecure config permissions (auto-fix warning)
- Invalid config values (validation errors)

---

## Common Gotchas

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
// Searches in order (first found wins, no merging):
// 1. ./config.json          (current directory)
// 2. ~/.nugs/config.json
// 3. ~/.config/nugs/config.json
```

**5. Security Auto-Fix**
- Config permissions checked on every read
- Unix: Auto-fixes to `0600` if insecure
- Windows: Permission check skipped (relies on NTFS ACLs)

**6. Token Prefix Stripping**
```go
// Automatically strips "Bearer " prefix from tokens
cfg.Token = strings.TrimPrefix(cfg.Token, "Bearer ")
```

### API & Network

**7. Context Propagation Is Required**
```go
// ❌ BAD - Don't create context in libraries
func FetchCatalog() (*Catalog, error) {
    ctx := context.Background()  // ❌ Wrong
}

// ✅ GOOD - Accept context from caller
func FetchCatalog(ctx context.Context) (*Catalog, error) {
    // Can respect cancellation/timeout
}
```

**8. Format Fallback Chain**
- ALAC (1) → FLAC (2) → AAC (5)
- MQA (3) → FLAC (2)
- 360RA (4) → MQA (3)
- Max 10 fallback attempts (prevents infinite loops)

### File System

**9. Path Sanitization**
```go
// ❌ BAD - Unsafe filename
filename := track.SongTitle  // May contain / or other invalid chars

// ✅ GOOD - Sanitized
filename := helpers.Sanitise(track.SongTitle)
```

**10. Rclone Path Confusion**
```go
// Local downloads: ALWAYS use outPath/videoOutPath
// Remote uploads: ONLY use rclonePath/rcloneVideoPath
// These are separate - rclonePath does NOT affect local downloads
```

### Build & Platform

**11. Build Tags Are Critical**
```go
// ❌ BAD - Will compile for all platforms
func AcquireLock(path string) error

// ✅ GOOD - Platform-specific implementations
// filelock_unix.go:
//go:build !windows

// filelock_windows.go:
//go:build windows
```

**12. Go Build vs Make Build**
```bash
# ❌ BAD - Creates binary in project root
go build ./cmd/nugs

# ✅ GOOD - Installs to ~/.local/bin/nugs
make build
```

**For more gotchas:** See [ARCHITECTURE.md - Common Gotchas](./docs/ARCHITECTURE.md#common-gotchas)

---

## Recent Improvements

### Video First-Class Citizen (2026-02-08)

**Implemented:**
- `defaultOutputs` config field for media type preference
- Media type modifiers for all catalog commands
- Emoji indicators for media availability (🎵 audio, 🎬 video, 📹 both)
- Video-aware gap detection and coverage
- Both-format downloads

**Command Examples:**
```bash
# List commands with media filters
nugs list video                  # Only video artists
nugs list 1125 video             # Billy Strings videos only

# Gap detection
nugs gaps 1125 video             # Video gaps only
nugs gaps 1125 both              # Shows missing either format
nugs gaps 1125 fill video        # Download all video gaps
```

**Files Modified:**
- `internal/model/structs.go` - Added `defaultOutputs` to Config, MediaType enum
- `internal/catalog/handlers.go` - Media-aware analysis functions
- `internal/catalog/media_filter.go` - Media type detection and helpers
- `internal/helpers/` - Path detection for audio/video formats
- `cmd/nugs/main.go` - Media type command parsing and download integration

### Shell Completions (2026-02-06)

**Implemented:**
- `nugs completion <shell>` - Generate shell-specific completion scripts
- Support for bash, zsh, fish, and PowerShell
- Comprehensive command, flag, and argument completions

**Installation:**
```bash
# Zsh (oh-my-zsh) - most common setup
mkdir -p ~/.oh-my-zsh/custom/completions
nugs completion zsh > ~/.oh-my-zsh/custom/completions/_nugs
# Add to .zshrc BEFORE oh-my-zsh.sh: fpath=($ZSH/custom/completions $fpath)

# Bash
nugs completion bash > ~/.bash_completion.d/nugs

# Fish
nugs completion fish > ~/.config/fish/completions/nugs.fish
```

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

### Code Quality Improvements (2026-02-05)

**Replaced deprecated APIs:**
- `ioutil.ReadFile` → `os.ReadFile` (Go 1.16+)
- `ioutil.WriteFile` → `os.WriteFile` (Go 1.16+)
- 10 occurrences updated across codebase

**Added concurrent safety:**
- POSIX file locking with `syscall.Flock`
- Atomic write operations (temp file + rename)
- Retry logic for lock acquisition

---

## Breaking Changes & Migrations

### rclonePath Behavior Change (2026-02-05)

**⚠️ BREAKING CHANGE:** The `rclonePath` field no longer affects local download paths.

**Before:** `rclonePath` was used as a fallback for local base path
**After:** `rclonePath` ONLY affects remote storage uploads

**Migration:**
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

**See [CONFIG.md - Migrations](./docs/CONFIG.md#migrations) for full details.**

---

## Troubleshooting

### Build Issues

**"command not found: make"**
```bash
sudo apt install make  # Linux
brew install make      # macOS
```

**"package not found"**
```bash
go mod tidy
go mod download
```

### Runtime Issues

**Config file location confusion:**
- Auto-refresh config commands write to `~/.nugs/config.json`
- Main app reads from `./config.json`, `~/.nugs/config.json`, or `~/.config/nugs/config.json` (first found wins)

**Cache permission errors:**
```bash
chmod -R 755 ~/.cache/nugs
```

**File lock timeout:**
- Default timeout: 5 seconds (50 retries × 100ms)
- Check for stale lock files: `~/.cache/nugs/.catalog.lock`

**FFmpeg not found:**
```bash
# Install FFmpeg
sudo apt install ffmpeg  # Linux
brew install ffmpeg      # macOS

# Or set custom path in config.json:
"ffmpegNameStr": "/path/to/ffmpeg"
```

**Rclone not found:**

```bash
# Install rclone (see https://rclone.org/downloads/ for alternatives)
# Review the script before running:
curl -O https://rclone.org/install.sh
sudo bash install.sh

# Or disable in config.json:
"rcloneEnabled": false
```

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
