package main

// Type aliases bridging root package to internal/model during migration.
// These will be removed in Phase 12 when all code moves to internal packages.

import (
	"fmt"

	"github.com/jmagar/nugs-cli/internal/model"
)

// Type aliases for model types.
type (
	Config                = model.Config
	Args                  = model.Args
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

func init() {
	// Wire the colored help text into model.Args.Description()
	model.ArgsDescriptionFunc = argsDescription
}

func argsDescription() string {
	return fmt.Sprintf(`%s♪ Download music and videos from Nugs.net%s

%s◆ LIST COMMANDS%s
%s─────────────────────────────────────────────────────────────────────────────%s
  %s•%s %slist%s                              List all available artists
  %s•%s %slist >100%s                         Filter artists by show count (>, <, >=, <=, =)
  %s•%s %slist <artist_id>%s                  List all shows for a specific artist
  %s•%s %slist <artist_id> video%s            List shows filtered by media type (audio/video/both)
  %s•%s %slist <artist_id> "venue"%s          Filter shows by venue name
  %s•%s %slist <artist_id> latest <N>%s       Show latest N shows for an artist
  %s•%s %sgrab <artist_id> latest%s           Download latest shows from an artist

%s◆ CATALOG COMMANDS%s
%s─────────────────────────────────────────────────────────────────────────────%s
  %s•%s %supdate%s                            Fetch and cache latest catalog
  %s•%s %scache%s                             Show cache status and metadata
  %s•%s %sstats%s                             Display catalog statistics
  %s•%s %slatest [limit]%s                    Show latest additions (default 15)
  %s•%s %sgaps <id> [...]%s                   List missing shows only (one or more artists)
  %s•%s %sgaps <id> video%s                   Filter gaps by media type (audio/video/both)
  %s•%s %sgaps <id> --ids-only%s              Output just IDs for piping
  %s•%s %sgaps <id> fill%s                    Auto-download all missing shows
  %s•%s %scoverage [ids...]%s                 Show download coverage statistics
  %s•%s %srefresh enable|disable|set%s        Configure auto-refresh

%s◆ JSON OUTPUT LEVELS%s %s(--json <level>)%s
%s─────────────────────────────────────────────────────────────────────────────%s
  %s•%s %sminimal%s                           Essential fields only
  %s•%s %sstandard%s                          Adds location details (for shows)
  %s•%s %sextended%s                          All available metadata
  %s•%s %sraw%s                               Unmodified API response

%s◆ EXAMPLES%s
%s─────────────────────────────────────────────────────────────────────────────%s
  %s▸%s %snugs help%s
  %s▸%s %snugs list%s
  %s▸%s %snugs list 461%s
  %s▸%s %snugs list 461 "Red Rocks"%s
  %s▸%s %snugs list 1125 latest 5%s
  %s▸%s %snugs list ">100"%s
  %s▸%s %snugs 12345%s                        Download show by ID
  %s▸%s %snugs grab 461 latest%s              Download latest shows from artist
  %s▸%s %snugs update%s                       Update local catalog cache
  %s▸%s %snugs gaps 1125%s                    Find missing shows for artist
  %s▸%s %snugs gaps 1125 fill%s               Auto-download all missing shows
  %s▸%s %snugs coverage 1125 461%s            Check download coverage

  %s→%s Full URLs also work: %snugs https://play.nugs.net/release/12345%s
`,
		colorBold, colorReset,
		colorBold, colorReset,
		colorCyan, colorReset,
		colorGreen, colorReset, colorCyan, colorReset,
		colorGreen, colorReset, colorCyan, colorReset,
		colorGreen, colorReset, colorCyan, colorReset,
		colorGreen, colorReset, colorCyan, colorReset,
		colorGreen, colorReset, colorCyan, colorReset,
		colorGreen, colorReset, colorCyan, colorReset,
		colorGreen, colorReset, colorCyan, colorReset,
		colorBold, colorReset,
		colorCyan, colorReset,
		colorGreen, colorReset, colorCyan, colorReset,
		colorGreen, colorReset, colorCyan, colorReset,
		colorGreen, colorReset, colorCyan, colorReset,
		colorGreen, colorReset, colorCyan, colorReset,
		colorGreen, colorReset, colorCyan, colorReset,
		colorGreen, colorReset, colorCyan, colorReset,
		colorGreen, colorReset, colorCyan, colorReset,
		colorGreen, colorReset, colorCyan, colorReset,
		colorGreen, colorReset, colorCyan, colorReset,
		colorGreen, colorReset, colorCyan, colorReset,
		colorBold, colorReset, colorCyan, colorReset,
		colorCyan, colorReset,
		colorGreen, colorReset, colorCyan, colorReset,
		colorGreen, colorReset, colorCyan, colorReset,
		colorGreen, colorReset, colorCyan, colorReset,
		colorGreen, colorReset, colorCyan, colorReset,
		colorBold, colorReset,
		colorCyan, colorReset,
		colorYellow, colorReset, colorCyan, colorReset,
		colorYellow, colorReset, colorCyan, colorReset,
		colorYellow, colorReset, colorCyan, colorReset,
		colorYellow, colorReset, colorCyan, colorReset,
		colorYellow, colorReset, colorCyan, colorReset,
		colorYellow, colorReset, colorCyan, colorReset,
		colorYellow, colorReset, colorCyan, colorReset,
		colorYellow, colorReset, colorCyan, colorReset,
		colorYellow, colorReset, colorCyan, colorReset,
		colorYellow, colorReset, colorCyan, colorReset,
		colorYellow, colorReset, colorCyan, colorReset,
		colorYellow, colorReset, colorCyan, colorReset,
		colorCyan, colorReset, colorYellow, colorReset,
	)
}
