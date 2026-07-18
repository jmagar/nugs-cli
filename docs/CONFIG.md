# Nugs CLI Configuration

This document covers every JSON field in `internal/model.Config`. The automated
documentation check fails when a JSON-tagged field is added without appearing
here.

## File locations

The first existing file wins; settings are not merged:

1. `./config.json`
2. `~/.nugs/config.json`
3. `~/.config/nugs/config.json`

Interactive first-run setup writes `~/.nugs/config.json`. On Unix, configuration
is created with mode `0600` and its parent directory with mode `0700`.

## First-run setup

Run `nugs` with no existing config. Setup prompts for:

1. Email and a hidden password
2. Audio and video quality
3. Audio and video output directories
4. FFmpeg selection
5. Optional rclone remote, audio/video paths, transfers, and deletion behavior

Catalog auto-refresh is initialized to daily at `05:00` in
`America/New_York`; use `nugs refresh set` to change it.

## Complete field reference

| JSON field | Type | Purpose and behavior |
|---|---|---|
| `email` | string | Nugs.net account email. Used with `password` when `token` is empty. |
| `password` | string | Nugs.net password. Stored in the config but hidden during interactive entry. |
| `format` | integer | Audio quality, 1–5. Required and validated at startup. |
| `outPath` | string | Local audio download directory. Defaults to `Nugs downloads` in setup. |
| `videoOutPath` | string | Local video directory. Setup defaults it to `outPath`. |
| `videoFormat` | integer | Video quality, 1–5. Required and validated at startup. |
| `defaultOutputs` | string | Default media selection: `audio`, `video`, or `both`; empty resolves to `audio`. |
| `wantRes` | string | Computed from `videoFormat`; do not set manually. Values are `480`, `720`, `1080`, `1440`, or `2160`. |
| `token` | string | Session token for Apple/Google authentication. A leading `Bearer ` is stripped. |
| `useFfmpegEnvVar` | boolean | Use `ffmpeg` from `PATH` when true. |
| `ffmpegNameStr` | string | Explicit FFmpeg executable when not using `PATH`. |
| `forceVideo` | boolean | Deprecated compatibility field; prefer a media modifier or `defaultOutputs`. |
| `skipVideos` | boolean | Deprecated compatibility field; prefer a media modifier or `defaultOutputs`. |
| `skipChapters` | boolean | Skip embedding chapter metadata into video output. |
| `rcloneEnabled` | boolean | Enable remote upload after downloads. |
| `rcloneRemote` | string | Configured rclone remote name. Runtime startup verifies rclone availability, not the remote name itself. |
| `rclonePath` | string | Remote audio destination. Never used as a local download path. |
| `rcloneVideoPath` | string | Remote video destination; setup defaults it to `rclonePath`. |
| `deleteAfterUpload` | boolean | Delete local media only after upload verification succeeds. |
| `rcloneTransfers` | integer | Parallel rclone transfers. Setup accepts positive integers and defaults to 4. |
| `catalogAutoRefresh` | boolean | Check at startup whether the catalog is due for refresh. |
| `catalogRefreshTime` | string | Local scheduled time in `HH:MM` form. |
| `catalogRefreshTimezone` | string | IANA timezone such as `America/New_York`. |
| `catalogRefreshInterval` | string | `hourly`, `daily`, or `weekly`. |
| `watchedArtists` | array of strings | Artist IDs managed by `nugs watch add/remove/list`. |
| `watchInterval` | string | Go duration for the generated watch timer, such as `1h`, `30m`, or `6h`. |
| `gotifyUrl` | string | Gotify server base URL used by watch notifications. |
| `gotifyToken` | string | Gotify application token. Notification priority is selected by the application. |
| `skipSizePreCalculation` | boolean | Skip size probing before downloads. When false, probes use 8 workers, 5-second track/request timeouts, and a 60-second overall maximum. |

`urls` is runtime CLI state tagged `json:"-"`; it is intentionally not a config
field.

## Format values

### Audio `format`

| Value | Format |
|---:|---|
| 1 | 16-bit 44.1 kHz ALAC |
| 2 | 16-bit 44.1 kHz FLAC |
| 3 | 24-bit 48 kHz MQA |
| 4 | 360 Reality Audio |
| 5 | 150 Kbps AAC |

### Video `videoFormat`

| Value | Resolution |
|---:|---|
| 1 | 480p |
| 2 | 720p |
| 3 | 1080p |
| 4 | 1440p |
| 5 | 4K / best available |

## Example

Use interactive setup for the initial file. A representative complete file is:

```json
{
  "email": "user@example.com",
  "password": "replace-me",
  "format": 2,
  "videoFormat": 3,
  "defaultOutputs": "audio",
  "outPath": "/srv/media/music",
  "videoOutPath": "/srv/media/video",
  "useFfmpegEnvVar": true,
  "skipChapters": false,
  "rcloneEnabled": false,
  "rcloneTransfers": 4,
  "catalogAutoRefresh": true,
  "catalogRefreshTime": "05:00",
  "catalogRefreshTimezone": "America/New_York",
  "catalogRefreshInterval": "daily",
  "watchedArtists": ["1125"],
  "watchInterval": "1h",
  "skipSizePreCalculation": false
}
```

For Apple/Google accounts, leave `email` and `password` empty and provide a
`token`; see [token.md](token.md).

## Validation boundaries

Startup validates audio/video formats and `defaultOutputs`. Refresh configuration
validates time, timezone, and interval when set interactively. Rclone setup
validates that transfer count is positive, while normal startup only checks that
the rclone executable is available. Remote reachability is discovered during
remote operations.

## Security

Credentials are stored as plaintext JSON protected by filesystem permissions.

- Keep the file at mode `0600` on Unix.
- Never commit it or include it in logs/issues.
- Prefer a token for Apple/Google accounts.
- Use HTTPS for Gotify endpoints.
- Do not pass credentials in CLI arguments.

## Auto-refresh

```bash
nugs refresh enable
nugs refresh disable
nugs refresh set
```

Supported intervals are hourly, daily, and weekly. Hourly refresh uses elapsed
time; daily/weekly schedules use `catalogRefreshTime` and
`catalogRefreshTimezone`.

## Watch and Gotify

```bash
nugs watch add 1125
nugs watch list
nugs watch check
nugs watch enable
```

`watchedArtists` is normally modified through the CLI. `watchInterval` controls
the generated systemd timer. Gotify is enabled when both `gotifyUrl` and
`gotifyToken` are configured.

## Rclone paths

- Local audio: `outPath`
- Local video: `videoOutPath`
- Remote audio: `<rcloneRemote>:<rclonePath>`
- Remote video: `<rcloneRemote>:<rcloneVideoPath>`

## Migrations

### `rclonePath` local-path behavior (2026-02-05)

Older versions could use `rclonePath` as a local path fallback. Current versions
never do so. Set local directories explicitly:

```json
{
  "outPath": "/mnt/media/music",
  "videoOutPath": "/mnt/media/video",
  "rcloneEnabled": true,
  "rcloneRemote": "archive",
  "rclonePath": "/Music",
  "rcloneVideoPath": "/Video"
}
```

## Environment variables

The Go CLI does not provide environment-variable overrides for configuration
fields. Environment variables used by CI or unrelated historical clients are not
part of this CLI contract.
