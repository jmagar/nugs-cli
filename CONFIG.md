# Nugs CLI Configuration Reference

Complete reference for all configuration options in Nugs CLI.

## Table of Contents

- [Config Structure](#config-structure)
- [File Locations](#file-locations)
- [Field Reference](#field-reference)
  - [Authentication](#authentication)
  - [Download Quality](#download-quality)
  - [Output Paths](#output-paths)
  - [FFmpeg Integration](#ffmpeg-integration)
  - [Rclone Cloud Uploads](#rclone-cloud-uploads)
  - [Catalog Auto-Refresh](#catalog-auto-refresh)
  - [Performance](#performance)
- [Validation Rules](#validation-rules)
- [Security](#security)
- [Examples](#examples)
- [Migrations](#migrations)
- [Environment Variables](#environment-variables)

---

## Config Structure

The `Config` struct in `internal/model/types.go` contains **23 fields** organized by category:

| Category | Fields | Purpose |
|----------|--------|---------|
| Authentication | 3 | Nugs.net account credentials |
| Download Quality | 4 | Audio/video format selection |
| Output Paths | 3 | Local download directories |
| FFmpeg Integration | 4 | Video processing |
| Rclone Cloud Uploads | 6 | Cloud storage automation |
| Catalog Auto-Refresh | 4 | Automatic catalog updates |
| Performance | 1 | Optimization flags |

**Total:** 23 configuration fields

---

## File Locations

### Search Order

Config files are searched in this order (first found wins):

1. `~/.nugs/config.json` (recommended)
2. `~/.config/nugs/config.json` (XDG standard)

**No merging:** Only the first found config is used.

### File Creation

**First-time setup writes to:** `~/.nugs/config.json`

**Subsequent writes use:** Same file that was read (`config.LoadedConfigPath`)

**Permissions:**
- Directory: `0700` (owner-only access)
- File: `0600` (owner read/write only)

### Security Auto-Fix

On every config read:
- **Unix:** Auto-fixes insecure permissions (warns and sets to `0600`)
- **Windows:** Permission check skipped (relies on NTFS ACLs)

---

## Field Reference

### Authentication

#### `email` (string)

**Purpose:** Nugs.net account email for OAuth authentication

**Required:** Yes (unless using `token`)

**Example:** `"user@example.com"`

**Validation:** None

**Security:** ⚠️ Stored in plain text

**Interactive prompt:** "Enter your Nugs email:"

---

#### `password` (string)

**Purpose:** Nugs.net account password for OAuth authentication

**Required:** Yes (unless using `token`)

**Example:** `"your_password_here"`

**Validation:** None

**Security:** ⚠️ Stored in plain text, **visible during interactive setup**

**Interactive prompt:** "Enter your Nugs password:"

**Workaround:** Use `token` authentication instead (see `docs/token.md`)

---

#### `token` (string)

**Purpose:** Pre-existing auth token for Apple/Google accounts or to avoid password storage

**Required:** Alternative to `email`/`password`

**Example:** `"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`

**Validation:** "Bearer " prefix automatically stripped

**Security:** ⚠️ Stored in plain text (but not visible during input)

**How to obtain:** See `docs/token.md` for extraction from Nugs.net web client

**Note:** If both `token` and `email`/`password` are set, token takes precedence

---

### Download Quality

#### `format` (int 1-5)

**Purpose:** Audio quality selection

**Default:** `4` (360 Reality Audio / best available)

**Options:**
- `1` - 16-bit 44.1kHz ALAC (Apple Lossless)
- `2` - 16-bit 44.1kHz FLAC (most common lossless)
- `3` - 24-bit 48kHz MQA (Master Quality Authenticated)
- `4` - 360 Reality Audio / best available (default)
- `5` - 150 Kbps AAC (HLS-only fallback)

**Validation:** Must be between 1 and 5

**Error message:** `"track Format must be between 1 and 5"`

**Fallback chain:**
- ALAC (1) → FLAC (2) → AAC (5)
- MQA (3) → FLAC (2)
- 360RA (4) → MQA (3)

**Interactive prompt:** Dropdown with format descriptions

---

#### `videoFormat` (int 1-5)

**Purpose:** Video resolution selection

**Default:** `5` (4K / best available)

**Options:**
- `1` - 480p
- `2` - 720p
- `3` - 1080p
- `4` - 1440p
- `5` - 4K / best available (default)

**Validation:** Must be between 1 and 5

**Error message:** `"video format must be between 1 and 5"`

**Fallback chain:**
- 1440p (4) → 1080p (3)
- 1080p (3) → 720p (2)
- 720p (2) → 480p (1)

**Interactive prompt:** Dropdown with resolution options

---

#### `defaultOutputs` (string)

**Purpose:** Media type preference for downloads

**Default:** `"audio"` (download audio only)

**Options:**
- `"audio"` - Download audio only
- `"video"` - Download video only
- `"both"` - Download both audio and video

**Validation:** Must be "audio", "video", or "both"

**Error message:** `"invalid defaultOutputs: "<value>" (must be audio, video, or both)"`

**Overrides:** Can be overridden per-command with `audio`, `video`, or `both` modifiers

**Examples:**
```bash
# Use config default
nugs 23329

# Override to video
nugs 23329 video

# Override to both
nugs 23329 both
```

---

#### `wantRes` (string)

**Purpose:** Computed resolution string from `videoFormat`

**Default:** Computed automatically

**Values:**
- `videoFormat=1` → `wantRes="854x480"`
- `videoFormat=2` → `wantRes="1280x720"`
- `videoFormat=3` → `wantRes="1920x1080"`
- `videoFormat=4` → `wantRes="2560x1440"`
- `videoFormat=5` → `wantRes="2160"` (4K, matches any 4K variant)

**Note:** Read-only, do not set manually

---

### Output Paths

#### `outPath` (string)

**Purpose:** Local download directory for audio files

**Default:** `"Nugs downloads"` (relative to current directory)

**Example:** `"/home/user/Music/Nugs"`

**Validation:** None (directory created if doesn't exist)

**Interactive prompt:** "Enter output path for audio downloads:"

**Note:** This is the **local** download path, NOT affected by `rclonePath`

---

#### `videoOutPath` (string)

**Purpose:** Local download directory for video files

**Default:** Same as `outPath` (audio and video in same location)

**Example:** `"/home/user/Videos/Nugs"`

**Validation:** None (directory created if doesn't exist)

**Interactive prompt:** Not prompted (defaults to `outPath`)

**Note:** If empty, automatically set to `outPath` value

---

#### `urls` ([]string)

**Purpose:** Parsed URLs from CLI arguments

**Default:** `nil` (empty array)

**Example:** `["https://play.nugs.net/#/catalog/recording/23329"]`

**Note:** Internal field, populated by CLI parser, not user-configurable

---

### FFmpeg Integration

#### `useFfmpegEnvVar` (bool)

**Purpose:** Whether to use FFmpeg from system PATH vs local binary

**Default:** `false` (interactive setup), varies at runtime

**Options:**
- `true` - Use FFmpeg from system PATH
- `false` - Search for local FFmpeg binary (`.ffmpeg`, `./ffmpeg`, binary directory)

**Interactive prompt:** "Use FFmpeg from PATH? (y/n)"

**Resolution order when `false`:**
1. Custom binary from `ffmpegNameStr` (if not default)
2. `./ffmpeg` in current directory
3. FFmpeg next to executable
4. Fallback to system PATH

---

#### `ffmpegNameStr` (string)

**Purpose:** Custom FFmpeg binary name or absolute path

**Default:** `"ffmpeg"` (use default binary)

**Example:** `"/usr/local/bin/ffmpeg"` or `"ffmpeg-custom"`

**Validation:** Checked for existence at runtime

**Error message:** `"ffmpeg not found in PATH (install ffmpeg or set ffmpegNameStr to an absolute/local binary path)"`

**Interactive prompt:** Not prompted (advanced users only)

---

#### `skipChapters` (bool)

**Purpose:** Skip embedding chapter markers in video files

**Default:** `false` (embed chapters if available)

**Options:**
- `true` - Skip chapter embedding (faster conversion)
- `false` - Embed chapters from track metadata (default)

**Note:** Chapters are embedded during TS→MP4 conversion via FFmpeg

---

#### `forceVideo` (bool) **[DEPRECATED]**

**Status:** ⚠️ **Deprecated** - Use `defaultOutputs="video"` instead

**Purpose:** Force video downloads (legacy)

**Migration:** Set `defaultOutputs: "video"`

---

#### `skipVideos` (bool) **[DEPRECATED]**

**Status:** ⚠️ **Deprecated** - Use `defaultOutputs="audio"` instead

**Purpose:** Skip video downloads (legacy)

**Migration:** Set `defaultOutputs: "audio"` (this is the default)

---

### Rclone Cloud Uploads

#### `rcloneEnabled` (bool)

**Purpose:** Enable automatic cloud uploads via rclone after downloads

**Default:** `false` (no cloud uploads)

**Options:**
- `true` - Upload to rclone remote after download
- `false` - Local downloads only

**Requires:** `rclone` installed and `rcloneRemote`/`rclonePath` configured

**Interactive prompt:** "Enable rclone uploads? (y/n)"

**Note:** Uploads happen automatically after each album/video download completes

---

#### `rcloneRemote` (string)

**Purpose:** Rclone remote name for audio uploads

**Default:** `""` (not configured)

**Example:** `"gdrive"`, `"dropbox"`, `"onedrive"`

**Validation:** Remote must exist in rclone config (`rclone listremotes`)

**Error message:** `"rclone remote not found: <remote>"`

**Interactive prompt:** "Enter rclone remote name:"

**Configure rclone:** Run `rclone config` to create remotes

---

#### `rclonePath` (string)

**Purpose:** Remote base path for audio uploads (on the rclone remote)

**Default:** `""` (not configured)

**Example:** `"/Music"`, `"/Music/Nugs"`, `"/"`

**Validation:** None (path created if doesn't exist on remote)

**Interactive prompt:** "Enter remote path for audio uploads:"

**⚠️ CRITICAL:** This is the **remote** path, NOT local. Local downloads use `outPath`.

**Breaking Change:** Before 2026-02-05, `rclonePath` incorrectly affected local downloads. See [Migrations](#migrations).

---

#### `rcloneVideoPath` (string)

**Purpose:** Remote base path for video uploads (on the rclone remote)

**Default:** Same as `rclonePath` (audio and video in same remote location)

**Example:** `"/Videos"`, `"/Videos/Nugs"`

**Validation:** None (path created if doesn't exist on remote)

**Interactive prompt:** Not prompted (defaults to `rclonePath`)

**Note:** If empty, automatically set to `rclonePath` value

---

#### `deleteAfterUpload` (bool)

**Purpose:** Delete local files after successful upload to rclone

**Default:** `true` (if rclone enabled during interactive setup)

**Options:**
- `true` - Delete local files after verified upload
- `false` - Keep local files after upload

**Safety:** Upload is verified with `rclone check --one-way` before deletion

**Error handling:** If verification fails, local files are **NOT** deleted

**Interactive prompt:** "Delete local files after upload? (y/n)"

---

#### `rcloneTransfers` (int)

**Purpose:** Number of parallel rclone transfer threads

**Default:** `4`

**Range:** 1-128 (practical limit)

**Validation:** Must be >= 1

**Error message:** `"transfers must be a positive integer"`

**Interactive prompt:** "Number of rclone transfers:" (default: 4)

**Performance:** Higher values = faster uploads but more bandwidth/memory

**Recommended:**
- 4-8 for home internet
- 8-16 for fast connections
- 16-32 for server environments

---

### Catalog Auto-Refresh

#### `catalogAutoRefresh` (bool)

**Purpose:** Enable automatic catalog updates on schedule

**Default:** `true` (enabled)

**Options:**
- `true` - Auto-update catalog at scheduled time
- `false` - Manual updates only (`nugs catalog update`)

**Trigger:** Checked at startup, updates if schedule elapsed

**Configure:** Use `nugs catalog config` to change schedule

---

#### `catalogRefreshTime` (string)

**Purpose:** Time of day to refresh catalog (HH:MM format)

**Default:** `"05:00"` (5:00 AM)

**Format:** 24-hour time, HH:MM

**Example:** `"03:00"`, `"14:30"`, `"23:00"`

**Validation:** Must match regex `^\d{2}:\d{2}$`

**Error message:** `"invalid refresh time format: <value> (expected HH:MM)"`

**Interactive prompt:** "Catalog refresh time (HH:MM):" (default: 05:00)

---

#### `catalogRefreshTimezone` (string)

**Purpose:** IANA timezone for refresh time interpretation

**Default:** `"America/New_York"` (EST/EDT)

**Format:** IANA timezone database name

**Examples:**
- `"America/Los_Angeles"` (PST/PDT)
- `"America/Chicago"` (CST/CDT)
- `"Europe/London"` (GMT/BST)
- `"Asia/Tokyo"` (JST)

**Validation:** Must be valid IANA timezone

**Error message:** `"invalid timezone <value>: unknown time zone <value>"`

**Common mistake:** ❌ `"EST"` → ✅ `"America/New_York"`

**Find your timezone:** https://en.wikipedia.org/wiki/List_of_tz_database_time_zones

**Interactive prompt:** "Timezone for refresh:" (default: America/New_York)

---

#### `catalogRefreshInterval` (string)

**Purpose:** How often to refresh catalog

**Default:** `"daily"`

**Options:**
- `"daily"` - Refresh every day at `catalogRefreshTime`
- `"weekly"` - Refresh once per week at `catalogRefreshTime`

**Validation:** Must be "daily" or "weekly"

**Error message:** `"invalid interval: <value> (must be 'daily' or 'weekly')"`

**Interactive prompt:** "Refresh interval (daily/weekly):" (default: daily)

**Note:** Weekly refresh happens on the same day of week as first refresh

---

### Performance

#### `skipSizePreCalculation` (bool)

**Purpose:** Skip pre-calculation of album size before download

**Default:** `false` (calculate size)

**Options:**
- `true` - Skip size calculation (faster startup)
- `false` - Calculate size (accurate progress/ETA)

**Trade-off:**
- ✅ **Skip:** Faster start, but no total size or accurate ETA
- ✅ **Calculate:** Slower start (5-10s), but accurate progress bar

**Method:** Uses 8 concurrent HEAD requests with 60s timeout

**Interactive prompt:** Not prompted (advanced users only)

---

## Validation Rules

### Format Validation

**At config parse time:**

| Field | Rule | Error Message |
|-------|------|---------------|
| `format` | 1-5 | `"track Format must be between 1 and 5"` |
| `videoFormat` | 1-5 | `"video format must be between 1 and 5"` |
| `defaultOutputs` | "audio", "video", or "both" | `"invalid defaultOutputs: "<value>" (must be audio, video, or both)"` |
| `catalogRefreshTimezone` | Valid IANA timezone | `"invalid timezone <value>: unknown time zone <value>"` |
| `catalogRefreshTime` | HH:MM format | `"invalid refresh time format: <value> (expected HH:MM)"` |
| `catalogRefreshInterval` | "daily" or "weekly" | `"invalid interval: <value> (must be 'daily' or 'weekly')"` |
| `rcloneTransfers` | >= 1 | `"transfers must be a positive integer"` |

### Runtime Validation

**At execution time:**

| Field | Rule | Error Message |
|-------|------|---------------|
| `ffmpegNameStr` | Binary exists | `"ffmpeg not found in PATH (install ffmpeg or set ffmpegNameStr to an absolute/local binary path)"` |
| `rcloneRemote` | Remote exists | `"rclone remote not found: <remote>"` |
| `email`/`password` | Valid credentials | `"authentication failed: invalid credentials"` |
| `token` | Valid token | `"authentication failed: invalid token"` |

---

## Security

### Credentials Storage

**⚠️ CRITICAL:** All credentials are stored in **plain text** with no encryption.

**Fields affected:**
- `email` - Plain text
- `password` - Plain text, **visible during interactive setup**
- `token` - Plain text

**Protection:**
- ✅ File permissions (`0600` - owner read/write only)
- ✅ Directory permissions (`0700` - owner access only)
- ✅ Auto-fix for insecure permissions (Unix)

**Mitigations:**
1. Use `token` instead of `password` (not visible during input)
2. Ensure `chmod 600 ~/.nugs/config.json`
3. Never commit config.json to version control
4. Add `config.json` to `.gitignore`

### File Permissions

**Automatic security checks:**

```bash
# On every config read:
if permissions != 0600:
    warn("Config has insecure permissions")
    if unix:
        chmod(config, 0600)  # Auto-fix
        warn("Auto-fix applied: chmod 600 <path>")
    else:  # Windows
        warn("Fix manually: icacls <path> /inheritance:r /grant:r %USERNAME%:RW")
```

**Expected permissions:**
```bash
# Unix/Linux/macOS
-rw------- 1 user user 436 Feb 07 23:13 config.json

# Windows (via icacls)
owner:(R,W)
```

### Best Practices

**DO:**
1. ✅ Store config in `~/.nugs/config.json`
2. ✅ Use `chmod 600 ~/.nugs/config.json`
3. ✅ Use token authentication when possible
4. ✅ Review auto-fix warnings
5. ✅ Add `config.json` to `.gitignore`

**DON'T:**
1. ❌ Use world-readable permissions (`chmod 644`)
2. ❌ Commit config.json to version control
3. ❌ Share config files (contains credentials)
4. ❌ Store config in shared/network directories
5. ❌ Ignore permission warnings

---

## Examples

### Minimal Valid Config

```json
{
  "email": "user@example.com",
  "password": "your_password",
  "format": 2,
  "videoFormat": 5,
  "outPath": "~/Music/Nugs"
}
```

### Token Authentication (Recommended)

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "format": 2,
  "videoFormat": 5,
  "outPath": "~/Music/Nugs"
}
```

### Audio Only (No Videos)

```json
{
  "email": "user@example.com",
  "password": "your_password",
  "format": 2,
  "videoFormat": 5,
  "defaultOutputs": "audio",
  "outPath": "~/Music/Nugs"
}
```

### With Rclone Upload

```json
{
  "email": "user@example.com",
  "password": "your_password",
  "format": 2,
  "videoFormat": 5,
  "outPath": "~/Music/Nugs",
  "rcloneEnabled": true,
  "rcloneRemote": "gdrive",
  "rclonePath": "/Music/Nugs",
  "deleteAfterUpload": true,
  "rcloneTransfers": 8
}
```

### Full Featured Config

```json
{
  "token": "your_auth_token_here",
  "format": 2,
  "videoFormat": 5,
  "defaultOutputs": "both",
  "outPath": "/home/user/Music/Nugs",
  "videoOutPath": "/home/user/Videos/Nugs",
  "useFfmpegEnvVar": true,
  "ffmpegNameStr": "",
  "skipChapters": false,
  "rcloneEnabled": true,
  "rcloneRemote": "gdrive",
  "rclonePath": "/Music/Nugs",
  "rcloneVideoPath": "/Videos/Nugs",
  "deleteAfterUpload": true,
  "rcloneTransfers": 8,
  "catalogAutoRefresh": true,
  "catalogRefreshTime": "03:00",
  "catalogRefreshTimezone": "America/Los_Angeles",
  "catalogRefreshInterval": "weekly",
  "skipSizePreCalculation": false
}
```

### Common Invalid Configs

#### Invalid Format Values

```json
{
  "format": 6,        // ❌ INVALID - must be 1-5
  "videoFormat": 0    // ❌ INVALID - must be 1-5
}
```
**Error:** `track Format must be between 1 and 5`

#### Invalid defaultOutputs

```json
{
  "defaultOutputs": "videos"  // ❌ INVALID - typo ("videos" vs "video")
}
```
**Error:** `invalid defaultOutputs: "videos" (must be audio, video, or both)`

#### Invalid Timezone

```json
{
  "catalogRefreshTimezone": "EST"  // ❌ INVALID - use IANA timezone
}
```
**Error:** `invalid timezone EST: unknown time zone EST`

**Fix:** Use `"America/New_York"` instead

#### Invalid Refresh Time

```json
{
  "catalogRefreshTime": "5am"  // ❌ INVALID - wrong format
}
```
**Error:** `invalid refresh time format: 5am (expected HH:MM)`

**Fix:** Use `"05:00"`

#### Invalid Refresh Interval

```json
{
  "catalogRefreshInterval": "hourly"  // ❌ INVALID - not supported
}
```
**Error:** `invalid interval: hourly (must be 'daily' or 'weekly')`

#### Missing Rclone Configuration

```json
{
  "rcloneEnabled": true,
  "rcloneRemote": "",      // ❌ Missing
  "rclonePath": ""         // ❌ Missing
}
```
**Result:** Runtime errors when attempting uploads

**Fix:** Set `rcloneRemote` and `rclonePath` or disable rclone

---

## Migrations

### Breaking Change: rclonePath Behavior (2026-02-05)

**⚠️ BREAKING CHANGE:** The `rclonePath` field no longer affects local download paths.

#### Previous Behavior (before 2026-02-05)

```go
// rclonePath was used as a fallback for local base path
basePath := cfg.OutPath
if cfg.RclonePath != "" {
    basePath = cfg.RclonePath  // ❌ Confusing dual-purpose
}
```

#### New Behavior (after 2026-02-05)

```go
// rclonePath ONLY affects remote storage uploads
basePath := cfg.OutPath  // Always use outPath for local downloads
remotePath := cfg.RclonePath  // Only for remote storage
```

#### Migration Guide

If you previously set `rclonePath` expecting it to control local download locations:

**1. Update your config:**
- Move your desired local path to `outPath`
- Keep `rclonePath` for the remote storage path only

**2. Example migration:**

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

**3. Impact:**
- **Local-only users:** Update `outPath` to your preferred download location
- **Rclone users:** `outPath` for local, `rclonePath` for remote (clear separation)

#### Rationale

This change eliminates a confusing "leaky abstraction" where a field named "rclone**Path**" (implying remote storage) was also controlling local filesystem behavior. The new design provides clear separation of concerns: `outPath` = local, `rclonePath` = remote.

---

## Environment Variables

### Currently Supported

| Variable | Purpose | Where Used |
|----------|---------|------------|
| `NUGS_DETACHED` | Process running in background | `internal/runtime/status.go:17` |
| `NUGS_THEME` | Color theme ("vivid" or "nordonedark") | `internal/ui/theme.go:46` |
| `HOME` | User home directory for config/cache paths | Config path resolution |
| `PATH` | FFmpeg binary resolution | `internal/config/config.go` |
| `TERM` | Terminal color capability detection | `internal/ui/theme.go` |
| `COLORTERM` | Truecolor support detection | `internal/ui/theme.go` |

### Future Consideration

These environment variables are **NOT** currently implemented but documented in Python client:

- `NUGS_EMAIL` - Override email from config
- `NUGS_PASSWORD` - Override password from config
- `NUGS_TOKEN` - Override token from config
- `NUGS_FORMAT` - Override audio format
- `NUGS_VIDEO_FORMAT` - Override video format

**Note:** The Go CLI uses config.json exclusively. Environment variable overrides may be added in future versions.

---

## Interactive Setup

First-time setup prompts:

```bash
$ nugs 23329
Config not found. Let's set it up!

Enter your Nugs email: user@example.com
Enter your Nugs password: ******

Audio format:
  1. 16-bit 44.1kHz ALAC
  2. 16-bit 44.1kHz FLAC
  3. 24-bit 48kHz MQA
  4. 360 Reality Audio (default)
  5. 150 Kbps AAC
Select format (1-5) [4]: 2

Video format:
  1. 480p
  2. 720p
  3. 1080p
  4. 1440p
  5. 4K / best (default)
Select format (1-5) [5]: 5

Enter output path for audio downloads [Nugs downloads]: ~/Music/Nugs

Use FFmpeg from PATH? (y/n) [y]: y

Enable rclone uploads? (y/n) [n]: y
Enter rclone remote name: gdrive
Enter remote path for audio uploads: /Music/Nugs
Number of rclone transfers [4]: 8
Delete local files after upload? (y/n) [y]: y

Catalog auto-refresh enabled by default.
Refresh time (HH:MM) [05:00]: 03:00
Timezone [America/New_York]: America/Los_Angeles
Refresh interval (daily/weekly) [daily]: weekly

✓ Config saved to ~/.nugs/config.json
```

---

## Troubleshooting

### Config File Not Found

**Error:** "Config file not found"

**Solution:**
1. Run `nugs` with any command to trigger interactive setup
2. Manually create `~/.nugs/config.json` with minimal config

### Insecure Permissions

**Warning:** "Config file has insecure permissions (0644)"

**Solution:**
- Unix: `chmod 600 ~/.nugs/config.json` (auto-fixed)
- Windows: `icacls ~/.nugs/config.json /inheritance:r /grant:r %USERNAME%:RW`

### Invalid Format Values

**Error:** "track Format must be between 1 and 5"

**Solution:** Set `format` to a value between 1-5

### Invalid Timezone

**Error:** "invalid timezone EST: unknown time zone EST"

**Solution:** Use IANA timezone (e.g., "America/New_York" instead of "EST")

**Find your timezone:** https://en.wikipedia.org/wiki/List_of_tz_database_time_zones

### Rclone Not Found

**Error:** "rclone is not installed or not available in PATH"

**Solution:**
1. Install rclone: https://rclone.org/downloads/
2. Or disable rclone: `"rcloneEnabled": false`

### FFmpeg Not Found

**Error:** "ffmpeg not found in PATH"

**Solution:**
1. Install FFmpeg: https://ffmpeg.org/download.html
2. Set custom path: `"ffmpegNameStr": "/path/to/ffmpeg"`
3. Place `ffmpeg` binary in project root

---

## See Also

- [CLAUDE.md](./CLAUDE.md) - Development guide and quick start
- [ARCHITECTURE.md](./ARCHITECTURE.md) - Package structure and patterns
- [README.md](./README.md) - User documentation
- [docs/token.md](./docs/token.md) - Token extraction guide
