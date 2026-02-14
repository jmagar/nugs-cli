package model

// Auth holds OAuth token response fields.
type Auth struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// Payload represents a JWT payload.
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

// UserInfo holds user profile information.
type UserInfo struct {
	Sub               string `json:"sub"`
	PreferredUsername string `json:"preferred_username"`
	Name              string `json:"name"`
	Email             string `json:"email"`
	EmailVerified     bool   `json:"email_verified"`
}

// SubInfo holds subscription information.
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

// StreamParams holds streaming session parameters.
type StreamParams struct {
	SubscriptionID          string
	SubCostplanIDAccessList string
	UserID                  string
	StartStamp              string
	EndStamp                string
}

// Product represents a purchasable product/format for a show.
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

// ProductFormatList describes a specific format for a product.
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

// AlbArtResp represents an album/show response from the API.
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

// AlbumMeta wraps the API response for a single album.
type AlbumMeta struct {
	MethodName                  string      `json:"methodName"`
	ResponseAvailabilityCode    int         `json:"responseAvailabilityCode"`
	ResponseAvailabilityCodeStr string      `json:"responseAvailabilityCodeStr"`
	Response                    *AlbArtResp `json:"Response"`
}

// Token represents an API token response.
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

// PlistMeta represents a playlist response.
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

// Track represents a single track in an album or playlist.
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

// StreamMeta holds stream URL and access metadata.
type StreamMeta struct {
	StreamLink         string `json:"streamLink"`
	Streamer           string `json:"streamer"`
	UserID             string `json:"userID"`
	Mason              any    `json:"mason"`
	SubContentAccess   int    `json:"subContentAccess"`
	StashContentAccess int    `json:"stashContentAccess"`
}

// Quality describes an audio quality level.
type Quality struct {
	Specs     string
	Extension string
	URL       string
	Format    int
}

// ArtistMeta holds the API response for an artist's catalog.
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

// Artist represents an artist with their metadata.
type Artist struct {
	ArtistID   int    `json:"artistID"`
	ArtistName string `json:"artistName"`
	NumShows   int    `json:"numShows"`
	NumAlbums  int    `json:"numAlbums"`
}

// ArtistListResp holds the API response for listing artists.
type ArtistListResp struct {
	MethodName                  string `json:"methodName"`
	ResponseAvailabilityCode    int    `json:"responseAvailabilityCode"`
	ResponseAvailabilityCodeStr string `json:"responseAvailabilityCodeStr"`
	Response                    struct {
		Artists []Artist `json:"artists"`
	} `json:"Response"`
}

// PurchasedManResp represents a purchased manifest response.
type PurchasedManResp struct {
	FileURL      string `json:"fileURL"`
	ResponseCode int    `json:"responseCode"`
}

// ArtistListOutput represents JSON output for list artists command.
type ArtistListOutput struct {
	Artists []ArtistOutput `json:"artists"`
	Total   int            `json:"total"`
}

// ArtistOutput represents a single artist in JSON output.
type ArtistOutput struct {
	ArtistID   int    `json:"artistID"`
	ArtistName string `json:"artistName"`
	NumShows   int    `json:"numShows"`
	NumAlbums  int    `json:"numAlbums"`
}

// ShowListOutput represents JSON output for list shows command.
type ShowListOutput struct {
	ArtistID   int          `json:"artistID"`
	ArtistName string       `json:"artistName"`
	Shows      []ShowOutput `json:"shows"`
	Total      int          `json:"total"`
}

// ShowOutput represents a single show in JSON output.
type ShowOutput struct {
	ContainerID int    `json:"containerID"`
	Date        string `json:"date"`
	Title       string `json:"title"`
	Venue       string `json:"venue"`
	VenueCity   string `json:"venueCity,omitempty"`
	VenueState  string `json:"venueState,omitempty"`
}
