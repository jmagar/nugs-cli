# Nugs CLI

Download your favorite live shows from Nugs.net, browse 13,000+ concerts offline, and never miss a new release.

Built for Deadheads, jam band fans, and anyone who wants their live music collection organized and accessible.

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
  - [Watch Command](#watch-command)
- [FFmpeg Setup](#ffmpeg-setup)
- [Command Reference](#command-reference)
  - [Shell Completions](#shell-completions)
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
- **Watch automation**: Monitor artists, auto-download new shows via systemd timer
- **Push notifications**: Gotify alerts when new shows are downloaded
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

**Step 2: Configure your credentials** in one of these locations:
- `./config.json` (current directory)
- `~/.nugs/config.json` (recommended)
- `~/.config/nugs/config.json` (XDG standard)

```json
{
  "email": "your-email@example.com",
  "password": "your-password",
  "outPath": "/path/to/downloads",
  "format": 2
}
```

> **🔒 Security:** After creating your config, set secure permissions: `chmod 600 ~/.nugs/config.json`
> **💡 Tip:** Most folks use `format: 2` (FLAC) for the best quality-to-size ratio.

**Step 3: Download your first show**:

```bash
nugs grab 23329
```

**Step 4: Browse the catalog**:

```bash
nugs list
nugs list 1125  # Billy Strings
```

> **ℹ️ Note:** Running `nugs` with no arguments shows the latest 15 catalog additions - a quick way to discover new releases!

---

## Configuration

Now that you've seen what Nugs CLI can do, let's customize it for your workflow.

The config file can be placed in one of three locations (checked in this order, first found wins — no merging):
1. `./config.json` - Current directory
2. `~/.nugs/config.json` - Recommended location
3. `~/.config/nugs/config.json` - XDG standard location

On first run, you'll be prompted to create it.

### Core Settings

| Option | Description | Values |
|--------|-------------|--------|
| `email` | Your Nugs.net email | string |
| `password` | Your Nugs.net password | string |
| `format` | Audio download quality | `1` = 16-bit/44.1kHz ALAC<br>`2` = 16-bit/44.1kHz FLAC<br>`3` = 24-bit/48kHz MQA<br>`4` = 360 Reality Audio<br>`5` = 150 Kbps AAC |
| `videoFormat` | Video download quality | `1` = 480p<br>`2` = 720p<br>`3` = 1080p<br>`4` = 1440p<br>`5` = 4K/best available |
| `defaultOutputs` | Default media type preference | `audio` (default) - Prefer audio downloads<br>`video` - Prefer video downloads<br>`both` - Download both formats when available |
| `outPath` | Download destination | Path (created if doesn't exist) |
| `videoOutPath` | Video download destination (defaults to `outPath`) | Path |

### Advanced Settings

| Option | Description | Default |
|--------|-------------|---------|
| `token` | Auth token for Apple/Google accounts ([guide](token.md)) | - |
| `useFfmpegEnvVar` | Use FFmpeg from PATH vs local dir | `false` |
| `ffmpegNameStr` | Custom FFmpeg binary name or absolute path | `"ffmpeg"` |
| `skipChapters` | Skip embedding chapter markers in video files | `false` |
| `skipSizePreCalculation` | Skip pre-download size calculation (faster start, no ETA) | `false` |
| `forceVideo` | **Deprecated** — use `defaultOutputs: "video"` instead | `false` |
| `skipVideos` | **Deprecated** — use `defaultOutputs: "audio"` instead | `false` |

### Rclone Settings

| Option | Description |
|--------|-------------|
| `rcloneEnabled` | Enable auto-upload to cloud storage |
| `rcloneRemote` | Rclone remote name (e.g., `gdrive`) |
| `rclonePath` | Path on remote (e.g., `/Music/Nugs`) |
| `rcloneVideoPath` | Remote path for videos (defaults to `rclonePath`) |
| `deleteAfterUpload` | Delete local files after successful upload |
| `rcloneTransfers` | Number of parallel transfers (default: 4) |

> **Migration note (2026-02-05):**
> `rclonePath` is now remote-only. Local download detection and storage always use `outPath`.
> If you previously relied on `rclonePath` for local folder checks, move/update your `outPath`
> to the local download root.

### Catalog Auto-Refresh

| Option | Description | Default |
|--------|-------------|---------|
| `catalogAutoRefresh` | Enable automatic catalog updates | `true` |
| `catalogRefreshTime` | Time to refresh (24-hour format) | `"05:00"` |
| `catalogRefreshTimezone` | Timezone for refresh time | `"America/New_York"` |
| `catalogRefreshInterval` | Refresh frequency | `"daily"` |

### Watch & Notifications

| Option | Description | Default |
|--------|-------------|---------|
| `watchedArtists` | Artist IDs to monitor for new shows | `[]` |
| `watchInterval` | How often to check for new shows (Go duration: `30m`, `1h`, `6h`) | `"1h"` |
| `gotifyUrl` | Gotify server URL for push notifications | `""` |
| `gotifyToken` | Gotify app token | `""` |

Auto-refresh is **enabled by default** to keep your catalog up-to-date. Configure via commands:

```bash
nugs refresh set      # Change schedule (time/timezone/interval)
nugs refresh disable  # Disable auto-refresh
nugs refresh enable   # Re-enable with current settings
```

---

## Supported Media Types

| Type | URL Example | Shorthand |
|------|-------------|-----------|
| **Album** | `https://play.nugs.net/release/23329` | `23329` |
| **Artist (all)** | `https://play.nugs.net/#/artist/461` | `461 full` |
| **Artist (latest)** | `https://play.nugs.net/#/artist/461/latest` | `grab 461 latest` |
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
nugs grab 23329
nugs grab https://play.nugs.net/release/23329
```

**Download multiple albums:**

```bash
nugs grab 23329 23790 24105
```

**Download from text file:**

```bash
nugs /path/to/urls.txt
```

**Download artist's latest shows:**

```bash
nugs grab 1125 latest        # Billy Strings (respects defaultOutputs)
nugs grab 1125 latest video  # Latest videos only
nugs grab 461 latest         # Grateful Dead
```

**Download entire artist catalog:**

```bash
nugs 1125 full               # Billy Strings - all shows (respects defaultOutputs)
nugs 1125 full video         # Billy Strings - all videos only
nugs 461 full both           # Grateful Dead - download both audio and video
```

**Download both formats:**

```bash
nugs grab 23329 both         # Download both audio and video for show 23329
```

**Override quality settings:**

```bash
nugs grab -f 3 23329               # MQA quality
nugs -F 5 video-url                # 4K video
nugs grab -o /mnt/storage/music 23329  # Custom output path
```

**Hotkeys during an active download:**

| Key | Action |
|-----|--------|
| `Shift+P` | Pause / resume current download |
| `Shift+C` | Cancel current download |

---

### Browse & List

**List all artists:**

```bash
nugs list                # All artists with media indicators (🎵 🎬 📹)
nugs list audio          # Only artists with audio shows
nugs list video          # Only artists with video shows
nugs list both           # Only artists with both formats
```

**View artist's shows:**

```bash
nugs list 1125           # Billy Strings (all shows with 🎵 🎬 📹 indicators)
nugs list 1125 audio     # Billy Strings audio shows only
nugs list 1125 video     # Billy Strings video shows only
nugs list 1125 both      # Billy Strings shows with both formats
nugs list 461            # Grateful Dead
```

**Media Type Indicators:**
- 🎵 Audio
- 🎬 Video
- 📹 Both audio and video

**Filter and search shows:**

```bash
# Filter shows by venue
nugs list 461 "Red Rocks"

# Find artists with 100+ shows
nugs list ">100"
nugs list "<=50"
nugs list "=25"

# Get artist's latest 5 shows
nugs list 1125 latest 5
```

> **💡 Advanced users:** All commands support `--json <level>` output for piping to `jq` or other tools.

---

### Catalog Management

Want to browse Nugs.net's 13,000+ shows offline? The catalog system has you covered. Update the catalog once, then query it instantly without hitting Nugs.net servers.

#### Update Catalog

**Fetch latest catalog** (updates cache with current Nugs.net catalog):

```bash
nugs update
```

Output:

```text
✓ Catalog updated successfully
  Total shows: 13,253
  Update time: 1 seconds
  Cache location: /home/user/.cache/nugs
```

#### Cache Status

**View cache information:**

```bash
nugs cache
```

Output:

```text
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
nugs stats
```

Output:

```text
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
nugs latest              # Default: 15 shows (both formats with emoji)
nugs latest video        # Latest video releases only
nugs latest 50           # Last 50 shows (both formats)
nugs latest 50 audio     # Last 50 audio releases
```

Output:

```text
Latest 15 Shows in Catalog:

      Artist               Date        Title                      Media
   1. 🎬 Daniel Donato     02/03/26    02/03/26 Missoula, MT      Video
   2. 🎵 String Cheese...  07/18/00    07/18/00 Mt. Shasta, CA    Audio
   3. 📹 Dizgo             01/30/26    01/30/26 Columbus, OH      Both
   ...
```

#### Gap Detection

**Find missing shows in your collection:**

```bash
nugs gaps 1125               # Billy Strings (respects defaultOutputs)
nugs gaps 1125 audio         # Audio gaps only
nugs gaps 1125 video         # Video gaps only
nugs gaps 1125 both          # Both audio AND video gaps (stricter - shows you have either audio OR video but not both)
nugs gaps 1125 461 1045      # Multiple artists at once
```

Output:

```text
  ID       Date         Title                                    Media
  ────────────────────────────────────────────────────────────────────
  46385    12/14/25     12/14/25 ACL Live Austin, TX             🎵
  46380    12/13/25     12/13/25 The Criterion Oklahoma City     🎬
  ...
```

**Get IDs only for piping:**

```bash
nugs gaps 1125 --ids-only
nugs gaps 1125 video --ids-only  # Video gaps only
# Output: 46385
#         46380
#         46375

# Download all audio gaps
nugs gaps 1125 audio --ids-only | xargs -n1 nugs grab

# Download all video gaps
nugs gaps 1125 video --ids-only | xargs -I {} nugs grab {} video

# Download first 10 gaps
nugs gaps 1125 --ids-only | head -10 | xargs -n1 nugs grab
```

**Auto-download all missing shows:**

```bash
nugs gaps 1045 fill              # Fill gaps (respects defaultOutputs)
nugs gaps 1045 fill video        # Fill all video gaps
nugs gaps 1045 fill both         # Fill all shows where you're missing either format
```

Output:

```text
Filling Gaps: Phish (Video)

  Total Missing:    234 shows

⬇ Downloading 1/234: 2025-12-14 - 12/14/25 Madison Square Garden... 🎬
⬇ Downloading 2/234: 2025-12-13 - 12/13/25 Madison Square Garden... 🎬
...

Download Summary:
  Total Attempted:         234
  Successfully Downloaded: 232
  Failed:                 2
```

#### Coverage Statistics

**Check download coverage for artists:**

```bash
# Single artist (respects defaultOutputs)
nugs coverage 1125

# Coverage for specific media type
nugs coverage 1125 video         # Video coverage only
nugs coverage 1125 audio         # Audio coverage only
nugs coverage 1125 both          # Both formats coverage

# Multiple artists
nugs coverage 1125 461 1045

# All artists with downloads (auto-detects from output directory)
nugs coverage
nugs coverage video              # Video coverage for all artists
```

Output:

```text
Download Coverage Statistics (Video)

  Artist ID    Artist Name              Downloaded    Total    Coverage    Media
  ──────────────────────────────────────────────────────────────────────────────
        1125   Billy Strings                    12      156       7.7%    🎬
         461   Phish                            45      234      19.2%    🎬
        1045   Widespread Panic                 78      445      17.5%    🎬
```

---

### JSON Output

All catalog commands support `--json <level>` for machine-readable output. Advanced users can pipe JSON to `jq` or other tools for custom filtering.

**Levels:**
- `minimal` - Essential fields only
- `standard` - Adds location details
- `extended` - All metadata
- `raw` - Unmodified API response

**Example:**

```bash
# Get cache status as JSON
nugs cache --json standard

# Pipe to jq for custom filtering
nugs stats --json standard | jq '.topArtists[:3]'
```

---

## Advanced Features

### Auto-Refresh

The catalog cache automatically updates on a schedule. **Auto-refresh is enabled by default** (5am EST, daily).

**Configure custom schedule:**

```bash
nugs refresh set
# Enter refresh time: 03:00
# Enter timezone: America/Los_Angeles
# Enter interval: weekly
```

**Disable auto-refresh:**

```bash
nugs refresh disable
```

**Re-enable auto-refresh:**

```bash
nugs refresh enable
```

> **🔄 Auto-refresh triggers at startup when:**
> - Auto-refresh is enabled
> - Current time is past the configured refresh time
> - Cache hasn't been updated within the refresh interval

### Gap Detection

Find shows you haven't downloaded yet:

**Basic gap detection:**

```bash
nugs gaps 1125  # Shows you're missing
```

**Integration with downloads:**

```bash
# Download all missing shows
nugs gaps 1125 --ids-only | xargs -n1 nugs grab

# Download 5 most recent gaps
nugs gaps 1125 --ids-only | head -5 | xargs -n1 nugs grab

# Download in parallel (3 concurrent)
nugs gaps 1125 --ids-only | xargs -P 3 -n1 nugs grab

# Save gaps to file for later
nugs gaps 1125 --ids-only > billy-gaps.txt
```

**Check multiple artists:**

```bash
# All at once
nugs gaps 1125 461 1045

# Or use a loop for more control
for artist in 1125 461 1045; do
  echo "Gaps for artist $artist:"
  nugs gaps $artist --ids-only | wc -l
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
nugs grab 23329  # Downloads and uploads to gdrive:/Music/Nugs/
```

> **📍 Smart Gap Detection:** Gap detection checks both local storage AND your rclone remote, so you won't accidentally re-download shows that are already in the cloud.

---

### Watch Command

Monitor artists and automatically download new shows on a schedule. Uses a systemd user timer (Linux) — no root required.

**Manage your watch list:**

```bash
nugs watch add 1125       # Watch Billy Strings
nugs watch add 461        # Watch Grateful Dead
nugs watch list           # Show watched artists with names
nugs watch remove 461     # Stop watching an artist
```

**One-shot check** (updates catalog and downloads gaps for all watched artists):

```bash
nugs watch check
```

**Automated scheduling** (systemd timer, fires on boot + every `watchInterval`):

```bash
nugs watch enable         # Write unit files, enable and start timer
nugs watch disable        # Stop, disable, remove unit files
```

**With Gotify push notifications** (add to `~/.nugs/config.json`):

```json
{
  "watchedArtists": ["1125", "461"],
  "watchInterval": "1h",
  "gotifyUrl": "http://your-gotify-server:8080",
  "gotifyToken": "your-app-token"
}
```

Notification behavior:
- **New downloads**: single summary notification (priority 5) — `"3 new show(s) downloaded"`
- **Errors only**: error notification (priority 7) with failure details
- **Nothing new**: silent — no hourly noise when everything is up-to-date

View timer logs:

```bash
journalctl --user -u nugs-watch.service -f
systemctl --user list-timers
```

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
nugs grab <artist_id> latest  # Download artist's latest shows
nugs <artist_id> full         # Download artist's entire catalog
nugs -f <format> <url>        # Override audio format
nugs -F <format> <url>        # Override video format
nugs -o <path> <url>          # Custom output directory
```

### List Commands

```bash
nugs list                                    # List all artists with media indicators
nugs list audio                              # List only artists with audio
nugs list video                              # List only artists with video
nugs list >100                               # Filter artists by show count
nugs list <=50                               # Operators: >, <, >=, <=, =
nugs list <artist_id>                        # List all artist's shows with 🎵🎬📹
nugs list <artist_id> audio                  # List artist's audio shows only
nugs list <artist_id> video                  # List artist's video shows only
nugs list <artist_id> both                   # Shows with both formats
nugs list <artist_id> latest <N>             # List latest N shows
nugs list <artist_id> "venue"                # Filter shows by venue name
nugs list --json <level>                     # JSON output
nugs list <artist_id> --json <level>         # JSON output
```

**Examples:**

```bash
# List latest 5 shows from Billy Strings
nugs list 1125 latest 5

# List Billy Strings video shows only
nugs list 1125 video

# Find all Grateful Dead shows at Red Rocks
nugs list 461 "Red Rocks"

# Find all Billy Strings video shows at Ryman
nugs list 1125 video "ryman"

# List artists with video content
nugs list video
```

### Catalog Commands

Short-form aliases (recommended):

```bash
nugs update                              # Update catalog cache
nugs cache                               # View cache status
nugs stats                               # View statistics
nugs latest [limit] [audio|video|both]   # View latest additions
nugs gaps <id> [audio|video|both]        # List missing shows
nugs gaps <id> [...]  --ids-only         # IDs only (for piping)
nugs gaps <id> [audio|video|both] fill   # Auto-download missing shows (auth required)
nugs coverage [ids...] [audio|video|both] # Coverage stats
nugs refresh enable                      # Enable auto-refresh
nugs refresh disable                     # Disable auto-refresh
nugs refresh set                         # Configure auto-refresh (interactive)
```

> All short forms are equivalent to `nugs catalog <cmd>` (e.g. `nugs update` = `nugs catalog update`, `nugs refresh set` = `nugs catalog config set`).

### Watch Commands

```bash
nugs watch add <artistID>          # Add artist to watch list
nugs watch remove <artistID>       # Remove artist from watch list
nugs watch list                    # Show watched artists with names
nugs watch check [audio|video]     # Update catalog + fill gaps for all watched artists (auth required)
nugs watch enable                  # Write systemd unit files and enable timer (Linux)
nugs watch disable                 # Stop timer and remove unit files
```

### Session Commands

```bash
nugs status   # Show status of any active download (PID, progress, current item)
nugs cancel   # Cancel the active download session
```

### Global Options

```bash
-f, --format <1-5>      Audio format (1=ALAC 2=FLAC 3=MQA 4=360RA 5=AAC)
-F <1-5>                Video format (1=480p 2=720p 3=1080p 4=1440p 5=4K)
-o <path>               Output directory (overrides outPath in config)
--skip-chapters         Skip chapter markers in video files
--json <level>          JSON output: minimal | standard | extended | raw
--help, -h              Show help
```

> `--force-video` and `--skip-videos` are deprecated. Use `defaultOutputs: "video"` or `defaultOutputs: "audio"` in config instead.

### Shell Completions

Enable tab completion for commands, flags, and arguments in your shell:

**Bash:**
```bash
# Install system-wide
sudo nugs completion bash > /etc/bash_completion.d/nugs

# Or user-only
nugs completion bash > ~/.bash_completion.d/nugs
source ~/.bashrc
```

**Zsh (vanilla):**
```bash
# Create completion directory if needed
mkdir -p ~/.zsh/completion

# Install completion
nugs completion zsh > ~/.zsh/completion/_nugs

# Add to ~/.zshrc if not already present
echo 'fpath=(~/.zsh/completion $fpath)' >> ~/.zshrc
echo 'autoload -Uz compinit && compinit' >> ~/.zshrc

# Reload shell
source ~/.zshrc
```

**Zsh (oh-my-zsh):**
```bash
# Install to oh-my-zsh custom directory
mkdir -p ~/.oh-my-zsh/custom/completions
nugs completion zsh > ~/.oh-my-zsh/custom/completions/_nugs

# Add to your .zshrc BEFORE oh-my-zsh.sh is sourced
# (Add this line before the "source $ZSH/oh-my-zsh.sh" line)
fpath=($ZSH/custom/completions $fpath)

# Reload shell
exec zsh
```

**Fish:**
```bash
# Install completion
nugs completion fish > ~/.config/fish/completions/nugs.fish

# Reload completions
source ~/.config/fish/completions/nugs.fish
```

**PowerShell:**
```powershell
# Add to PowerShell profile
nugs completion powershell >> $PROFILE

# Reload profile
. $PROFILE
```

**Completions include:**
- Top-level commands: `list`, `catalog`, `watch`, `status`, `cancel`, `completion`, `help`
- Short aliases: `update`, `cache`, `stats`, `latest`, `gaps`, `coverage`, `refresh`, `grab`
- Catalog subcommands: `update`, `cache`, `stats`, `latest`, `gaps`, `coverage`, `config`, `list`
- Watch subcommands: `add`, `remove`, `list`, `check`, `enable`, `disable`
- Config subcommands: `enable`, `disable`, `set`
- Media modifiers: `audio`, `video`, `both`
- Flags: `-f`, `-F`, `-o`, `--json`, `--skip-chapters`
- Format values: `1-5` for audio/video formats
- JSON levels: `minimal`, `standard`, `extended`, `raw`
- Shell types: `bash`, `zsh`, `fish`, `powershell`

---

## Examples

### Example 1: Download Billy Strings Shows

```bash
# Latest shows only
nugs grab 1125 latest

# Entire catalog (all 430+ shows)
nugs 1125 full

# Specific show
nugs grab 23329
```

### Example 2: Find and Download Missing Dead & Company Shows

```bash
# See what you're missing
nugs gaps 1045

# Download all gaps
nugs gaps 1045 --ids-only | xargs -n1 nugs grab

# Or just the 10 most recent
nugs gaps 1045 --ids-only | head -10 | xargs -n1 nugs grab
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
# Filter shows by venue (case-insensitive)
nugs list 1125 "Red Rocks"

# Advanced: Use JSON output with jq
nugs list 1125 --json standard | \
  jq -r '.shows[] | select(.venue | contains("Red Rocks")) | .containerID' | \
  xargs -n1 nugs grab
```

### Example 5: Daily Catalog Update Script

```bash
#!/bin/bash
# daily-catalog-update.sh

# Update catalog
nugs update

# View the stats
nugs stats
```

### Example 6: Check Collection Coverage

```bash
# Get coverage stats for your favorite artists
nugs coverage

# Check video coverage
nugs coverage video

# Or check individual artists
nugs gaps 1125  # Shows coverage percentage
nugs gaps 461
nugs gaps 1045
```

### Example 7: Video-First Workflows

**Configure for video preference:**

```json
{
  "defaultOutputs": "video",
  "videoFormat": 5,
  "outPath": "/mnt/storage/nugs"
}
```

**Browse and download videos:**

```bash
# Find artists with video content
nugs list video

# View Billy Strings videos
nugs list 1125 video

# Download latest videos
nugs grab 1125 latest video

# Fill all video gaps
nugs gaps 1125 video fill

# Check video coverage
nugs coverage 1125 video
```

### Example 8: Comprehensive Collection (Both Formats)

**Download both audio and video:**

```bash
# Single show - both formats
nugs grab 46201 both

# Artist's latest - both formats
nugs grab 1125 latest both

# Fill gaps for both formats
nugs gaps 1125 both fill

# Find shows where you're missing either format
nugs gaps 1125 both
```

**Check what you have:**

```bash
# Overall coverage (audio)
nugs coverage 1125 audio

# Video coverage
nugs coverage 1125 video

# Shows with both formats downloaded
nugs coverage 1125 both
```

---

## Troubleshooting

### Common Issues

> **❌ Error:** "No cache found - run 'nugs update' first"
>
> **✅ Solution:** Run `nugs update` to download the catalog cache.

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
> - Run `nugs update` to refresh the catalog
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
