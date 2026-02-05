# Nugs.net Python Client

> **✅ PRODUCTION READY** - Modern async Python client for the nugs.net music streaming and download API

[![Python 3.11+](https://img.shields.io/badge/python-3.11+-blue.svg)](https://www.python.org/downloads/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Status](https://img.shields.io/badge/status-production%20ready-brightgreen.svg)](https://github.com/)

**🎉 All download functionality is now complete and working!**

## Features

- 🎵 **Complete API Coverage**: Albums, videos, playlists, livestreams, artist catalogs
- ⚡ **Async-First**: Built with `httpx` and `asyncio` for high performance
- 🎯 **Type Safe**: Full type hints and Pydantic models
- 🔐 **Dual Authentication**: Email/password or pre-existing token support
- 📦 **Quality Selection**: All formats (ALAC, FLAC, MQA, 360 RA, AAC) with automatic fallback
- 📹 **Video Support**: All resolutions (480p-4K) with chapter embedding
- 🔄 **Resume Support**: Interrupted downloads can be resumed
- 🎬 **HLS Processing**: Automatic manifest parsing, decryption, and conversion
- 📊 **Progress Tracking**: Rich progress bars for downloads

## Installation

### Using uv (recommended)
```bash
uv add nugs-client
```

### Using pip
```bash
pip install nugs-client
```

### Development Installation
```bash
git clone https://github.com/yourusername/nugs-client.git
cd nugs-client
uv sync --dev
```

## Requirements

- Python 3.11 or higher
- FFmpeg (for HLS tracks and video processing)
  - **Linux**: `sudo apt install ffmpeg`
  - **macOS**: `brew install ffmpeg`
  - **Windows**: Download from [FFmpeg website](https://ffmpeg.org/download.html)

## Quick Start

### Basic Usage

```python
import asyncio
from nugs_client import NugsClient, NugsConfig

async def main():
    # Configure the client
    config = NugsConfig(
        email="your.email@example.com",
        password="your_password",
        audio_format=2,  # FLAC
        output_path="downloads"
    )
    
    # Download an album
    async with NugsClient(config) as client:
        album_path = await client.download_album("23329")
        print(f"Downloaded to: {album_path}")

if __name__ == "__main__":
    asyncio.run(main())
```

### Using Environment Variables

```bash
# Create .env file
cp .env.example .env
# Edit .env with your credentials
```

```python
from nugs_client import NugsClient

async def main():
    # Automatically loads from .env
    async with NugsClient() as client:
        await client.download_album("23329")
```

## Authentication

### Option 1: Email/Password
```python
config = NugsConfig(
    email="your.email@example.com",
    password="your_password"
)
```

### Option 2: Pre-existing Token
For Apple/Google authenticated accounts:
```python
config = NugsConfig(
    token="your_access_token_here"
)
```

See [token.md](https://github.com/Sorrow446/Nugs-Downloader/blob/main/token.md) for how to extract tokens.

## Supported Content Types

| Content Type | URL Example | Method |
|--------------|-------------|--------|
| Album | `https://play.nugs.net/release/23329` | `download_album(id)` |
| Artist Catalog | `https://play.nugs.net/artist/461` | `download_artist(id)` |
| User Playlist | `https://play.nugs.net/#/playlists/playlist/1215400` | `download_playlist(id)` |
| Catalog Playlist | `https://2nu.gs/3PmqXLW` | `download_catalog_playlist(url)` |
| Video | `https://play.nugs.net/#/videos/artist/.../27323` | `download_video(id)` |
| Livestream | `https://play.nugs.net/watch/livestreams/exclusive/30119` | `download_video(id, is_livestream=True)` |

## Audio Formats

| Format | Quality | Format ID |
|--------|---------|-----------|
| ALAC | 16-bit / 44.1 kHz | 1 |
| FLAC | 16-bit / 44.1 kHz | 2 (default) |
| MQA | 24-bit / 48 kHz | 3 |
| 360 Reality Audio | Spatial | 4 |
| AAC | 150 Kbps | 5 |

The client automatically selects the best available format and falls back gracefully if unavailable.

## Video Formats

| Resolution | Format ID |
|------------|-----------|
| 480p | 1 |
| 720p | 2 |
| 1080p | 3 (default) |
| 1440p | 4 |
| 4K/UHD | 5 |

## Advanced Usage

### Download with Custom Quality
```python
async with NugsClient() as client:
    # MQA audio
    await client.download_album("23329", quality=3)
    
    # 4K video
    await client.download_video("27323", resolution=5)
```

### Download Entire Artist Catalog
```python
async with NugsClient() as client:
    paths = await client.download_artist("461", skip_videos=False)
    print(f"Downloaded {len(paths)} albums")
```

### Download Playlist
```python
async with NugsClient() as client:
    # User playlist
    await client.download_playlist("1215400")
    
    # Catalog playlist (short link)
    await client.download_catalog_playlist("https://2nu.gs/3PmqXLW")
```

### Video with Chapters
```python
config = NugsConfig(
    video_format=3,      # 1080p
    skip_chapters=False  # Embed chapters
)

async with NugsClient(config) as client:
    await client.download_video("27323")
```

### Concurrent Downloads
```python
from nugs_client import DownloadManager

config = NugsConfig(max_concurrent=5)  # 5 simultaneous downloads
async with NugsClient(config) as client:
    await client.download_artist("461")
```

## Configuration Options

All options can be set via `NugsConfig` or environment variables:

| Option | Env Var | Default | Description |
|--------|---------|---------|-------------|
| `email` | `NUGS_EMAIL` | None | Account email |
| `password` | `NUGS_PASSWORD` | None | Account password |
| `token` | `NUGS_TOKEN` | None | Pre-existing access token |
| `audio_format` | `NUGS_AUDIO_FORMAT` | 2 (FLAC) | Audio quality (1-5) |
| `video_format` | `NUGS_VIDEO_FORMAT` | 3 (1080p) | Video resolution (1-5) |
| `output_path` | `NUGS_OUTPUT_PATH` | "downloads" | Download directory |
| `use_system_ffmpeg` | `NUGS_USE_SYSTEM_FFMPEG` | True | Use system FFmpeg |
| `skip_videos` | `NUGS_SKIP_VIDEOS` | False | Skip videos in artist downloads |
| `skip_chapters` | `NUGS_SKIP_CHAPTERS` | False | Skip video chapter embedding |
| `max_concurrent` | `NUGS_MAX_CONCURRENT` | 3 | Concurrent downloads |
| `retry_attempts` | `NUGS_RETRY_ATTEMPTS` | 3 | HTTP retry attempts |

## CLI Usage (Optional)

Install with CLI support:
```bash
uv add nugs-client[cli]
```

Download content:
```bash
nugs download "https://play.nugs.net/release/23329"
nugs download --audio-format 3 --output ~/music "https://play.nugs.net/artist/461"
```

Authenticate:
```bash
nugs auth your.email@example.com
```

## Troubleshooting

### FFmpeg Not Found
```
FFmpegError: FFmpeg not found in system PATH
```
**Solution**: Install FFmpeg or set `use_system_ffmpeg=False` and place `ffmpeg` binary in project root.

### Authentication Failed
```
AuthenticationError: Invalid credentials
```
**Solution**: 
- Verify email/password are correct
- For Apple/Google accounts, extract and use access token
- Check if subscription is active

### Quality Not Available
```
QualityNotAvailableError: MQA not available for this track
```
**Solution**: The client automatically falls back to lower quality. This is informational.

### HLS Decryption Failed
```
HLSError: Failed to decrypt segment
```
**Solution**: 
- Ensure FFmpeg is installed
- Check internet connection
- Try again (may be temporary API issue)

## Development

### Setup
```bash
git clone https://github.com/yourusername/nugs-client.git
cd nugs-client
uv sync --dev
```

### Run Tests
```bash
# Unit tests only
uv run pytest -m "not integration"

# All tests (requires credentials)
export NUGS_EMAIL="your@email.com"
export NUGS_PASSWORD="your_password"
uv run pytest

# With coverage
uv run pytest --cov
```

### Type Checking
```bash
uv run mypy src/nugs_client
```

### Linting
```bash
uv run ruff check src/ tests/
uv run ruff format src/ tests/
```

## Architecture

```
nugs_client/
├── client.py          # Main NugsClient class
├── auth.py            # Authentication handler
├── models.py          # Pydantic data models
├── endpoints.py       # API endpoint methods
├── downloader.py      # Download manager
├── hls.py             # HLS manifest parsing & decryption
├── video.py           # Video processing
├── constants.py       # API constants
├── exceptions.py      # Custom exceptions
└── utils.py           # Helper utilities
```

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Add tests
5. Run linting and tests
6. Commit (`git commit -m 'Add amazing feature'`)
7. Push (`git push origin feature/amazing-feature`)
8. Open a Pull Request

## License

MIT License - see [LICENSE](LICENSE) file for details

## Disclaimer

- This project is not affiliated with nugs.net
- Use responsibly and respect content licensing
- Ensure you have proper subscription/purchase rights for downloaded content

## Credits

- Based on the excellent [Nugs-Downloader](https://github.com/Sorrow446/Nugs-Downloader) by Sorrow446
- API documentation derived from reverse engineering efforts

## Links

- [nugs.net Official Site](https://www.nugs.net/)
- [Issue Tracker](https://github.com/yourusername/nugs-client/issues)
- [Changelog](CHANGELOG.md)
