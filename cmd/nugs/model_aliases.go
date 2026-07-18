package main

// Command-layer aliases for the internal model contract.

import "github.com/jmagar/nugs-cli/internal/model"

// Type aliases for model types.
type (
	Config                = model.Config
	Transport             = model.Transport
	BatchProgressState    = model.BatchProgressState
	RuntimeStatus         = model.RuntimeStatus
	RuntimeControl        = model.RuntimeControl
	ContainerWithDate     = model.ContainerWithDate
	ShowStatus            = model.ShowStatus
	ArtistCatalogAnalysis = model.ArtistCatalogAnalysis
	ProgressBoxState      = model.ProgressBoxState
	WriteCounter          = model.WriteCounter

	Auth              = model.Auth
	Payload           = model.Payload
	UserInfo          = model.UserInfo
	SubInfo           = model.SubInfo
	StreamParams      = model.StreamParams
	Product           = model.Product
	ProductFormatList = model.ProductFormatList
	AlbArtResp        = model.AlbArtResp
	AlbumMeta         = model.AlbumMeta
	Token             = model.Token
	PlistMeta         = model.PlistMeta
	Track             = model.Track
	StreamMeta        = model.StreamMeta
	Quality           = model.Quality
	ArtistMeta        = model.ArtistMeta
	Artist            = model.Artist
	ArtistListResp    = model.ArtistListResp
	PurchasedManResp  = model.PurchasedManResp
	ArtistListOutput  = model.ArtistListOutput
	ArtistOutput      = model.ArtistOutput
	ShowListOutput    = model.ShowListOutput
	ShowOutput        = model.ShowOutput

	LatestCatalogResp   = model.LatestCatalogResp
	CacheMeta           = model.CacheMeta
	ArtistsIndex        = model.ArtistsIndex
	ContainersIndex     = model.ContainersIndex
	ContainerIndexEntry = model.ContainerIndexEntry
	ArtistMetaCache     = model.ArtistMetaCache

	MediaType = model.MediaType
)

// Re-export MediaType constants
const (
	MediaTypeUnknown = model.MediaTypeUnknown
	MediaTypeAudio   = model.MediaTypeAudio
	MediaTypeVideo   = model.MediaTypeVideo
	MediaTypeBoth    = model.MediaTypeBoth
)

// Re-export functions
var ParseMediaType = model.ParseMediaType

// Re-export JSON output level constants
const (
	JSONLevelMinimal  = model.JSONLevelMinimal
	JSONLevelStandard = model.JSONLevelStandard
	JSONLevelExtended = model.JSONLevelExtended
	JSONLevelRaw      = model.JSONLevelRaw
)

// Re-export message priority constants
const (
	MessagePriorityStatus  = model.MessagePriorityStatus
	MessagePriorityWarning = model.MessagePriorityWarning
	MessagePriorityError   = model.MessagePriorityError
)
