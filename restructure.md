# Restructure Nugs CLI to Modern Go Monorepo

## Context

The Nugs CLI is a Go-based tool for downloading music from Nugs.net. Currently all 42 `.go` files (~10,538 lines) live in a single flat `package main` at the project root. This makes the code hard to test in isolation, hard to navigate, and prevents clear separation of concerns. The goal is to restructure into a modern Go monorepo using `cmd/` + `internal/` conventions while preserving identical CLI behavior.

**Current state:** Single `package main`, all files in root
**Target state:** `cmd/nugs/main.go` entry point + 13 `internal/` packages
**Module:** `github.com/jmagar/nugs-cli` (unchanged)
**Go version:** 1.24.12 (unchanged)

---

## Target Directory Structure

```
github.com/jmagar/nugs-cli/
├── cmd/
│   └── nugs/
│       └── main.go              # Entry point (~200 lines): init banner, main(), bootstrap(), run(), dispatch()
├── internal/
│   ├── model/                   # Layer 0 - Pure types, zero internal deps
│   │   ├── types.go             # Config, Args, MediaType, BatchProgressState, WriteCounter
│   │   ├── api_types.go         # Auth, Payload, UserInfo, SubInfo, StreamParams, AlbArtResp, Track, etc.
│   │   ├── catalog_types.go     # LatestCatalogResp, CacheMeta, ArtistsIndex, ShowStatus, etc.
│   │   └── constants.go         # JSON levels, MessagePriority, format constants
│   ├── helpers/                 # Layer 0 - Utility functions, imports only model
│   │   ├── paths.go             # sanitise, buildAlbumFolderName, makeDirs, fileExists, getOutPathForMedia, etc.
│   │   ├── errors.go            # handleErr, contains, processUrls, readTxtFile
│   │   └── helpers_test.go
│   ├── ui/                      # Layer 1 - Terminal output, imports model
│   │   ├── theme.go             # Color vars, initColorPalette, supportsTruecolor, symbols
│   │   ├── format.go            # Table, progress bars, box drawing, stripAnsiCodes, padRight
│   │   ├── output.go            # printSuccess/Error/Info/Warning/Download/Upload/Music
│   │   ├── progress.go          # ProgressBoxState display methods, renderProgressBox
│   │   └── format_render_test.go
│   ├── cache/                   # Layer 1 - Cache I/O, imports model + helpers
│   │   ├── cache.go             # getCacheDir, readCatalogCache, writeCatalogCache, buildIndexes
│   │   ├── artist_cache.go      # readArtistMetaCache, writeArtistMetaCache
│   │   └── filelock.go          # FileLock, AcquireLock, Release, WithCacheLock (+build tags)
│   ├── api/                     # Layer 2 - HTTP client, imports model + helpers
│   │   ├── client.go            # HTTP client, auth, getUserInfo, getSubInfo, extractLegToken
│   │   ├── catalog_api.go       # getAlbumMeta, getArtistMeta, getLatestCatalog, getStreamMeta
│   │   ├── url_parser.go        # checkUrl, regexStrings, parsePaidLstreamShowID
│   │   └── url_parser_test.go
│   ├── config/                  # Layer 2 - Config management, imports model + helpers + ui
│   │   ├── config.go            # readConfig, writeConfig, parseCfg, parseArgs, resolveFfmpegBinary
│   │   ├── config_test.go
│   │   └── config_ffmpeg_test.go
│   ├── rclone/                  # Layer 2 - Cloud storage, imports model + helpers + ui
│   │   ├── rclone.go            # checkRcloneAvailable, uploadToRclone, remotePathExists, etc.
│   │   └── rclone_test.go
│   ├── runtime/                 # Layer 2 - Session control, imports model + helpers + ui + cache
│   │   ├── status.go            # RuntimeStatus I/O, initRuntimeStatus, finalizeRuntimeStatus
│   │   ├── control.go           # crawlController, waitIfPausedOrCancelled, hotkey handling
│   │   ├── progress_box.go      # setCurrentProgressBox, getCurrentProgressBox
│   │   ├── detach_common.go     # isReadOnlyCommand, shouldAutoDetach
│   │   ├── detach_unix.go       # spawnDetached (unix)
│   │   ├── detach_windows.go    # spawnDetached (windows)
│   │   ├── cancel_unix.go
│   │   ├── cancel_windows.go
│   │   ├── hotkey_input_unix.go
│   │   ├── hotkey_input_windows.go
│   │   ├── process_alive_unix.go
│   │   ├── process_alive_windows.go
│   │   ├── signal_persistence_unix.go
│   │   └── signal_persistence_windows.go
│   ├── completion/              # Layer 1 - Shell completions, imports ui
│   │   └── completions.go       # completionCommand, bash/zsh/fish/powershell generators
│   ├── download/                # Layer 3 - Download engine, imports model + helpers + ui + api + rclone + runtime
│   │   ├── audio.go             # album, downloadTrack, processTrack, decryptTrack, tsToAac
│   │   ├── video.go             # video, getVideoSku, getShowMediaType, downloadVideo, tsToMp4
│   │   ├── batch.go             # artist, playlist, catalogPlist, paidLstream
│   │   └── tier3.go             # Tier3 metadata/quality helpers
│   ├── catalog/                 # Layer 3 - Catalog commands, imports model + helpers + ui + api + cache + rclone + config
│   │   ├── handlers.go          # catalogUpdate, catalogCacheStatus, catalogStats, catalogLatest
│   │   ├── gaps.go              # catalogGaps, catalogGapsFill, showExists, analyzeArtistCatalog
│   │   ├── coverage.go          # catalogCoverage, catalogList
│   │   ├── autorefresh.go       # shouldAutoRefresh, autoRefreshIfNeeded, enable/disable/configure
│   │   ├── media_filter.go      # analyzeArtistCatalogMediaAware, showExistsForMedia
│   │   └── handlers_test.go
│   ├── list/                    # Layer 3 - List/display commands, imports model + helpers + ui + api + cache
│   │   └── list.go              # listArtists, listArtistShows, listArtistShowsByVenue, displayWelcome
│   └── testutil/                # Shared test helpers
│       └── testutil.go          # withTempHome, captureStdout, buildTestArtistMeta
├── Makefile
├── go.mod
├── go.sum
├── README.md
└── CLAUDE.md
```

---

## Package Dependency Graph (Strict DAG - No Cycles)

```
Layer 0 (leaf):  model, helpers
Layer 1:         ui (→ model), cache (→ model, helpers), completion (→ ui)
Layer 2:         api (→ model, helpers), config (→ model, helpers, ui),
                 rclone (→ model, helpers, ui), runtime (→ model, helpers, ui, cache)
Layer 3:         download (→ model, helpers, ui, api, rclone, runtime)
                 catalog (→ model, helpers, ui, api, cache, rclone, config)
                 list (→ model, helpers, ui, api, cache)
Layer 4 (top):   cmd/nugs (→ all internal packages, wires everything)
```

**Critical rule: No package in Layer N imports another package in Layer N or higher.**

---

## Pre-Refactor Baseline (capture BEFORE starting)

Before any code changes, capture baselines to compare against:

```bash
# 1. Capture binary size
go build -o /tmp/nugs-baseline . && ls -la /tmp/nugs-baseline

# 2. Capture help output
/tmp/nugs-baseline help 2>&1 > /tmp/nugs-help-baseline.txt

# 3. Capture test results
go test ./... -count=1 -v 2>&1 > /tmp/nugs-test-baseline.txt

# 4. Capture package list
go list ./... > /tmp/nugs-packages-baseline.txt

# 5. Capture completion output
/tmp/nugs-baseline completion bash > /tmp/nugs-completion-baseline.txt
```

These baselines are used in Gate 5 (Binary Equivalence) throughout the migration.

---

## Verification Gates

Every phase MUST pass ALL gates before proceeding. If any gate fails, fix the issue in-phase before moving on. Each passing phase gets its own commit.

### Gate Protocol (applied after every phase)

```
GATE 1 - Compilation
  go build ./cmd/nugs          # Binary compiles
  go build ./...               # All packages compile
  GOOS=windows go build ./...  # Cross-platform compile (from Phase 4+)

GATE 2 - Tests
  go test ./... -count=1       # All tests pass (no cache)
  go test ./... -race          # No data races

GATE 3 - Static Analysis
  go vet ./...                 # No vet issues

GATE 4 - Dependency Hygiene
  # Verify no circular imports (Go compiler catches this, but be explicit)
  go build ./internal/model     # model compiles alone
  go build ./internal/helpers   # helpers compiles with only model dep
  # (layer-by-layer verification for each new package)

GATE 5 - Binary Equivalence (from Phase 1+)
  # Build both old and new, compare help output
  go build -o /tmp/nugs-old .          # (only while root package main still exists)
  go build -o /tmp/nugs-new ./cmd/nugs
  diff <(/tmp/nugs-old help 2>&1) <(/tmp/nugs-new help 2>&1)

GATE 6 - Commit
  git add -A && git commit    # One commit per phase
```

### Rollback Criteria

If a phase cannot pass all gates after reasonable effort:
1. `git stash` or `git reset --soft HEAD` to undo the phase
2. Identify the blocking issue (usually circular deps or missing exports)
3. Adjust the plan for that phase and retry
4. Never proceed to the next phase with failing gates

---

## Migration Phases

Each phase produces a compilable, test-passing state. Each phase ends with a commit.

### Phase 0: Scaffold directories + build plumbing

- Create `cmd/nugs/` and all `internal/*/` directories
- Create a minimal `cmd/nugs/main.go` that just imports root package main (forwarding pattern)
- Create `internal/testutil/testutil.go` with shared test helpers extracted from existing tests
- Update Makefile: `go build -o ~/.local/bin/nugs ./cmd/nugs`

**Gates:**
- `go build ./cmd/nugs` produces working binary
- `go test ./... -count=1` all existing tests pass
- `go vet ./...` clean
- `nugs help` output identical to pre-refactor

**Commit:** `refactor: scaffold monorepo directory structure`

---

### Phase 1: Extract `internal/model` (pure types)

**Source:** `structs.go` (types only, NOT color/symbol vars or init)

- Move all struct definitions: `Config`, `Args`, `Auth`, `Payload`, `UserInfo`, `SubInfo`, `StreamParams`, `Product`, `AlbArtResp`, `AlbumMeta`, `Track`, `Quality`, `ArtistMeta`, `MediaType` enum, `CacheMeta`, `LatestCatalogResp`, `ShowStatus`, `ArtistCatalogAnalysis`, etc.
- Move `ProgressBoxState` struct definition (NOT color-dependent methods)
- Move `WriteCounter` struct (NOT the `Write` method that depends on progress rendering)
- Move `RuntimeStatus`, `RuntimeControl` structs
- Move JSON level constants, `MessagePriority` constants
- Keep `Args.Description()` method stub that returns plain text (color formatting moves to `cmd/nugs`)
- **Key:** This package has ZERO imports of other `internal/` packages

**Gates:**
- `go build ./internal/model` compiles with zero internal deps
- `go build ./...` full build passes
- `go test ./... -count=1` all tests pass
- `go vet ./...` clean
- Verify `internal/model` imports: `go list -f '{{.Imports}}' ./internal/model` contains NO `github.com/jmagar/nugs-cli/internal/*`

**Commit:** `refactor: extract internal/model with pure type definitions`

---

### Phase 2: Extract `internal/helpers`

**Source:** `helpers.go`

- Move: `handleErr`, `sanitise`, `buildAlbumFolderName`, `makeDirs`, `fileExists`, `processUrls`, `readTxtFile`, `contains`, `validatePath`, `getVideoOutPath`, `getRcloneBasePath`, `getOutPathForMedia`, `getRclonePathForMedia`, `calculateLocalSize`
- Update signatures to accept `*model.Config` instead of `*Config`
- Move `helpers_test.go` → `internal/helpers/helpers_test.go`

**Gates:**
- `go build ./internal/helpers` compiles with only `model` dep
- `go test ./internal/helpers/... -count=1 -v` all helper tests pass
- `go test ./... -count=1` full suite passes
- `go vet ./...` clean
- Verify imports: `go list -f '{{.Imports}}' ./internal/helpers` only has `model` as internal dep

**Commit:** `refactor: extract internal/helpers with utility functions`

---

### Phase 3: Extract `internal/ui`

**Source:** `structs.go` (color vars, symbols, palette init), `format.go`, `output.go`, `tier3_methods.go` (display methods)

- Move color variables + `initColorPalette()` + `supportsTruecolor()` + `supports256Color()` → `ui/theme.go`
- Move symbol variables (`symbolCheck`, `symbolCross`, etc.) → `ui/theme.go`
- Move `Table` type + rendering methods → `ui/format.go`
- Move progress bar rendering, box drawing → `ui/format.go`
- Move `printSuccess/Error/Info/Warning/Download/Upload/Music` → `ui/output.go`
- Move `describeAudioFormat`, `describeVideoFormat`, `getMediaTypeIndicator` → `ui/output.go`
- Move `runErrorCount`, `runWarningCount` counters → `ui/output.go`
- Move `ProgressBoxState` display functions → `ui/progress.go` (as standalone functions taking `*model.ProgressBoxState`)
- Move `getQualityName` → `ui/output.go`
- Move `format_render_test.go` → `internal/ui/format_render_test.go`

**Gates:**
- `go build ./internal/ui` compiles with only `model` dep
- `go test ./internal/ui/... -count=1 -v` format render tests pass
- `go test ./... -count=1` full suite passes
- `go vet ./...` clean
- Color init works: build binary, run `nugs help`, verify colored output appears

**Commit:** `refactor: extract internal/ui with theme, formatting, and output`

---

### Phase 4: Extract `internal/cache`

**Source:** `cache.go`, `filelock.go`

- Move all cache I/O: `getCacheDir`, `readCatalogCache`, `writeCatalogCache`, `readCacheMeta`, `buildArtistIndex`, `buildContainerIndex`
- Move artist meta cache: `readArtistMetaCache`, `writeArtistMetaCache`
- Move `FileLock`, `AcquireLock`, `Release`, `WithCacheLock` → `cache/filelock.go`
- Add proper build tags: `filelock.go` → `filelock_unix.go` + `filelock_windows.go` (stub)

**Gates:**
- `go build ./internal/cache` compiles
- `GOOS=windows go build ./internal/cache` cross-compiles (filelock stub)
- `go test ./... -count=1` full suite passes
- `go vet ./...` clean
- Functional: `nugs catalog cache` reads cache correctly

**Commit:** `refactor: extract internal/cache with file locking`

---

### Phase 5: Extract `internal/api`

**Source:** `api_client.go`, `url_parser.go`

- Move HTTP client, auth functions, all API endpoint functions
- Move global `client`, `jar`, `qualityMap` vars (package-scoped in `api`)
- Move `checkUrl`, `regexStrings`, URL parsing functions
- Move `url_parser_test.go` → `internal/api/url_parser_test.go`

**Gates:**
- `go build ./internal/api` compiles
- `go test ./internal/api/... -count=1 -v` URL parser tests pass
- `go test ./... -count=1` full suite passes
- `go vet ./...` clean

**Commit:** `refactor: extract internal/api with HTTP client and URL parser`

---

### Phase 6: Extract `internal/config`

**Source:** `config.go`

- Move `readConfig`, `writeConfig`, `parseCfg`, `parseArgs`, `resolveFfmpegBinary`
- Move `normalizeCliAliases`, `isShowCountFilterToken`, `promptForConfig`
- Move `config_test.go`, `config_ffmpeg_test.go`

**Gates:**
- `go build ./internal/config` compiles
- `go test ./internal/config/... -count=1 -v` config + ffmpeg tests pass
- `go test ./... -count=1` full suite passes
- `go vet ./...` clean

**Commit:** `refactor: extract internal/config with config management`

---

### Phase 7: Extract `internal/rclone`

**Source:** `rclone.go`

- Move all rclone functions: `checkRcloneAvailable`, `uploadToRclone`, `remotePathExists`, `listRemoteArtistFolders`, `parseRcloneProgressLine`, etc.
- Move `rclone_test.go` → `internal/rclone/rclone_test.go`

**Gates:**
- `go build ./internal/rclone` compiles
- `go test ./internal/rclone/... -count=1 -v` rclone tests pass
- `go test ./... -count=1` full suite passes
- `go vet ./...` clean

**Commit:** `refactor: extract internal/rclone with cloud storage integration`

---

### Phase 8: Extract `internal/runtime`

**Source:** `runtime_status.go`, `crawl_control.go`, `progress_box_global.go`, all `detach_*.go`, `cancel_*.go`, `hotkey_input_*.go`, `process_alive_*.go`, `signal_persistence_*.go`

- Move all runtime session I/O, crawl control, hotkey handling
- Move all platform-specific files with build tags intact
- Move `detach_common.go` → `runtime/detach_common.go`

**Gates:**
- `go build ./internal/runtime` compiles
- `GOOS=windows go build ./internal/runtime` cross-compiles
- `GOOS=darwin go build ./internal/runtime` cross-compiles
- `go test ./... -count=1` full suite passes
- `go vet ./...` clean
- Functional: `nugs status` works, `nugs cancel` works

**Commit:** `refactor: extract internal/runtime with session and platform code`

---

### Phase 9: Extract `internal/completion`

**Source:** `completions.go`

- Move completion script generators

**Gates:**
- `go build ./internal/completion` compiles
- `go test ./... -count=1` full suite passes
- `go vet ./...` clean
- Functional: `nugs completion bash` produces valid script
- Functional: `nugs completion zsh` produces valid script

**Commit:** `refactor: extract internal/completion with shell completions`

---

### Phase 10: Extract `internal/download`

**Source:** `download.go`, `video.go`, `batch.go`, `tier3_methods.go` (non-display parts)

- Move `album`, `downloadTrack`, `processTrack`, `decryptTrack`, `tsToAac`
- Move `video`, `getVideoSku`, `getShowMediaType`, `downloadVideo`, `tsToMp4`
- Move `artist`, `playlist`, `catalogPlist`, `paidLstream`
- Handle `WriteCounter.Write()` callback pattern: add `OnPauseCheck func() error` and `OnProgress func(...)` fields, wired by callers

**Gates:**
- `go build ./internal/download` compiles
- `GOOS=windows go build ./internal/download` cross-compiles
- `go test ./... -count=1` full suite passes
- `go test ./... -race` no data races
- `go vet ./...` clean

**Commit:** `refactor: extract internal/download with audio, video, and batch engines`

---

### Phase 11: Extract `internal/catalog` + `internal/list`

**Source:** `catalog_handlers.go`, `catalog_autorefresh.go`, `catalog_analysis_mediaaware.go`, `list_commands.go`

- Move all catalog command handlers → `catalog/`
- Move list commands → `list/`
- Move `catalog_handlers_test.go` → `internal/catalog/handlers_test.go`

**Gates:**
- `go build ./internal/catalog` compiles
- `go build ./internal/list` compiles
- `go test ./internal/catalog/... -count=1 -v` catalog tests pass
- `go test ./... -count=1` full suite passes
- `go vet ./...` clean
- Functional: `nugs catalog stats` works
- Functional: `nugs catalog latest` works
- Functional: `nugs list artists` works

**Commit:** `refactor: extract internal/catalog and internal/list`

---

### Phase 12: Finalize `cmd/nugs/main.go` + cleanup

**Source:** `main.go` (what remains)

- `main.go` becomes thin orchestrator (~200 lines):
  - `init()` - banner printing
  - `main()` - calls `bootstrap()` then `run()`
  - `bootstrap()` - config detection, JSON flag parsing, calls `config.ParseCfg()`
  - `run()` - command routing, auth, dispatch
  - `handleListCommand()`, `handleCatalogCommand()`, `handleArtistShorthand()` - routing
  - `dispatch()` - URL iteration → delegates to `download.*` and `catalog.*`
  - `parseMediaModifier()` - media type extraction from args
- Remove ALL root-level `.go` files (they've all been moved to `internal/`)
- Verify no root-level `.go` files remain: `ls *.go` should return nothing

**Gates:**
- `go build ./cmd/nugs` produces binary
- `ls *.go 2>/dev/null | wc -l` returns 0 (no root .go files)
- `go test ./... -count=1` full suite passes
- `go test ./... -race` no data races
- `go vet ./...` clean
- `GOOS=windows go build ./cmd/nugs` cross-compiles
- `GOOS=darwin go build ./cmd/nugs` cross-compiles
- Binary size within 15% of pre-refactor binary
- `make build` works end-to-end
- **Full smoke test (all commands):**
  - `nugs help` - help output matches pre-refactor
  - `nugs list artists` - lists artists
  - `nugs list 1125` - lists shows for artist
  - `nugs catalog stats` - shows statistics
  - `nugs catalog cache` - shows cache info
  - `nugs catalog latest` - shows latest additions
  - `nugs catalog latest 5 video` - media modifier works
  - `nugs status` - shows runtime status
  - `nugs cancel` - cancel command works
  - `nugs completion bash` - generates bash completions
  - `nugs completion zsh` - generates zsh completions
  - `nugs 1125 latest` - artist shorthand (requires auth, optional)
- **Package count verification:**
  - `go list ./... | wc -l` returns expected count (~15 packages)
  - `go list ./... | grep internal | wc -l` returns ~13 (all internal packages)
- **No leaked exports:** `go doc ./internal/model` shows only intended public types

**Commit:** `refactor: finalize cmd/nugs entry point, remove root package`

---

## Key Design Decisions

### Shared Types (Circular Dependency Prevention)

`internal/model` is a **pure type-only leaf package** with zero internal imports. Every other package imports `model` for type definitions. This eliminates circular deps.

### Global State Migration

| Current Global | New Location | Strategy |
|---|---|---|
| `color*` vars | `ui.ColorRed`, etc. | Package-level vars in `ui/theme.go` |
| `symbol*` vars | `ui.SymbolCheck`, etc. | Package-level vars in `ui/theme.go` |
| `runErrorCount/Warning` | `ui.RunErrorCount` | Package-level vars in `ui/output.go` |
| `currentProgressBox` | `runtime.CurrentProgressBox` | Package-level in `runtime/progress_box.go` |
| `crawlerCtrl` | `runtime` package-level | Stays package-scoped |
| HTTP `client`, `jar` | `api` package-level | Stays package-scoped |

### ProgressBoxState Method Split

- **Struct definition** → `model/types.go`
- **Pure methods** (SetMessage, RequestRender, ResetForNewAlbum, Validate) → stay as methods on struct in `model`
- **Color-dependent display** (GetDisplayMessage) → standalone function in `ui/progress.go` accepting `*model.ProgressBoxState`

### WriteCounter Callback Pattern

`WriteCounter` struct stays in `model` but its `Write()` method (which calls `waitIfPausedOrCancelled` and `printProgress`) moves to `download`. Add callback fields:

```go
type WriteCounter struct {
    // ... existing fields
    OnPauseCheck func() error           // wired to runtime.WaitIfPausedOrCancelled
    OnProgress   func(downloaded int64)  // wired to ui.PrintProgress
}
```

### Args.Description()

The `Description()` method on `Args` references color variables. Solution: move to a helper function in `cmd/nugs/` that builds the colored description using `ui.ColorBold` etc., and set it via `go-arg`'s description mechanism.

### Platform-Specific Files

All platform files move to `internal/runtime/` with existing build tags preserved. `filelock.go` gets proper build tags added (`filelock_unix.go` + `filelock_windows.go` stub).

---

## Current File → New Package Mapping

| Current File | New Location | Package |
|---|---|---|
| `structs.go` (types) | `internal/model/types.go`, `api_types.go`, `catalog_types.go` | `model` |
| `structs.go` (colors/symbols) | `internal/ui/theme.go` | `ui` |
| `helpers.go` | `internal/helpers/paths.go`, `errors.go` | `helpers` |
| `format.go` | `internal/ui/format.go` | `ui` |
| `output.go` | `internal/ui/output.go` | `ui` |
| `tier3_methods.go` | `internal/ui/output.go` + `internal/download/tier3.go` | `ui` / `download` |
| `cache.go` | `internal/cache/cache.go`, `artist_cache.go` | `cache` |
| `filelock.go` | `internal/cache/filelock_unix.go`, `filelock_windows.go` | `cache` |
| `api_client.go` | `internal/api/client.go`, `catalog_api.go` | `api` |
| `url_parser.go` | `internal/api/url_parser.go` | `api` |
| `config.go` | `internal/config/config.go` | `config` |
| `rclone.go` | `internal/rclone/rclone.go` | `rclone` |
| `runtime_status.go` | `internal/runtime/status.go` | `runtime` |
| `crawl_control.go` | `internal/runtime/control.go` | `runtime` |
| `progress_box_global.go` | `internal/runtime/progress_box.go` | `runtime` |
| `detach_common.go` | `internal/runtime/detach_common.go` | `runtime` |
| `detach_unix.go` | `internal/runtime/detach_unix.go` | `runtime` |
| `detach_windows.go` | `internal/runtime/detach_windows.go` | `runtime` |
| `cancel_unix.go` | `internal/runtime/cancel_unix.go` | `runtime` |
| `cancel_windows.go` | `internal/runtime/cancel_windows.go` | `runtime` |
| `hotkey_input_unix.go` | `internal/runtime/hotkey_input_unix.go` | `runtime` |
| `hotkey_input_windows.go` | `internal/runtime/hotkey_input_windows.go` | `runtime` |
| `process_alive_unix.go` | `internal/runtime/process_alive_unix.go` | `runtime` |
| `process_alive_windows.go` | `internal/runtime/process_alive_windows.go` | `runtime` |
| `signal_persistence_unix.go` | `internal/runtime/signal_persistence_unix.go` | `runtime` |
| `signal_persistence_windows.go` | `internal/runtime/signal_persistence_windows.go` | `runtime` |
| `completions.go` | `internal/completion/completions.go` | `completion` |
| `download.go` | `internal/download/audio.go` | `download` |
| `video.go` | `internal/download/video.go` | `download` |
| `batch.go` | `internal/download/batch.go` | `download` |
| `catalog_handlers.go` | `internal/catalog/handlers.go`, `gaps.go`, `coverage.go` | `catalog` |
| `catalog_autorefresh.go` | `internal/catalog/autorefresh.go` | `catalog` |
| `catalog_analysis_mediaaware.go` | `internal/catalog/media_filter.go` | `catalog` |
| `list_commands.go` | `internal/list/list.go` | `list` |
| `main.go` | `cmd/nugs/main.go` | `main` |

### Test File Mapping

| Current Test File | New Location |
|---|---|
| `catalog_handlers_test.go` | `internal/catalog/handlers_test.go` |
| `config_test.go` | `internal/config/config_test.go` |
| `config_ffmpeg_test.go` | `internal/config/config_ffmpeg_test.go` |
| `format_render_test.go` | `internal/ui/format_render_test.go` |
| `helpers_test.go` | `internal/helpers/helpers_test.go` |
| `rclone_test.go` | `internal/rclone/rclone_test.go` |
| `url_parser_test.go` | `internal/api/url_parser_test.go` |

---

## Updated Makefile

```makefile
.PHONY: build clean install test vet check

build:
	@mkdir -p ~/.local/bin
	@echo "Building nugs..."
	@go build -o ~/.local/bin/nugs ./cmd/nugs
	@echo "done: ~/.local/bin/nugs"

clean:
	@rm -f ~/.local/bin/nugs ./nugs ./nugs-cli

test:
	@go test ./... -count=1

test-race:
	@go test ./... -race

vet:
	@go vet ./...

# Run all verification gates
check: build test vet
	@echo "All gates passed"

install: build
```

---

## Cross-Cutting Function Dependencies

Functions called across file boundaries (the key coupling points that must be resolved during migration):

| Called Function | Defined In | Called From | New Package |
|---|---|---|---|
| `sanitise()` | helpers.go | download, video, catalog, batch, rclone | `helpers` |
| `buildAlbumFolderName()` | helpers.go | download, catalog | `helpers` |
| `handleErr()` | helpers.go | everywhere | `helpers` |
| `makeDirs()` | helpers.go | download, video, batch, main | `helpers` |
| `getVideoOutPath()` | helpers.go | download, video, output | `helpers` |
| `getRcloneBasePath()` | helpers.go | rclone, output, catalog | `helpers` |
| `printSuccess/Error/Info/Warning()` | output.go | everywhere | `ui` |
| `printHeader/Section/KeyValue()` | format.go | everywhere | `ui` |
| `printProgress()` | format.go | download | `ui` |
| `renderProgressBox()` | format.go | download, video, crawl_control | `ui` |
| `readConfig/writeConfig()` | config.go | main, catalog_autorefresh | `config` |
| `auth()` | api_client.go | main | `api` |
| `getAlbumMeta()` | api_client.go | download, catalog | `api` |
| `getArtistMeta/Cached()` | api_client.go | batch, catalog, list_commands | `api` |
| `getStreamMeta()` | api_client.go | download, video | `api` |
| `checkUrl()` | url_parser.go | main | `api` |
| `getCacheDir()` | cache.go | runtime_status, detach, catalog | `cache` |
| `readCatalogCache()` | cache.go | catalog_handlers | `cache` |
| `uploadToRclone()` | rclone.go | download, video, batch | `rclone` |
| `remotePathExists()` | rclone.go | catalog_handlers | `rclone` |
| `waitIfPausedOrCancelled()` | crawl_control.go | download, video, batch | `runtime` |
| `setCurrentProgressBox()` | progress_box_global.go | batch, download | `runtime` |
| `isReadOnlyCommand()` | detach_common.go | main, crawl_control | `runtime` |
| `setupSessionPersistence()` | signal_*.go | main | `runtime` |

---

## Risks & Mitigations

| Risk | Mitigation |
|---|---|
| Circular dependencies during migration | Strict layered DAG; `model` has zero internal imports |
| `init()` ordering changes | Color `init()` in `ui/theme.go` runs on import; `cmd/nugs` imports `ui` early |
| Platform build breakage | Test `GOOS=windows go build` and `GOOS=darwin go build` after Phase 4+ |
| Test failures from moved packages | Extract shared test helpers to `internal/testutil/` first (Phase 0) |
| Large PR / hard to review | Each phase is a separate commit; can be reviewed incrementally |
| Global state races | Keep same package-level var pattern (no behavioral change, just new home) |
| Binary behavior changes | Gate 5 (Binary Equivalence) catches regressions at every phase |
| Missing exports after package split | Gate 1 compilation catches immediately; fix in-phase |
