# Phase 2: Security & Performance Review

## Security Findings

### SEC-01: API credentials hardcoded in source code

- **Severity:** High
- **File:** `main.go:47-48`
- **CWE:** CWE-798 (Use of Hard-Coded Credentials)
- **Description:** The API key and client ID are embedded as string constants:
  ```go
  devKey   = "x7f54tgbdyc64y656thy47er4"
  clientId = "Eg7HuH873H65r5rt325UytR5429"
  ```
  These values are compiled directly into the binary and are trivially extractable via `strings nugs | grep -E '[a-zA-Z0-9]{20,}'`. They are also permanently visible in the Git history. If these credentials are ever revoked or rotated by Nugs.net, every deployed binary becomes non-functional.
- **Attack Scenario:** An attacker can extract these credentials from the binary and use them to access the Nugs.net API directly, potentially impersonating the application or abusing rate limits.
- **Remediation:** Since these appear to be public API keys extracted from the Nugs.net Android APK (a common pattern in reverse-engineered music clients), the immediate risk is limited. However, document them explicitly as public keys. For defense-in-depth, allow overriding via environment variables.

```go
func getDevKey() string {
    if key := os.Getenv("NUGS_DEV_KEY"); key != "" {
        return key
    }
    return "x7f54tgbdyc64y656thy47er4" // Public Android APK key
}
```

---

### SEC-02: User password stored in plaintext in config.json

- **Severity:** High
- **File:** `structs.go` (Config struct), `main.go:506-507` (promptForConfig)
- **CWE:** CWE-256 (Plaintext Storage of a Password)
- **Description:** The user's Nugs.net password is stored in plaintext in `config.json`:
  ```json
  {
    "email": "user@example.com",
    "password": "s3cretP@ss!"
  }
  ```
  While `readConfig()` (line 1184-1202) does check file permissions and auto-fixes to 0600, the password is still written to disk unencrypted. On shared systems, backup programs, log aggregation, or accidental `cat config.json` in a terminal share session could expose the password.
- **Attack Scenario:** On a shared machine or if the filesystem is compromised, the password is immediately accessible. If the user reuses this password for other services, the blast radius extends beyond Nugs.net.
- **Remediation:**
  1. **Immediate:** The existing permission check and auto-fix (0600) is a good first step. Keep it.
  2. **Better:** Store an OAuth refresh token instead of the password. The `auth()` function already returns an `access_token` and `refresh_token`. Store only the refresh token and re-authenticate using it.
  3. **Best:** Use OS keychain integration (macOS Keychain, Linux libsecret, Windows Credential Manager) via a library like `zalando/go-keyring`.

---

### SEC-03: Password echoed to terminal during first-time setup

- **Severity:** Medium
- **File:** `main.go:393-396`
- **CWE:** CWE-549 (Missing Password Field Masking)
- **Description:** During the first-run config setup, the password is read from a standard `bufio.Scanner` with no terminal echo suppression:
  ```go
  fmt.Printf("Enter your Nugs.net password: ")
  scanner.Scan()
  password := strings.TrimSpace(scanner.Text())
  ```
  The password is displayed in cleartext on the terminal as the user types. This is visible to anyone observing the screen, in terminal recordings (asciinema), and in terminal scrollback buffers.
- **Remediation:** Use `golang.org/x/term` (already a dependency) for password input:

```go
import "golang.org/x/term"

fmt.Printf("Enter your Nugs.net password: ")
passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
if err != nil {
    return fmt.Errorf("reading password: %w", err)
}
fmt.Println() // Newline after hidden input
password := strings.TrimSpace(string(passwordBytes))
```

---

### SEC-04: JWT payload parsed without signature verification

- **Severity:** Medium
- **File:** `main.go:1370-1382`
- **CWE:** CWE-345 (Insufficient Verification of Data Authenticity)
- **Description:** The `extractLegToken()` function decodes the JWT payload without verifying the signature:
  ```go
  func extractLegToken(tokenStr string) (string, string, error) {
      payload := strings.SplitN(tokenStr, ".", 3)[1]
      decoded, err := base64.RawURLEncoding.DecodeString(payload)
      // ... extracts LegacyToken and LegacyUguid from claims
  }
  ```
  While this is acceptable for a client-side tool (the token was received directly from the auth server over HTTPS), there is no bounds checking on the `SplitN` result. If `tokenStr` contains fewer than 2 dots, `[1]` will panic with an index-out-of-range error.
- **Remediation:** Add bounds validation:

```go
func extractLegToken(tokenStr string) (string, string, error) {
    parts := strings.SplitN(tokenStr, ".", 3)
    if len(parts) < 2 {
        return "", "", fmt.Errorf("malformed JWT: expected 3 parts, got %d", len(parts))
    }
    decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
    if err != nil {
        return "", "", fmt.Errorf("decoding JWT payload: %w", err)
    }
    // ...
}
```

---

### SEC-05: HTTP client has no timeout -- potential indefinite hang

- **Severity:** Medium
- **File:** `main.go:65`
- **CWE:** CWE-400 (Uncontrolled Resource Consumption)
- **Description:** The global HTTP client is created with no timeout:
  ```go
  client = &http.Client{Jar: jar}
  ```
  Go's default `http.Client` has zero timeout, meaning requests can block indefinitely if the server is slow, unresponsive, or the connection is disrupted without a TCP RST. This can cause the application to hang indefinitely, consuming system resources (goroutines, file descriptors, memory).
- **Attack Scenario:** A malicious or misconfigured proxy between the client and `play.nugs.net` performs a "slowloris"-style attack, sending response bytes at an extremely slow rate. The application hangs forever.
- **Remediation:**

```go
client = &http.Client{
    Jar:     jar,
    Timeout: 60 * time.Second,
}
```

For download operations that legitimately take longer, use a separate client or per-request context with a longer timeout.

---

### SEC-06: Hardcoded temp file "temp_enc.ts" -- predictable file path

- **Severity:** Medium
- **File:** `main.go:1848`
- **CWE:** CWE-377 (Insecure Temporary File)
- **Description:** The `decryptTrack()` function reads encrypted data from a hardcoded path `"temp_enc.ts"` in the current working directory. This path is:
  1. **Predictable:** An attacker who can write to the CWD can replace it with a symlink to a sensitive file, causing the application to read and "decrypt" (corrupting) that file.
  2. **Not unique:** Multiple concurrent downloads in the same directory will overwrite each other's temp files, causing data corruption.
  3. **Not cleaned up on error:** If decryption fails, the encrypted file remains on disk.
- **Remediation:** Use `os.CreateTemp` for temporary files and accept the path as a parameter.

```go
func decryptTrack(key, iv []byte, encPath string) ([]byte, error) {
    encData, err := os.ReadFile(encPath)
    if err != nil {
        return nil, fmt.Errorf("reading encrypted data: %w", err)
    }
    defer os.Remove(encPath) // Clean up temp file
    // ... decryption logic
}
```

---

### SEC-07: pkcs5Trimming vulnerable to padding oracle information leak

- **Severity:** Medium
- **File:** `main.go:1842-1845`
- **CWE:** CWE-354 (Improper Validation of Integrity Check Value)
- **Description:** The PKCS5/PKCS7 padding removal function performs no validation:
  ```go
  func pkcs5Trimming(data []byte) []byte {
      padding := data[len(data)-1]
      return data[:len(data)-int(padding)]
  }
  ```
  If `data` is empty, this panics (index out of range). If the `padding` value exceeds `len(data)`, this panics (slice bounds out of range). If the data is corrupted, `padding` could be 0 (creating an infinite-length slice) or 255 (creating a negative-length slice). While this is a local CLI tool (not a network service), the crash behavior on malformed data can be disruptive during batch downloads.
- **Remediation:** Validate padding before trimming (see TD-05 in Phase 1 for the full code example with PKCS5 verification).

---

### SEC-08: Runtime control files writable by any local user (0644 permissions)

- **Severity:** Low
- **File:** `runtime_status.go:116`
- **CWE:** CWE-732 (Incorrect Permission Assignment for Critical Resource)
- **Description:** Runtime status and control files are written with 0644 permissions:
  ```go
  if err := writeFileAtomic(runtimeStatusPath, data, 0644); err != nil {
  ```
  The control file (`runtime-control.json`) accepts `pause` and `cancel` commands. Any local user who can write to `~/.cache/nugs/runtime-control.json` can pause or cancel another user's download. While this is a minor issue on single-user systems, on shared machines it allows denial-of-service against running downloads.
- **Remediation:** Use 0600 for control files that accept commands:

```go
writeFileAtomic(runtimeStatusPath, data, 0600)
```

---

### SEC-09: No TLS certificate validation override or pinning

- **Severity:** Low (Informational)
- **File:** `main.go:65`
- **Description:** The HTTP client uses Go's default TLS configuration, which validates certificates against the system trust store. This is the correct default behavior. There is no `InsecureSkipVerify: true` anywhere in the codebase, which is a positive finding. However, there is also no certificate pinning, meaning a compromised CA could issue a fraudulent certificate for `play.nugs.net`.
- **Note:** Certificate pinning is generally not recommended for CLI tools as it creates update and maintenance burden. The current default behavior is appropriate.

---

### SEC-10: exec.Command used safely -- no shell injection vectors found

- **Severity:** Low (Positive Finding)
- **File:** `main.go` (throughout)
- **Description:** All `exec.Command` calls use the safe variadic argument form (not `exec.Command("sh", "-c", userInput)`). Arguments are passed as separate strings, preventing shell injection. The `validatePath()` function (line 542-548) additionally blocks null bytes and newlines in paths. The comment at line 544 correctly notes that `exec.Command` handles shell metacharacters safely.

  Examples of safe usage:
  ```go
  exec.Command("rclone", "copy", localPath, remoteFullPath, transfersFlag)
  exec.Command(ffmpegNameStr, "-i", "pipe:", "-c:a", "copy", outPath)
  ```

---

### SEC-11: OAuth token transmitted in HTTP headers over HTTPS

- **Severity:** Low (Positive Finding)
- **File:** `main.go:1285`, `main.go:1308`
- **Description:** Bearer tokens are transmitted via the `Authorization` header to HTTPS endpoints. The `authUrl` (`https://id.nugs.net/...`), `subInfoUrl`, and `userInfoUrl` all use HTTPS. The token is never logged or printed to stdout (except potentially in error messages if the HTTP request fails with a non-200 status). This is correct behavior.

---

## Performance Findings

### PERF-01: Regex recompilation on every call to sanitise() -- O(n * compile) per batch

- **Severity:** High
- **File:** `main.go:1229-1232`
- **Estimated Impact:** 10-50ms wasted per file operation. In a batch download of 500 tracks across 10 albums, this adds 5-25 seconds of pure overhead.
- **Description:** The `sanitise()` function compiles a regex from the constant `sanRegexStr` on every invocation:
  ```go
  func sanitise(filename string) string {
      san := regexp.MustCompile(sanRegexStr).ReplaceAllString(filename, "_")
      return strings.TrimSuffix(san, "\t")
  }
  ```
  Regex compilation involves NFA/DFA construction and memory allocation. The regex pattern `[\/:*?"><|]` is simple enough that pre-compilation eliminates all overhead.
- **Optimization:**

```go
var sanRegex = regexp.MustCompile(sanRegexStr)

func sanitise(filename string) string {
    return strings.TrimSuffix(sanRegex.ReplaceAllString(filename, "_"), "\t")
}
```

---

### PERF-02: checkUrl() recompiles 12 regexes per URL -- O(12 * compile) per URL

- **Severity:** High
- **File:** `main.go:1359-1368`
- **Estimated Impact:** 60-100ms per URL. In batch mode with 100 URLs, this adds 6-10 seconds of overhead.
- **Description:** The `checkUrl()` function iterates over 12 regex patterns, compiling each one from its string representation on every call:
  ```go
  func checkUrl(_url string) (string, int) {
      for i, regexStr := range regexStrings {
          regex := regexp.MustCompile(regexStr)
          match := regex.FindStringSubmatch(_url)
          // ...
      }
  }
  ```
  This is especially wasteful because all 12 patterns are constants defined at package level.
- **Optimization:**

```go
var compiledRegexes = func() []*regexp.Regexp {
    compiled := make([]*regexp.Regexp, len(regexStrings))
    for i, pattern := range regexStrings {
        compiled[i] = regexp.MustCompile(pattern)
    }
    return compiled
}()

func checkUrl(urlStr string) (string, int) {
    for i, re := range compiledRegexes {
        if match := re.FindStringSubmatch(urlStr); match != nil {
            return match[1], i
        }
    }
    return "", -1
}
```

---

### PERF-03: contains() function uses O(n) linear scan instead of map lookup

- **Severity:** Medium
- **File:** `main.go:201-208`
- **Estimated Impact:** Negligible for small lists, but O(n^2) overall when called in a loop (processUrls).
- **Description:** The `contains()` function performs a case-insensitive linear scan:
  ```go
  func contains(lines []string, value string) bool {
      for _, line := range lines {
          if strings.EqualFold(line, value) {
              return true
          }
      }
      return false
  }
  ```
  This is called from `processUrls()` for every URL in the input list, making the overall deduplication O(n^2). For typical usage (10-50 URLs), this is not a problem. For batch operations with text files containing 500+ URLs, it becomes noticeable.
- **Optimization:** Use a `map[string]bool` for deduplication:

```go
func processUrls(urls []string) ([]string, error) {
    seen := make(map[string]bool)
    var processed []string
    // ... use seen[strings.ToLower(url)] for O(1) lookups
}
```

---

### PERF-04: No HTTP connection pooling configuration

- **Severity:** Medium
- **File:** `main.go:65`
- **Estimated Impact:** 100-500ms per connection establishment (TLS handshake). Over 100 API calls in a batch operation, this adds 10-50 seconds vs. connection reuse.
- **Description:** The HTTP client uses Go's default transport, which does support connection pooling but with default limits (100 idle connections, 2 per host). For the Nugs.net API, all requests go to a small number of hosts (`play.nugs.net`, `streamapi.nugs.net`, `id.nugs.net`). The default settings are adequate for this use case.

  However, there is no explicit `MaxIdleConnsPerHost` configuration, and the default of 2 connections per host may be suboptimal for batch downloads that make many sequential API calls to the same endpoint.
- **Optimization:**

```go
transport := &http.Transport{
    MaxIdleConnsPerHost: 10,
    IdleConnTimeout:     90 * time.Second,
}
client = &http.Client{
    Jar:       jar,
    Timeout:   60 * time.Second,
    Transport: transport,
}
```

---

### PERF-05: Date sorting parses date strings on every comparison -- O(n log n * parse)

- **Severity:** Medium
- **File:** `main.go` (listArtistShows and similar), `catalog_handlers.go` (catalogLatest)
- **Estimated Impact:** For an artist with 500 shows, sort performs ~4,500 comparisons, each parsing 2 dates = 9,000 `time.Parse` calls. At ~1us per parse, this adds ~9ms -- not catastrophic but easily avoidable.
- **Description:** The sorting comparator parses date strings inside the `less` function:
  ```go
  sort.Slice(allContainers, func(i, j int) bool {
      ti, _ := time.Parse("2006-01-02", allContainers[i].dateStr)
      tj, _ := time.Parse("2006-01-02", allContainers[j].dateStr)
      return ti.Before(tj)
  })
  ```
  Each comparison re-parses both date strings. For `n` items, `sort.Slice` makes O(n log n) comparisons, resulting in O(2n log n) parse operations.
- **Optimization:** Parse dates once before sorting:

```go
type showWithParsedDate struct {
    container *AlbArtResp
    date      time.Time
}

parsed := make([]showWithParsedDate, len(allContainers))
for i, c := range allContainers {
    t, _ := time.Parse("2006-01-02", c.dateStr)
    parsed[i] = showWithParsedDate{container: c.container, date: t}
}

sort.Slice(parsed, func(i, j int) bool {
    return parsed[i].date.Before(parsed[j].date)
})
```

---

### PERF-06: No retry logic for transient network failures

- **Severity:** Medium
- **File:** `main.go` (all API functions)
- **Estimated Impact:** A single transient 503 or network timeout aborts the entire batch download, potentially wasting hours of previous download time.
- **Description:** All HTTP requests to the Nugs.net API are single-attempt with no retry logic. Common transient failures (HTTP 429 Too Many Requests, HTTP 503 Service Unavailable, TCP reset, DNS resolution timeout) result in immediate error propagation and batch termination.

  For a tool designed for long-running batch downloads (downloading an entire artist catalog of 500+ shows), this makes the operation fragile. The user must manually restart and the tool must re-check all already-downloaded items.
- **Remediation:** Implement retry with exponential backoff for idempotent GET requests:

```go
func (c *NugsClient) getWithRetry(ctx context.Context, url string, result interface{}) error {
    maxRetries := 3
    for attempt := 0; attempt <= maxRetries; attempt++ {
        err := c.get(ctx, url, result)
        if err == nil {
            return nil
        }
        if !isRetryable(err) {
            return err
        }
        backoff := time.Duration(1<<uint(attempt)) * time.Second
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(backoff):
        }
    }
    return fmt.Errorf("failed after %d retries", maxRetries)
}
```

---

### PERF-07: calculateLocalSize walks entire directory tree synchronously before upload

- **Severity:** Low
- **File:** `main.go:553-571`
- **Estimated Impact:** For a large album directory with 100+ files, `filepath.Walk` adds 10-50ms of I/O before the upload starts.
- **Description:** Before uploading via rclone, `calculateLocalSize()` performs a full directory walk to calculate total bytes. This is used only for progress bar initialization. The directory walk is synchronous and blocks the upload from starting.
- **Remediation:** This is acceptable for the progress bar use case. If optimization is desired, the size could be accumulated during the download phase (since each track's size is already known) rather than walking the filesystem after download completes.

---

### PERF-08: parseHumanizedBytes called multiple times per progress update

- **Severity:** Low
- **File:** `main.go:739-775`
- **Estimated Impact:** Negligible -- string parsing is fast. But the function is called 3 times per rclone progress update (speed, total, uploaded), and rclone reports progress every second.
- **Description:** The `parseHumanizedBytes()` function parses human-readable byte strings (e.g., "8.2 MB") back to integer bytes. It is called within the upload progress callback (lines 638, 644, 645) on every progress line from rclone. This is a reverse operation that could be avoided if rclone's `--use-json-log` flag were used to get machine-readable output.
- **Remediation:** Consider using `rclone --use-json-log` for machine-readable progress output, or accept the current approach as adequate for progress display purposes.

---

### PERF-09: ProgressBoxState mutex held during rendering operations

- **Severity:** Low
- **File:** `format.go:505-695`, `main.go:626-654`
- **Estimated Impact:** Microseconds of contention per render cycle. The upload progress callback correctly releases the lock before calling `renderProgressBox()`.
- **Description:** The `renderProgressBox()` function reads from `ProgressBoxState` fields that are protected by a mutex. The pattern in the upload callback is correct -- it acquires the lock, updates fields, releases the lock, then renders:
  ```go
  progressBox.mu.Lock()
  // ... update fields ...
  progressBox.mu.Unlock()
  renderProgressBox(progressBox) // Render outside lock
  ```
  However, `renderProgressBox()` reads the same fields without acquiring the lock, creating a potential data race if another goroutine updates the state between the unlock and the render. In practice, the visual impact of a stale render is negligible (progress bar shows slightly old data for one frame).
- **Remediation:** For correctness, either render while holding the lock (accepting the contention) or snapshot the relevant fields while holding the lock and render from the snapshot.

---

## Critical Issues for Phase 3 Context

### Testing-Relevant

1. **SEC-02:** Password storage in plaintext should be tested -- verify that config files are created with 0600 permissions, verify that the permission warning fires on insecure permissions.
2. **SEC-03:** Password echo should be tested -- verify that password input uses terminal echo suppression.
3. **SEC-04:** JWT parsing should be tested with malformed tokens (fewer than 2 dots, invalid base64, missing fields) to prevent panics.
4. **SEC-07:** pkcs5Trimming should be tested with edge cases: empty input, padding > data length, padding = 0.
5. **PERF-01/02:** Pre-compiled regexes should be benchmarked to verify the performance improvement.

### Documentation-Relevant

1. **SEC-01:** Hardcoded API credentials should be documented as public keys with provenance (extracted from Android APK).
2. **SEC-02:** Config file security model should be documented: permissions, auto-fix behavior, password storage warning.
3. **PERF-06:** The lack of retry logic should be documented as a known limitation with a workaround (manually restart).

---

## Summary

| Category | Critical | High | Medium | Low | Total |
|----------|----------|------|--------|-----|-------|
| Security | 0 | 2 | 5 | 4 | **11** |
| Performance | 0 | 2 | 4 | 3 | **9** |
| **Totals** | **0** | **4** | **9** | **7** | **20** |

### Highest Priority Security Fixes

1. **SEC-02:** Store OAuth refresh token instead of plaintext password
2. **SEC-03:** Mask password input during first-run setup (trivial fix using existing dependency)
3. **SEC-04:** Add bounds checking to JWT payload extraction (prevents panic)
4. **SEC-05:** Set HTTP client timeout to prevent indefinite hangs

### Highest Priority Performance Fixes

1. **PERF-01:** Pre-compile `sanitise()` regex (eliminates thousands of unnecessary compilations per batch)
2. **PERF-02:** Pre-compile `checkUrl()` regexes (eliminates 12 compilations per URL)
3. **PERF-06:** Add retry logic for transient network failures (prevents batch abort on single failure)
