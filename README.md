# Nugs-Downloader
Nugs downloader written in Go.
![](https://i.imgur.com/NOsQjnP.png)
![](https://i.imgur.com/BEudufy.png)
[Windows, Linux, macOS, and Android binaries](https://github.com/Sorrow446/Nugs-Downloader/releases)

# Building from Source

**Requirements:** Go 1.16 or later

```bash
# Clone the repository
git clone https://github.com/Sorrow446/Nugs-Downloader.git
cd Nugs-Downloader

# Build using Makefile (recommended)
make build

# Or build manually
mkdir -p bin
go build -o bin/nugs

# Optionally install to system (requires sudo)
make install
```

The binary will be created at `bin/nugs` and can be run from anywhere if you add `bin/` to your PATH or use `make install`.

# Setup
Input credentials into config file.
Configure any other options if needed.
|Option|Info|
| --- | --- |
|email|Email address.
|password|Password.
|format|Track download quality. 1 = 16-bit / 44.1 kHz ALAC, 2 = 16-bit / 44.1 kHz FLAC, 3 = 24-bit / 48 kHz MQA, 4 = 360 Reality Audio / best available, 5 = 150 Kbps AAC.
|videoFormat|Video download format. 1 = 480p, 2 = 720p, 3 = 1080p, 4 = 1440p, 5 = 4K / best available. **FFmpeg needed, see below.**
|outPath|Where to download to. Path will be made if it doesn't already exist.
|token|Token to auth with Apple and Google accounts ([how to get token](https://github.com/Sorrow446/Nugs-Downloader/blob/main/token.md)). Ignore if you're using a regular account.
|useFfmpegEnvVar|true = call FFmpeg from environment variable, false = call from script dir.

**FFmpeg is needed for TS -> MP4 losslessly for videos & HLS-only tracks, see below.**  

# FFmpeg Setup
[Windows (gpl)](https://github.com/BtbN/FFmpeg-Builds/releases)    
Linux: `sudo apt install ffmpeg`    
Termux `pkg install ffmpeg`    
Place in Nugs DL's script/binary directory if using FFmpeg binary.

If you don't have root in Linux, you can have Nugs DL look for the binary in the same dir by setting the `useFfmpegEnvVar` option to false.

## Supported Media
|Type|URL example|
| --- | --- |
|Album|`https://play.nugs.net/release/23329` or just `23329` (album ID)
|Artist|`https://play.nugs.net/#/artist/461/latest`, `https://play.nugs.net/#/artist/461`
|Catalog playlist|`https://2nu.gs/3PmqXLW`
|Exclusive Livestream|`https://play.nugs.net/watch/livestreams/exclusive/30119`
|Purchased Livestream|`https://www.nugs.net/on/demandware.store/Sites-NugsNet-Site/default/Stash-QueueVideo?skuID=624598&showID=30367&perfDate=10-29-2022&artistName=Billy%20Strings&location=10-29-2022%20Exploreasheville%2ecom%20Arena%20Asheville%2c%20NC&format=liveHdStream` Wrap in double quotes on Windows.
|User playlist|`https://play.nugs.net/#/playlists/playlist/1215400`, `https://play.nugs.net/library/playlist/1261211`
|Video|`https://play.nugs.net/#/videos/artist/1045/Dead%20and%20Company/container/27323` Wrap in double quotes on Windows.
|Webcast|`https://play.nugs.net/#/my-webcasts/5826189-30369-0-624602`

## List Commands
You can browse the catalog without downloading:

**List all artists:**
```bash
nugs list artists
```

**List all shows for an artist:**
```bash
nugs list 461
```
Replace `461` with any artist ID from the artist list.

**Download an artist's latest shows:**
```bash
nugs 461 latest
```
This automatically downloads the latest shows from the specified artist (shorthand for the full artist/latest URL).

### JSON Output

List commands support JSON output with the `--json <level>` flag for scripting and integration with tools like `jq`:

**Output Levels:**
- `minimal` - Essential fields only (ID, name, date, title, venue)
- `standard` - Adds location details (city, state)
- `extended` - All available metadata fields
- `raw` - Unmodified API response

**Examples:**

```bash
# Get artists as JSON (sorted alphabetically)
nugs list artists --json standard

# Get shows with location details
nugs list 1125 --json standard

# Filter with jq - artists with 100+ shows
nugs list artists --json standard | jq '.artists[] | select(.numShows > 100)'

# Get latest 5 shows
nugs list 461 --json minimal | jq '.shows[:5]'

# Find shows at specific venue
nugs list 461 --json standard | jq '.shows[] | select(.venue == "Madison Square Garden")'
```

**Output Formats:**

*Artists (minimal/standard/extended - all same data):*
```json
{
  "artists": [
    {
      "artistID": 461,
      "artistName": "Grateful Dead",
      "numShows": 2500,
      "numAlbums": 350
    }
  ],
  "total": 1
}
```

*Shows (minimal):*
```json
{
  "artistID": 461,
  "artistName": "Grateful Dead",
  "shows": [
    {
      "containerID": 12345,
      "date": "2024-10-15",
      "title": "Fall Tour 2024",
      "venue": "Madison Square Garden"
    }
  ],
  "total": 1
}
```

*Shows (standard - adds location):*
```json
{
  "shows": [
    {
      "containerID": 12345,
      "date": "2024-10-15",
      "title": "Fall Tour 2024",
      "venue": "Madison Square Garden",
      "venueCity": "New York",
      "venueState": "NY"
    }
  ]
}
```

# Usage
Args take priority over the config file.

Download two albums:
```bash
nugs https://play.nugs.net/release/23329 https://play.nugs.net/release/23790
```

Download using simple album IDs (shorthand):
```bash
nugs 23329 23790
```

Download a single album and from two text files:
```bash
nugs https://play.nugs.net/release/23329 /path/to/urls1.txt /path/to/urls2.txt
```

Download a user playlist and video:
```bash
nugs https://play.nugs.net/#/playlists/playlist/1215400 "https://play.nugs.net/#/videos/artist/1045/Dead%20and%20Company/container/27323"
```

## Command Line Options

```
Usage: nugs [--format FORMAT] [--videoformat VIDEOFORMAT] [--outpath OUTPATH] [--force-video] [--skip-videos] [--skip-chapters] [URLS [URLS ...]]

Special Commands:
  help                           Show this help message
  list artists                   List all available artists
  list <artist_id>               List all shows for a specific artist
  list artists --json <level>    Output artists as JSON (minimal/standard/extended/raw)
  list <artist_id> --json <level> Output shows as JSON
  <artist_id> latest             Download latest shows from an artist

Positional arguments:
  URLS                           Album/artist URLs, IDs, or text files containing URLs

Options:
  --format FORMAT, -f FORMAT
                         Track download format.
                         1 = 16-bit / 44.1 kHz ALAC
                         2 = 16-bit / 44.1 kHz FLAC
                         3 = 24-bit / 48 kHz MQA
                         4 = 360 Reality Audio / best available
                         5 = 150 Kbps AAC [default: -1]
  --videoformat VIDEOFORMAT, -F VIDEOFORMAT
                         Video download format.
                         1 = 480p
                         2 = 720p
                         3 = 1080p
                         4 = 1440p
                         5 = 4K / best available [default: -1]
  --outpath OUTPATH, -o OUTPATH
                         Where to download to. Path will be made if it doesn't already exist.
  --force-video          Forces video when it co-exists with audio in release URLs.
  --skip-videos          Skips videos in artist URLs.
  --skip-chapters        Skips chapters for videos.
  --help, -h             display this help and exit
```
 
# Disclaimer
- I will not be responsible for how you use Nugs Downloader.    
- Nugs brand and name is the registered trademark of its respective owner.    
- Nugs Downloader has no partnership, sponsorship or endorsement with Nugs.
