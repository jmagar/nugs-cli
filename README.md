# Nugs CLI

`nugs` is a local Go CLI for downloading media that your Nugs.net account can
access, browsing the catalog, tracking collection gaps, and automating artist
watch lists. It is not an MCP server or a Python/npm package.

## Requirements

- Go 1.25.12 or newer when building from source
- Make
- FFmpeg for video and HLS conversion
- Optional: rclone for remote uploads

## Install

Download a platform archive from
[GitHub Releases](https://github.com/jmagar/nugs-cli/releases), verify it against
`checksums.txt`, and place `nugs` (`nugs.exe` on Windows) on `PATH`.

To build from source:

```bash
git clone https://github.com/jmagar/nugs-cli.git
cd nugs-cli
make build
nugs version
```

`make build` installs a versioned binary under `~/.local/lib/nugs/` and updates
the stable `~/.local/bin/nugs` symlink.

## Quick start

Run the interactive setup rather than hand-authoring an incomplete config:

```bash
nugs
chmod 600 ~/.nugs/config.json
nugs list
nugs grab 23329
```

The setup prompts for account credentials, audio/video quality, local output
paths, FFmpeg, and optional rclone settings. The config is written with mode
`0600` on Unix. Password input is hidden.

For Apple or Google accounts, use a session token; see
[Token authentication](docs/token.md).

## Configuration

The first existing file wins; files are not merged:

1. `./config.json`
2. `~/.nugs/config.json`
3. `~/.config/nugs/config.json`

See [Configuration reference](docs/CONFIG.md) for every supported field,
defaults, validation, watch settings, and Gotify settings. Never commit a real
config file or pass credentials as command arguments.

## Commands

### Download

```bash
nugs grab 23329
nugs grab 23329 video
nugs grab 23329 both
nugs 1125 latest audio
nugs 1125 full
nugs -f 2 23329
nugs -F 3 23329
```

Supported inputs include numeric release IDs and the documented Nugs release,
artist, playlist, video, livestream, and webcast URL forms. See
[Command reference](docs/COMMANDS.md) for the exact accepted patterns.

During an attached interactive download:

| Key | Action |
|---|---|
| `Shift+P` | Pause or resume |
| `Shift+C` | Cancel |
| `Ctrl+C` | Interrupt |

### Browse

```bash
nugs list
nugs list audio
nugs list 1125
nugs list 1125 video
nugs list 1125 latest 5
nugs list 1125 "Red Rocks"
nugs list '>100'
```

Quote comparison filters so the shell does not interpret `>` or `<` as
redirection.

### Catalog

```bash
nugs update
nugs cache
nugs stats
nugs latest
nugs latest 50
nugs gaps 1125
nugs gaps 1125 video --ids-only
nugs gaps 1125 audio fill
nugs coverage 1125
nugs coverage
nugs refresh enable
nugs refresh disable
nugs refresh set
```

`nugs latest` does **not** support `audio`, `video`, or `both` filters because
the latest-catalog response lacks product details. Use `nugs list <artist_id>
<media>` for media-filtered browsing.

With no artist IDs, `nugs coverage` scans artists discovered in configured local
and remote download folders. It does not query account subscriptions.

### Watch automation

```bash
nugs watch add 1125
nugs watch remove 1125
nugs watch list
nugs watch check
nugs watch check video
nugs watch enable
nugs watch disable
```

`watch enable` installs and enables the Linux systemd timer; `watch disable`
removes it. `watch check` updates the catalog and fills gaps for watched artists.
Gotify notifications use `gotifyUrl` and `gotifyToken`; notification priorities
are selected by the application, not by a config field.

### Runtime control

```bash
nugs status
nugs cancel
nugs completion bash
nugs completion zsh
nugs completion fish
nugs completion powershell
```

### Structured output

Commands that expose structured output accept `--json` with `minimal`,
`standard`, `extended`, or `raw` where documented. Do not assume mutation-only
commands support JSON; consult [Command reference](docs/COMMANDS.md).

## Media formats

Audio `-f` values:

| Value | Format |
|---:|---|
| 1 | 16-bit 44.1 kHz ALAC |
| 2 | 16-bit 44.1 kHz FLAC |
| 3 | 24-bit 48 kHz MQA |
| 4 | 360 Reality Audio |
| 5 | 150 Kbps AAC |

Video `-F` values:

| Value | Resolution |
|---:|---|
| 1 | 480p |
| 2 | 720p |
| 3 | 1080p |
| 4 | 1440p |
| 5 | 4K / best available |

## Rclone safety

- `outPath` and `videoOutPath` are local download directories.
- `rclonePath` and `rcloneVideoPath` are remote paths.
- When `deleteAfterUpload` is enabled, local deletion occurs only after upload
  verification succeeds.

See [Configuration migrations](docs/CONFIG.md#migrations) if upgrading from a
version where `rclonePath` affected local paths.

## Development

```bash
make test
make verify
```

`make verify` checks formatting, module integrity, vet, race tests, documentation
links/config-field coverage, static analysis, vulnerability analysis, and
supported cross-builds. CI runs the same contracts.

The source of truth for contributor architecture and workflow is
[CLAUDE.md](CLAUDE.md). Detailed references:

- [Commands](docs/COMMANDS.md)
- [Configuration](docs/CONFIG.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Upstream API notes](docs/nugs-api-endpoints.md)
- [CLI quick reference](docs/QUICK_REFERENCE.md)

## Release integrity

Release automation publishes archives for Linux, macOS, and Windows, a
`checksums.txt` file, and GitHub build provenance. CI smoke-tests each native
binary before publication. The `nugs version` output includes version, commit,
and build date for rollback and incident diagnosis.

## Safety and legal use

Use the CLI only for media your account is authorized to access. Do not share
credentials, tokens, signed media URLs, or downloaded content. This project is
not affiliated with Nugs.net.
