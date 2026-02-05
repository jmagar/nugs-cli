# Nugs Client - Quick Reference Guide

## Installation

```bash
cd /home/jmagar/code/nugs-client
uv sync
```

## Basic Usage

```python
import asyncio
from nugs_client import NugsClient

async def main():
    async with NugsClient() as client:
        # Downloads will work once you connect the pieces
        await client.download_album("23329")

asyncio.run(main())
```

## Configuration (.env file)

```bash
NUGS_EMAIL=your@email.com
NUGS_PASSWORD=your_password
NUGS_AUDIO_FORMAT=2      # 1=ALAC, 2=FLAC, 3=MQA, 4=360RA, 5=AAC
NUGS_VIDEO_FORMAT=3      # 1=480p, 2=720p, 3=1080p, 4=1440p, 5=4K
NUGS_OUTPUT_PATH=downloads
```

## Common Tasks

### Get Album Info
```python
album = await client.get_album("23329")
print(f"{album.artist_name} - {album.container_info}")
print(f"Tracks: {len(album.all_tracks)}")

# Cover artwork URL (public)
print(album.cover_art_url)
```

### Get Artist Catalog
```python
containers = await client.get_artist("461")
print(f"Found {len(containers)} releases")
```

### Download Album
```python
path = await client.download_album("23329", quality=2)  # FLAC
```

### Download Video
```python
path = await client.download_video("27323", resolution=3)  # 1080p
```

### Download from URL
```python
path = await client.download_url("https://play.nugs.net/release/23329")
```

## Testing

```bash
# Run tests
uv run pytest

# With coverage
uv run pytest --cov

# Type check
uv run mypy src/nugs_client

# Lint
uv run ruff check src/nugs_client
```

## Project Structure

```
src/nugs_client/
├── client.py          # Main NugsClient class
├── auth.py            # Authentication
├── endpoints.py       # API endpoints
├── downloader.py      # Download manager
├── hls.py            # HLS processing
├── video.py          # Video processing
├── models.py         # Data models
├── config.py         # Configuration
├── constants.py      # API constants
├── utils.py          # Utilities
└── exceptions.py     # Exceptions
```

## Key Classes

- **NugsClient** - Main client, use with `async with`
- **NugsConfig** - Configuration management
- **AuthHandler** - Authentication and tokens
- **CatalogAPI** - Metadata fetching
- **StreamAPI** - Stream URL generation
- **DownloadManager** - File downloads
- **HLSProcessor** - HLS stream processing
- **VideoProcessor** - Video downloads

## Error Handling

```python
from nugs_client.exceptions import (
    AuthenticationError,
    DownloadError,
    HLSError,
    ContentNotFoundError,
)

try:
    async with NugsClient() as client:
        await client.download_album("23329")
except AuthenticationError:
    print("Login failed")
except ContentNotFoundError:
    print("Album not found")
except DownloadError:
    print("Download failed")
```

## Audio Formats

| Format | Quality | Format ID |
|--------|---------|-----------|
| ALAC | 16-bit / 44.1 kHz | 1 |
| FLAC | 16-bit / 44.1 kHz | 2 |
| MQA | 24-bit / 48 kHz | 3 |
| 360 RA | Spatial | 4 |
| AAC | 150 Kbps | 5 |

## Video Formats

| Resolution | Format ID |
|------------|-----------|
| 480p | 1 |
| 720p | 2 |
| 1080p | 3 |
| 1440p | 4 |
| 4K | 5 |

## Supported URLs

- Albums: `https://play.nugs.net/release/23329`
- Artists: `https://play.nugs.net/artist/461`
- Playlists: `https://play.nugs.net/#/playlists/playlist/1215400`
- Catalog: `https://2nu.gs/3PmqXLW`
- Videos: `https://play.nugs.net/#/videos/artist/.../27323`

## Dependencies

### Runtime
- httpx - HTTP client
- pydantic - Data validation
- m3u8 - HLS parsing
- pycryptodome - Encryption
- rich - Progress bars

### External
- FFmpeg - Required for HLS and video

## Documentation

- `README.md` - Complete user guide
- `COMPLETION_SUMMARY.md` - Feature overview
- `FINAL_STATUS.md` - Implementation status
- `NEXT_STEPS.md` - Future enhancements

## Help

```bash
# Verify installation
python verify_installation.py

# Check structure
tree -L 3

# View logs
# (Add logging configuration as needed)
```

---

**Quick Start**: Copy `.env.example` to `.env`, add credentials, run `basic_usage.py`!
