package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// parseShowFilter parses a filter expression like "shows >100" into operator and value
// Returns operator (">", "<", ">=", "<=", "="), value, and error
func parseShowFilter(filter string) (string, int, error) {
	// Match patterns: >N, <N, >=N, <=N, =N
	re := regexp.MustCompile(`^(>=|<=|>|<|=)\s*(\d+)$`)
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

// applyShowFilter filters artists based on show count operator and value
func applyShowFilter(artists []Artist, operator string, value int) []Artist {
	var filtered []Artist
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

// listArtists fetches and displays a formatted list of all artists available on Nugs.net.
// Supports filtering by show count with: shows >N, shows <N, shows >=N, shows <=N, shows =N
// The output includes artist ID, name, number of shows, and number of albums.
// Returns an error if the artist list cannot be fetched from the API.
func listArtists(jsonLevel string, showFilter string) error {
	if jsonLevel == "" {
		printInfo("Fetching artist catalog...")
	}
	artistList, err := getArtistList()
	if err != nil {
		printError("Failed to get artist list")
		return err
	}

	artists := artistList.Response.Artists
	// Apply show filter if provided
	var filterOperator string
	var filterValue int
	if showFilter != "" {
		filterOperator, filterValue, err = parseShowFilter(showFilter)
		if err != nil {
			return err
		}
		artists = applyShowFilter(artists, filterOperator, filterValue)
	}
	if len(artists) == 0 {
		if jsonLevel != "" {
			emptyOutput := ArtistListOutput{Artists: []ArtistOutput{}, Total: 0}
			jsonData, err := json.MarshalIndent(emptyOutput, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal empty output: %w", err)
			}
			fmt.Println(string(jsonData))
		} else {
			if showFilter != "" {
				printWarning(fmt.Sprintf("No artists found with shows %s%d", filterOperator, filterValue))
			} else {
				printWarning("No artists found")
			}
		}
		return nil
	}

	if jsonLevel != "" {
		// Raw mode: output full API response, applying filter if active
		if jsonLevel == JSONLevelRaw {
			if showFilter != "" {
				// Filter was applied, so output the filtered list (raw unfiltered
				// API response would contradict the user's filter intent)
				artistList.Response.Artists = artists
			}
			jsonData, err := json.MarshalIndent(artistList, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Println(string(jsonData))
			return nil
		}

		// Sort alphabetically for non-raw JSON output
		sort.Slice(artists, func(i, j int) bool {
			return strings.ToLower(artists[i].ArtistName) < strings.ToLower(artists[j].ArtistName)
		})

		// Build structured JSON output (same for minimal/standard/extended)
		output := ArtistListOutput{
			Artists: make([]ArtistOutput, len(artists)),
			Total:   len(artists),
		}
		for i, artist := range artists {
			output.Artists[i] = ArtistOutput{
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
		// Sort alphabetically for table output
		sort.Slice(artists, func(i, j int) bool {
			return strings.ToLower(artists[i].ArtistName) < strings.ToLower(artists[j].ArtistName)
		})

		// Table output
		if showFilter != "" {
			printSection(fmt.Sprintf("Found %d artists with shows %s%d", len(artists), filterOperator, filterValue))
		} else {
			printSection(fmt.Sprintf("Found %d artists", len(artists)))
		}

		table := NewTable([]TableColumn{
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
		printInfo("To list shows for an artist, use: nugs list <artist_id>")
	}
	return nil
}

// displayWelcome shows a welcome screen with latest shows from the catalog
func displayWelcome() error {
	printHeader("Welcome to Nugs Downloader")

	// Fetch latest catalog additions
	catalog, err := getLatestCatalog()
	if err != nil {
		printWarning(fmt.Sprintf("Unable to fetch latest shows: %v", err))
		fmt.Println()
		return err
	}

	if len(catalog.Response.RecentItems) == 0 {
		printWarning("No recent shows available")
		fmt.Println()
		return nil
	}

	printSection("Latest Additions to Catalog")

	// Show top 15 latest additions
	showCount := min(15, len(catalog.Response.RecentItems))

	table := NewTable([]TableColumn{
		{Header: "Artist ID", Width: 10, Align: "right"},
		{Header: "Artist", Width: 25, Align: "left"},
		{Header: "Date", Width: 12, Align: "left"},
		{Header: "Title", Width: 40, Align: "left"},
		{Header: "Venue", Width: 25, Align: "left"},
	})

	for i := range showCount {
		item := catalog.Response.RecentItems[i]

		// Format location
		location := item.Venue
		if item.VenueCity != "" {
			location = fmt.Sprintf("%s, %s", item.VenueCity, item.VenueState)
		}

		table.AddRow(
			fmt.Sprintf("%s%d%s", colorPurple, item.ArtistID, colorReset),
			fmt.Sprintf("%s%s%s", colorGreen, item.ArtistName, colorReset),
			fmt.Sprintf("%s%s%s", colorYellow, item.ShowDateFormattedShort, colorReset),
			fmt.Sprintf("%s%s%s", colorCyan, item.ContainerInfo, colorReset),
			location,
		)
	}

	table.Print()
	fmt.Println()

	printSection("Quick Start")
	quickStartCommands := []string{
		fmt.Sprintf("%snugs list%s - Browse all artists", colorCyan, colorReset),
		fmt.Sprintf("%snugs list 1125%s - View Billy Strings shows", colorCyan, colorReset),
		fmt.Sprintf("%snugs list 1125 \"Red Rocks\"%s - Filter by venue", colorCyan, colorReset),
		fmt.Sprintf("%snugs list \">100\"%s - Filter artists by show count", colorCyan, colorReset),
		fmt.Sprintf("%snugs grab 1125 latest%s - Download latest shows", colorCyan, colorReset),
		fmt.Sprintf("%snugs list artists --json standard | jq%s - Export artist list as JSON", colorCyan, colorReset),
		fmt.Sprintf("%snugs gaps 1125 fill%s - Fill missing shows", colorCyan, colorReset),
		fmt.Sprintf("%snugs completion bash%s - Generate shell completions", colorCyan, colorReset),
		fmt.Sprintf("%snugs help%s - View all commands", colorCyan, colorReset),
	}
	printList(quickStartCommands, colorGreen)

	printSection("Catalog Workflow")
	catalogWorkflowCommands := []string{
		fmt.Sprintf("%snugs update%s - Refresh local catalog cache", colorCyan, colorReset),
		fmt.Sprintf("%snugs cache%s - Inspect cache status and metadata", colorCyan, colorReset),
		fmt.Sprintf("%snugs stats%s - See catalog-wide statistics", colorCyan, colorReset),
		fmt.Sprintf("%snugs latest 10%s - Show most recent additions", colorCyan, colorReset),
		fmt.Sprintf("%snugs coverage 1125 461%s - Check collection coverage", colorCyan, colorReset),
		fmt.Sprintf("%snugs refresh set%s - Configure auto-refresh schedule", colorCyan, colorReset),
	}
	printList(catalogWorkflowCommands, colorGreen)
	fmt.Println()
	printInfo("Tip: Quote comparison filters (for example, \">100\") to avoid shell redirection.")
	fmt.Println()

	return nil
}

// listArtistShows fetches and displays all shows for a specific artist identified by artistId.
// The output is sorted by date in reverse chronological order (newest first) and includes
// container ID, date, title, and venue for each show.
// Optional mediaFilter filters shows by media type (audio/video/both). Use MediaTypeUnknown for no filter.
// Returns an error if the artist metadata cannot be fetched from the API.
func listArtistShows(artistId string, jsonLevel string, mediaFilter ...MediaType) error {
	mf := MediaTypeUnknown
	if len(mediaFilter) > 0 {
		mf = mediaFilter[0]
	}
	if jsonLevel == "" {
		printInfo("Fetching artist shows...")
	}
	allMeta, err := getArtistMeta(artistId)
	if err != nil {
		printError("Failed to get artist metadata")
		return err
	}

	if len(allMeta) == 0 {
		printWarning("No metadata found for this artist")
		return nil
	}

	// Extract artist name from first container
	artistName := "Unknown Artist"
	if len(allMeta) > 0 && len(allMeta[0].Response.Containers) > 0 {
		artistName = allMeta[0].Response.Containers[0].ArtistName
	}

	// Collect all containers from all paginated responses
	var allContainers []ContainerWithDate

	for _, meta := range allMeta {
		for _, container := range meta.Response.Containers {
			// Detect media type for this show
			showMedia := getShowMediaType(container)

			// Apply media filter if specified
			if mf != MediaTypeUnknown && !matchesMediaFilter(showMedia, mf) {
				continue
			}

			dateStr := container.PerformanceDateShortYearFirst
			if dateStr == "" {
				dateStr = container.PerformanceDate
			}
			allContainers = append(allContainers, ContainerWithDate{
				Container: container,
				DateStr:   dateStr,
				MediaType: showMedia,
			})
		}
	}

	if len(allContainers) == 0 {
		if jsonLevel != "" {
			artistIdInt, _ := strconv.Atoi(artistId)
			emptyOutput := ShowListOutput{
				ArtistID:   artistIdInt,
				ArtistName: artistName,
				Shows:      []ShowOutput{},
				Total:      0,
			}
			jsonData, err := json.MarshalIndent(emptyOutput, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal empty output: %w", err)
			}
			fmt.Println(string(jsonData))
		} else {
			if mf != MediaTypeUnknown {
				printWarning(fmt.Sprintf("No %s shows found for %s", mf, artistName))
			} else {
				printWarning(fmt.Sprintf("No shows found for %s", artistName))
			}
		}
		return nil
	}

	// Sort by date in reverse chronological order (newest first)
	// Empty dates go to the end
	sort.Slice(allContainers, func(i, j int) bool {
		dateI := allContainers[i].DateStr
		dateJ := allContainers[j].DateStr

		// Push empty dates to end
		if dateI == "" && dateJ != "" {
			return false
		}
		if dateI != "" && dateJ == "" {
			return true
		}
		if dateI == "" && dateJ == "" {
			return false
		}

		// Sort by date descending (newest first)
		return dateI > dateJ
	})

	if jsonLevel != "" {
		// Raw mode: output full API response as-is (array of paginated responses)
		if jsonLevel == JSONLevelRaw {
			jsonData, err := json.MarshalIndent(allMeta, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Println(string(jsonData))
			return nil
		}

		// Build structured JSON output for minimal/standard/extended
		artistIdInt, _ := strconv.Atoi(artistId)

		if jsonLevel == JSONLevelExtended {
			// Extended: output full container structs with all fields
			shows := make([]*AlbArtResp, len(allContainers))
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
			// Minimal or Standard: use ShowOutput struct
			output := ShowListOutput{
				ArtistID:   artistIdInt,
				ArtistName: artistName,
				Shows:      make([]ShowOutput, len(allContainers)),
				Total:      len(allContainers),
			}

			for i, item := range allContainers {
				show := ShowOutput{
					ContainerID: item.Container.ContainerID,
					Date:        item.DateStr,
					Title:       item.Container.ContainerInfo,
					Venue:       item.Container.VenueName,
				}

				// Standard level includes location details
				if jsonLevel == JSONLevelStandard {
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
		// Table output
		filterLabel := ""
		if mf != MediaTypeUnknown {
			filterLabel = fmt.Sprintf(" (%s)", mf)
		}
		printSection(fmt.Sprintf("%s - %d shows%s", artistName, len(allContainers), filterLabel))

		table := NewTable([]TableColumn{
			{Header: "Type", Width: 6, Align: "center"},
			{Header: "ID", Width: 10, Align: "left"},
			{Header: "Date", Width: 12, Align: "left"},
			{Header: "Title", Width: 42, Align: "left"},
			{Header: "Venue", Width: 25, Align: "left"},
		})

		for _, item := range allContainers {
			container := item.Container
			mediaIndicator := getMediaTypeIndicator(item.MediaType)
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
			colorCyan, colorReset, symbolAudio, symbolVideo, symbolBoth)
		printInfo("To download a show, use: nugs <container_id>")
	}
	return nil
}

// Cache I/O Functions
// listArtistShowsByVenue filters artist shows by venue name (case-insensitive substring match)
func listArtistShowsByVenue(artistId string, venueFilter string, jsonLevel string) error {
	// Validate artistId is numeric
	if _, err := strconv.Atoi(artistId); err != nil {
		return fmt.Errorf("invalid artist ID: %s (must be numeric)", artistId)
	}

	if jsonLevel == "" {
		fmt.Printf("Fetching shows at venues matching \"%s\"...\n", venueFilter)
	}

	allMeta, err := getArtistMeta(artistId)
	if err != nil {
		printError("Failed to get artist metadata")
		return err
	}

	if len(allMeta) == 0 {
		printWarning("No metadata found for this artist")
		return nil
	}

	// Extract artist name from first container
	artistName := "Unknown Artist"
	if len(allMeta[0].Response.Containers) > 0 {
		artistName = allMeta[0].Response.Containers[0].ArtistName
	}

	// Collect and filter containers by venue (case-insensitive substring match)
	// Checks both VenueName and Venue fields since API may populate either
	var filteredContainers []ContainerWithDate
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
				filteredContainers = append(filteredContainers, ContainerWithDate{
					Container: container,
					DateStr:   dateStr,
					MediaType: MediaTypeUnknown, // Not used for venue filtering
				})
			}
		}
	}

	if len(filteredContainers) == 0 {
		if jsonLevel != "" {
			artistIdInt, _ := strconv.Atoi(artistId)
			emptyOutput := ShowListOutput{
				ArtistID:   artistIdInt,
				ArtistName: artistName,
				Shows:      []ShowOutput{},
				Total:      0,
			}
			jsonData, err := json.MarshalIndent(emptyOutput, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal empty output: %w", err)
			}
			fmt.Println(string(jsonData))
		} else {
			printWarning(fmt.Sprintf("No shows found for %s at venues matching \"%s\"", artistName, venueFilter))
		}
		return nil
	}

	// Sort by date in reverse chronological order (newest first)
	sort.Slice(filteredContainers, func(i, j int) bool {
		dateI := filteredContainers[i].DateStr
		dateJ := filteredContainers[j].DateStr

		// Push empty dates to end
		if dateI == "" && dateJ != "" {
			return false
		}
		if dateI != "" && dateJ == "" {
			return true
		}
		if dateI == "" && dateJ == "" {
			return false
		}

		// Sort by date descending (newest first)
		return dateI > dateJ
	})

	if jsonLevel != "" {
		// Raw mode not applicable for filtered results - use extended instead
		artistIdInt, _ := strconv.Atoi(artistId)

		if jsonLevel == JSONLevelExtended || jsonLevel == JSONLevelRaw {
			// Extended: output full container structs with all fields
			shows := make([]*AlbArtResp, len(filteredContainers))
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
			// Minimal or Standard: use ShowOutput struct
			output := ShowListOutput{
				ArtistID:   artistIdInt,
				ArtistName: artistName,
				Shows:      make([]ShowOutput, len(filteredContainers)),
				Total:      len(filteredContainers),
			}

			for i, item := range filteredContainers {
				show := ShowOutput{
					ContainerID: item.Container.ContainerID,
					Date:        item.DateStr,
					Title:       item.Container.ContainerInfo,
					Venue:       item.Container.VenueName,
				}

				// Standard level includes location details
				if jsonLevel == JSONLevelStandard {
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
		// Table output
		printSection(fmt.Sprintf("%s - Shows at \"%s\" (%d shows)", artistName, venueFilter, len(filteredContainers)))

		table := NewTable([]TableColumn{
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
		printInfo("To download a show, use: nugs <container_id>")
	}
	return nil
}

// listArtistLatestShows displays the latest N shows for an artist
func listArtistLatestShows(artistId string, limit int, jsonLevel string) error {
	if jsonLevel == "" {
		fmt.Printf("Fetching latest %d shows...\n", limit)
	}

	allMeta, err := getArtistMeta(artistId)
	if err != nil {
		printError("Failed to get artist metadata")
		return err
	}

	if len(allMeta) == 0 {
		printWarning("No metadata found for this artist")
		return nil
	}

	// Extract artist name from first container
	artistName := "Unknown Artist"
	if len(allMeta) > 0 && len(allMeta[0].Response.Containers) > 0 {
		artistName = allMeta[0].Response.Containers[0].ArtistName
	}

	// Collect all containers from all paginated responses
	var allContainers []ContainerWithDate

	for _, meta := range allMeta {
		for _, container := range meta.Response.Containers {
			dateStr := container.PerformanceDateShortYearFirst
			if dateStr == "" {
				dateStr = container.PerformanceDate
			}
			allContainers = append(allContainers, ContainerWithDate{
				Container: container,
				DateStr:   dateStr,
				MediaType: MediaTypeUnknown, // Not used for latest shows
			})
		}
	}

	if len(allContainers) == 0 {
		if jsonLevel != "" {
			artistIdInt, _ := strconv.Atoi(artistId)
			emptyOutput := ShowListOutput{
				ArtistID:   artistIdInt,
				ArtistName: artistName,
				Shows:      []ShowOutput{},
				Total:      0,
			}
			jsonData, err := json.MarshalIndent(emptyOutput, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal empty output: %w", err)
			}
			fmt.Println(string(jsonData))
		} else {
			printWarning(fmt.Sprintf("No shows found for %s", artistName))
		}
		return nil
	}

	// Sort by date in reverse chronological order (newest first)
	sort.Slice(allContainers, func(i, j int) bool {
		dateI := allContainers[i].DateStr
		dateJ := allContainers[j].DateStr

		// Push empty dates to end
		if dateI == "" && dateJ != "" {
			return false
		}
		if dateI != "" && dateJ == "" {
			return true
		}
		if dateI == "" && dateJ == "" {
			return false
		}

		// Sort by date descending (newest first)
		return dateI > dateJ
	})

	// Limit to N latest shows
	if limit > len(allContainers) {
		limit = len(allContainers)
	}
	latestContainers := allContainers[:limit]

	if jsonLevel != "" {
		// Raw mode not applicable for limited results - use extended instead
		artistIdInt, _ := strconv.Atoi(artistId)

		if jsonLevel == JSONLevelExtended || jsonLevel == JSONLevelRaw {
			// Extended: output full container structs with all fields
			shows := make([]*AlbArtResp, len(latestContainers))
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
			// Minimal or Standard: use ShowOutput struct
			output := ShowListOutput{
				ArtistID:   artistIdInt,
				ArtistName: artistName,
				Shows:      make([]ShowOutput, len(latestContainers)),
				Total:      len(latestContainers),
			}

			for i, item := range latestContainers {
				show := ShowOutput{
					ContainerID: item.Container.ContainerID,
					Date:        item.DateStr,
					Title:       item.Container.ContainerInfo,
					Venue:       item.Container.VenueName,
				}

				// Standard level includes location details
				if jsonLevel == JSONLevelStandard {
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
		// Table output
		printHeader(fmt.Sprintf("%s - Latest %d Shows", artistName, len(latestContainers)))

		table := NewTable([]TableColumn{
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
			colorCyan, symbolInfo, colorReset, colorBold, colorReset)
	}
	return nil
}

func resolveCatPlistId(plistUrl string) (string, error) {
	req, err := client.Get(plistUrl)
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

func catalogPlist(_plistId, legacyToken string, cfg *Config, streamParams *StreamParams) error {
	plistId, err := resolveCatPlistId(_plistId)
	if err != nil {
		fmt.Println("Failed to resolve playlist ID.")
		return err
	}
	err = playlist(plistId, legacyToken, cfg, streamParams, true)
	return err
}
