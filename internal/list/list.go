package list

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jmagar/nugs-cli/internal/api"
	"github.com/jmagar/nugs-cli/internal/model"
	"github.com/jmagar/nugs-cli/internal/ui"
)

var showFilterRegex = regexp.MustCompile(`^(>=|<=|>|<|=)\s*(\d+)$`)

// ParseShowFilter parses a filter expression like "shows >100" into operator and value.
func ParseShowFilter(filter string) (string, int, error) {
	re := showFilterRegex
	matches := re.FindStringSubmatch(filter)
	if matches == nil {
		return "", 0, fmt.Errorf("invalid filter format: %s (expected: >N, <N, >=N, <=N, or =N)", filter)
	}

	operator := matches[1]
	value, err := strconv.Atoi(matches[2])
	if err != nil {
		return "", 0, fmt.Errorf("invalid number in filter: %s", matches[2])
	}

	return operator, value, nil
}

// ApplyShowFilter filters artists based on show count operator and value.
func ApplyShowFilter(artists []model.Artist, operator string, value int) []model.Artist {
	var filtered []model.Artist
	for _, artist := range artists {
		include := false
		switch operator {
		case ">":
			include = artist.NumShows > value
		case "<":
			include = artist.NumShows < value
		case ">=":
			include = artist.NumShows >= value
		case "<=":
			include = artist.NumShows <= value
		case "=":
			include = artist.NumShows == value
		}
		if include {
			filtered = append(filtered, artist)
		}
	}
	return filtered
}

// ListArtists fetches and displays a formatted list of all artists.
func ListArtists(ctx context.Context, jsonLevel string, showFilter string) error {
	if jsonLevel == "" {
		ui.PrintInfo("Fetching artist catalog...")
	}
	artistList, err := api.GetArtistList(ctx)
	if err != nil {
		ui.PrintError("Failed to get artist list")
		return err
	}

	artists := artistList.Response.Artists
	var filterOperator string
	var filterValue int
	if showFilter != "" {
		filterOperator, filterValue, err = ParseShowFilter(showFilter)
		if err != nil {
			return err
		}
		artists = ApplyShowFilter(artists, filterOperator, filterValue)
	}
	if len(artists) == 0 {
		if jsonLevel != "" {
			emptyOutput := model.ArtistListOutput{Artists: []model.ArtistOutput{}, Total: 0}
			jsonData, err := json.MarshalIndent(emptyOutput, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal empty output: %w", err)
			}
			fmt.Println(string(jsonData))
		} else {
			if showFilter != "" {
				ui.PrintWarning(fmt.Sprintf("No artists found with shows %s%d", filterOperator, filterValue))
			} else {
				ui.PrintWarning("No artists found")
			}
		}
		return nil
	}

	if jsonLevel != "" {
		if jsonLevel == model.JSONLevelRaw {
			if showFilter != "" {
				artistList.Response.Artists = artists
			}
			jsonData, err := json.MarshalIndent(artistList, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Println(string(jsonData))
			return nil
		}

		sort.Slice(artists, func(i, j int) bool {
			return strings.ToLower(artists[i].ArtistName) < strings.ToLower(artists[j].ArtistName)
		})

		output := model.ArtistListOutput{
			Artists: make([]model.ArtistOutput, len(artists)),
			Total:   len(artists),
		}
		for i, artist := range artists {
			output.Artists[i] = model.ArtistOutput{
				ArtistID:   artist.ArtistID,
				ArtistName: artist.ArtistName,
				NumShows:   artist.NumShows,
				NumAlbums:  artist.NumAlbums,
			}
		}
		jsonData, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(jsonData))
	} else {
		sort.Slice(artists, func(i, j int) bool {
			return strings.ToLower(artists[i].ArtistName) < strings.ToLower(artists[j].ArtistName)
		})

		if showFilter != "" {
			ui.PrintSection(fmt.Sprintf("Found %d artists with shows %s%d", len(artists), filterOperator, filterValue))
		} else {
			ui.PrintSection(fmt.Sprintf("Found %d artists", len(artists)))
		}

		table := ui.NewTable([]ui.TableColumn{
			{Header: "ID", Width: 8, Align: "left"},
			{Header: "Name", Width: 55, Align: "left"},
			{Header: "Shows", Width: 10, Align: "right"},
			{Header: "Albums", Width: 10, Align: "right"},
		})

		for _, artist := range artists {
			table.AddRow(
				strconv.Itoa(artist.ArtistID),
				artist.ArtistName,
				strconv.Itoa(artist.NumShows),
				strconv.Itoa(artist.NumAlbums),
			)
		}

		table.Print()
		ui.PrintInfo("To list shows for an artist, use: nugs list <artist_id>")
	}
	return nil
}

// DisplayWelcome shows a welcome screen with latest shows from the catalog.
func DisplayWelcome(ctx context.Context) error {
	ui.PrintHeader("Welcome to Nugs Downloader")

	catalog, err := api.GetLatestCatalog(ctx)
	if err != nil {
		ui.PrintWarning(fmt.Sprintf("Unable to fetch latest shows: %v", err))
		fmt.Println()
		return err
	}

	if len(catalog.Response.RecentItems) == 0 {
		ui.PrintWarning("No recent shows available")
		fmt.Println()
		return nil
	}

	ui.PrintSection("Latest Additions to Catalog")

	showCount := min(15, len(catalog.Response.RecentItems))

	table := ui.NewTable([]ui.TableColumn{
		{Header: "Artist ID", Width: 10, Align: "right"},
		{Header: "Artist", Width: 25, Align: "left"},
		{Header: "Date", Width: 12, Align: "left"},
		{Header: "Title", Width: 40, Align: "left"},
		{Header: "Venue", Width: 25, Align: "left"},
	})

	for i := range showCount {
		item := catalog.Response.RecentItems[i]

		location := item.Venue
		if item.VenueCity != "" {
			location = fmt.Sprintf("%s, %s", item.VenueCity, item.VenueState)
		}

		table.AddRow(
			fmt.Sprintf("%s%d%s", ui.ColorPurple, item.ArtistID, ui.ColorReset),
			fmt.Sprintf("%s%s%s", ui.ColorGreen, item.ArtistName, ui.ColorReset),
			fmt.Sprintf("%s%s%s", ui.ColorYellow, item.ShowDateFormattedShort, ui.ColorReset),
			fmt.Sprintf("%s%s%s", ui.ColorCyan, item.ContainerInfo, ui.ColorReset),
			location,
		)
	}

	table.Print()
	fmt.Println()

	ui.PrintSection("Quick Start")
	quickStartCommands := []string{
		fmt.Sprintf("%snugs list%s - Browse all artists", ui.ColorCyan, ui.ColorReset),
		fmt.Sprintf("%snugs list 1125%s - View Billy Strings shows", ui.ColorCyan, ui.ColorReset),
		fmt.Sprintf("%snugs list 1125 \"Red Rocks\"%s - Filter by venue", ui.ColorCyan, ui.ColorReset),
		fmt.Sprintf("%snugs list \">100\"%s - Filter artists by show count", ui.ColorCyan, ui.ColorReset),
		fmt.Sprintf("%snugs grab 1125 latest%s - Download latest shows", ui.ColorCyan, ui.ColorReset),
		fmt.Sprintf("%snugs list artists --json standard | jq%s - Export artist list as JSON", ui.ColorCyan, ui.ColorReset),
		fmt.Sprintf("%snugs gaps 1125 fill%s - Fill missing shows", ui.ColorCyan, ui.ColorReset),
		fmt.Sprintf("%snugs completion bash%s - Generate shell completions", ui.ColorCyan, ui.ColorReset),
		fmt.Sprintf("%snugs help%s - View all commands", ui.ColorCyan, ui.ColorReset),
	}
	ui.PrintList(quickStartCommands, ui.ColorGreen)

	ui.PrintSection("Catalog Workflow")
	catalogWorkflowCommands := []string{
		fmt.Sprintf("%snugs update%s - Refresh local catalog cache", ui.ColorCyan, ui.ColorReset),
		fmt.Sprintf("%snugs cache%s - Inspect cache status and metadata", ui.ColorCyan, ui.ColorReset),
		fmt.Sprintf("%snugs stats%s - See catalog-wide statistics", ui.ColorCyan, ui.ColorReset),
		fmt.Sprintf("%snugs latest 10%s - Show most recent additions", ui.ColorCyan, ui.ColorReset),
		fmt.Sprintf("%snugs coverage 1125 461%s - Check collection coverage", ui.ColorCyan, ui.ColorReset),
		fmt.Sprintf("%snugs refresh set%s - Configure auto-refresh schedule", ui.ColorCyan, ui.ColorReset),
	}
	ui.PrintList(catalogWorkflowCommands, ui.ColorGreen)
	fmt.Println()
	ui.PrintInfo("Tip: Quote comparison filters (for example, \">100\") to avoid shell redirection.")
	fmt.Println()

	return nil
}

// ListArtistShows fetches and displays all shows for a specific artist.
func ListArtistShows(ctx context.Context, artistId string, jsonLevel string, deps *Deps, mediaFilter ...model.MediaType) error {
	mf := model.MediaTypeUnknown
	if len(mediaFilter) > 0 {
		mf = mediaFilter[0]
	}
	if jsonLevel == "" {
		ui.PrintInfo("Fetching artist shows...")
	}
	availType := 1
	if mf == model.MediaTypeVideo || mf == model.MediaTypeBoth {
		availType = 2
	}
	allMeta, err := api.GetArtistMetaWithAvailType(ctx, artistId, availType)
	if err != nil {
		ui.PrintError("Failed to get artist metadata")
		return err
	}

	if len(allMeta) == 0 {
		ui.PrintWarning("No metadata found for this artist")
		return nil
	}

	artistName := "Unknown Artist"
	if len(allMeta) > 0 && len(allMeta[0].Response.Containers) > 0 {
		artistName = allMeta[0].Response.Containers[0].ArtistName
	}

	var allContainers []model.ContainerWithDate

	for _, meta := range allMeta {
		for _, container := range meta.Response.Containers {
			var showMedia model.MediaType
			if deps.GetShowMediaType != nil {
				showMedia = deps.GetShowMediaType(container)
			} else {
				showMedia = model.MediaTypeAudio
			}

			if mf != model.MediaTypeUnknown {
				if deps.MatchesMediaFilter != nil {
					if !deps.MatchesMediaFilter(showMedia, mf) {
						continue
					}
				}
			}

			dateStr := container.PerformanceDateShortYearFirst
			if dateStr == "" {
				dateStr = container.PerformanceDate
			}
			allContainers = append(allContainers, model.ContainerWithDate{
				Container: container,
				DateStr:   dateStr,
				MediaType: showMedia,
			})
		}
	}

	if len(allContainers) == 0 {
		if jsonLevel != "" {
			artistIdInt, _ := strconv.Atoi(artistId)
			emptyOutput := model.ShowListOutput{
				ArtistID:   artistIdInt,
				ArtistName: artistName,
				Shows:      []model.ShowOutput{},
				Total:      0,
			}
			jsonData, err := json.MarshalIndent(emptyOutput, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal empty output: %w", err)
			}
			fmt.Println(string(jsonData))
		} else {
			if mf != model.MediaTypeUnknown {
				ui.PrintWarning(fmt.Sprintf("No %s shows found for %s", mf, artistName))
			} else {
				ui.PrintWarning(fmt.Sprintf("No shows found for %s", artistName))
			}
		}
		return nil
	}

	sort.Slice(allContainers, func(i, j int) bool {
		dateI := allContainers[i].DateStr
		dateJ := allContainers[j].DateStr
		if dateI == "" && dateJ != "" {
			return false
		}
		if dateI != "" && dateJ == "" {
			return true
		}
		if dateI == "" && dateJ == "" {
			return false
		}
		return dateI > dateJ
	})

	if jsonLevel != "" {
		if jsonLevel == model.JSONLevelRaw {
			jsonData, err := json.MarshalIndent(allMeta, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Println(string(jsonData))
			return nil
		}

		artistIdInt, _ := strconv.Atoi(artistId)

		if jsonLevel == model.JSONLevelExtended {
			shows := make([]*model.AlbArtResp, len(allContainers))
			for i, item := range allContainers {
				shows[i] = item.Container
			}
			output := map[string]any{
				"artistID":   artistIdInt,
				"artistName": artistName,
				"shows":      shows,
				"total":      len(allContainers),
			}
			jsonData, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Println(string(jsonData))
		} else {
			output := model.ShowListOutput{
				ArtistID:   artistIdInt,
				ArtistName: artistName,
				Shows:      make([]model.ShowOutput, len(allContainers)),
				Total:      len(allContainers),
			}

			for i, item := range allContainers {
				show := model.ShowOutput{
					ContainerID: item.Container.ContainerID,
					Date:        item.DateStr,
					Title:       item.Container.ContainerInfo,
					Venue:       item.Container.VenueName,
				}

				if jsonLevel == model.JSONLevelStandard {
					show.VenueCity = item.Container.VenueCity
					show.VenueState = item.Container.VenueState
				}

				output.Shows[i] = show
			}

			jsonData, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Println(string(jsonData))
		}
	} else {
		filterLabel := ""
		if mf != model.MediaTypeUnknown {
			filterLabel = fmt.Sprintf(" (%s)", mf)
		}
		ui.PrintSection(fmt.Sprintf("%s - %d shows%s", artistName, len(allContainers), filterLabel))

		table := ui.NewTable([]ui.TableColumn{
			{Header: "Type", Width: 6, Align: "center"},
			{Header: "ID", Width: 10, Align: "left"},
			{Header: "Date", Width: 12, Align: "left"},
			{Header: "Title", Width: 42, Align: "left"},
			{Header: "Venue", Width: 25, Align: "left"},
		})

		for _, item := range allContainers {
			container := item.Container
			mediaIndicator := ui.GetMediaTypeIndicator(item.MediaType)
			table.AddRow(
				mediaIndicator,
				strconv.Itoa(container.ContainerID),
				item.DateStr,
				container.ContainerInfo,
				container.VenueName,
			)
		}

		table.Print()
		fmt.Printf("\n%sLegend:%s %s Audio  %s Video  %s Both\n",
			ui.ColorCyan, ui.ColorReset, ui.SymbolAudio, ui.SymbolVideo, ui.SymbolBoth)
		ui.PrintInfo("To download a show, use: nugs <container_id>")
	}
	return nil
}

// ListArtistShowsByVenue filters artist shows by venue name.
func ListArtistShowsByVenue(ctx context.Context, artistId string, venueFilter string, jsonLevel string) error {
	if _, err := strconv.Atoi(artistId); err != nil {
		return fmt.Errorf("invalid artist ID: %s (must be numeric)", artistId)
	}

	if jsonLevel == "" {
		fmt.Printf("Fetching shows at venues matching \"%s\"...\n", venueFilter)
	}

	allMeta, err := api.GetArtistMeta(ctx, artistId)
	if err != nil {
		ui.PrintError("Failed to get artist metadata")
		return err
	}

	if len(allMeta) == 0 {
		ui.PrintWarning("No metadata found for this artist")
		return nil
	}

	artistName := "Unknown Artist"
	if len(allMeta[0].Response.Containers) > 0 {
		artistName = allMeta[0].Response.Containers[0].ArtistName
	}

	var filteredContainers []model.ContainerWithDate
	venueFilterLower := strings.ToLower(venueFilter)

	for _, meta := range allMeta {
		for _, container := range meta.Response.Containers {
			venueNameLower := strings.ToLower(container.VenueName)
			venueLower := strings.ToLower(container.Venue)
			if strings.Contains(venueNameLower, venueFilterLower) || strings.Contains(venueLower, venueFilterLower) {
				dateStr := container.PerformanceDateShortYearFirst
				if dateStr == "" {
					dateStr = container.PerformanceDate
				}
				filteredContainers = append(filteredContainers, model.ContainerWithDate{
					Container: container,
					DateStr:   dateStr,
					MediaType: model.MediaTypeUnknown,
				})
			}
		}
	}

	if len(filteredContainers) == 0 {
		if jsonLevel != "" {
			artistIdInt, _ := strconv.Atoi(artistId)
			emptyOutput := model.ShowListOutput{
				ArtistID:   artistIdInt,
				ArtistName: artistName,
				Shows:      []model.ShowOutput{},
				Total:      0,
			}
			jsonData, err := json.MarshalIndent(emptyOutput, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal empty output: %w", err)
			}
			fmt.Println(string(jsonData))
		} else {
			ui.PrintWarning(fmt.Sprintf("No shows found for %s at venues matching \"%s\"", artistName, venueFilter))
		}
		return nil
	}

	sort.Slice(filteredContainers, func(i, j int) bool {
		dateI := filteredContainers[i].DateStr
		dateJ := filteredContainers[j].DateStr
		if dateI == "" && dateJ != "" {
			return false
		}
		if dateI != "" && dateJ == "" {
			return true
		}
		if dateI == "" && dateJ == "" {
			return false
		}
		return dateI > dateJ
	})

	if jsonLevel != "" {
		artistIdInt, _ := strconv.Atoi(artistId)

		if jsonLevel == model.JSONLevelExtended || jsonLevel == model.JSONLevelRaw {
			shows := make([]*model.AlbArtResp, len(filteredContainers))
			for i, item := range filteredContainers {
				shows[i] = item.Container
			}
			output := map[string]any{
				"artistID":    artistIdInt,
				"artistName":  artistName,
				"venueFilter": venueFilter,
				"shows":       shows,
				"total":       len(filteredContainers),
			}
			jsonData, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Println(string(jsonData))
		} else {
			output := model.ShowListOutput{
				ArtistID:   artistIdInt,
				ArtistName: artistName,
				Shows:      make([]model.ShowOutput, len(filteredContainers)),
				Total:      len(filteredContainers),
			}

			for i, item := range filteredContainers {
				show := model.ShowOutput{
					ContainerID: item.Container.ContainerID,
					Date:        item.DateStr,
					Title:       item.Container.ContainerInfo,
					Venue:       item.Container.VenueName,
				}

				if jsonLevel == model.JSONLevelStandard {
					show.VenueCity = item.Container.VenueCity
					show.VenueState = item.Container.VenueState
				}

				output.Shows[i] = show
			}

			jsonData, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Println(string(jsonData))
		}
	} else {
		ui.PrintSection(fmt.Sprintf("%s - Shows at \"%s\" (%d shows)", artistName, venueFilter, len(filteredContainers)))

		table := ui.NewTable([]ui.TableColumn{
			{Header: "ID", Width: 10, Align: "left"},
			{Header: "Date", Width: 12, Align: "left"},
			{Header: "Title", Width: 45, Align: "left"},
			{Header: "Venue", Width: 30, Align: "left"},
		})

		for _, item := range filteredContainers {
			container := item.Container
			table.AddRow(
				strconv.Itoa(container.ContainerID),
				item.DateStr,
				container.ContainerInfo,
				container.VenueName,
			)
		}

		table.Print()
		ui.PrintInfo("To download a show, use: nugs <container_id>")
	}
	return nil
}

// ListArtistLatestShows displays the latest N shows for an artist.
func ListArtistLatestShows(ctx context.Context, artistId string, limit int, jsonLevel string) error {
	if jsonLevel == "" {
		fmt.Printf("Fetching latest %d shows...\n", limit)
	}

	allMeta, err := api.GetArtistMeta(ctx, artistId)
	if err != nil {
		ui.PrintError("Failed to get artist metadata")
		return err
	}

	if len(allMeta) == 0 {
		ui.PrintWarning("No metadata found for this artist")
		return nil
	}

	artistName := "Unknown Artist"
	if len(allMeta) > 0 && len(allMeta[0].Response.Containers) > 0 {
		artistName = allMeta[0].Response.Containers[0].ArtistName
	}

	var allContainers []model.ContainerWithDate

	for _, meta := range allMeta {
		for _, container := range meta.Response.Containers {
			dateStr := container.PerformanceDateShortYearFirst
			if dateStr == "" {
				dateStr = container.PerformanceDate
			}
			allContainers = append(allContainers, model.ContainerWithDate{
				Container: container,
				DateStr:   dateStr,
				MediaType: model.MediaTypeUnknown,
			})
		}
	}

	if len(allContainers) == 0 {
		if jsonLevel != "" {
			artistIdInt, _ := strconv.Atoi(artistId)
			emptyOutput := model.ShowListOutput{
				ArtistID:   artistIdInt,
				ArtistName: artistName,
				Shows:      []model.ShowOutput{},
				Total:      0,
			}
			jsonData, err := json.MarshalIndent(emptyOutput, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal empty output: %w", err)
			}
			fmt.Println(string(jsonData))
		} else {
			ui.PrintWarning(fmt.Sprintf("No shows found for %s", artistName))
		}
		return nil
	}

	sort.Slice(allContainers, func(i, j int) bool {
		dateI := allContainers[i].DateStr
		dateJ := allContainers[j].DateStr
		if dateI == "" && dateJ != "" {
			return false
		}
		if dateI != "" && dateJ == "" {
			return true
		}
		if dateI == "" && dateJ == "" {
			return false
		}
		return dateI > dateJ
	})

	if limit > len(allContainers) {
		limit = len(allContainers)
	}
	latestContainers := allContainers[:limit]

	if jsonLevel != "" {
		artistIdInt, _ := strconv.Atoi(artistId)

		if jsonLevel == model.JSONLevelExtended || jsonLevel == model.JSONLevelRaw {
			shows := make([]*model.AlbArtResp, len(latestContainers))
			for i, item := range latestContainers {
				shows[i] = item.Container
			}
			output := map[string]any{
				"artistID":   artistIdInt,
				"artistName": artistName,
				"limit":      limit,
				"shows":      shows,
				"total":      len(latestContainers),
			}
			jsonData, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Println(string(jsonData))
		} else {
			output := model.ShowListOutput{
				ArtistID:   artistIdInt,
				ArtistName: artistName,
				Shows:      make([]model.ShowOutput, len(latestContainers)),
				Total:      len(latestContainers),
			}

			for i, item := range latestContainers {
				show := model.ShowOutput{
					ContainerID: item.Container.ContainerID,
					Date:        item.DateStr,
					Title:       item.Container.ContainerInfo,
					Venue:       item.Container.VenueName,
				}

				if jsonLevel == model.JSONLevelStandard {
					show.VenueCity = item.Container.VenueCity
					show.VenueState = item.Container.VenueState
				}

				output.Shows[i] = show
			}

			jsonData, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Println(string(jsonData))
		}
	} else {
		ui.PrintHeader(fmt.Sprintf("%s - Latest %d Shows", artistName, len(latestContainers)))

		table := ui.NewTable([]ui.TableColumn{
			{Header: "ID", Width: 10, Align: "right"},
			{Header: "Date", Width: 12, Align: "left"},
			{Header: "Title", Width: 50, Align: "left"},
			{Header: "Venue", Width: 30, Align: "left"},
		})

		for _, item := range latestContainers {
			table.AddRow(
				fmt.Sprintf("%d", item.Container.ContainerID),
				item.DateStr,
				item.Container.ContainerInfo,
				item.Container.VenueName,
			)
		}

		table.Print()
		fmt.Printf("\n%s%s%s To download: %snugs <container_id>%s\n\n",
			ui.ColorCyan, ui.SymbolInfo, ui.ColorReset, ui.ColorBold, ui.ColorReset)
	}
	return nil
}

// ResolveCatPlistId resolves a catalog playlist URL to its GUID.
func ResolveCatPlistId(plistUrl string) (string, error) {
	req, err := api.Client.Get(plistUrl)
	if err != nil {
		return "", err
	}
	req.Body.Close()
	if req.StatusCode != http.StatusOK {
		return "", errors.New(req.Status)
	}
	location := req.Request.URL.String()
	u, err := url.Parse(location)
	if err != nil {
		return "", err
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return "", err
	}
	resolvedId := q.Get("plGUID")
	if resolvedId == "" {
		return "", errors.New("not a catalog playlist")
	}
	return resolvedId, nil
}

// CatalogPlist downloads a catalog playlist.
func CatalogPlist(ctx context.Context, plistId, legacyToken string, cfg *model.Config, streamParams *model.StreamParams, deps *Deps) error {
	resolvedId, err := ResolveCatPlistId(plistId)
	if err != nil {
		fmt.Println("Failed to resolve playlist ID.")
		return err
	}
	return deps.Playlist(ctx, resolvedId, legacyToken, cfg, streamParams, true)
}
