package model

import (
	"strings"
	"time"
)

// JSON output levels
const (
	JSONLevelMinimal  = "minimal"
	JSONLevelStandard = "standard"
	JSONLevelExtended = "extended"
	JSONLevelRaw      = "raw"
)

// Message priority constants for progress box messages
const (
	MessagePriorityStatus  = 1 // Info messages (cyan, info symbol)
	MessagePriorityWarning = 2 // Warning messages (yellow, warning symbol)
	MessagePriorityError   = 3 // Error messages (red, cross symbol)
)

// Shared download constants.
const (
	MaxProgressPercent = 100
	KBpsDivisor        = 1000

	UnknownSizeLabel       = "Unknown"
	UnknownSizeLabelLower  = "unknown"
	UnknownResolutionLabel = "unknown"
	ZeroBytesLabel         = "0 B"
	CalculatingSizeLabel   = "calculating..."

	StatusMessageDuration = 5 * time.Second
	SkipMessageDuration   = 3 * time.Second

	MaxFormatFallbackAttempts = 10

	PreCalcConcurrency       = 8
	PreCalcPerTrackTimeout   = 5 * time.Second
	PreCalcPerRequestTimeout = 5 * time.Second
	PreCalcMaxTimeout        = 60 * time.Second

	AlbumFolderMaxRunes = 120
	VideoNameMaxRunes   = 110

	BatchShowNumberFormat = "Show %d/%d: %s"

	VideoShowNumberDefault    = "Video"
	VideoTrackNameDefault     = "Video Stream"
	VideoDownloadStatusLabel  = "Downloading video stream"
	VideoConvertStatusLabel   = "Converting TS to MP4"
	VideoUploadStatusLabel    = "Uploading video to rclone"
	VideoOnDemandFormatLabel  = "VIDEO ON DEMAND"
	LiveHDVideoFormatLabel    = "LIVE HD VIDEO"
	AvailableAvailabilityType = "AVAILABLE"
	ShowContainerType         = "Show"

	Res2160 = "2160"
	Res4K   = "4K"
	Resp    = "p"
)

var TrackStreamMetaFormatProbeOrder = [4]int{1, 4, 7, 10}

// MediaType represents the type of media content (audio, video, or both)
type MediaType int

const (
	MediaTypeUnknown MediaType = 0
	MediaTypeAudio   MediaType = 1
	MediaTypeVideo   MediaType = 2
	MediaTypeBoth    MediaType = 3
)

// String returns the string representation of the MediaType
func (m MediaType) String() string {
	switch m {
	case MediaTypeAudio:
		return "audio"
	case MediaTypeVideo:
		return "video"
	case MediaTypeBoth:
		return "both"
	default:
		return "unknown"
	}
}

// ParseMediaType converts a string to a MediaType
func ParseMediaType(s string) MediaType {
	switch strings.ToLower(s) {
	case "audio":
		return MediaTypeAudio
	case "video":
		return MediaTypeVideo
	case "both":
		return MediaTypeBoth
	default:
		return MediaTypeUnknown
	}
}

// HasAudio returns true if the media type includes audio
func (m MediaType) HasAudio() bool {
	return m == MediaTypeAudio || m == MediaTypeBoth
}

// HasVideo returns true if the media type includes video
func (m MediaType) HasVideo() bool {
	return m == MediaTypeVideo || m == MediaTypeBoth
}
