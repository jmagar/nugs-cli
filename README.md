# Nugs CLI

Download your favorite live shows from Nugs.net, browse 13,000+ concerts offline, and never miss a new release.

Built for Deadheads, jam band fans, and anyone who wants their live music collection organized and accessible.

![Nugs CLI terminal interface showing catalog browsing and artist listings](https://i.imgur.com/NOsQjnP.png)
![Nugs CLI download progress showing multiple format options and statistics](https://i.imgur.com/BEudufy.png)

[![Release](https://img.shields.io/github/v/release/jmagar/nugs-cli)](https://github.com/jmagar/nugs-cli/releases)
[![Go Version](https://img.shields.io/badge/Go-1.16+-blue.svg)](https://golang.org)
[![Platform](https://img.shields.io/badge/platform-linux%20%7C%20macos%20%7C%20windows-lightgrey)](https://github.com/jmagar/nugs-cli/releases)

## Table of Contents

- [Features](#features)
- [Installation](#installation)
  - [Pre-built Binaries](#pre-built-binaries)
  - [Building from Source](#building-from-source)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Supported Media Types](#supported-media-types)
- [Usage](#usage)
  - [Downloading Content](#downloading-content)
  - [Browse & List](#browse--list)
  - [Catalog Management](#catalog-management)
  - [JSON Output](#json-output)
- [Advanced Features](#advanced-features)
  - [Auto-Refresh](#auto-refresh)
  - [Gap Detection](#gap-detection)
  - [Rclone Integration](#rclone-integration)
- [FFmpeg Setup](#ffmpeg-setup)
- [Command Reference](#command-reference)
- [Examples](#examples)
- [Troubleshooting](#troubleshooting)
- [Disclaimer](#disclaimer)

---

## Features

### Media Download
- Multiple formats: ALAC, FLAC, MQA, 360 Reality Audio, AAC
- Video support: Up to 4K with chapter markers
- Batch downloads: Process multiple albums, playlists, or text files
- Artist shortcuts: Download entire catalogs or latest releases

### Catalog Management
- Offline browsing: Cache entire Nugs.net catalog (13,000+ shows)
- Fast search: Instant statistics and show discovery
- Gap detection: Find missing shows in your collection
- Auto-refresh: Automatic daily/weekly catalog updates
- JSON output: Script-friendly structured data

### Discovery & Analysis
- List artists: Browse all 300+ artists
- View shows: See complete discography for any artist
- Statistics: Top artists, date ranges, collection coverage
- Latest additions: Track new releases

### Power User Features
- **Rclone integration**: Auto-upload to cloud storage
- **Flexible configuration**: CLI args override config file
- **File locking**: Prevents corruption from concurrent writes

---

## Installation

### Pre-built Binaries

Download the latest release for your platform:
- [Windows, Linux, macOS binaries](https://github.com/jmagar/nugs-cli/releases)

### Building from Source

**Requirements:** Go 1.16+ and Make

```bash
git clone https://github.com/jmagar/nugs-cli.git
cd nugs-cli
make build
```

The binary installs to `~/.local/bin/nugs` (already in your PATH).

> **📖 Developers:** See [CLAUDE.md](CLAUDE.md) for detailed build instructions and development guide.

---

## Quick Start

Alright, let's get you downloading shows in about 2 minutes.

**Step 1: Run initial setup** (displays welcome screen and creates config):
```bash
nugs
```

**Step 2: Configure your credentials** in `~/.nugs/config.json`:
```json
{
  "email": "your-email@example.com",
  "password": "your-password",
  "outPath": "/path/to/downloads",
  "format": 2
}
```

> **💡 Tip:** Most folks use `format: 2` (FLAC) for the best quality-to-size ratio.

**Step 3: Download your first show**:
```bash
nugs 23329
```

**Step 4: Browse the catalog**:
```bash
nugs list artists
nugs list 1125  # Billy Strings
```

> **ℹ️ Note:** Running `nugs` with no arguments shows the latest 15 catalog additions - a quick way to discover new releases!

---

## Configuration

Now that you've seen what Nugs CLI can do, let's customize it for your workflow.

The config file lives at `~/.nugs/config.json`. On first run, you'll be prompted to create it.

### Core Settings

| Option | Description | Values |
|--------|-------------|--------|
| `email` | Your Nugs.net email | string |
| `password` | Your Nugs.net password | string |
| `format` | Audio download quality | `1` = 16-bit/44.1kHz ALAC<br>`2` = 16-bit/44.1kHz FLAC<br>`3` = 24-bit/48kHz MQA<br>`4` = 360 Reality Audio<br>`5` = 150 Kbps AAC |
| `videoFormat` | Video download quality | `1` = 480p<br>`2` = 720p<br>`3` = 1080p<br>`4` = 1440p<br>`5` = 4K/best available |
| `outPath` | Download destination | Path (created if doesn't exist) |

### Advanced Settings

| Option | Description | Default |
|--------|-------------|---------|
| `token` | Auth token for Apple/Google accounts ([guide](token.md)) | - |
| `useFfmpegEnvVar` | Use FFmpeg from PATH vs script dir | `true` |
| `forceVideo` | Force video when audio+video available | `false` |
| `skipVideos` | Skip videos in artist downloads | `false` |
| `skipChapters` | Skip chapter markers for videos | `false` |

### Rclone Settings

| Option | Description |
|--------|-------------|
| `rcloneEnabled` | Enable auto-upload to cloud storage |
| `rcloneRemote` | Rclone remote name (e.g., `gdrive`) |
| `rclonePath` | Path on remote (e.g., `/Music/Nugs`) |
| `deleteAfterUpload` | Delete local files after successful upload |
| `rcloneTransfers` | Number of parallel transfers (default: 4) |

### Catalog Auto-Refresh

| Option | Description | Default |
|--------|-------------|---------|
| `catalogAutoRefresh` | Enable automatic catalog updates | `false` |
| `catalogRefreshTime` | Time to refresh (24-hour format) | `"05:00"` |
| `catalogRefreshTimezone` | Timezone for refresh time | `"America/New_York"` |
| `catalogRefreshInterval` | Refresh frequency | `"daily"` or `"weekly"` |

Configure via commands:
```bash
nugs catalog config enable   # Enable with defaults (5am EST daily)
nugs catalog config set      # Interactive configuration
nugs catalog config disable  # Disable auto-refresh
```

---

## Supported Media Types

| Type | URL Example | Shorthand |
|------|-------------|-----------|
| **Album** | `https://play.nugs.net/release/23329` | `23329` |
| **Artist (all)** | `https://play.nugs.net/#/artist/461` | `461` |
| **Artist (latest)** | `https://play.nugs.net/#/artist/461/latest` | `461 latest` |
| **Catalog Playlist** | `https://2nu.gs/3PmqXLW` | - |
| **User Playlist** | `https://play.nugs.net/#/playlists/playlist/1215400` | - |
| **Video** | `https://play.nugs.net/#/videos/artist/1045/.../27323` | - |
| **Livestream (exclusive)** | `https://play.nugs.net/watch/livestreams/exclusive/30119` | - |
| **Livestream (purchased)** | `https://www.nugs.net/on/demandware.store/...` | - |
| **Webcast** | `https://play.nugs.net/#/my-webcasts/5826189-30369-0-624602` | - |

> **⚠️ Windows Users:** Wrap URLs containing special characters in quotes to avoid shell parsing issues.

---

## Usage

### Downloading Content

Let's get to the good stuff - actually downloading music.

**Download single album:**
```bash
nugs 23329
nugs https://play.nugs.net/release/23329
```

**Download multiple albums:**
```bash
nugs 23329 23790 24105
```

**Download from text file:**
```bash
nugs /path/to/urls.txt
```

**Download artist's latest shows:**
```bash
nugs 1125 latest  # Billy Strings
nugs 461 latest   # Grateful Dead
```

**Download entire artist catalog:**
```bash
nugs https://play.nugs.net/#/artist/461
```

**Override quality settings:**
```bash
nugs -f 3 23329                    # MQA quality
nugs -F 5 video-url                # 4K video
nugs -o /mnt/storage/music 23329   # Custom output path
```

---

### Browse & List

**List all artists:**
```bash
nugs list artists
```

**View artist's shows:**
```bash
nugs list 1125  # Billy Strings
nugs list 461   # Grateful Dead
```

**JSON output for scripting:**
```bash
# Get all artists as JSON
nugs list artists --json standard

# Get shows with jq filtering
nugs list 461 --json standard | jq '.shows[] | select(.venue == "Red Rocks")'

# Find artists with 100+ shows
nugs list artists --json standard | jq '.artists[] | select(.numShows > 100)'

# Get latest 5 shows
nugs list 1125 --json minimal | jq '.shows[:5]'
```

---

### Catalog Management

Want to browse Nugs.net's 13,000+ shows offline? The catalog system has you covered. Update the catalog once, then query it instantly without hitting Nugs.net servers.

#### Update Catalog

**Fetch latest catalog** (updates cache with current Nugs.net catalog):
```bash
nugs catalog update
```

Output:
```
✓ Catalog updated successfully
  Total shows: 13,253
  Update time: 1 seconds
  Cache location: /home/user/.cache/nugs
```

#### Cache Status

**View cache information:**
```bash
nugs catalog cache
```

Output:
```
Catalog Cache Status:

  Location:     /home/user/.cache/nugs
  Last Updated: 2026-02-05 14:30:00 (2 hours ago)
  Total Shows:  13,253
  Artists:      335 unique
  Cache Size:   7.4 MB
  Version:      v1.0.0
```

#### Statistics

**View catalog statistics:**
```bash
nugs catalog stats
```

Output:
```
Catalog Statistics:

  Total Shows:    13,253
  Total Artists:  335 unique
  Date Range:     1965-01-01 to 2026-02-04

Top 10 Artists by Show Count:

  ID       Artist                    Shows
  ──────────────────────────────────────────
  1125     Billy Strings             430
  22       Umphrey's McGee           415
  1084     Spafford                  411
  ...
```

#### Latest Additions

**View recently added shows:**
```bash
nugs catalog latest       # Default: 15 shows
nugs catalog latest 50    # Show 50 most recent
```

Output:
```
Latest 15 Shows in Catalog:

   1. Daniel Donato         02/03/26    02/03/26 Missoula, MT
   2. The String Cheese...  07/18/00    07/18/00 Mt. Shasta, CA
   3. Dizgo                 01/30/26    01/30/26 Columbus, OH
   ...
```

#### Gap Detection

**Find missing shows in your collection:**
```bash
nugs catalog gaps 1125  # Billy Strings
```

Output:
```
Gap Analysis: Billy Strings

  Total Shows:      430
  Downloaded:       23 (5.3%)
  Missing:          407 (94.7%)

Missing Shows:

  ID       Date         Title
  ────────────────────────────────────────────────────────────
  46385    12/14/25     12/14/25 ACL Live Austin, TX
  46380    12/13/25     12/13/25 The Criterion Oklahoma City
  ...

To download: nugs <container_id>
Example: nugs 46385
```

**Get IDs only for piping:**
```bash
nugs catalog gaps 1125 --ids-only
# Output: 46385
#         46380
#         46375

# Download all gaps
nugs catalog gaps 1125 --ids-only | xargs -n1 nugs

# Download first 10 gaps
nugs catalog gaps 1125 --ids-only | head -10 | xargs -n1 nugs
```

---

### JSON Output

All catalog commands support `--json <level>` for machine-readable output:

**Levels:**
- `minimal` - Essential fields only
- `standard` - Adds location details
- `extended` - All metadata
- `raw` - Unmodified API response

**Examples:**

```bash
# Cache status as JSON
nugs catalog cache --json standard | jq .
{
  "exists": true,
  "lastUpdated": "2026-02-05T14:30:00Z",
  "ageSeconds": 7200,
  "totalShows": 13253,
  "totalArtists": 335,
  "cacheVersion": "v1.0.0",
  "fileSizeHuman": "7.4 MB"
}

# Statistics as JSON
nugs catalog stats --json standard | jq '.topArtists[:3]'
[
  {"artistID": 1125, "artistName": "Billy Strings", "showCount": 430},
  {"artistID": 22, "artistName": "Umphrey's McGee", "showCount": 415},
  {"artistID": 1084, "artistName": "Spafford", "showCount": 411}
]

# Gap analysis as JSON
nugs catalog gaps 1125 --json standard | jq '{total: .totalShows, missing: .missing}'
{
  "total": 430,
  "missing": 407
}
```

---

## Advanced Features

### Auto-Refresh

Automatically update the catalog cache on a schedule:

**Enable with defaults** (5am EST, daily):
```bash
nugs catalog config enable
```

**Configure custom schedule:**
```bash
nugs catalog config set
# Enter refresh time: 03:00
# Enter timezone: America/Los_Angeles
# Enter interval: weekly
```

**Disable:**
```bash
nugs catalog config disable
```

> **🔄 Auto-refresh triggers at startup when:**
> - Auto-refresh is enabled
> - Current time is past the configured refresh time
> - Cache hasn't been updated within the refresh interval

### Gap Detection

Find shows you haven't downloaded yet:

**Basic gap detection:**
```bash
nugs catalog gaps 1125  # Shows you're missing
```

**Integration with downloads:**
```bash
# Download all missing shows
nugs catalog gaps 1125 --ids-only | xargs -n1 nugs

# Download 5 most recent gaps
nugs catalog gaps 1125 --ids-only | head -5 | xargs -n1 nugs

# Download in parallel (3 concurrent)
nugs catalog gaps 1125 --ids-only | xargs -P 3 -n1 nugs

# Save gaps to file for later
nugs catalog gaps 1125 --ids-only > billy-gaps.txt
```

**Check multiple artists:**
```bash
for artist in 1125 461 1045; do
  echo "Gaps for artist $artist:"
  nugs catalog gaps $artist --ids-only | wc -l
done
```

### Rclone Integration

Automatically upload downloads to cloud storage (Google Drive, Dropbox, etc.):

**1. Install and configure rclone:**
```bash
# Install rclone
curl https://rclone.org/install.sh | sudo bash

# Configure a remote (follow interactive prompts)
rclone config
```

**2. Enable in Nugs config** (`~/.nugs/config.json`):
```json
{
  "rcloneEnabled": true,
  "rcloneRemote": "gdrive",
  "rclonePath": "/Music/Nugs",
  "deleteAfterUpload": false,
  "rcloneTransfers": 4
}
```

**3. Download and upload automatically:**
```bash
nugs 23329  # Downloads and uploads to gdrive:/Music/Nugs/
```

> **📍 Smart Gap Detection:** Gap detection checks both local storage AND your rclone remote, so you won't accidentally re-download shows that are already in the cloud.

---

## FFmpeg Setup

FFmpeg is required for:
- **Video downloads** (TS → MP4 conversion)
- **HLS-only audio tracks** (M3U8 → audio file)

### Installation

**Linux:**
```bash
sudo apt install ffmpeg
```

**macOS:**
```bash
brew install ffmpeg
```

**Windows:**
1. Download from [FFmpeg Builds](https://github.com/BtbN/FFmpeg-Builds/releases)
2. Extract and add to PATH, or place in Nugs directory

**Termux (Android):**
```bash
pkg install ffmpeg
```

### Configuration

**Use FFmpeg from PATH** (recommended):
```json
{
  "useFfmpegEnvVar": true
}
```

**Use FFmpeg from script directory** (if you can't install system-wide):
```json
{
  "useFfmpegEnvVar": false
}
```
Just drop the `ffmpeg` binary in the same directory as `nugs`.

---

## Command Reference

### Download Commands

```bash
nugs <url|id>...              # Download one or more albums
nugs <artist_id> latest       # Download artist's latest shows
nugs -f <format> <url>        # Override audio format
nugs -F <format> <url>        # Override video format
nugs -o <path> <url>          # Custom output directory
```

### List Commands

```bash
nugs list artists             # List all artists
nugs list <artist_id>         # List artist's shows
nugs list artists --json <level>      # JSON output
nugs list <artist_id> --json <level>  # JSON output
```

### Catalog Commands

```bash
nugs catalog update           # Update catalog cache
nugs catalog cache            # View cache status
nugs catalog stats            # View statistics
nugs catalog latest [limit]   # View latest additions
nugs catalog gaps <artist_id> # Find missing shows
nugs catalog gaps <id> --ids-only     # IDs only (for piping)
nugs catalog config enable    # Enable auto-refresh
nugs catalog config disable   # Disable auto-refresh
nugs catalog config set       # Configure auto-refresh
```

### Global Options

```bash
--format, -f         Audio format (1-5)
--videoformat, -F    Video format (1-5)
--outpath, -o        Output directory
--force-video        Force video over audio
--skip-videos        Skip videos in downloads
--skip-chapters      Skip chapter markers
--json               JSON output level
--help, -h           Show help
```

---

## Examples

### Example 1: Download Billy Strings' Latest Shows
```bash
# Method 1: Using shorthand
nugs 1125 latest

# Method 2: Using full URL
nugs https://play.nugs.net/#/artist/1125/latest
```

### Example 2: Find and Download Missing Dead & Company Shows
```bash
# See what you're missing
nugs catalog gaps 1045

# Download all gaps
nugs catalog gaps 1045 --ids-only | xargs -n1 nugs

# Or just the 10 most recent
nugs catalog gaps 1045 --ids-only | head -10 | xargs -n1 nugs
```

### Example 3: Batch Download from File
```bash
# Create a file with show IDs
cat > shows.txt << EOF
23329
23790
24105
EOF

# Download all
nugs shows.txt
```

### Example 4: Find All Red Rocks Shows
```bash
# Get Billy Strings shows at Red Rocks
nugs list 1125 --json standard | \
  jq '.shows[] | select(.venue | contains("Red Rocks"))'

# Download them
nugs list 1125 --json standard | \
  jq -r '.shows[] | select(.venue | contains("Red Rocks")) | .containerID' | \
  xargs -n1 nugs
```

### Example 5: Daily Catalog Update Script
```bash
#!/bin/bash
# daily-catalog-update.sh

# Update catalog
nugs catalog update

# Email yourself the stats
nugs catalog stats --json standard | \
  jq -r '"New catalog stats:\nTotal shows: \(.totalShows)\nTop artist: \(.topArtists[0].artistName) (\(.topArtists[0].showCount) shows)"' | \
  mail -s "Nugs Catalog Update" you@example.com
```

### Example 6: Check Collection Coverage
```bash
# Get coverage stats for your favorite artists
for artist in 1125 461 1045; do
  echo -n "Artist $artist: "
  nugs catalog gaps $artist --json standard | \
    jq -r '"\(.downloaded)/\(.totalShows) (\(.downloadedPct | floor)%)"'
done
```

---

## Troubleshooting

### Common Issues

> **❌ Error:** "No cache found - run 'nugs catalog update' first"
>
> **✅ Solution:** Run `nugs catalog update` to download the catalog cache.

> **❌ Error:** FFmpeg not found
>
> **✅ Solution:**
> - Install FFmpeg (see [FFmpeg Setup](#ffmpeg-setup))
> - Or set `useFfmpegEnvVar: false` in config and place FFmpeg in the same directory as `nugs`

> **❌ Error:** Auth failed / Invalid credentials
>
> **✅ Solution:**
> - Double-check your email/password in config.json
> - For Apple/Google accounts, use token authentication ([guide](token.md))

> **❌ Error:** "No audio available"
>
> **✅ Solution:**
> - The show might be video-only - try `--force-video`
> - Or it might not be available on your subscription tier

> **❌ Error:** Rclone upload fails
>
> **✅ Solution:**
> - Verify rclone is installed: `rclone version`
> - Test your remote: `rclone ls <remote_name>:`
> - Check your config.json paths

> **⚠️ Issue:** Gap detection shows wrong results
>
> **✅ Solution:**
> - Make sure `outPath` in config matches your actual download location
> - Run `nugs catalog update` to refresh the catalog
> - Verify you haven't manually moved or renamed files

### Getting Help

1. Check this README first
2. Look at [Issues](https://github.com/jmagar/nugs-cli/issues)
3. Open a new issue with:
   - Your OS and Go version
   - Command you ran
   - Full error message
   - Relevant config (redact credentials)

---

## Disclaimer

- I will not be responsible for how you use Nugs CLI
- Nugs brand and name is the registered trademark of its respective owner
- Nugs CLI has no partnership, sponsorship or endorsement with Nugs.net
- This tool is for personal use only - respect copyright and terms of service
- Only download content you have legal access to through your subscription

---

## Contributing

Contributions welcome! Please open an issue first to discuss proposed changes.

For development setup, architecture details, and coding guidelines, see [CLAUDE.md](CLAUDE.md).

## Credits

Originally forked from [Sorrow446/Nugs-Downloader](https://github.com/Sorrow446/Nugs-Downloader)

Catalog caching, auto-refresh, gap detection, and modern improvements by [jmagar](https://github.com/jmagar)
