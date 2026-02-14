package model

import "time"

// Config holds the user's configuration.
type Config struct {
	Email                  string   `json:"email"`
	Password               string   `json:"password"`
	Urls                   []string `json:"urls,omitempty"`
	Format                 int      `json:"format"`
	OutPath                string   `json:"outPath"`
	VideoOutPath           string   `json:"videoOutPath,omitempty"`
	VideoFormat            int      `json:"videoFormat"`
	DefaultOutputs         string   `json:"defaultOutputs,omitempty"`
	WantRes                string   `json:"wantRes,omitempty"`
	Token                  string   `json:"token"`
	UseFfmpegEnvVar        bool     `json:"useFfmpegEnvVar"`
	FfmpegNameStr          string   `json:"ffmpegNameStr,omitempty"`
	ForceVideo             bool     `json:"forceVideo,omitempty"`
	SkipVideos             bool     `json:"skipVideos,omitempty"`
	SkipChapters           bool     `json:"skipChapters,omitempty"`
	RcloneEnabled          bool     `json:"rcloneEnabled,omitempty"`
	RcloneRemote           string   `json:"rcloneRemote,omitempty"`
	RclonePath             string   `json:"rclonePath,omitempty"`
	RcloneVideoPath        string   `json:"rcloneVideoPath,omitempty"`
	DeleteAfterUpload      bool     `json:"deleteAfterUpload,omitempty"`
	RcloneTransfers        int      `json:"rcloneTransfers,omitempty"`
	CatalogAutoRefresh     bool     `json:"catalogAutoRefresh,omitempty"`
	CatalogRefreshTime     string   `json:"catalogRefreshTime,omitempty"`
	CatalogRefreshTimezone string   `json:"catalogRefreshTimezone,omitempty"`
	CatalogRefreshInterval string   `json:"catalogRefreshInterval,omitempty"`
	SkipSizePreCalculation bool     `json:"skipSizePreCalculation,omitempty"`
}

// ArgsDescriptionFunc is set by package main's init() (in cmd/nugs/model_aliases.go)
// to provide colored help text. This is a startup-time side effect that wires the
// root package's argsDescription() into the model layer. Tests that call
// Description() should set this explicitly or accept the empty-string default.
// If nil, Description() returns an empty string (go-arg will use default help).
var ArgsDescriptionFunc func() string

// Args holds CLI arguments parsed by go-arg.
type Args struct {
	Urls         []string `arg:"positional"`
	Format       int      `arg:"-f" default:"-1" help:"Track download format.\n\t\t\t 1 = 16-bit / 44.1 kHz ALAC\n\t\t\t 2 = 16-bit / 44.1 kHz FLAC\n\t\t\t 3 = 24-bit / 48 kHz MQA\n\t\t\t 4 = 360 Reality Audio / best available\n\t\t\t 5 = 150 Kbps AAC"`
	VideoFormat  int      `arg:"-F" default:"-1" help:"Video download format.\n\t\t\t 1 = 480p\n\t\t\t 2 = 720p\n\t\t\t 3 = 1080p\n\t\t\t 4 = 1440p\n\t\t\t 5 = 4K / best available"`
	OutPath      string   `arg:"-o" help:"Where to download to. Path will be made if it doesn't already exist."`
	ForceVideo   bool     `arg:"--force-video" help:"[Deprecated] Use 'nugs grab <id> video' or set defaultOutputs in config."`
	SkipVideos   bool     `arg:"--skip-videos" help:"[Deprecated] Use 'nugs grab <id> audio' or set defaultOutputs in config."`
	SkipChapters bool     `arg:"--skip-chapters" help:"Skips chapters for videos."`
}

// Description provides custom help text for go-arg.
// It delegates to ArgsDescriptionFunc if set, allowing the root package
// to inject colored output without model depending on UI code.
func (Args) Description() string {
	if ArgsDescriptionFunc != nil {
		return ArgsDescriptionFunc()
	}
	return ""
}

// Transport is used as a custom HTTP transport.
type Transport struct{}

// BatchProgressState tracks progress across multiple albums/shows in a batch operation.
type BatchProgressState struct {
	CurrentAlbum int
	TotalAlbums  int
	Complete     int
	Failed       int
	Skipped      int
	StartTime    time.Time
	CurrentTitle string
}

// Validate ensures batch progress state fields are consistent and within valid bounds.
func (b *BatchProgressState) Validate() {
	if b == nil {
		return
	}
	if b.CurrentAlbum > b.TotalAlbums {
		b.CurrentAlbum = b.TotalAlbums
	}
	total := b.Complete + b.Failed + b.Skipped
	if total > b.TotalAlbums {
		if b.Complete > b.TotalAlbums {
			b.Complete = b.TotalAlbums
			b.Failed = 0
			b.Skipped = 0
		} else if b.Complete+b.Failed > b.TotalAlbums {
			b.Failed = b.TotalAlbums - b.Complete
			b.Skipped = 0
		} else {
			b.Skipped = b.TotalAlbums - b.Complete - b.Failed
		}
	}
	if b.CurrentAlbum < 0 {
		b.CurrentAlbum = 0
	}
	if b.Complete < 0 {
		b.Complete = 0
	}
	if b.Failed < 0 {
		b.Failed = 0
	}
	if b.Skipped < 0 {
		b.Skipped = 0
	}
}

// RuntimeStatus tracks the state of a running crawl.
type RuntimeStatus struct {
	PID        int    `json:"pid"`
	State      string `json:"state"`
	StartedAt  string `json:"startedAt"`
	UpdatedAt  string `json:"updatedAt"`
	Label      string `json:"label,omitempty"`
	Percentage int    `json:"percentage,omitempty"`
	Speed      string `json:"speed,omitempty"`
	Current    string `json:"current,omitempty"`
	Total      string `json:"total,omitempty"`
	Errors     int    `json:"errors"`
	Warnings   int    `json:"warnings"`
}

// RuntimeControl holds pause/cancel signals for crawl control.
type RuntimeControl struct {
	Pause     bool   `json:"pause"`
	Cancel    bool   `json:"cancel"`
	UpdatedAt string `json:"updatedAt"`
}

// ContainerWithDate pairs a show container with its date string and optional media type.
type ContainerWithDate struct {
	Container *AlbArtResp
	DateStr   string
	MediaType MediaType
}

// ShowStatus stores a show and whether it is already downloaded.
type ShowStatus struct {
	Show       *AlbArtResp `json:"show"`
	Downloaded bool        `json:"downloaded"`
	MediaType  MediaType   `json:"mediaType"`
}

// ArtistCatalogAnalysis stores the computed status for all shows for one artist.
type ArtistCatalogAnalysis struct {
	ArtistID      string       `json:"artistID"`
	ArtistName    string       `json:"artistName"`
	TotalShows    int          `json:"totalShows"`
	Downloaded    int          `json:"downloaded"`
	Missing       int          `json:"missing"`
	Shows         []ShowStatus `json:"shows"`
	MissingShows  []ShowStatus `json:"missingShows"`
	DownloadPct   float64      `json:"downloadedPct"`
	MissingPct    float64      `json:"missingPct"`
	CacheUsed     bool         `json:"cacheUsed"`
	CacheStaleUse bool         `json:"cacheStaleUse"`
	MediaFilter   MediaType    `json:"mediaFilter"`
}
