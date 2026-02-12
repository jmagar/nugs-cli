# Nugs CLI Commands

Complete reference of all commands, subcommands, aliases, and flags.

---

## Global Flags

| Flag | Description |
|------|-------------|
| `-f <1-5>` | Audio download format: 1=ALAC, 2=FLAC, 3=MQA, 4=360RA, 5=AAC |
| `-F <1-5>` | Video download format: 1=480p, 2=720p, 3=1080p, 4=1440p, 5=4K |
| `-o <path>` | Output directory (created if missing) |
| `--force-video` | [Deprecated] Use `nugs grab <id> video` or `defaultOutputs` config |
| `--skip-videos` | [Deprecated] Use `nugs grab <id> audio` or `defaultOutputs` config |
| `--skip-chapters` | Skip chapters for video downloads |
| `--json <level>` | JSON output: `minimal`, `standard`, `extended`, `raw` |
| `--help` | Show help |

---

## Download Commands

Downloads require authentication (email/password or token in config).

### Direct URL Download

```
nugs <url> [url2 url3 ...]
```

Accepts any nugs.net URL:

| URL Pattern | Type |
|-------------|------|
| `https://play.nugs.net/release/<id>` | Album |
| `https://play.nugs.net/#/playlists/playlist/<id>` | Playlist |
| `https://play.nugs.net/library/playlist/<id>` | Playlist |
| `https://2nu.gs/<shortcode>` | Short URL (playlist) |
| `https://play.nugs.net/#/videos/artist/<aid>/<name>/<id>` | Video |
| `https://play.nugs.net/artist/<id>` | Full artist catalog |
| `https://play.nugs.net/artist/<id>/latest` | Latest artist shows |
| `https://play.nugs.net/artist/<id>/albums` | Artist albums |
| `https://play.nugs.net/livestream/<id>/exclusive` | Livestream |
| `https://play.nugs.net/watch/livestreams/exclusive/<id>` | Livestream |
| `https://play.nugs.net/#/my-webcasts/<id>` | Webcast |
| `https://play.nugs.net/library/webcast/<id>` | Webcast |
| `https://www.nugs.net/.../Stash-QueueVideo?...` | Paid livestream |
| `<numeric_id>` | Album by ID |

### Grab (Download Alias)

```
nugs grab <url_or_id> [media_modifier]
```

`grab` strips itself and passes args through. Equivalent to `nugs <url_or_id>`.

Media modifiers: `audio`, `video`, `both`

```
nugs grab 23329              # Download album 23329
nugs grab 23329 video        # Download video version
nugs grab 23329 both         # Download audio + video
```

### Artist Shortcuts

```
nugs <artist_id> latest [audio|video|both]
nugs <artist_id> full [audio|video|both]
```

| Command | Description |
|---------|-------------|
| `nugs 1125 latest` | Download latest shows from artist 1125 |
| `nugs 1125 latest video` | Download latest video shows |
| `nugs 1125 full` | Download entire artist catalog |
| `nugs 1125 full both` | Download full catalog (audio + video) |

---

## List Commands (Live API)

List commands query the nugs.net API in real-time. No authentication required.

### List Artists

```
nugs list
nugs list artists
nugs list [audio|video|both]
```

Lists all available artists on nugs.net.

### List Artists by Show Count

```
nugs list <operator><number>
nugs list artists shows <operator><number>
```

Filter artists by number of shows. Operators: `>`, `<`, `>=`, `<=`, `=`

```
nugs list >100               # Artists with more than 100 shows
nugs list <=50               # Artists with 50 or fewer shows
nugs list =25                # Artists with exactly 25 shows
```

### List Artist Shows

```
nugs list <artist_id> [audio|video|both]
```

Lists all shows for a specific artist.

```
nugs list 1125               # All Billy Strings shows
nugs list 1125 video         # Only video shows
nugs list 1125 both          # Shows with both formats
```

### List Shows by Venue

```
nugs list <artist_id> <venue_name>
nugs list <artist_id> shows <venue_name>
```

Filter shows by venue name (partial match).

```
nugs list 461 "Red Rocks"
nugs list 1125 shows "Capitol Theatre"
```

### List Latest Shows

```
nugs list <artist_id> latest [N]
```

Lists the N most recent shows for an artist (default: 10).

```
nugs list 1125 latest        # Latest 10 shows
nugs list 1125 latest 25     # Latest 25 shows
```

---

## Catalog Commands (Offline Cache)

Catalog commands work with a local cache at `~/.cache/nugs/`. No authentication required (except `gaps fill`).

### Update Catalog

```
nugs catalog update
nugs update
```

Fetches the latest catalog from nugs.net and updates the local cache.

### Cache Status

```
nugs catalog cache
nugs cache
```

Shows cache information (last updated, size, entry count).

### Catalog Statistics

```
nugs catalog stats
nugs stats
```

Displays catalog statistics (total shows, artists, date ranges).

### Latest Additions

```
nugs catalog latest [limit]
nugs latest [limit]
```

Shows the most recently added shows (default: 15).

```
nugs latest                  # Latest 15 additions
nugs latest 50               # Latest 50 additions
```

Note: Media type filters are not supported for `catalog latest` (data source lacks format details).

### Gap Detection

```
nugs catalog gaps <artist_id> [...] [audio|video|both] [--ids-only]
nugs gaps <artist_id> [...] [audio|video|both] [--ids-only]
```

Finds shows you haven't downloaded yet by comparing the catalog against your local/remote library.

| Flag | Description |
|------|-------------|
| `--ids-only` | Output only show IDs (for piping) |
| `audio` | Only check audio gaps |
| `video` | Only check video gaps |
| `both` | Check gaps for both formats |

```
nugs gaps 1125               # Missing Billy Strings shows
nugs gaps 1125 video         # Missing video shows
nugs gaps 1125 --ids-only    # Just IDs for scripting
nugs gaps 1125 461           # Gaps for multiple artists
```

### Gap Fill (Requires Auth)

```
nugs catalog gaps <artist_id> [...] [audio|video|both] fill
nugs gaps <artist_id> [...] [audio|video|both] fill
```

Automatically downloads all missing shows for an artist.

```
nugs gaps 1125 fill          # Download all missing shows
nugs gaps 1125 video fill    # Download missing video shows
nugs gaps 1125 461 fill      # Fill gaps for multiple artists
```

### Catalog List

```
nugs catalog list <artist_id> [...] [audio|video|both]
```

Lists all shows for an artist from the local cache (offline).

```
nugs catalog list 1125
nugs catalog list 1125 video
nugs catalog list 1125 461   # Multiple artists
```

### Coverage Report

```
nugs catalog coverage [artist_ids...] [audio|video|both]
nugs coverage [artist_ids...] [audio|video|both]
```

Shows download coverage statistics (total vs downloaded vs missing).

```
nugs coverage 1125           # Coverage for Billy Strings
nugs coverage 1125 video     # Video coverage
nugs coverage                # Coverage for all subscribed artists
```

---

## Auto-Refresh Configuration

Manages automatic catalog cache refresh at startup.

### Enable Auto-Refresh

```
nugs catalog config enable
nugs refresh enable
```

### Disable Auto-Refresh

```
nugs catalog config disable
nugs refresh disable
```

### Configure Auto-Refresh

```
nugs catalog config set
nugs refresh set
```

Interactive configuration for refresh schedule (daily/weekly), time, and timezone.

---

## Runtime Commands

### Status

```
nugs status
```

Shows the status of any running or recently completed download session (PID, state, progress).

### Cancel

```
nugs cancel
```

Cancels an active download crawl by sending a cancellation signal to the running process.

---

## Shell Completions

```
nugs completion <shell>
```

Generates shell completion scripts.

| Shell | Command |
|-------|---------|
| Bash | `nugs completion bash > /etc/bash_completion.d/nugs` |
| Zsh | `nugs completion zsh > ~/.zsh/completion/_nugs` |
| oh-my-zsh | `nugs completion zsh > ~/.oh-my-zsh/custom/completions/_nugs` |
| Fish | `nugs completion fish > ~/.config/fish/completions/nugs.fish` |
| PowerShell | `nugs completion powershell >> $PROFILE` |

---

## Help

```
nugs help
nugs --help
```

---

## Alias Summary

These top-level shortcuts expand to their full catalog equivalents:

| Alias | Expands To |
|-------|-----------|
| `nugs grab <args>` | `nugs <args>` |
| `nugs update` | `nugs catalog update` |
| `nugs cache` | `nugs catalog cache` |
| `nugs stats` | `nugs catalog stats` |
| `nugs latest` | `nugs catalog latest` |
| `nugs gaps` | `nugs catalog gaps` |
| `nugs coverage` | `nugs catalog coverage` |
| `nugs refresh <action>` | `nugs catalog config <action>` |
| `nugs list` | `nugs list artists` |
| `nugs list >100` | `nugs list artists shows >100` |
| `nugs list 1125 "Red Rocks"` | `nugs list 1125 shows "Red Rocks"` |
| `nugs help` | `nugs --help` |

---

## Hotkeys (During Downloads)

| Key | Action |
|-----|--------|
| `p` | Pause/resume download |
| `c` | Cancel download |

---

## Media Type Modifiers

Many commands accept a media type modifier:

| Modifier | Description |
|----------|-------------|
| `audio` | Audio only (ALAC/FLAC/MQA/360RA/AAC) |
| `video` | Video only (480p-4K) |
| `both` | Both audio and video |

The default media type is controlled by the `defaultOutputs` config field (default: `"audio"`).
