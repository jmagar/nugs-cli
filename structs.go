package main

import (
	"fmt"
	"time"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

// Unicode symbols
const (
	symbolCheck    = "✓"
	symbolCross    = "✗"
	symbolArrow    = "→"
	symbolMusic    = "♪"
	symbolUpload   = "⬆"
	symbolDownload = "⬇"
	symbolInfo     = "ℹ"
	symbolWarning  = "⚠"
	symbolGear     = "⚙"
	symbolPackage  = "📦"
	symbolRocket   = "🚀"
)

type Transport struct{}

type WriteCounter struct {
	Total      int64
	TotalStr   string
	Downloaded int64
	Percentage int
	StartTime  int64
}

type Config struct {
	Email                  string   `json:"email"`
	Password               string   `json:"password"`
	Urls                   []string `json:"urls,omitempty"`
	Format                 int      `json:"format"`
	OutPath                string   `json:"outPath"`
	VideoFormat            int      `json:"videoFormat"`
	WantRes                string   `json:"wantRes,omitempty"`
	Token                  string   `json:"token"`
	UseFfmpegEnvVar        bool     `json:"useFfmpegEnvVar"`
	FfmpegNameStr          string   `json:"ffmpegNameStr,omitempty"`
	ForceVideo             bool     `json:"forceVideo,omitempty"`
	SkipVideos             bool     `json:"skipVideos,omitempty"`
	SkipChapters           bool     `json:"skipChapters,omitempty"`
	RcloneEnabled          bool     `json:"rcloneEnabled,omitempty"`
	RcloneRemote           string   `json:"rcloneRemote,omitempty"`
	RclonePath             string   `json:"rclonePath,omitempty"` // Path on remote storage (NOT local base path)
	DeleteAfterUpload      bool     `json:"deleteAfterUpload,omitempty"`
	RcloneTransfers        int      `json:"rcloneTransfers,omitempty"`
	CatalogAutoRefresh     bool     `json:"catalogAutoRefresh,omitempty"`
	CatalogRefreshTime     string   `json:"catalogRefreshTime,omitempty"`     // "05:00" (24-hour format)
	CatalogRefreshTimezone string   `json:"catalogRefreshTimezone,omitempty"` // "America/New_York"
	CatalogRefreshInterval string   `json:"catalogRefreshInterval,omitempty"` // "daily" or "weekly"
}

type Args struct {
	Urls         []string `arg:"positional"`
	Format       int      `arg:"-f" default:"-1" help:"Track download format.\n			 1 = 16-bit / 44.1 kHz ALAC\n			 2 = 16-bit / 44.1 kHz FLAC\n			 3 = 24-bit / 48 kHz MQA\n			 4 = 360 Reality Audio / best available\n			 5 = 150 Kbps AAC"`
	VideoFormat  int      `arg:"-F" default:"-1" help:"Video download format.\n			 1 = 480p\n			 2 = 720p\n			 3 = 1080p\n			 4 = 1440p\n			 5 = 4K / best available"`
	OutPath      string   `arg:"-o" help:"Where to download to. Path will be made if it doesn't already exist."`
	ForceVideo   bool     `arg:"--force-video" help:"Forces video when it co-exists with audio in release URLs."`
	SkipVideos   bool     `arg:"--skip-videos" help:"Skips videos in artist URLs."`
	SkipChapters bool     `arg:"--skip-chapters" help:"Skips chapters for videos."`
}

// Description provides custom help text
func (Args) Description() string {
	return fmt.Sprintf(`%s♪ Download music and videos from Nugs.net%s

%s◆ LIST COMMANDS%s
%s─────────────────────────────────────────────────────────────────────────────%s
  %s•%s %slist%s                              List all available artists
  %s•%s %slist >100%s                         Filter artists by show count (>, <, >=, <=, =)
  %s•%s %slist <artist_id>%s                  List all shows for a specific artist
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

type Auth struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

type Payload struct {
	Nbf         int      `json:"nbf"`
	Exp         int      `json:"exp"`
	Iss         string   `json:"iss"`
	Aud         []string `json:"aud"`
	ClientID    string   `json:"client_id"`
	Sub         string   `json:"sub"`
	AuthTime    int      `json:"auth_time"`
	Idp         string   `json:"idp"`
	Email       string   `json:"email"`
	LegacyToken string   `json:"legacy_token"`
	LegacyUguid string   `json:"legacy_uguid"`
	Jti         string   `json:"jti"`
	Sid         string   `json:"sid"`
	Iat         int      `json:"iat"`
	Scope       []string `json:"scope"`
	Amr         []string `json:"amr"`
}

type UserInfo struct {
	Sub               string `json:"sub"`
	PreferredUsername string `json:"preferred_username"`
	Name              string `json:"name"`
	Email             string `json:"email"`
	EmailVerified     bool   `json:"email_verified"`
}

type SubInfo struct {
	StripeMetaData struct {
		SubscriptionID      string `json:"subscriptionId"`
		InvoiceID           string `json:"invoiceId"`
		PaymentIntentStatus any    `json:"paymentIntentStatus"`
		ReturnURL           any    `json:"returnUrl"`
		RedirectURL         any    `json:"redirectUrl"`
		PaymentError        any    `json:"paymentError"`
	} `json:"stripeMetaData"`
	IsTrialAvailable        bool   `json:"isTrialAvailable"`
	AllowAddNewSubscription bool   `json:"allowAddNewSubscription"`
	ID                      string `json:"id"`
	LegacySubscriptionID    string `json:"legacySubscriptionId"`
	Status                  string `json:"status"`
	IsContentAccessible     bool   `json:"isContentAccessible"`
	StartedAt               string `json:"startedAt"`
	EndsAt                  string `json:"endsAt"`
	TrialEndsAt             string `json:"trialEndsAt"`
	Plan                    struct {
		ID              string  `json:"id"`
		Price           float64 `json:"price"`
		Period          int     `json:"period"`
		TrialPeriodDays int     `json:"trialPeriodDays"`
		PlanID          string  `json:"planId"`
		Description     string  `json:"description"`
		ServiceLevel    string  `json:"serviceLevel"`
		StartsAt        any     `json:"startsAt"`
		EndsAt          any     `json:"endsAt"`
	} `json:"plan"`
	Promo struct {
		ID            string  `json:"id"`
		PromoCode     string  `json:"promoCode"`
		PromoPrice    float64 `json:"promoPrice"`
		Description   string  `json:"description"`
		PromoStartsAt any     `json:"promoStartsAt"`
		PromoEndsAt   any     `json:"promoEndsAt"`
		Plan          struct {
			ID              string  `json:"id"`
			Price           float64 `json:"price"`
			Period          int     `json:"period"`
			TrialPeriodDays int     `json:"trialPeriodDays"`
			PlanID          string  `json:"planId"`
			Description     string  `json:"description"`
			ServiceLevel    string  `json:"serviceLevel"`
			StartsAt        any     `json:"startsAt"`
			EndsAt          any     `json:"endsAt"`
		} `json:"plan"`
		Gateway string `json:"gateway"`
	}
}

type StreamParams struct {
	SubscriptionID          string
	SubCostplanIDAccessList string
	UserID                  string
	StartStamp              string
	EndStamp                string
}

type Product struct {
	ProductStatusType    int    `json:"productStatusType"`
	SkuIDExt             any    `json:"skuIDExt"`
	FormatStr            string `json:"formatStr"`
	SkuID                int    `json:"skuID"`
	Cost                 int    `json:"cost"`
	CostplanID           int    `json:"costplanID"`
	Pricing              any    `json:"pricing"`
	Bundles              []any  `json:"bundles"`
	NumPublicPricePoints int    `json:"numPublicPricePoints"`
	CartLink             string `json:"cartLink"`
	LiveEventInfo        struct {
		IsEventLive                  bool   `json:"isEventLive"`
		EventID                      int    `json:"eventID"`
		EventStartDateStr            string `json:"eventStartDateStr"`
		EventEndDateStr              string `json:"eventEndDateStr"`
		TimeZoneToDisplay            any    `json:"timeZoneToDisplay"`
		OffsetFromLocalTimeToDisplay int    `json:"offsetFromLocalTimeToDisplay"`
		UTCoffset                    int    `json:"UTCoffset"`
		EventCode                    any    `json:"eventCode"`
		LinkType                     int    `json:"linkType"`
	} `json:"liveEventInfo"`
	SaleWindowInfo struct {
		IsEventSelling               bool `json:"isEventSelling"`
		SswID                        int  `json:"sswID"`
		TimeZoneToDisplay            any  `json:"timeZoneToDisplay"`
		OffsetFromLocalTimeToDisplay int  `json:"offsetFromLocalTimeToDisplay"`
		SaleStartDateStr             any  `json:"saleStartDateStr"`
		SaleEndDateStr               any  `json:"saleEndDateStr"`
	} `json:"saleWindowInfo"`
	IosCost         int `json:"iosCost"`
	IosPlanName     any `json:"iosPlanName"`
	GooglePlanName  any `json:"googlePlanName"`
	GoogleCost      int `json:"googleCost"`
	NumDiscs        int `json:"numDiscs"`
	IsSubStreamOnly int `json:"isSubStreamOnly"`
}

type ProductFormatList struct {
	PfType     int    `json:"pfType"`
	FormatStr  string `json:"formatStr"`
	SkuID      int    `json:"skuID"`
	Cost       int    `json:"cost"`
	CostplanID int    `json:"costplanID"`
	PfTypeStr  string `json:"pfTypeStr"`
	LiveEvent  struct {
		EventID                      int `json:"eventID"`
		EventStartDateStr            any `json:"eventStartDateStr"`
		EventEndDateStr              any `json:"eventEndDateStr"`
		TimeZoneToDisplay            any `json:"timeZoneToDisplay"`
		OffsetFromLocalTimeToDisplay int `json:"offsetFromLocalTimeToDisplay"`
		UTCoffset                    int `json:"UTCoffset"`
		EventCode                    any `json:"eventCode"`
		LinkType                     int `json:"linkType"`
	} `json:"liveEvent"`
	Salewindow struct {
		SswID                        int `json:"sswID"`
		TimeZoneToDisplay            any `json:"timeZoneToDisplay"`
		OffsetFromLocalTimeToDisplay int `json:"offsetFromLocalTimeToDisplay"`
		SaleStartDateStr             any `json:"saleStartDateStr"`
		SaleEndDateStr               any `json:"saleEndDateStr"`
	} `json:"salewindow"`
	SkuCode         string `json:"skuCode"`
	IsSubStreamOnly int    `json:"isSubStreamOnly"`
}

type AlbArtResp struct {
	NumReviews                int       `json:"numReviews"`
	TotalContainerRunningTime int       `json:"totalContainerRunningTime"`
	HhmmssTotalRunningTime    string    `json:"hhmmssTotalRunningTime"`
	Products                  []Product `json:"products"`
	Subscriptions             any       `json:"subscriptions"`
	Tracks                    []Track   `json:"tracks"`
	Pics                      []struct {
		PicID   int    `json:"picID"`
		OrderID int    `json:"orderID"`
		Height  int    `json:"height"`
		Width   int    `json:"width"`
		Caption string `json:"caption"`
		URL     string `json:"url"`
	} `json:"pics"`
	Recommendations []any `json:"recommendations"`
	Reviews         struct {
		ContainerID int `json:"containerID"`
		Items       []struct {
			ReviewStatus    int    `json:"reviewStatus"`
			ReviewStatusStr string `json:"reviewStatusStr"`
			ContainerID     int    `json:"containerID"`
			ReviewID        int    `json:"reviewID"`
			ReviewerName    string `json:"reviewerName"`
			ReviewDate      string `json:"reviewDate"`
			Review          string `json:"review"`
		} `json:"items"`
		IsMoreRecords bool `json:"isMoreRecords"`
		TotalPages    int  `json:"totalPages"`
		TotalRecords  int  `json:"totalRecords"`
		NumPerPage    int  `json:"numPerPage"`
		PageNum       int  `json:"pageNum"`
	} `json:"reviews"`
	Notes []struct {
		NoteID int    `json:"noteID"`
		Note   string `json:"note"`
	} `json:"notes"`
	CategoryID       int    `json:"categoryID"`
	Labels           any    `json:"labels"`
	PrevContainerID  int    `json:"prevContainerID"`
	NextContainerID  int    `json:"nextContainerID"`
	PrevContainerURL string `json:"prevContainerURL"`
	NextContainerURL string `json:"nextContainerURL"`
	VolumeName       string `json:"volumeName"`
	CdArtWorkList    []struct {
		DiscNumber     int    `json:"discNumber"`
		ArtWorkType    int    `json:"artWorkType"`
		ArtWorkTypeStr string `json:"artWorkTypeStr"`
		TemplateType   int    `json:"templateType"`
		ArtWorkPath    string `json:"artWorkPath"`
	} `json:"cdArtWorkList"`
	ContainerGroups         any    `json:"containerGroups"`
	VideoURL                any    `json:"videoURL"`
	VideoImage              any    `json:"videoImage"`
	VideoTitle              any    `json:"videoTitle"`
	VideoDesc               any    `json:"videoDesc"`
	VodPlayerImage          string `json:"vodPlayerImage"`
	IsInSubscriptionProgram bool   `json:"isInSubscriptionProgram"`
	SvodskuID               int    `json:"svodskuID"`
	LicensorName            string `json:"licensorName"`
	AffID                   int    `json:"affID"`
	PageURL                 string `json:"pageURL"`
	CoverImage              any    `json:"coverImage"`
	VenueName               string `json:"venueName"`
	VenueCity               string `json:"venueCity"`
	VenueState              string `json:"venueState"`
	ArtistName              string `json:"artistName"`
	AccessList              []any  `json:"accessList"`
	AvailabilityType        int    `json:"availabilityType"`
	AvailabilityTypeStr     string `json:"availabilityTypeStr"`
	Venue                   string `json:"venue"`
	Img                     struct {
		PicID   int    `json:"picID"`
		OrderID int    `json:"orderID"`
		Height  int    `json:"height"`
		Width   int    `json:"width"`
		Caption string `json:"caption"`
		URL     string `json:"url"`
	} `json:"img"`
	ContainerID                   int                  `json:"containerID"`
	ContainerInfo                 string               `json:"containerInfo"`
	PerformanceDate               string               `json:"performanceDate"`
	PerformanceDateFormatted      string               `json:"performanceDateFormatted"`
	PerformanceDateYear           string               `json:"performanceDateYear"`
	PerformanceDateShort          string               `json:"performanceDateShort"`
	PerformanceDateShortYearFirst string               `json:"performanceDateShortYearFirst"`
	PerformanceDateAbbr           string               `json:"performanceDateAbbr"`
	SongList                      any                  `json:"songList"`
	ReleaseDate                   any                  `json:"releaseDate"`
	ReleaseDateFormatted          string               `json:"releaseDateFormatted"`
	ActiveState                   string               `json:"activeState"`
	ContainerType                 int                  `json:"containerType"`
	ContainerTypeStr              string               `json:"containerTypeStr"`
	Songs                         []Track              `json:"songs"`
	SalesLast30                   int                  `json:"salesLast30"`
	SalesAllTime                  int                  `json:"salesAllTime"`
	DateCreated                   string               `json:"dateCreated"`
	EpochDateCreated              float64              `json:"epochDateCreated"`
	ProductFormatList             []*ProductFormatList `json:"productFormatList"`
	ContainsPreviewVideo          int                  `json:"containsPreviewVideo"`
	ArtistID                      int                  `json:"artistID"`
	ContainerCategoryID           int                  `json:"containerCategoryID"`
	ContainerCategoryName         any                  `json:"containerCategoryName"`
	ContainerCode                 string               `json:"containerCode"`
	ContainerIDExt                any                  `json:"containerIDExt"`
	ExtImage                      string               `json:"extImage"`
	VideoChapters                 []any                `json:"videoChapters"`
}

type AlbumMeta struct {
	MethodName                  string      `json:"methodName"`
	ResponseAvailabilityCode    int         `json:"responseAvailabilityCode"`
	ResponseAvailabilityCodeStr string      `json:"responseAvailabilityCodeStr"`
	Response                    *AlbArtResp `json:"Response"`
}

type Token struct {
	MethodName string `json:"methodName"`
	Response   struct {
		TokenValue     string `json:"tokenValue"`
		ReturnCode     int    `json:"returnCode"`
		ReturnCodeStr  string `json:"returnCodeStr"`
		NnCustomerAuth any    `json:"nnCustomerAuth"`
	} `json:"Response"`
	ResponseAvailabilityCode    int    `json:"responseAvailabilityCode"`
	ResponseAvailabilityCodeStr string `json:"responseAvailabilityCodeStr"`
	SessionState                int    `json:"sessionState"`
	SessionStateStr             string `json:"sessionStateStr"`
}

type PlistMeta struct {
	MethodName string `json:"methodName"`
	Response   struct {
		TotalRunningTime       int    `json:"totalRunningTime"`
		HhmmssTotalRunningTime string `json:"hhmmssTotalRunningTime"`
		ID                     int    `json:"ID"`
		UserID                 int    `json:"userID"`
		Items                  []struct {
			ID                int   `json:"ID"`
			OrderID           int   `json:"orderID"`
			Track             Track `json:"track"`
			PlaylistContainer struct {
				TotalRunningTime       int `json:"totalRunningTime"`
				HhmmssTotalRunningTime any `json:"hhmmssTotalRunningTime"`
				Img                    struct {
					PicID   int    `json:"picID"`
					OrderID int    `json:"orderID"`
					Height  int    `json:"height"`
					Width   int    `json:"width"`
					Caption string `json:"caption"`
					URL     string `json:"url"`
				} `json:"img"`
				ContainerInfo          string    `json:"containerInfo"`
				Products               []Product `json:"products"`
				VenueName              string    `json:"venueName"`
				VenueCity              string    `json:"venueCity"`
				VenueState             string    `json:"venueState"`
				ArtistName             string    `json:"artistName"`
				Venue                  string    `json:"venue"`
				ContainerID            int       `json:"containerID"`
				PerformanceDate        string    `json:"performanceDate"`
				ReleaseDate            any       `json:"releaseDate"`
				ContainerType          int       `json:"containerType"`
				ArtistID               int       `json:"artistID"`
				TitleType              int       `json:"titleType"`
				StrTotalRunningTime    string    `json:"strTotalRunningTime"`
				ContainerCategoryID    int       `json:"containerCategoryID"`
				ContainerCategoryName  any       `json:"containerCategoryName"`
				ContainerCategoryOrder int       `json:"containerCategoryOrder"`
				Availability           int       `json:"availability"`
				TicketImage            any       `json:"ticketImage"`
				UnavailableNote        any       `json:"unavailableNote"`
				Numasterisks           string    `json:"numasterisks"`
				CoverImage             any       `json:"coverImage"`
			} `json:"playlistContainer"`
		} `json:"items"`
		CreateDate          any    `json:"createDate"`
		PlayListName        string `json:"playListName"`
		AlreadyExistsFlag   bool   `json:"alreadyExistsFlag"`
		PlayListUserInvalid bool   `json:"playListUserInvalid"`
		PlaylistImage       any    `json:"playlistImage"`
		NumTracks           int    `json:"numTracks"`
		GeneratedGUID       any    `json:"generatedGUID"`
		ShortenedLink       any    `json:"shortenedLink"`
	} `json:"Response"`
	ResponseAvailabilityCode    int    `json:"responseAvailabilityCode"`
	ResponseAvailabilityCodeStr string `json:"responseAvailabilityCodeStr"`
	SessionState                int    `json:"sessionState"`
	SessionStateStr             string `json:"sessionStateStr"`
}

type Track struct {
	AccessList             []any     `json:"accessList"`
	HhmmssTotalRunningTime string    `json:"hhmmssTotalRunningTime"`
	TrackLabel             string    `json:"trackLabel"`
	TrackURL               string    `json:"trackURL"`
	SongID                 int       `json:"songID"`
	SongTitle              string    `json:"songTitle"`
	TotalRunningTime       int       `json:"totalRunningTime"`
	DiscNum                int       `json:"discNum"`
	TrackNum               int       `json:"trackNum"`
	SetNum                 int       `json:"setNum"`
	ClipURL                string    `json:"clipURL"`
	TrackID                int       `json:"trackID"`
	TrackExclude           int       `json:"trackExclude"`
	Rootpath               any       `json:"rootpath"`
	SourcePath             any       `json:"sourcePath"`
	SourceFilename         any       `json:"sourceFilename"`
	SourceFilePath         any       `json:"sourceFilePath"`
	RootPathReal           any       `json:"rootPathReal"`
	SourceFilePathReal     any       `json:"sourceFilePathReal"`
	SkuIDExt               any       `json:"skuIDExt"`
	TransportMethod        string    `json:"transportMethod"`
	StrTotalRunningTime    any       `json:"strTotalRunningTime"`
	Products               []Product `json:"products"`
	Subscriptions          any       `json:"subscriptions"`
	AudioProduct           any       `json:"audioProduct"`
	AudioLosslessProduct   any       `json:"audioLosslessProduct"`
	AudioHDProduct         any       `json:"audioHDProduct"`
	VideoProduct           any       `json:"videoProduct"`
	LivestreamProduct      any       `json:"livestreamProduct"`
	Mp4Product             any       `json:"mp4Product"`
	VideoondemandProduct   any       `json:"videoondemandProduct"`
	CdProduct              any       `json:"cdProduct"`
	LiveHDstreamProduct    any       `json:"liveHDstreamProduct"`
	HDvideoondemandProduct any       `json:"HDvideoondemandProduct"`
	VinylProduct           any       `json:"vinylProduct"`
	DsdProduct             any       `json:"dsdProduct"`
	DvdProduct             any       `json:"dvdProduct"`
	Reality360Product      any       `json:"reality360Product"`
	ContainerGroups        any       `json:"containerGroups"`
	IDList                 string    `json:"IDList"`
	PlayListID             int       `json:"playListID"`
	CatalogQueryType       int       `json:"catalogQueryType"`
}

type StreamMeta struct {
	StreamLink         string `json:"streamLink"`
	Streamer           string `json:"streamer"`
	UserID             string `json:"userID"`
	Mason              any    `json:"mason"`
	SubContentAccess   int    `json:"subContentAccess"`
	StashContentAccess int    `json:"stashContentAccess"`
}

type Quality struct {
	Specs     string
	Extension string
	URL       string
	Format    int
}

type ArtistMeta struct {
	MethodName                  string `json:"methodName"`
	ResponseAvailabilityCode    int    `json:"responseAvailabilityCode"`
	ResponseAvailabilityCodeStr string `json:"responseAvailabilityCodeStr"`
	Response                    struct {
		HeaderName          any           `json:"headerName"`
		Packages            any           `json:"packages"`
		Containers          []*AlbArtResp `json:"containers"`
		CategoryID          int           `json:"categoryID"`
		ArtistID            int           `json:"artistID"`
		ArtistName          any           `json:"artistName"`
		LoadingState        int           `json:"loadingState"`
		TotalMatchedRecords int           `json:"totalMatchedRecords"`
		NnCheckSum          int           `json:"nnCheckSum"`
	} `json:"Response"`
}

// Artist represents an artist with their metadata
type Artist struct {
	ArtistID   int    `json:"artistID"`
	ArtistName string `json:"artistName"`
	NumShows   int    `json:"numShows"`
	NumAlbums  int    `json:"numAlbums"`
}

type ArtistListResp struct {
	MethodName                  string `json:"methodName"`
	ResponseAvailabilityCode    int    `json:"responseAvailabilityCode"`
	ResponseAvailabilityCodeStr string `json:"responseAvailabilityCodeStr"`
	Response                    struct {
		Artists []Artist `json:"artists"`
	} `json:"Response"`
}

type PurchasedManResp struct {
	FileURL      string `json:"fileURL"`
	ResponseCode int    `json:"responseCode"`
}

// ArtistListOutput represents JSON output for list artists command
type ArtistListOutput struct {
	Artists []ArtistOutput `json:"artists"`
	Total   int            `json:"total"`
}

type ArtistOutput struct {
	ArtistID   int    `json:"artistID"`
	ArtistName string `json:"artistName"`
	NumShows   int    `json:"numShows"`
	NumAlbums  int    `json:"numAlbums"`
}

// ShowListOutput represents JSON output for list shows command
type ShowListOutput struct {
	ArtistID   int          `json:"artistID"`
	ArtistName string       `json:"artistName"`
	Shows      []ShowOutput `json:"shows"`
	Total      int          `json:"total"`
}

type ShowOutput struct {
	ContainerID int    `json:"containerID"`
	Date        string `json:"date"`
	Title       string `json:"title"`
	Venue       string `json:"venue"`
	VenueCity   string `json:"venueCity,omitempty"`
	VenueState  string `json:"venueState,omitempty"`
}

// LatestCatalogResp represents the response from catalog.latest
type LatestCatalogResp struct {
	MethodName                  string `json:"methodName"`
	ResponseAvailabilityCode    int    `json:"responseAvailabilityCode"`
	ResponseAvailabilityCodeStr string `json:"responseAvailabilityCodeStr"`
	Response                    struct {
		RecentItems []struct {
			ContainerInfo          string `json:"containerInfo"`
			ArtistName             string `json:"artistName"`
			ShowDateFormattedShort string `json:"showDateFormattedShort"`
			ArtistID               int    `json:"artistID"`
			ContainerID            int    `json:"containerID"`
			PerformanceDateStr     string `json:"performanceDateStr"`
			PostedDate             string `json:"postedDate"`
			VenueCity              string `json:"venueCity"`
			VenueState             string `json:"venueState"`
			Venue                  string `json:"venue"`
			PageURL                string `json:"pageURL"`
			CategoryID             int    `json:"categoryID"`
			ImageURL               string `json:"imageURL"`
		} `json:"recentItems"`
	} `json:"Response"`
}

// CacheMeta stores metadata about the cached catalog
type CacheMeta struct {
	LastUpdated    time.Time `json:"lastUpdated"`  // RFC3339
	CacheVersion   string    `json:"cacheVersion"` // "v1.0.0"
	TotalShows     int       `json:"totalShows"`
	TotalArtists   int       `json:"totalArtists"`
	ApiMethod      string    `json:"apiMethod"`      // "catalog.latest"
	UpdateDuration string    `json:"updateDuration"` // e.g. "2.5s"
}

// ArtistsIndex provides fast artist name → ID lookup
type ArtistsIndex struct {
	Index map[string]int `json:"index"` // "grateful dead" → 461
}

// ContainersIndex maps container IDs to artist information
type ContainersIndex struct {
	Containers map[int]ContainerIndexEntry `json:"containers"`
}

// ContainerIndexEntry stores basic show info for gap detection
type ContainerIndexEntry struct {
	ArtistID        int    `json:"artistID"`
	ArtistName      string `json:"artistName"`
	ContainerInfo   string `json:"containerInfo"`
	PerformanceDate string `json:"performanceDate"`
}

// ArtistMetaCache stores cached artist metadata pages from catalog.containersAll.
type ArtistMetaCache struct {
	ArtistID string        `json:"artistID"`
	CachedAt time.Time     `json:"cachedAt"`
	Pages    []*ArtistMeta `json:"pages"`
}

// ShowStatus stores a show and whether it is already downloaded.
type ShowStatus struct {
	Show       *AlbArtResp `json:"show"`
	Downloaded bool        `json:"downloaded"`
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
}
