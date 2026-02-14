package main

// Type aliases bridging root package to internal/model during migration.
// These will be removed in Phase 12 when all code moves to internal packages.

import (
	"fmt"
	"strings"

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
	var b strings.Builder

	heading := func(title string) {
		fmt.Fprintf(&b, "\n%s◆ %s%s\n", colorBold, title, colorReset)
		fmt.Fprintf(&b, "%s─────────────────────────────────────────────────────────────────────────────%s\n", colorCyan, colorReset)
	}
	headingWithNote := func(title, note string) {
		fmt.Fprintf(&b, "\n%s◆ %s%s %s%s%s\n", colorBold, title, colorReset, colorCyan, note, colorReset)
		fmt.Fprintf(&b, "%s─────────────────────────────────────────────────────────────────────────────%s\n", colorCyan, colorReset)
	}
	cmd := func(syntax, description string) {
		fmt.Fprintf(&b, "  %s•%s %s%s%s %s\n", colorGreen, colorReset, colorCyan, syntax, colorReset, description)
	}
	example := func(syntax string) {
		fmt.Fprintf(&b, "  %s▸%s %s%s%s\n", colorYellow, colorReset, colorCyan, syntax, colorReset)
	}
	exampleWithDesc := func(syntax, desc string) {
		fmt.Fprintf(&b, "  %s▸%s %s%s%s %s\n", colorYellow, colorReset, colorCyan, syntax, colorReset, desc)
	}

	fmt.Fprintf(&b, "%s♪ Download music and videos from Nugs.net%s\n", colorBold, colorReset)

	heading("LIST COMMANDS")
	cmd("list", "                             List all available artists")
	cmd("list >100", "                        Filter artists by show count (>, <, >=, <=, =)")
	cmd("list <artist_id>", "                 List all shows for a specific artist")
	cmd("list <artist_id> video", "           List shows filtered by media type (audio/video/both)")
	cmd(`list <artist_id> "venue"`, "         Filter shows by venue name")
	cmd("list <artist_id> latest <N>", "      Show latest N shows for an artist")
	cmd("grab <artist_id> latest", "          Download latest shows from an artist")

	heading("CATALOG COMMANDS")
	cmd("update", "                           Fetch and cache latest catalog")
	cmd("cache", "                            Show cache status and metadata")
	cmd("stats", "                            Display catalog statistics")
	cmd("latest [limit]", "                   Show latest additions (default 15)")
	cmd("gaps <id> [...]", "                  List missing shows only (one or more artists)")
	cmd("gaps <id> video", "                  Filter gaps by media type (audio/video/both)")
	cmd("gaps <id> --ids-only", "             Output just IDs for piping")
	cmd("gaps <id> fill", "                   Auto-download all missing shows")
	cmd("coverage [ids...]", "                Show download coverage statistics")
	cmd("refresh enable|disable|set", "       Configure auto-refresh")

	headingWithNote("JSON OUTPUT LEVELS", "(--json <level>)")
	cmd("minimal", "                          Essential fields only")
	cmd("standard", "                         Adds location details (for shows)")
	cmd("extended", "                         All available metadata")
	cmd("raw", "                              Unmodified API response")

	heading("EXAMPLES")
	example("nugs help")
	example("nugs list")
	example("nugs list 461")
	example(`nugs list 461 "Red Rocks"`)
	example("nugs list 1125 latest 5")
	example(`nugs list ">100"`)
	exampleWithDesc("nugs 12345", "                       Download show by ID")
	example("nugs grab 461 latest")
	example("nugs update")
	example("nugs gaps 1125")
	example("nugs gaps 1125 fill")
	example("nugs coverage 1125 461")

	fmt.Fprintf(&b, "\n  %s→%s Full URLs also work: %snugs https://play.nugs.net/release/12345%s\n",
		colorCyan, colorReset, colorYellow, colorReset)

	return b.String()
}
