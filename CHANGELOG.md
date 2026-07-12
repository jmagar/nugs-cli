# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.0.2](https://github.com/jmagar/nugs-cli/compare/v0.0.1...v0.0.2) (2026-07-11)


### Fixed

* **ci:** switch OpenWiki to local openai-compatible proxy ([6774b71](https://github.com/jmagar/nugs-cli/commit/6774b71a281c493f5caca1364c3f6487453be4e0))

## [v1.0.0] - 2026-02-24

The first release of **nugs-cli** — a complete rewrite and fork of [Sorrow446/Nugs-Downloader](https://github.com/Sorrow446/Nugs-Downloader). This release transforms a single-file Go script into a production-grade CLI with 14 internal packages, a catalog system, watch automation, and cloud upload integration.

### ⚠️ Breaking Changes

- **`rclonePath` no longer affects local downloads**: `rclonePath` now ONLY controls remote upload destinations. Use `outPath` for local download paths.
  - Migration: Set `outPath` to your desired local directory explicitly
- **Module renamed**: `github.com/Sorrow446/Nugs-Downloader` → `github.com/jmagar/nugs-cli`
- **Go 1.24+ required**: Minimum Go version bumped from 1.16 to 1.24
- **Binary location changed**: Now installs to `~/.local/bin/nugs` via `make build`

### ✨ Features

**Catalog System**
- Local catalog caching of the entire Nugs.net library (~31,000 shows) with four index files for fast lookups (4e49848)
- Gap detection: find missing shows in your collection per artist (694718e)
- Catalog stats with date ranges and visual status indicators (4fe4889, e4088d3)
- New shows indicator: see what was added since your last catalog update (4e49848)
- Auto-refresh system with configurable daily/weekly schedule and timezone support

**Watch & Automation**
- `nugs watch [artistID]` — poll for new shows, auto-download or notify (ede09c0)
- Systemd integration: `nugs watch install` generates `.service` + `.timer` units (ede09c0)
- Per-artist Gotify notifications during multi-artist watch runs (bb99245)

**Notifications**
- Gotify push notifications for download completions, catalog updates, and errors (ede09c0)
- Configurable via `gotifyURL`, `gotifyToken`, `gotifyPriority`

**API Resilience**
- Rate limiting, circuit breaker, and automatic retry with exponential backoff (974d89e)
- `context.Context` threading through all API functions for proper cancellation (e6dcd6c)

**Media Type System**
- Audio/video/both filtering on all catalog commands (dfa1a5d)
- Media type detection with emoji indicators: 🎵 audio, 🎬 video, 📹 both (0587602)
- `nugs gaps 1125 video` — video-only gap detection and fill (31fb22f)
- Separate `defaultOutputs` config for audio vs video paths (5900d9c)

**Upload Progress**
- Real-time rclone upload progress with percent, speed, and ETA in the progress box (acd09fd, 8bea651)
- Upload verification before local file deletion (0ffdf11)

**UI Overhaul**
- Unicode formatting with color-coded output throughout the CLI (30358ef, c978771)
- Welcome screen showing latest shows from Nugs.net (0c106c1)
- JSON output mode for all list commands with multiple detail levels (c47f5b6)
- Responsive table rendering with proper ANSI handling (59d15ed)

**Video Downloads**
- Enhanced video format detection with fallback chains (cd6a23f)
- Video downloads from artist URLs (a7ce96c)
- Skip video-only albums option (4385434)
- `--skip-chapters` option for video downloads (360a68c)

**Shell Completions**
- `nugs completion <shell>` for bash, zsh, fish, and PowerShell (245960f)

**Developer Experience**
- Comprehensive command documentation (cd6a23f)
- `make build` installs to `~/.local/bin/nugs` (fdfd20f)
- Race detector in test suite (28cbc82)

### 🐛 Bug Fixes

- Fix deadlocks in upload progress callbacks (29bee36, 5fabd62)
- Fix progress box phase state machine — reject unknown transitions, reset between albums (c032303)
- Dynamically update ShowTotal during progress tracking (1112f6b)
- Preserve JSON output when remote scan fails (e50245a)
- Fix availType to fetch downloadable shows correctly (911c7f0)
- Harden gap detection and upload verification (0ffdf11)
- Mutex protection for batch state (06446e4)
- Replace string error matching with `errors.Is` (9a49654)
- Redact filesystem paths in diagnostics (eb33400)
- Fix rclone progress parsing and progress box rendering (1d81be9)
- Handle ANSI color codes properly in table truncation (59d15ed)
- Fix livestream URL parsing (1a2e465, d27dbef, 24d4104)
- Fix API stream URL resolution (c1aeab2)

### 🔧 Refactoring

- **Complete monorepo restructuring**: Decomposed single-file monolith into 14 internal packages with strict dependency tiers (03ae139 → dbd27b5)
  - `internal/model` — Pure type definitions (Tier 0)
  - `internal/helpers` — Utility functions (Tier 1)
  - `internal/ui` — Theme, formatting, output (Tier 1)
  - `internal/api` — HTTP client, URL parser (Tier 1)
  - `internal/cache` — File locking, catalog cache (Tier 1)
  - `internal/config` — Config management (Tier 2)
  - `internal/rclone` — Cloud storage integration (Tier 2)
  - `internal/runtime` — Session and platform code (Tier 2)
  - `internal/catalog` — Catalog operations, watch, gap detection (Tier 3)
  - `internal/download` — Audio, video, batch engines (Tier 3)
  - `internal/list` — Artist/show list output (Tier 3)
  - `internal/notify` — Gotify push alerts (Tier 3)
  - `internal/completion` — Shell completions (Tier 3)
  - `internal/testutil` — Shared test helpers (Tier 0)
- Storage adapter pattern for download backends (d02bc65)
- Rename `ApiMethod` → `APIMethod` per Go conventions (75cbd27)
- `context.Context` threading through rclone, list, and all packages (adb540b, c2798ee)

### 🧪 Testing

- Comprehensive unit tests for upload progress (48df4dc)
- Watch tests: 12 watch tests + 7 notification cases (ede09c0)
- Per-artist notification tests (bb99245)
- Edge cases for `IsShowDownloadable` (a9b3894)
- Shared test utilities: `WithTempHome`, `CaptureStdout`, `ChdirTemp` (internal/testutil)

### 📚 Documentation

- Full CLAUDE.md development guide with architecture overview (1adfb2b)
- ARCHITECTURE.md — package structure, dependencies, design patterns
- CONFIG.md — complete configuration reference (all 23 fields)
- COMMANDS.md — comprehensive command examples
- Migration guide for `rclonePath` behavior change

### 🔨 Maintenance

- `.gitignore` to protect credentials and build artifacts (0385bad)
- Applied `gofmt -s` formatting across codebase (58cec8a)
- Addressed 80+ PR review comments across all packages

### 👥 Contributors

- @jmagar (94 commits)
- @Sorrow446 (20 commits — original upstream)
- @twalker1998 (4 commits — skip video-only albums)
- @marksibert (1 commit — livestream URL fix)

[v1.0.0]: https://github.com/jmagar/nugs-cli/releases/tag/v1.0.0
