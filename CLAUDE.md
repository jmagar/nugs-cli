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

```
nugs/
├── main.go                    # Entry point, CLI dispatcher, download logic
├── structs.go                 # Data structures, config, API responses
├── catalog_handlers.go        # Catalog commands (update, cache, stats, latest, gaps)
├── catalog_autorefresh.go     # Auto-refresh logic and configuration
├── filelock.go                # POSIX file locking for concurrent safety
├── config.json                # User configuration (gitignored)
├── README.md                  # User documentation
├── CLAUDE.md                  # This file - development documentation
├── .docs/                     # Session logs and planning docs
│   ├── sessions/              # Development session documentation
│   ├── deployment-log.md      # Deployment history
│   └── services-ports.md      # Port registry
├── .cache/                    # Temporary files (gitignored)
└── .gitignore                 # Git ignore rules
```

### Key Files

**main.go** (~2900 lines)
- CLI argument parsing
- Command dispatcher
- Download orchestration
- API client functions
- Cache I/O functions
- List commands (artists, shows)

**structs.go** (~200 lines)
- Configuration struct (`Config`)
- API response structs (`LatestCatalogResp`, `ArtistMeta`, `ShowMeta`)
- Cache metadata (`CacheMeta`)
- CLI arguments (`Args`)

**catalog_handlers.go** (375 lines)
- `catalogUpdate()` - Update catalog cache
- `catalogCacheStatus()` - View cache information
- `catalogStats()` - Show catalog statistics
- `catalogLatest()` - Display latest additions
- `catalogGaps()` - Find missing shows in collection

**catalog_autorefresh.go** (220 lines)
- `shouldAutoRefresh()` - Check if refresh needed
- `autoRefreshIfNeeded()` - Execute auto-refresh if conditions met
- `enableAutoRefresh()` - Enable auto-refresh with defaults
- `disableAutoRefresh()` - Disable auto-refresh
- `configureAutoRefresh()` - Interactive configuration

**filelock.go** (107 lines)
- `AcquireLock()` - POSIX file locking with retry logic
- `Release()` - Release acquired lock
- `WithCacheLock()` - Helper for catalog cache locking

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

**Config File Location:**

Config files are searched in this order:
1. `./config.json` (current directory)
2. `~/.nugs/config.json` (recommended, user home)
3. `~/.config/nugs/config.json` (XDG standard)

**Reading Config:**

```go
func readConfig() (*Config, error) {
    // Checks all three locations in order
    // Returns first found config
    // Warns if permissions are insecure (not 0600)
}
```

**Default Config Values:**
- `catalogAutoRefresh`: `true` (enabled by default)
- `catalogRefreshTime`: `"05:00"` (5am)
- `catalogRefreshTimezone`: `"America/New_York"` (EST)
- `catalogRefreshInterval`: `"daily"`

**Writing Config:**

```go
func writeConfig(cfg *Config) error {
    // Always writes to ./config.json
    // Used by auto-refresh config commands
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

### Rclone Integration

**Upload after download:**

```go
if cfg.RcloneEnabled {
    remotePath := cfg.RclonePath + "/" + albumFolder
    cmd := exec.Command("rclone", "copy", localPath, remotePath,
        "--transfers", strconv.Itoa(cfg.RcloneTransfers))
    cmd.Run()

    if cfg.DeleteAfterUpload {
        os.RemoveAll(localPath)
    }
}
```

**Gap detection with rclone:**

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

---

## Testing

### Manual Testing

**Build and test:**

```bash
# Build (ALWAYS use make, never go build directly)
make build

# Verify tests pass
go test ./... -count=1

# Verify linting
go vet ./...

# Test download
nugs 23329

# Test catalog
nugs update
nugs stats
nugs gaps 1125

# Test auto-refresh
nugs refresh set
```

### Test Data

**Artist IDs for testing:**
- `1125` - Billy Strings (430 shows)
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
- Missing FFmpeg
- Concurrent catalog updates
- Network interruptions
- Invalid artist IDs
- Invalid show IDs
- Rclone not installed (when enabled)
- Cache corruption
- Disk full

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
- `structs.go` - Added `defaultOutputs` to Config, MediaType enum
- `catalog_handlers.go` - Media-aware analysis functions
- `format.go` - Media type detection and helpers
- `helpers.go` - Path detection for audio/video formats
- `main.go` - Media type command parsing and download integration
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
- `completions.go` (430 lines) - Completion script generators for all supported shells

**Files Modified:**
- `main.go` - Added completion command dispatcher (line ~3885)
- `detach_common.go` - Added "completion" to read-only commands list
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
- Added shorthand parser in main.go (lines 2846-2861)
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
- `catalog_handlers.go` (375 lines)
- `catalog_autorefresh.go` (220 lines)
- `filelock.go` (107 lines)

**Files Modified:**
- `structs.go` - Added cache structures and config fields
- `main.go` - Added catalog dispatcher and cache I/O
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
- See `structs.go` line 61 for field documentation
- See `uploadToRclone()` in `main.go` for remote path construction

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
