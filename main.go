package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// JSON output levels
const (
	JSONLevelMinimal  = "minimal"
	JSONLevelStandard = "standard"
	JSONLevelExtended = "extended"
	JSONLevelRaw      = "raw"
)

func init() {
	// Check if --json flag or completion command is present, if so, suppress banner
	if slices.Contains(os.Args, "--json") {
		return
	}
	// Suppress banner for completion command to avoid breaking shell completion parsing
	if len(os.Args) > 1 && os.Args[1] == "completion" {
		return
	}
	fmt.Println(`
 _____                ____                _           _
|   | |_ _ ___ ___   |    \ ___ _ _ _ ___| |___ ___ _| |___ ___
| | | | | | . |_ -|  |  |  | . | | | |   | | . | .'| . | -_|  _|
|_|___|___|_  |___|  |____/|___|_____|_|_|_|___|__,|___|___|_|
	  |___|`)
}

func main() {
	cfg, jsonLevel := bootstrap()
	run(cfg, jsonLevel)
}

// bootstrap handles early setup: session persistence, working directory,
// config file detection, JSON flag parsing, and config/arg parsing.
func bootstrap() (*Config, string) {
	setupSessionPersistence()

	// Check if any config file exists, if not, prompt to create one
	configExists := false
	homeDir, _ := os.UserHomeDir()
	configSearchPaths := []string{
		"config.json",
		filepath.Join(homeDir, ".nugs", "config.json"),
		filepath.Join(homeDir, ".config", "nugs", "config.json"),
	}
	for _, p := range configSearchPaths {
		if _, statErr := os.Stat(p); statErr == nil {
			configExists = true
			break
		}
	}
	if !configExists {
		err := promptForConfig()
		if err != nil {
			handleErr("Failed to create config.", err, true)
		}
	}

	// Check if first argument is "help" before parsing
	if len(os.Args) > 1 && os.Args[1] == "help" {
		os.Args[1] = "--help"
	}

	// Check for --json flag with level parameter BEFORE parsing config
	// This removes it from os.Args so the arg parser doesn't complain
	jsonLevel := "" // empty = table output, "minimal"/"standard"/"extended"/"raw"
	for i := 0; i < len(os.Args); i++ {
		if os.Args[i] == "--json" {
			if i+1 >= len(os.Args) {
				fmt.Println("Error: --json flag requires a level argument (minimal, standard, extended, raw)")
				printInfo("Usage: nugs list artists --json <level>")
				os.Exit(0)
			}
			jsonLevel = os.Args[i+1]
			os.Args = append(os.Args[:i], os.Args[i+2:]...)
			break
		}
	}

	// Validate json level
	if jsonLevel != "" && jsonLevel != JSONLevelMinimal && jsonLevel != JSONLevelStandard && jsonLevel != JSONLevelExtended && jsonLevel != JSONLevelRaw {
		fmt.Printf("Invalid JSON level: %s. Valid options: %s, %s, %s, %s\n", jsonLevel, JSONLevelMinimal, JSONLevelStandard, JSONLevelExtended, JSONLevelRaw)
		os.Exit(0)
	}

	cfg, err := parseCfg()
	if err != nil {
		handleErr("Failed to parse config/args.", err, true)
	}
	cfg.Urls = normalizeCliAliases(cfg.Urls)
	printActiveRuntimeHint(os.Getpid(), cfg.Urls)

	return cfg, jsonLevel
}

// run is the main orchestration function: handles early-exit commands (detach, status,
// cancel, completion), runtime tracking, environment setup, command routing (list,
// catalog, artist shortcuts), authentication, and dispatching URL downloads.
func run(cfg *Config, jsonLevel string) {
	if maybeDetachAndExit(os.Args[1:], cfg.Urls) {
		return
	}

	if len(cfg.Urls) == 1 && cfg.Urls[0] == "status" {
		printRuntimeStatus()
		return
	}

	if len(cfg.Urls) == 1 && cfg.Urls[0] == "cancel" {
		status, err := readRuntimeStatus()
		if err != nil {
			printWarning("No active crawl status found")
			return
		}
		if status.State != "running" {
			printWarning(fmt.Sprintf("No running crawl found (state: %s)", status.State))
			return
		}
		if err := requestRuntimeCancel(); err != nil {
			handleErr("Failed to request crawl cancellation.", err, false)
			return
		}
		_ = cancelProcessByPID(status.PID)
		printSuccess(fmt.Sprintf("Cancellation requested for crawl pid=%d", status.PID))
		return
	}

	// Completion command - generate shell completion scripts
	if len(cfg.Urls) > 0 && cfg.Urls[0] == "completion" {
		err := completionCommand(cfg.Urls)
		if err != nil {
			handleErr("Completion command failed.", err, true)
		}
		return
	}

	trackRuntime := !isReadOnlyCommand(cfg.Urls)
	if trackRuntime {
		initRuntimeStatus()
	}
	runCancelled := false
	defer func() {
		if !trackRuntime {
			return
		}
		if runCancelled {
			finalizeRuntimeStatus("cancelled")
			return
		}
		finalizeRuntimeStatus("completed")
	}()
	stopHotkeys := startCrawlHotkeysIfNeeded(cfg.Urls)
	defer stopHotkeys()

	// Auto-refresh catalog cache if needed
	err := autoRefreshIfNeeded(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Auto-refresh warning: %v\n", err)
	}

	// Check if rclone is available when enabled
	if cfg.RcloneEnabled {
		err = checkRcloneAvailable(jsonLevel != "")
		if err != nil {
			handleErr("Rclone check failed.", err, true)
		}
	}
	printStartupEnvironment(cfg, jsonLevel)

	// Show welcome screen if no arguments provided
	if len(cfg.Urls) == 0 {
		err := displayWelcome()
		if err != nil {
			fmt.Printf("Error displaying welcome screen: %v\n", err)
		}
		return
	}

	// Route list and catalog commands (pre-auth)
	if handleListCommand(cfg, jsonLevel) {
		return
	}
	if handleCatalogCommand(cfg, jsonLevel) {
		return
	}

	// Handle "<artistID> latest/full" shorthand
	if len(cfg.Urls) == 2 {
		if handled := handleArtistShorthand(cfg); handled {
			return
		}
	}

	// Authenticate
	var token string
	err = makeDirs(cfg.OutPath)
	if err != nil {
		handleErr("Failed to make output folder.", err, true)
	}
	if cfg.Token == "" {
		token, err = auth(cfg.Email, cfg.Password)
		if err != nil {
			handleErr("Failed to auth.", err, true)
		}
	} else {
		token = cfg.Token
	}
	userId, err := getUserInfo(token)
	if err != nil {
		handleErr("Failed to get user info.", err, true)
	}
	subInfo, err := getSubInfo(token)
	if err != nil {
		handleErr("Failed to get subcription info.", err, true)
	}
	legacyToken, uguID, err := extractLegToken(token)
	if err != nil {
		handleErr("Failed to extract legacy token.", err, true)
	}
	planDesc, isPromo := getPlan(subInfo)
	if !subInfo.IsContentAccessible {
		planDesc = "no active subscription"
	}
	printSuccess(fmt.Sprintf("Signed in - %s%s%s", colorCyan, planDesc, colorReset))
	streamParams := parseStreamParams(userId, subInfo, isPromo)

	// Handle "catalog gaps <artist_id> [...] fill" (requires auth)
	if len(cfg.Urls) >= 4 && cfg.Urls[0] == "catalog" && cfg.Urls[1] == "gaps" && cfg.Urls[len(cfg.Urls)-1] == "fill" {
		handleCatalogGapsFill(cfg, streamParams, jsonLevel)
		return
	}

	runCancelled = dispatch(cfg, streamParams, legacyToken, uguID)
}

// handleListCommand routes "list" subcommands. Returns true if handled.
func handleListCommand(cfg *Config, jsonLevel string) bool {
	if len(cfg.Urls) == 0 || cfg.Urls[0] != "list" {
		return false
	}
	if len(cfg.Urls) < 2 {
		printInfo("Usage: nugs list artists | list <artist_id> [shows \"venue\" | latest <N>]")
		fmt.Println("       list <show_count_filter>")
		fmt.Println("       list <artist_id> [\"venue\" | latest <N>]")
		return true
	}

	subCmd := cfg.Urls[1]
	if subCmd == "artists" {
		showFilter := ""
		if len(cfg.Urls) > 2 && cfg.Urls[2] == "shows" {
			if len(cfg.Urls) < 4 {
				printInfo("Usage: nugs list artists shows <operator><number>")
				fmt.Println("Or:    list artists shows <operator><number>")
				fmt.Println("Examples:")
				fmt.Println("  list >100")
				fmt.Println("  list <=50")
				fmt.Println("  list =25")
				fmt.Println("Operators: >, <, >=, <=, =")
				return true
			}
			showFilter = cfg.Urls[3]
		}
		err := listArtists(jsonLevel, showFilter)
		if err != nil {
			handleErr("List artists failed.", err, true)
		}
		return true
	}

	artistId := subCmd

	// Check for venue filter: list <artist_id> shows "venue"
	if len(cfg.Urls) > 2 && cfg.Urls[2] == "shows" {
		if len(cfg.Urls) < 4 {
			printInfo("Usage: nugs list <artist_id> shows \"<venue_name>\"")
			fmt.Println("Or:    list <artist_id> shows \"<venue_name>\"")
			fmt.Println("Example: list 461 \"Red Rocks\"")
			return true
		}
		venueFilter := strings.Join(cfg.Urls[3:], " ")
		err := listArtistShowsByVenue(artistId, venueFilter, jsonLevel)
		if err != nil {
			handleErr("List shows by venue failed.", err, true)
		}
		return true
	}

	// Check for latest N: list <artist_id> latest <N>
	if len(cfg.Urls) > 2 && cfg.Urls[2] == "latest" {
		limit := 10
		if len(cfg.Urls) > 3 {
			if parsedLimit, parseErr := strconv.Atoi(cfg.Urls[3]); parseErr == nil {
				if parsedLimit < 1 {
					fmt.Println("Error: limit must be a positive number (got", parsedLimit, ")")
					return true
				}
				limit = parsedLimit
			}
		}
		err := listArtistLatestShows(artistId, limit, jsonLevel)
		if err != nil {
			handleErr("List latest shows failed.", err, true)
		}
		return true
	}

	// Default: list all shows for artist
	err := listArtistShows(artistId, jsonLevel)
	if err != nil {
		handleErr("List shows failed.", err, true)
	}
	return true
}

// handleCatalogCommand routes pre-auth "catalog" subcommands. Returns true if handled.
// Note: "catalog gaps ... fill" requires auth and is handled separately in run().
// parseMediaModifier scans args for media type modifiers (audio/video/both)
// and returns the MediaType along with remaining args (with modifier removed).
func parseMediaModifier(args []string) (MediaType, []string) {
	for i, arg := range args {
		mediaType := ParseMediaType(arg)
		if mediaType != MediaTypeUnknown {
			// Remove this arg and return rest
			remaining := append([]string{}, args[:i]...)
			remaining = append(remaining, args[i+1:]...)
			return mediaType, remaining
		}
	}
	return MediaTypeUnknown, args
}

func handleCatalogCommand(cfg *Config, jsonLevel string) bool {
	if len(cfg.Urls) == 0 || cfg.Urls[0] != "catalog" {
		return false
	}
	// "catalog gaps ... fill" requires auth — skip here, handled post-auth in run()
	isCatalogGapsFill := len(cfg.Urls) >= 4 && cfg.Urls[1] == "gaps" && cfg.Urls[len(cfg.Urls)-1] == "fill"
	if isCatalogGapsFill {
		return false
	}

	if len(cfg.Urls) < 2 {
		printInfo("Usage: nugs catalog update")
		fmt.Println("       catalog cache")
		fmt.Println("       catalog stats")
		fmt.Println("       catalog latest [limit]")
		fmt.Println("       catalog list <artist_id> [...]")
		fmt.Println("       catalog gaps <artist_id> [...] [fill]")
		fmt.Println("       catalog gaps <artist_id> [...] --ids-only")
		fmt.Println("       catalog coverage [artist_ids...]")
		fmt.Println("       catalog config enable|disable|set")
		return true
	}

	subCmd := cfg.Urls[1]
	switch subCmd {
	case "update":
		err := catalogUpdate(jsonLevel)
		if err != nil {
			handleErr("Catalog update failed.", err, true)
		}
	case "cache":
		err := catalogCacheStatus(jsonLevel)
		if err != nil {
			handleErr("Catalog cache status failed.", err, true)
		}
	case "stats":
		err := catalogStats(jsonLevel)
		if err != nil {
			handleErr("Catalog stats failed.", err, true)
		}
	case "latest":
		limit := 15
		argsAfterLatest := []string{}
		if len(cfg.Urls) > 2 {
			argsAfterLatest = cfg.Urls[2:]
		}
		
		// Extract media modifier
		mediaFilter, remainingArgs := parseMediaModifier(argsAfterLatest)
		
		// Parse limit from remaining args
		if len(remainingArgs) > 0 {
			if parsedLimit, err := strconv.Atoi(remainingArgs[0]); err == nil {
				if parsedLimit < 1 {
					fmt.Println("Error: limit must be a positive number (got", parsedLimit, ")")
					return true
				}
				limit = parsedLimit
			}
		}
		err := catalogLatest(limit, jsonLevel, mediaFilter)
		if err != nil {
			handleErr("Catalog latest failed.", err, true)
		}
	case "gaps":
		if len(cfg.Urls) < 3 {
			printInfo("Usage: nugs catalog gaps <artist_id> [...] [audio|video|both] [fill]")
			fmt.Println("       catalog gaps <artist_id> [...] [audio|video|both] --ids-only")
			return true
		}
		
		// Extract media modifier from args after "catalog gaps"
		argsAfterGaps := cfg.Urls[2:]
		mediaFilter, argsAfterGaps := parseMediaModifier(argsAfterGaps)
		
		// Handle --ids-only flag
		idsOnly := false
		artistIds := []string{}
		for _, arg := range argsAfterGaps {
			if arg == "--ids-only" {
				idsOnly = true
				continue
			}
			artistIds = append(artistIds, arg)
		}
		
		if len(artistIds) == 0 {
			fmt.Println("Error: No artist IDs provided")
			return true
		}
		err := catalogGaps(artistIds, cfg, jsonLevel, idsOnly, mediaFilter)
		if err != nil {
			handleErr("Catalog gaps failed.", err, true)
		}
	case "list":
		if len(cfg.Urls) < 3 {
			printInfo("Usage: nugs catalog list <artist_id> [...] [audio|video|both]")
			return true
		}
		
		// Extract media modifier from args after "catalog list"
		argsAfterList := cfg.Urls[2:]
		mediaFilter, artistIds := parseMediaModifier(argsAfterList)
		
		err := catalogList(artistIds, cfg, jsonLevel, mediaFilter)
		if err != nil {
			handleErr("Catalog list failed.", err, true)
		}
	case "coverage":
		argsAfterCoverage := []string{}
		if len(cfg.Urls) > 2 {
			argsAfterCoverage = cfg.Urls[2:]
		}
		
		// Extract media modifier
		mediaFilter, artistIds := parseMediaModifier(argsAfterCoverage)
		
		err := catalogCoverage(artistIds, cfg, jsonLevel, mediaFilter)
		if err != nil {
			handleErr("Catalog coverage failed.", err, true)
		}
	case "config":
		if len(cfg.Urls) < 3 {
			printInfo("Usage: nugs catalog config enable|disable|set")
			return true
		}
		action := cfg.Urls[2]
		switch action {
		case "enable":
			err := enableAutoRefresh(cfg)
			if err != nil {
				handleErr("Enable auto-refresh failed.", err, true)
			}
		case "disable":
			err := disableAutoRefresh(cfg)
			if err != nil {
				handleErr("Disable auto-refresh failed.", err, true)
			}
		case "set":
			err := configureAutoRefresh(cfg)
			if err != nil {
				handleErr("Configure auto-refresh failed.", err, true)
			}
		default:
			fmt.Printf("Unknown config action: %s\n", action)
		}
	default:
		fmt.Printf("Unknown catalog command: %s\n", subCmd)
	}
	return true
}

// handleArtistShorthand handles "<artistID> latest/full [media]" shortcuts.
// Returns true if the input was handled (including error cases).
// Accepts optional media type modifier: "audio", "video", or "both"
func handleArtistShorthand(cfg *Config) bool {
	artistID, err := strconv.Atoi(cfg.Urls[0])
	if err != nil {
		return false
	}

	// Check for media type modifier (3rd arg: "audio", "video", "both")
	mediaModifier := ""
	if len(cfg.Urls) > 2 {
		candidate := strings.ToLower(cfg.Urls[2])
		if candidate == "audio" || candidate == "video" || candidate == "both" {
			mediaModifier = candidate
		}
	}

	switch cfg.Urls[1] {
	case "latest":
		artistUrl := fmt.Sprintf("https://play.nugs.net/artist/%d/latest", artistID)
		cfg.Urls = []string{artistUrl}
		if mediaModifier != "" {
			cfg.DefaultOutputs = mediaModifier
			printMusic(fmt.Sprintf("Downloading latest shows (%s) from %sartist %d%s", mediaModifier, colorBold, artistID, colorReset))
		} else {
			printMusic(fmt.Sprintf("Downloading latest shows from %sartist %d%s", colorBold, artistID, colorReset))
		}
	case "full":
		artistUrl := fmt.Sprintf("https://play.nugs.net/#/artist/%d", artistID)
		cfg.Urls = []string{artistUrl}
		if mediaModifier != "" {
			cfg.DefaultOutputs = mediaModifier
			printMusic(fmt.Sprintf("Downloading entire catalog (%s) from %sartist %d%s", mediaModifier, colorBold, artistID, colorReset))
		} else {
			printMusic(fmt.Sprintf("Downloading entire catalog from %sartist %d%s", colorBold, artistID, colorReset))
		}
	case "gaps", "update", "cache", "stats", "config", "coverage", "list":
		fmt.Printf("%s✗ Invalid syntax%s\n\n", colorRed, colorReset)
		fmt.Printf("Did you mean: %snugs catalog %s %d%s\n\n", colorBold, cfg.Urls[1], artistID, colorReset)
		fmt.Printf("Valid artist shortcuts:\n")
		fmt.Printf("  • %snugs %d latest%s [audio|video|both] - Download latest shows\n", colorBold, artistID, colorReset)
		fmt.Printf("  • %snugs %d full%s [audio|video|both]   - Download entire catalog\n\n", colorBold, artistID, colorReset)
		fmt.Printf("For catalog commands, use:\n")
		fmt.Printf("  • %snugs catalog %s %d%s\n", colorBold, cfg.Urls[1], artistID, colorReset)
		os.Exit(1)
	default:
		fmt.Printf("%s✗ Unknown command: %s%s\n\n", colorRed, cfg.Urls[1], colorReset)
		fmt.Printf("Valid artist shortcuts:\n")
		fmt.Printf("  • %snugs %d latest%s [audio|video|both] - Download latest shows\n", colorBold, artistID, colorReset)
		fmt.Printf("  • %snugs %d full%s [audio|video|both]   - Download entire catalog\n\n", colorBold, artistID, colorReset)
		os.Exit(1)
	}
	return false // continue to auth+dispatch with rewritten URL
}

// handleCatalogGapsFill handles the "catalog gaps <artist_id> [...] fill" command
// which requires authentication.
func handleCatalogGapsFill(cfg *Config, streamParams *StreamParams, jsonLevel string) {
	artistIds := []string{}
	for i := 2; i < len(cfg.Urls)-1; i++ {
		artistIds = append(artistIds, cfg.Urls[i])
	}
	if len(artistIds) == 0 {
		fmt.Println("Error: No artist IDs provided")
		fmt.Println("Usage: catalog gaps <artist_id> [...] fill")
		return
	}
	for idx, artistId := range artistIds {
		if idx > 0 && jsonLevel == "" {
			fmt.Println()
			fmt.Println(strings.Repeat("─", 80))
			fmt.Println()
		}
		err := catalogGapsFill(artistId, cfg, streamParams, jsonLevel)
		if err != nil {
			if len(artistIds) > 1 {
				printWarning(fmt.Sprintf("Failed to fill gaps for artist %s: %v", artistId, err))
				continue
			}
			handleErr("Catalog gaps fill failed.", err, true)
		}
	}
}

// dispatch iterates over URLs, resolves their type, and routes each to the
// appropriate handler (album, playlist, video, artist, etc.).
// Returns true if the run was cancelled.
func dispatch(cfg *Config, streamParams *StreamParams, legacyToken, uguID string) bool {
	albumTotal := len(cfg.Urls)
	var itemErr error
	completedItems := 0
	for albumNum, _url := range cfg.Urls {
		if err := waitIfPausedOrCancelled(); err != nil {
			if isCrawlCancelledErr(err) {
				printWarning("Crawl cancelled")
				return true
			}
		}
		errorsBefore := runErrorCount
		warningsBefore := runWarningCount
		fmt.Printf("\n%s%s Item %d of %d%s\n", colorBold, symbolPackage, albumNum+1, albumTotal, colorReset)
		itemId, mediaType := checkUrl(_url)
		if itemId == "" {
			fmt.Println("Invalid URL:", _url)
			continue
		}
		switch mediaType {
		case 0:
			itemErr = album(itemId, cfg, streamParams, nil, nil, nil)
		case 1, 2:
			itemErr = playlist(itemId, legacyToken, cfg, streamParams, false)
		case 3:
			itemErr = catalogPlist(itemId, legacyToken, cfg, streamParams)
		case 4, 10:
			itemErr = video(itemId, "", cfg, streamParams, nil, false, nil)
		case 5:
			itemErr = artist(itemId, cfg, streamParams)
		case 6, 7, 8:
			itemErr = video(itemId, "", cfg, streamParams, nil, true, nil)
		case 9:
			itemErr = paidLstream(itemId, uguID, cfg, streamParams)
		case 11:
			itemErr = album(itemId, cfg, streamParams, nil, nil, nil)
		}
		if itemErr != nil {
			if isCrawlCancelledErr(itemErr) {
				printWarning("Crawl cancelled")
				return true
			}
			handleErr("Item failed.", itemErr, false)
		}
		completedItems++
		itemErrors := runErrorCount - errorsBefore
		itemWarnings := runWarningCount - warningsBefore
		itemStatus := fmt.Sprintf("Item %d/%d complete", albumNum+1, albumTotal)
		if itemErr != nil || itemErrors > 0 {
			itemStatus = fmt.Sprintf("Item %d/%d completed with issues", albumNum+1, albumTotal)
		}
		printInfo(fmt.Sprintf("%s | health: errors=%d warnings=%d | run: %d/%d done",
			itemStatus, itemErrors, itemWarnings, completedItems, albumTotal))
	}
	fmt.Println()
	printSection("Run Summary")
	printInfo(fmt.Sprintf("Completed %d/%d items", completedItems, albumTotal))
	printInfo(fmt.Sprintf("Total health: errors=%d warnings=%d", runErrorCount, runWarningCount))
	return false
}
