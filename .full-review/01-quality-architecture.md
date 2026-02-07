# Phase 1: Code Quality & Architecture Review

## Code Quality Findings

**Source:** `.full-review/01a-code-quality.md`

### Critical (3 findings)

| ID | Finding | Location |
|----|---------|----------|
| CX-01 | `main()` is 518 lines with ~80+ cyclomatic complexity | `main.go:4059-4576` |
| MT-01 | Entire 9,150-line codebase is one `package main` -- no separation of concerns | All files |
| TD-01 | API credentials (`devKey`, `clientId`) hardcoded in source code | `main.go:47-48` |

### High (16 findings)

| ID | Finding | Location |
|----|---------|----------|
| CX-02 | `album()` is 168 lines with deep nesting and mixed concerns | `main.go:2197-2365` |
| CX-03 | `processTrack()` is 186 lines with 7 parameters | `main.go:1933-2118` |
| CX-04 | `video()` is 148 lines with multiple nested conditionals | `main.go:3848-3996` |
| MT-02 | Global mutable state (HTTP client, config path, error counters) | `main.go:63-71` |
| MT-04 | `main.go` is 4,576 lines -- 10x beyond any reasonable single file | `main.go` |
| DUP-01 | Three `listArtist*` functions share 90% identical code (~430 duplicated lines) | `main.go:2682-3198` |
| DUP-02 | HTTP request pattern repeated 8+ times without abstraction | `main.go` (throughout) |
| CC-01 | `handleErr()` uses boolean parameter to control panic vs. print | `main.go:147-153` |
| CC-02 | `os.Args` manipulation for `--json` flag is fragile | `main.go` (in `main()`) |
| CC-03 | Magic numbers in media type switch (0-11 without constants) | `main.go:4536-4553` |
| TD-02 | `sanitise()` recompiles regex on every call | `main.go:1229-1232` |
| TD-03 | `checkUrl()` recompiles 12 regexes on every call | `main.go:1359-1368` |
| TD-04 | `decryptTrack()` uses hardcoded filename "temp_enc.ts" | `main.go:1847-1861` |
| TD-05 | `pkcs5Trimming` has no bounds checking -- potential panic | `main.go:1842-1845` |
| EH-01 | Cookie jar error silently ignored at package init | `main.go:64` |
| EH-02 | `parseTimestamps()` silently ignores `time.Parse` errors | `main.go:1334-1339` |

### Medium (14 findings)

| ID | Finding | Location |
|----|---------|----------|
| CX-05 | `renderProgressBox()` is 190 lines of dense formatting | `format.go:505-695` |
| CX-06 | `catalogCoverage()` is 215 lines with two distinct code paths | `catalog_handlers.go:749-964` |
| CX-07 | Three `listArtist*` functions are each 170+ lines | `main.go:2682-3198` |
| CX-08 | `Args.Description()` uses 60+ positional color arguments | `structs.go:173-263` |
| MT-03 | Non-idiomatic Go naming (`_url`, `_meta`, `_panic`) | `main.go` (throughout) |
| MT-05 | `ProgressBoxState` has 30+ fields spanning multiple concerns | `format.go:402-459` |
| DUP-03 | Atomic file write pattern duplicated across multiple files | `main.go`, `runtime_status.go`, `catalog_handlers.go` |
| DUP-04 | JSON marshal-and-print pattern repeated across catalog commands | `catalog_handlers.go` (throughout) |
| CC-04 | Format codes use magic integers (1-5) without named constants | `main.go`, `tier3_methods.go:80-95` |
| TD-06 | Config contains plaintext password | `structs.go` |
| TD-07 | No `context.Context` propagation for cancellation and timeouts | `main.go` (throughout) |
| EH-03 | Error and print mixed in same flow -- double-reporting | `main.go` (multiple locations) |
| EH-04 | `resp.Body` not deferred-closed in several HTTP functions | `main.go` (API functions) |
| EH-05 | `resolveCatPlistId` closes body before checking status code | `main.go:3998-4004` |

### Low (5 findings)

| ID | Finding | Location |
|----|---------|----------|
| DUP-05 | Date parsing with fallback duplicated in 3+ functions | `main.go:2712-2718, 2897-2903, 3062-3068` |
| CC-05 | `getPlan()` uses `reflect.ValueOf` for zero-check | `main.go:1326-1332` |
| CC-06 | 33-line commented-out code block (old `decryptTrack`) | `main.go:1808-1840` |
| EH-06 | Inconsistent error wrapping -- some use `%w`, most do not | Throughout codebase |
| EH-07 | `checkUrl` returns `0` for "no match" which is also a valid media type | `main.go:1359-1368` |

---

## Architecture Findings

**Source:** `.full-review/01b-architecture.md`

### Critical (2 findings)

| ID | Finding | Impact |
|----|---------|--------|
| ARCH-1.1 | Monolithic `main.go` (~4,500 lines, 8+ responsibilities) | Unmaintainable, unsafe to modify |
| ARCH-2.1 | Global mutable state (HTTP client, config path, error counter) | Untestable, invisible coupling |

### High (7 findings)

| ID | Finding | Impact |
|----|---------|--------|
| ARCH-1.2 | Everything in `package main`, no sub-packages | No encapsulation, no reusability |
| ARCH-1.3 | UI/formatting tightly coupled to business logic | Cannot use logic outside terminal context |
| ARCH-2.2 | No interfaces or dependency injection | Testing impossible without real network/filesystem |
| ARCH-2.3 | Hard-coded API credentials in source code | Cannot rotate keys, visible in Git history |
| ARCH-4.1 | `containerWithDate` struct defined 3 times | Maintenance trap, DRY violation |
| ARCH-4.2 | `ProgressBoxState` has 30+ fields, 5+ responsibilities | SRP violation, fragile state machine |
| ARCH-6.1 | Four different error handling strategies | Unpredictable behavior, impossible error recovery |

### Medium (11 findings)

| ID | Finding | Impact |
|----|---------|--------|
| ARCH-3.1 | CLI dispatch is ~400-line if/else chain | Hard to extend, error-prone |
| ARCH-3.2 | No structured error returns from commands | Inconsistent success/failure signaling |
| ARCH-3.3 | JSON output built ad-hoc with `map[string]any` | Not type-safe, inconsistent |
| ARCH-4.3 | `Config` struct mixes auth, download, rclone, catalog concerns | Over-exposed credentials |
| ARCH-4.4 | Extensive use of `any` type in API response structs | Runtime type errors |
| ARCH-5.1 | No repository pattern for data access | Scattered file I/O, inconsistent |
| ARCH-5.2 | Sorting logic duplicated in 4+ locations | DRY violation |
| ARCH-5.3 | Regex compiled on every call | Unnecessary CPU/memory allocation |
| ARCH-5.4 | No retry/resilience patterns for HTTP calls | Fragile batch downloads |
| ARCH-6.2 | File locking Unix-only without build tags | Windows build will fail |
| ARCH-6.3 | Atomic file writes inconsistently applied | File corruption risk |

### Low / Positive (2 findings)

| ID | Finding | Impact |
|----|---------|--------|
| ARCH-1.4 | Platform-specific code well-structured with build tags | Good practice |
| ARCH-2.4 | Minimal external dependencies | Low supply chain risk |

---

## Critical Issues for Phase 2 Context

The following findings should inform the Security and Performance reviews:

### Security-Relevant

1. **TD-01 / ARCH-2.3:** Hard-coded API credentials (`devKey`, `clientId`) in source code. Evaluate exposure risk and rotation feasibility.
2. **MT-02 / ARCH-2.1:** Global HTTP client with shared cookie jar could lead to credential leakage across requests.
3. **TD-06:** Config stores plaintext password in JSON file on disk.
4. **ARCH-6.1:** Inconsistent error handling may lead to sensitive information disclosure (tokens, paths, API responses printed to terminal).
5. **TD-05:** `pkcs5Trimming` has no bounds checking -- malformed encrypted data could cause panics exposing internal state.
6. **TD-04:** Hardcoded temp filename "temp_enc.ts" -- potential for symlink attacks or data leakage via predictable temp files.

### Performance-Relevant

1. **TD-02 / TD-03 / ARCH-5.3:** Regex recompilation on every call to `sanitise()` and `checkUrl()`. Direct performance issue during batch downloads with thousands of file operations.
2. **ARCH-5.4:** No retry logic means batch downloads are fragile; a single transient error can abort hours of work.
3. **MT-05 / ARCH-4.2:** `ProgressBoxState` with 30+ fields and mutex may cause lock contention under heavy concurrent updates.
4. **TD-07:** No `context.Context` propagation means HTTP requests have no timeouts, risking indefinite hangs.
5. **EH-02:** Silent timestamp parsing failures could cause authentication timeouts and retries.

### Testing-Relevant

1. **ARCH-2.2:** No interfaces or dependency injection makes the codebase nearly untestable without real network/filesystem access.
2. **MT-02 / ARCH-2.1:** Global mutable state prevents test isolation and parallel test execution.
3. **DUP-01:** Three near-identical functions mean three sets of tests to maintain (or three untested paths).

---

## Combined Statistics

| Category | Critical | High | Medium | Low | Total |
|----------|----------|------|--------|-----|-------|
| Code Complexity | 1 | 4 | 3 | 0 | 8 |
| Maintainability | 1 | 2 | 2 | 0 | 5 |
| Code Duplication | 0 | 2 | 2 | 1 | 5 |
| Clean Code | 0 | 2 | 1 | 3 | 6 |
| Technical Debt | 1 | 4 | 2 | 0 | 7 |
| Error Handling | 0 | 2 | 4 | 1 | 7 |
| Architecture (Boundaries) | 2 | 3 | 0 | 1 | 6 |
| Architecture (Dependencies) | 1 | 2 | 0 | 1 | 4 |
| Architecture (API Design) | 0 | 1 | 2 | 0 | 3 |
| Architecture (Data Model) | 0 | 2 | 2 | 0 | 4 |
| Architecture (Design Patterns) | 0 | 0 | 4 | 0 | 4 |
| Architecture (Consistency) | 0 | 1 | 3 | 0 | 4 |
| **Totals (deduplicated)** | **5** | **23** | **25** | **7** | **60** |

Note: Several findings overlap between Code Quality and Architecture reviews (e.g., monolithic main.go, global state, regex recompilation, hardcoded credentials). Deduplicated count removes overlapping entries.
