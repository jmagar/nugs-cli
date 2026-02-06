package main

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/dustin/go-humanize"
	"github.com/grafov/m3u8"
)

// JSON output levels
const (
	JSONLevelMinimal  = "minimal"
	JSONLevelStandard = "standard"
	JSONLevelExtended = "extended"
	JSONLevelRaw      = "raw"
)

const (
	devKey         = "x7f54tgbdyc64y656thy47er4"
	clientId       = "Eg7HuH873H65r5rt325UytR5429"
	layout         = "01/02/2006 15:04:05"
	userAgent      = "NugsNet/3.26.724 (Android; 7.1.2; Asus; ASUS_Z01QD; Scale/2.0; en)"
	userAgentTwo   = "nugsnetAndroid"
	authUrl        = "https://id.nugs.net/connect/token"
	streamApiBase  = "https://streamapi.nugs.net/"
	subInfoUrl     = "https://subscriptions.nugs.net/api/v1/me/subscriptions"
	userInfoUrl    = "https://id.nugs.net/connect/userinfo"
	playerUrl      = "https://play.nugs.net/"
	sanRegexStr    = `[\/:*?"><|]`
	chapsFileFname = "chapters_nugs_dl_tmp.txt"
	durRegex       = `Duration: ([\d:.]+)`
	bitrateRegex   = `[\w]+(?:_(\d+)k_v\d+)`
)

var (
	jar, _ = cookiejar.New(nil)
	client = &http.Client{Jar: jar}
)

// loadedConfigPath tracks which config file was loaded so writeConfig can save to the same location
var loadedConfigPath string

var regexStrings = []string{
	`^https://play.nugs.net/release/(\d+)$`,
	`^https://play.nugs.net/#/playlists/playlist/(\d+)$`,
	`^https://play.nugs.net/library/playlist/(\d+)$`,
	`(^https://2nu.gs/[a-zA-Z\d]+$)`,
	`^https://play.nugs.net/#/videos/artist/\d+/.+/(\d+)$`,
	`^https://play.nugs.net/artist/(\d+)(?:/albums|/latest|)$`,
	`^https://play.nugs.net/livestream/(\d+)/exclusive$`,
	`^https://play.nugs.net/watch/livestreams/exclusive/(\d+)$`,
	`^https://play.nugs.net/#/my-webcasts/\d+-(\d+)-\d+-\d+$`,
	`^https://www.nugs.net/on/demandware.store/Sites-NugsNet-Site/d` +
		`efault/(?:Stash-QueueVideo|NugsVideo-GetStashVideo)\?([a-zA-Z0-9=%&-]+$)`,
	`^https://play.nugs.net/library/webcast/(\d+)$`,
	`^(\d+)$`,
}

var qualityMap = map[string]Quality{
	".alac16/": {Specs: "16-bit / 44.1 kHz ALAC", Extension: ".m4a", Format: 1},
	".flac16/": {Specs: "16-bit / 44.1 kHz FLAC", Extension: ".flac", Format: 2},
	// .mqa24/ must be above .flac?
	".mqa24/":  {Specs: "24-bit / 48 kHz MQA", Extension: ".flac", Format: 3},
	".flac?":   {Specs: "FLAC", Extension: ".flac", Format: 2},
	".s360/":   {Specs: "360 Reality Audio", Extension: ".mp4", Format: 4},
	".aac150/": {Specs: "150 Kbps AAC", Extension: ".m4a", Format: 5},
	".m4a?":    {Specs: "AAC", Extension: ".m4a", Format: 5},
	".m3u8?":   {Extension: ".m4a", Format: 6},
}

var resolveRes = map[int]string{
	1: "480",
	2: "720",
	3: "1080",
	4: "1440",
	5: "2160",
}

var trackFallback = map[int]int{
	1: 2,
	2: 5,
	3: 2,
	4: 3,
}

var resFallback = map[string]string{
	"720":  "480",
	"1080": "720",
	"1440": "1080",
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
	var speed int64 = 0
	n := len(p)
	wc.Downloaded += int64(n)
	percentage := float64(wc.Downloaded) / float64(wc.Total) * float64(100)
	wc.Percentage = int(percentage)
	toDivideBy := time.Now().UnixMilli() - wc.StartTime
	if toDivideBy != 0 {
		speed = int64(wc.Downloaded) / toDivideBy * 1000
	}
	printProgress(wc.Percentage, humanize.Bytes(uint64(speed)),
		humanize.Bytes(uint64(wc.Downloaded)), wc.TotalStr)
	return n, nil
}

func handleErr(errText string, err error, _panic bool) {
	errString := errText + "\n" + err.Error()
	if _panic {
		panic(errString)
	}
	fmt.Println(errString)
}

func wasRunFromSrc() bool {
	buildPath := filepath.Join(os.TempDir(), "go-build")
	return strings.HasPrefix(os.Args[0], buildPath)
}

func getScriptDir() (string, error) {
	var (
		ok    bool
		err   error
		fname string
	)
	runFromSrc := wasRunFromSrc()
	if runFromSrc {
		_, fname, _, ok = runtime.Caller(0)
		if !ok {
			return "", errors.New("failed to get script filename")
		}
	} else {
		fname, err = os.Executable()
		if err != nil {
			return "", err
		}
	}
	return filepath.Dir(fname), nil
}

func readTxtFile(path string) ([]string, error) {
	var lines []string
	f, err := os.OpenFile(path, os.O_RDONLY, 0755)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	if scanner.Err() != nil {
		return nil, scanner.Err()
	}
	return lines, nil
}

func contains(lines []string, value string) bool {
	for _, line := range lines {
		if strings.EqualFold(line, value) {
			return true
		}
	}
	return false
}

func processUrls(urls []string) ([]string, error) {
	var (
		processed []string
		txtPaths  []string
	)
	for _, _url := range urls {
		if strings.HasSuffix(_url, ".txt") && !contains(txtPaths, _url) {
			txtLines, err := readTxtFile(_url)
			if err != nil {
				return nil, err
			}
			for _, txtLine := range txtLines {
				if !contains(processed, txtLine) {
					txtLine = strings.TrimSuffix(txtLine, "/")
					processed = append(processed, txtLine)
				}
			}
			txtPaths = append(txtPaths, _url)
		} else {
			if !contains(processed, _url) {
				_url = strings.TrimSuffix(_url, "/")
				processed = append(processed, _url)
			}
		}
	}
	return processed, nil
}

// Colorized print functions
func printSuccess(msg string) {
	fmt.Printf("%s%s%s %s%s\n", colorGreen, symbolCheck, colorReset, msg, colorReset)
}

func printError(msg string) {
	fmt.Printf("%s%s%s %s%s\n", colorRed, symbolCross, colorReset, msg, colorReset)
}

func printInfo(msg string) {
	fmt.Printf("%s%s%s %s%s\n", colorBlue, symbolInfo, colorReset, msg, colorReset)
}

func printWarning(msg string) {
	fmt.Printf("%s%s%s %s%s\n", colorYellow, symbolWarning, colorReset, msg, colorReset)
}

func printDownload(msg string) {
	fmt.Printf("%s%s%s %s%s\n", colorCyan, symbolDownload, colorReset, msg, colorReset)
}

func printUpload(msg string) {
	fmt.Printf("%s%s%s %s%s\n", colorPurple, symbolUpload, colorReset, msg, colorReset)
}

func printMusic(msg string) {
	fmt.Printf("%s%s%s %s%s\n", colorGreen, symbolMusic, colorReset, msg, colorReset)
}

func checkRcloneAvailable(quiet bool) error {
	cmd := exec.Command("rclone", "version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("rclone is not installed or not available in PATH: %w\n"+
			"Please install rclone from https://rclone.org/downloads/ or disable rclone in config.json", err)
	}

	// Extract and display version (first line of output)
	if !quiet {
		lines := strings.Split(string(output), "\n")
		if len(lines) > 0 {
			printSuccess(fmt.Sprintf("Rclone is available: %s", strings.TrimSpace(lines[0])))
		}
	}

	return nil
}

func promptForConfig() error {
	scanner := bufio.NewScanner(os.Stdin)
	printHeader("First Time Setup")
	printInfo("No config.json found. Let's create one!")
	fmt.Println()

	// Email
	fmt.Printf("%s%s%s Enter your Nugs.net email: ", colorCyan, bulletArrow, colorReset)
	scanner.Scan()
	email := strings.TrimSpace(scanner.Text())
	if email == "" {
		return errors.New("email is required")
	}

	// Password
	fmt.Printf("%s%s%s Enter your Nugs.net password: ", colorCyan, bulletArrow, colorReset)
	scanner.Scan()
	password := strings.TrimSpace(scanner.Text())
	if password == "" {
		return errors.New("password is required")
	}

	// Format
	fmt.Println()
	printSection("Track Download Quality")
	qualityOptions := []string{
		"1 = 16-bit / 44.1 kHz ALAC",
		"2 = 16-bit / 44.1 kHz FLAC",
		"3 = 24-bit / 48 kHz MQA",
		fmt.Sprintf("4 = 360 Reality Audio / best available %s(recommended)%s", colorGreen, colorReset),
		"5 = 150 Kbps AAC",
	}
	printList(qualityOptions, colorYellow)
	fmt.Printf("\n%s%s%s Enter format choice [1-5] (default: 4): ", colorCyan, bulletArrow, colorReset)
	scanner.Scan()
	formatStr := strings.TrimSpace(scanner.Text())
	format := 4
	if formatStr != "" {
		var err error
		format, err = strconv.Atoi(formatStr)
		if err != nil || format < 1 || format > 5 {
			return errors.New("format must be between 1 and 5")
		}
	}

	// Video Format
	fmt.Println()
	printSection("Video Download Format")
	videoOptions := []string{
		"1 = 480p",
		"2 = 720p",
		"3 = 1080p",
		"4 = 1440p",
		fmt.Sprintf("5 = 4K / best available %s(recommended)%s", colorGreen, colorReset),
	}
	printList(videoOptions, colorYellow)
	fmt.Printf("\n%s%s%s Enter video format choice [1-5] (default: 5): ", colorCyan, bulletArrow, colorReset)
	scanner.Scan()
	videoFormatStr := strings.TrimSpace(scanner.Text())
	videoFormat := 5
	if videoFormatStr != "" {
		var err error
		videoFormat, err = strconv.Atoi(videoFormatStr)
		if err != nil || videoFormat < 1 || videoFormat > 5 {
			return errors.New("video format must be between 1 and 5")
		}
	}

	// Output Path
	fmt.Printf("\n%s%s%s Enter download directory (default: Nugs downloads): ", colorCyan, bulletArrow, colorReset)
	scanner.Scan()
	outPath := strings.TrimSpace(scanner.Text())
	if outPath == "" {
		outPath = "Nugs downloads"
	}

	// FFmpeg
	fmt.Printf("\n%s%s%s Use FFmpeg from system PATH? [y/N] (default: N): ", colorCyan, bulletArrow, colorReset)
	scanner.Scan()
	useFfmpegEnvVarStr := strings.ToLower(strings.TrimSpace(scanner.Text()))
	useFfmpegEnvVar := useFfmpegEnvVarStr == "y" || useFfmpegEnvVarStr == "yes"

	// Rclone
	fmt.Printf("\n%s%s%s Upload to remote using rclone? [y/N] (default: N): ", colorCyan, bulletArrow, colorReset)
	scanner.Scan()
	rcloneEnabledStr := strings.ToLower(strings.TrimSpace(scanner.Text()))
	rcloneEnabled := rcloneEnabledStr == "y" || rcloneEnabledStr == "yes"

	var rcloneRemote, rclonePath string
	var deleteAfterUpload bool
	var rcloneTransfers int

	if rcloneEnabled {
		fmt.Print("Enter rclone remote name (e.g., tootie): ")
		scanner.Scan()
		rcloneRemote = strings.TrimSpace(scanner.Text())
		if rcloneRemote == "" {
			return errors.New("rclone remote name is required")
		}

		fmt.Print("Enter remote path (e.g., /mnt/user/data/media/music): ")
		scanner.Scan()
		rclonePath = strings.TrimSpace(scanner.Text())
		if rclonePath == "" {
			return errors.New("rclone remote path is required")
		}

		fmt.Print("Enter number of parallel transfers (default: 4): ")
		scanner.Scan()
		transfersStr := strings.TrimSpace(scanner.Text())
		if transfersStr == "" {
			rcloneTransfers = 4
		} else {
			var err error
			rcloneTransfers, err = strconv.Atoi(transfersStr)
			if err != nil || rcloneTransfers < 1 {
				return errors.New("transfers must be a positive integer")
			}
		}

		fmt.Print("Delete local files after upload? [Y/n] (default: Y): ")
		scanner.Scan()
		deleteStr := strings.ToLower(strings.TrimSpace(scanner.Text()))
		deleteAfterUpload = deleteStr != "n" && deleteStr != "no"
	}

	// Create config object
	config := Config{
		Email:                  email,
		Password:               password,
		Format:                 format,
		VideoFormat:            videoFormat,
		OutPath:                outPath,
		Token:                  "",
		UseFfmpegEnvVar:        useFfmpegEnvVar,
		RcloneEnabled:          rcloneEnabled,
		RcloneRemote:           rcloneRemote,
		RclonePath:             rclonePath,
		DeleteAfterUpload:      deleteAfterUpload,
		RcloneTransfers:        rcloneTransfers,
		CatalogAutoRefresh:     true,
		CatalogRefreshTime:     "05:00",
		CatalogRefreshTimezone: "America/New_York",
		CatalogRefreshInterval: "daily",
	}

	// Write to file
	data, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return err
	}

	err = os.WriteFile("config.json", data, 0600)
	if err != nil {
		return err
	}

	fmt.Println()
	printSuccess("config.json created successfully!")
	printInfo("You can edit config.json later to change these settings.")
	fmt.Println()
	return nil
}

func validatePath(path string) error {
	// Only block null bytes and newlines which can cause real issues
	// exec.Command handles shell metacharacters safely
	if strings.ContainsAny(path, "\x00\n\r") {
		return fmt.Errorf("path contains invalid characters")
	}
	return nil
}

// uploadToRclone uploads the local directory at localPath to the configured rclone remote.
// If cfg.RcloneEnabled is false, the function returns immediately without error.
// The function uses cfg.RcloneTransfers (default 4) for parallel transfers and can
// optionally delete local files after successful upload verification if cfg.DeleteAfterUpload is true.
// Returns an error if:
//   - Path validation fails
//   - rclone copy command fails
//   - Upload verification fails (requires rclone check command)
//   - Local file deletion fails after successful upload
func uploadToRclone(localPath string, artistFolder string, cfg *Config) error {
	if !cfg.RcloneEnabled {
		return nil
	}

	// Validate paths before executing rclone command
	if err := validatePath(localPath); err != nil {
		return fmt.Errorf("invalid local path: %w", err)
	}

	// Validate artist folder path if provided
	if artistFolder != "" {
		if err := validatePath(artistFolder); err != nil {
			return fmt.Errorf("invalid artist folder: %w", err)
		}
	}

	// Default to 4 transfers if not set
	transfers := cfg.RcloneTransfers
	if transfers == 0 {
		transfers = 4
	}

	cmd, remoteFullPath, err := buildRcloneUploadCommand(localPath, artistFolder, cfg, transfers)
	if err != nil {
		return err
	}

	printUpload(fmt.Sprintf("Uploading to %s%s%s...", colorBold, remoteFullPath, colorReset))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("rclone upload failed: %w", err)
	}

	printSuccess("Upload complete!")

	if cfg.DeleteAfterUpload {
		// Verify upload before deleting local files
		printInfo("Verifying upload integrity...")
		verifyCmd, err := buildRcloneVerifyCommand(localPath, remoteFullPath)
		if err != nil {
			return fmt.Errorf("failed to build upload verification command: %w", err)
		}
		var verifyOut, verifyErr bytes.Buffer
		verifyCmd.Stdout = &verifyOut
		verifyCmd.Stderr = &verifyErr

		err = verifyCmd.Run()
		if err != nil {
			return fmt.Errorf("upload verification failed - NOT deleting local files: %w\nOutput: %s\nErrors: %s",
				err, verifyOut.String(), verifyErr.String())
		}

		printSuccess("Upload verified successfully")
		fmt.Printf("Deleting local files: %s\n", localPath)
		err = os.RemoveAll(localPath)
		if err != nil {
			return fmt.Errorf("failed to delete local files: %w", err)
		}
		printSuccess("Local files deleted")
	}

	return nil
}

func buildRcloneUploadCommand(localPath, artistFolder string, cfg *Config, transfers int) (*exec.Cmd, string, error) {
	remoteDest := cfg.RcloneRemote + ":" + cfg.RclonePath

	localInfo, err := os.Stat(localPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to stat local path: %w", err)
	}

	remoteParentPath := remoteDest
	if artistFolder != "" {
		remoteParentPath += "/" + artistFolder
	}
	remoteFullPath := remoteParentPath + "/" + filepath.Base(localPath)

	transfersFlag := fmt.Sprintf("--transfers=%d", transfers)
	if localInfo.IsDir() {
		return exec.Command("rclone", "copy", localPath, remoteFullPath, "-P", transfersFlag), remoteFullPath, nil
	}
	return exec.Command("rclone", "copyto", localPath, remoteFullPath, "-P", transfersFlag), remoteFullPath, nil
}

func buildRcloneVerifyCommand(localPath, remoteFullPath string) (*exec.Cmd, error) {
	localInfo, err := os.Stat(localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat local path for verification: %w", err)
	}

	if localInfo.IsDir() {
		return exec.Command("rclone", "check", "--one-way", localPath, remoteFullPath), nil
	}

	// rclone check operates on directories; for file uploads, compare parent dirs
	// and constrain comparison to the uploaded file only.
	localDir := filepath.Dir(localPath)
	remoteDir := path.Dir(remoteFullPath)
	fileName := filepath.Base(localPath)
	return exec.Command("rclone", "check", "--one-way", "--include", fileName, localDir, remoteDir), nil
}

// remotePathExists checks if a directory exists at the specified remotePath on the configured rclone remote.
// The remotePath is relative to cfg.RclonePath and should not include the remote name or base path.
// Returns false without error if cfg.RcloneEnabled is false.
// Returns true if the directory exists, false if it doesn't exist or on error.
func remotePathExists(remotePath string, cfg *Config) (bool, error) {
	if !cfg.RcloneEnabled {
		return false, nil
	}

	// Validate paths before executing rclone command
	if err := validatePath(remotePath); err != nil {
		return false, fmt.Errorf("invalid remote path: %w", err)
	}

	remoteDest := cfg.RcloneRemote + ":" + cfg.RclonePath
	fullPath := remoteDest + "/" + remotePath

	cmd := exec.Command("rclone", "lsf", fullPath, "--dirs-only")
	output, err := cmd.Output()

	if err != nil {
		// Check exit code to distinguish "doesn't exist" from other errors
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Exit code 3 means "directory not found" - this is expected
			if exitErr.ExitCode() == 3 {
				return false, nil
			}
			// Other exit codes indicate real errors
			return false, fmt.Errorf("rclone error checking remote path (exit %d): %w", exitErr.ExitCode(), err)
		}
		// Non-exit errors (e.g., rclone not found) are real errors
		return false, fmt.Errorf("failed to execute rclone: %w", err)
	}

	// If output is not empty, directory exists
	return len(output) > 0, nil
}

// listRemoteArtistFolders returns show folder names under one artist folder on remote storage.
func listRemoteArtistFolders(artistFolder string, cfg *Config) (map[string]struct{}, error) {
	folders := make(map[string]struct{})
	if !cfg.RcloneEnabled {
		return folders, nil
	}

	if err := validatePath(artistFolder); err != nil {
		return nil, fmt.Errorf("invalid artist folder: %w", err)
	}

	remoteDest := cfg.RcloneRemote + ":" + cfg.RclonePath
	fullPath := remoteDest + "/" + artistFolder

	cmd := exec.Command("rclone", "lsf", fullPath, "--dirs-only")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 3 {
			return folders, nil
		}
		return nil, fmt.Errorf("failed to list remote artist folders: %w", err)
	}

	for _, line := range strings.Split(string(output), "\n") {
		trimmed := strings.TrimSuffix(strings.TrimSpace(line), "/")
		if trimmed == "" {
			continue
		}
		folders[trimmed] = struct{}{}
	}

	return folders, nil
}

func parseCfg() (*Config, error) {
	cfg, err := readConfig()
	if err != nil {
		return nil, err
	}
	args := parseArgs()
	if args.Format != -1 {
		cfg.Format = args.Format
	}
	if args.VideoFormat != -1 {
		cfg.VideoFormat = args.VideoFormat
	}
	if cfg.Format < 1 || cfg.Format > 5 {
		return nil, errors.New("track Format must be between 1 and 5")
	}
	if cfg.VideoFormat < 1 || cfg.VideoFormat > 5 {
		return nil, errors.New("video format must be between 1 and 5")
	}
	cfg.WantRes = resolveRes[cfg.VideoFormat]
	if args.OutPath != "" {
		cfg.OutPath = args.OutPath
	}
	if cfg.OutPath == "" {
		cfg.OutPath = "Nugs downloads"
	}
	if cfg.Token != "" {
		cfg.Token = strings.TrimPrefix(cfg.Token, "Bearer ")
	}
	if cfg.UseFfmpegEnvVar {
		cfg.FfmpegNameStr = "ffmpeg"
	} else {
		cfg.FfmpegNameStr = "./ffmpeg"
	}
	cfg.Urls, err = processUrls(args.Urls)
	if err != nil {
		printError("Failed to process URLs")
		return nil, err
	}
	cfg.ForceVideo = args.ForceVideo
	cfg.SkipVideos = args.SkipVideos
	cfg.SkipChapters = args.SkipChapters
	return cfg, nil
}

func isShowCountFilterToken(s string) bool {
	re := regexp.MustCompile(`^(>=|<=|>|<|=)\d+$`)
	return re.MatchString(s)
}

// normalizeCliAliases maps the updated short command syntax to internal command routing.
func normalizeCliAliases(urls []string) []string {
	if len(urls) == 0 {
		return urls
	}

	switch urls[0] {
	case "list":
		// nugs list -> nugs list artists
		if len(urls) == 1 {
			return []string{"list", "artists"}
		}
		// nugs list >100 -> nugs list artists shows >100
		if len(urls) == 2 && isShowCountFilterToken(urls[1]) {
			return []string{"list", "artists", "shows", urls[1]}
		}
		// nugs list <artist_id> "Venue Name" -> nugs list <artist_id> shows "Venue Name"
		if len(urls) >= 3 {
			if _, err := strconv.Atoi(urls[1]); err == nil && urls[2] != "shows" && urls[2] != "latest" {
				normalized := []string{"list", urls[1], "shows"}
				normalized = append(normalized, urls[2:]...)
				return normalized
			}
		}
	case "grab":
		// nugs grab <artist_id> latest -> nugs <artist_id> latest
		if len(urls) >= 3 {
			if _, err := strconv.Atoi(urls[1]); err == nil && urls[2] == "latest" {
				return append([]string{urls[1], "latest"}, urls[3:]...)
			}
		}
	case "update", "cache", "stats", "latest", "gaps", "coverage":
		// Top-level catalog aliases, e.g. nugs gaps 1125 -> nugs catalog gaps 1125
		return append([]string{"catalog", urls[0]}, urls[1:]...)
	case "refresh":
		// nugs refresh enable|disable|set -> nugs catalog config enable|disable|set
		return append([]string{"catalog", "config"}, urls[1:]...)
	}

	return urls
}

func readConfig() (*Config, error) {
	// Try config locations in order: ./config.json, ~/.nugs/config.json, ~/.config/nugs/config.json
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configPaths := []string{
		"config.json",
		filepath.Join(homeDir, ".nugs", "config.json"),
		filepath.Join(homeDir, ".config", "nugs", "config.json"),
	}

	var data []byte
	var configPath string
	var lastErr error

	for _, path := range configPaths {
		data, err = os.ReadFile(path)
		if err == nil {
			configPath = path
			break
		}
		lastErr = err
	}

	if data == nil {
		return nil, fmt.Errorf("config file not found in any location (./config.json, ~/.nugs/config.json, ~/.config/nugs/config.json): %w", lastErr)
	}

	var obj Config
	err = json.Unmarshal(data, &obj)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config at %s: %w", configPath, err)
	}

	// Track which config file was loaded for writeConfig
	loadedConfigPath = configPath

	// Security warning for config file permissions
	fileInfo, err := os.Stat(configPath)
	if err == nil {
		mode := fileInfo.Mode()
		if mode.Perm()&0077 != 0 {
			fmt.Fprintf(os.Stderr, "%s WARNING: Config file has insecure permissions (%04o)\n", colorYellow+symbolWarning+colorReset, mode.Perm())
			fmt.Fprintf(os.Stderr, "   File: %s\n", configPath)
			fmt.Fprintf(os.Stderr, "   Risk: Config contains credentials and should only be readable by you\n")
			fmt.Fprintf(os.Stderr, "   Fix:  chmod 600 %s\n\n", configPath)
		}
	}

	return &obj, nil
}

func parseArgs() *Args {
	var args Args
	arg.MustParse(&args)
	return &args
}

func makeDirs(path string) error {
	err := os.MkdirAll(path, 0755)
	return err
}

func fileExists(path string) (bool, error) {
	f, err := os.Stat(path)
	if err == nil {
		return !f.IsDir(), nil
	} else if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func sanitise(filename string) string {
	san := regexp.MustCompile(sanRegexStr).ReplaceAllString(filename, "_")
	return strings.TrimSuffix(san, "	")
}

// buildAlbumFolderName constructs a sanitized folder name for an album
// from artist name and container info. This ensures consistent naming
// across all download and gap detection logic.
// maxLen parameter allows customizing the length limit (default 120 for albums, 110 for videos).
func buildAlbumFolderName(artistName, containerInfo string, maxLen ...int) string {
	limit := 120
	if len(maxLen) > 0 && maxLen[0] > 0 {
		limit = maxLen[0]
	}
	albumFolder := artistName + " - " + strings.TrimRight(containerInfo, " ")
	runes := []rune(albumFolder)
	if len(runes) > limit {
		albumFolder = string(runes[:limit])
	}
	return sanitise(albumFolder)
}

func auth(email, pwd string) (string, error) {
	data := url.Values{}
	data.Set("client_id", clientId)
	data.Set("grant_type", "password")
	data.Set("scope", "openid profile email nugsnet:api nugsnet:legacyapi offline_access")
	data.Set("username", email)
	data.Set("password", pwd)
	req, err := http.NewRequest(http.MethodPost, authUrl, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Add("User-Agent", userAgent)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	do, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return "", errors.New(do.Status)
	}
	var obj Auth
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return "", err
	}
	return obj.AccessToken, nil
}

func getUserInfo(token string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, userInfoUrl, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("User-Agent", userAgent)
	do, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return "", errors.New(do.Status)
	}
	var obj UserInfo
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return "", err
	}
	return obj.Sub, nil
}

func getSubInfo(token string) (*SubInfo, error) {
	req, err := http.NewRequest(http.MethodGet, subInfoUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("User-Agent", userAgent)
	do, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return nil, errors.New(do.Status)
	}
	var obj SubInfo
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return nil, err
	}
	return &obj, nil
}

func getPlan(subInfo *SubInfo) (string, bool) {
	if !reflect.ValueOf(subInfo.Plan).IsZero() {
		return subInfo.Plan.Description, false
	} else {
		return subInfo.Promo.Plan.Description, true
	}
}

func parseTimestamps(start, end string) (string, string) {
	startTime, _ := time.Parse(layout, start)
	endTime, _ := time.Parse(layout, end)
	parsedStart := strconv.FormatInt(startTime.Unix(), 10)
	parsedEnd := strconv.FormatInt(endTime.Unix(), 10)
	return parsedStart, parsedEnd
}

func parseStreamParams(userId string, subInfo *SubInfo, isPromo bool) *StreamParams {
	startStamp, endStamp := parseTimestamps(subInfo.StartedAt, subInfo.EndsAt)
	streamParams := &StreamParams{
		SubscriptionID:          subInfo.LegacySubscriptionID,
		SubCostplanIDAccessList: subInfo.Plan.PlanID,
		UserID:                  userId,
		StartStamp:              startStamp,
		EndStamp:                endStamp,
	}
	if isPromo {
		streamParams.SubCostplanIDAccessList = subInfo.Promo.Plan.PlanID
	} else {
		streamParams.SubCostplanIDAccessList = subInfo.Plan.PlanID
	}
	return streamParams
}

func checkUrl(_url string) (string, int) {
	for i, regexStr := range regexStrings {
		regex := regexp.MustCompile(regexStr)
		match := regex.FindStringSubmatch(_url)
		if match != nil {
			return match[1], i
		}
	}
	return "", 0
}

func extractLegToken(tokenStr string) (string, string, error) {
	payload := strings.SplitN(tokenStr, ".", 3)[1]
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return "", "", err
	}
	var obj Payload
	err = json.Unmarshal(decoded, &obj)
	if err != nil {
		return "", "", err
	}
	return obj.LegacyToken, obj.LegacyUguid, nil
}

func getAlbumMeta(albumId string) (*AlbumMeta, error) {
	req, err := http.NewRequest(http.MethodGet, streamApiBase+"api.aspx", nil)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("method", "catalog.container")
	query.Set("containerID", albumId)
	query.Set("vdisp", "1")
	req.URL.RawQuery = query.Encode()
	req.Header.Add("User-Agent", userAgent)
	do, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return nil, errors.New(do.Status)
	}
	var obj AlbumMeta
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return nil, err
	}
	return &obj, nil
}

func getPlistMeta(plistId, email, legacyToken string, cat bool) (*PlistMeta, error) {
	var path string
	if cat {
		path = "api.aspx"
	} else {
		path = "secureApi.aspx"
	}
	req, err := http.NewRequest(http.MethodGet, streamApiBase+path, nil)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	if cat {
		query.Set("method", "catalog.playlist")
		query.Set("plGUID", plistId)
	} else {
		query.Set("method", "user.playlist")
		query.Set("playlistID", plistId)
		query.Set("developerKey", devKey)
		query.Set("user", email)
		query.Set("token", legacyToken)
	}
	req.URL.RawQuery = query.Encode()
	req.Header.Add("User-Agent", userAgentTwo)
	do, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return nil, errors.New(do.Status)
	}
	var obj PlistMeta
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return nil, err
	}
	return &obj, nil
}

func getLatestCatalog() (*LatestCatalogResp, error) {
	req, err := http.NewRequest(http.MethodGet, streamApiBase+"api.aspx", nil)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("method", "catalog.latest")
	query.Set("vdisp", "1")
	req.URL.RawQuery = query.Encode()
	req.Header.Add("User-Agent", userAgentTwo)
	do, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return nil, errors.New(do.Status)
	}
	var obj LatestCatalogResp
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return nil, err
	}
	return &obj, nil
}

func getArtistMeta(artistId string) ([]*ArtistMeta, error) {
	var allArtistMeta []*ArtistMeta
	offset := 1
	query := url.Values{}
	query.Set("method", "catalog.containersAll")
	query.Set("limit", "100")
	query.Set("artistList", artistId)
	query.Set("availType", "1")
	query.Set("vdisp", "1")
	for {
		req, err := http.NewRequest(http.MethodGet, streamApiBase+"api.aspx", nil)
		if err != nil {
			return nil, err
		}
		query.Set("startOffset", strconv.Itoa(offset))
		req.URL.RawQuery = query.Encode()
		req.Header.Add("User-Agent", userAgent)
		do, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		if do.StatusCode != http.StatusOK {
			do.Body.Close()
			return nil, errors.New(do.Status)
		}
		var obj ArtistMeta
		err = json.NewDecoder(do.Body).Decode(&obj)
		do.Body.Close()
		if err != nil {
			return nil, err
		}
		retLen := len(obj.Response.Containers)
		if retLen == 0 {
			break
		}
		allArtistMeta = append(allArtistMeta, &obj)
		offset += retLen
	}
	return allArtistMeta, nil
}

// getArtistMetaCached returns artist metadata from a local cache when fresh.
// If cache is stale or missing, it refreshes from API and rewrites cache.
// If refresh fails and stale cache exists, stale cache is returned.
func getArtistMetaCached(artistID string, ttl time.Duration) (pages []*ArtistMeta, cacheUsed bool, cacheStaleUse bool, err error) {
	cachedPages, cachedAt, readErr := readArtistMetaCache(artistID)
	if readErr == nil && len(cachedPages) > 0 {
		if time.Since(cachedAt) <= ttl {
			return cachedPages, true, false, nil
		}
	}

	freshPages, fetchErr := getArtistMeta(artistID)
	if fetchErr == nil {
		_ = writeArtistMetaCache(artistID, freshPages)
		return freshPages, false, false, nil
	}

	if readErr == nil && len(cachedPages) > 0 {
		return cachedPages, true, true, nil
	}

	return nil, false, false, fetchErr
}

func getArtistMetaCachePath(artistID string) (string, error) {
	cacheDir, err := getCacheDir()
	if err != nil {
		return "", err
	}
	artistsDir := filepath.Join(cacheDir, "artists")
	if err := os.MkdirAll(artistsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create artist cache directory: %w", err)
	}
	return filepath.Join(artistsDir, fmt.Sprintf("artist_%s.json", artistID)), nil
}

func readArtistMetaCache(artistID string) ([]*ArtistMeta, time.Time, error) {
	cachePath, err := getArtistMetaCachePath(artistID)
	if err != nil {
		return nil, time.Time{}, err
	}
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, time.Time{}, err
	}
	var cached ArtistMetaCache
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to parse artist cache: %w", err)
	}
	return cached.Pages, cached.CachedAt, nil
}

func writeArtistMetaCache(artistID string, pages []*ArtistMeta) error {
	cachePath, err := getArtistMetaCachePath(artistID)
	if err != nil {
		return err
	}

	cached := ArtistMetaCache{
		ArtistID: artistID,
		CachedAt: time.Now(),
		Pages:    pages,
	}
	data, err := json.MarshalIndent(cached, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal artist cache: %w", err)
	}

	tmpPath := cachePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp artist cache: %w", err)
	}
	if err := os.Rename(tmpPath, cachePath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to rename artist cache: %w", err)
	}
	return nil
}

func getArtistList() (*ArtistListResp, error) {
	req, err := http.NewRequest(http.MethodGet, streamApiBase+"api.aspx", nil)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("method", "catalog.artists")
	query.Set("vdisp", "1")
	req.URL.RawQuery = query.Encode()
	req.Header.Add("User-Agent", userAgentTwo)
	do, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return nil, errors.New(do.Status)
	}
	var obj ArtistListResp
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return nil, err
	}
	return &obj, nil
}

func getPurchasedManUrl(skuID int, showID, userID, uguID string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, streamApiBase+"bigriver/vidPlayer.aspx", nil)
	if err != nil {
		return "", err
	}
	query := url.Values{}
	query.Set("skuId", strconv.Itoa(skuID))
	query.Set("showId", showID)
	query.Set("uguid", uguID)
	query.Set("nn_userID", userID)
	query.Set("app", "1")
	req.URL.RawQuery = query.Encode()
	req.Header.Add("User-Agent", userAgentTwo)
	do, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return "", errors.New(do.Status)
	}
	var obj PurchasedManResp
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return "", err
	}
	return obj.FileURL, nil
}

func getStreamMeta(trackId, skuId, format int, streamParams *StreamParams) (string, error) {
	req, err := http.NewRequest(http.MethodGet, streamApiBase+"bigriver/subPlayer.aspx", nil)
	if err != nil {
		return "", err
	}
	query := url.Values{}
	if format == 0 {
		query.Set("skuId", strconv.Itoa(skuId))
		query.Set("containerID", strconv.Itoa(trackId))
		query.Set("chap", "1")
	} else {
		query.Set("platformID", strconv.Itoa(format))
		query.Set("trackID", strconv.Itoa(trackId))
	}
	query.Set("app", "1")
	query.Set("subscriptionID", streamParams.SubscriptionID)
	query.Set("subCostplanIDAccessList", streamParams.SubCostplanIDAccessList)
	query.Set("nn_userID", streamParams.UserID)
	query.Set("startDateStamp", streamParams.StartStamp)
	query.Set("endDateStamp", streamParams.EndStamp)
	req.URL.RawQuery = query.Encode()
	req.Header.Add("User-Agent", userAgentTwo)
	do, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return "", errors.New(do.Status)
	}
	var obj StreamMeta
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return "", err
	}
	return obj.StreamLink, nil
}

func queryQuality(streamUrl string) *Quality {
	for k, v := range qualityMap {
		if strings.Contains(streamUrl, k) {
			v.URL = streamUrl
			return &v
		}
	}
	return nil
}

func downloadTrack(trackPath, _url string) error {
	f, err := os.OpenFile(trackPath, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer f.Close()
	req, err := http.NewRequest(http.MethodGet, _url, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Referer", playerUrl)
	req.Header.Add("User-Agent", userAgent)
	req.Header.Add("Range", "bytes=0-")
	do, err := client.Do(req)
	if err != nil {
		return err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK && do.StatusCode != http.StatusPartialContent {
		return errors.New(do.Status)
	}
	totalBytes := do.ContentLength
	counter := &WriteCounter{
		Total:     totalBytes,
		TotalStr:  humanize.Bytes(uint64(totalBytes)),
		StartTime: time.Now().UnixMilli(),
	}
	_, err = io.Copy(f, io.TeeReader(do.Body, counter))
	fmt.Println("")
	return err
}

func getTrackQual(quals []*Quality, wantFmt int) *Quality {
	for _, quality := range quals {
		if quality.Format == wantFmt {
			return quality
		}
	}
	return nil
}

func extractBitrate(manUrl string) string {
	regex := regexp.MustCompile(bitrateRegex)
	match := regex.FindStringSubmatch(manUrl)
	if match != nil {
		return match[1]
	}
	return ""
}

func parseHlsMaster(qual *Quality) error {
	req, err := client.Get(qual.URL)
	if err != nil {
		return err
	}
	defer req.Body.Close()
	if req.StatusCode != http.StatusOK {
		return errors.New(req.Status)
	}

	playlist, _, err := m3u8.DecodeFrom(req.Body, true)
	if err != nil {
		return err
	}
	master := playlist.(*m3u8.MasterPlaylist)
	sort.Slice(master.Variants, func(x, y int) bool {
		return master.Variants[x].Bandwidth > master.Variants[y].Bandwidth
	})
	variantUri := master.Variants[0].URI
	bitrate := extractBitrate(variantUri)
	if bitrate == "" {
		return errors.New("no regex match for manifest bitrate")
	}
	qual.Specs = bitrate + " Kbps AAC"
	manBase, q, err := getManifestBase(qual.URL)
	if err != nil {
		return err
	}
	qual.URL = manBase + variantUri + q
	return nil
}
func getKey(keyUrl string) ([]byte, error) {
	req, err := client.Get(keyUrl)
	if err != nil {
		return nil, err
	}
	defer req.Body.Close()
	if req.StatusCode != http.StatusOK {
		return nil, errors.New(req.Status)
	}
	buf := make([]byte, 16)
	_, err = io.ReadFull(req.Body, buf)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// func decryptTrack(key, iv []byte, inPath, outPath string) error {
// 	var stream cipher.Stream
// 	printInfo("Decrypting...")
// 	in_f, err := os.Open(inPath)
// 	if err != nil {
// 		return err
// 	}

// 	block, err := aes.NewCipher([]byte(key))
// 	if err != nil {
// 		in_f.Close()
// 		return err
// 	}
// 	stream = cipher.NewCTR(block, []byte(iv))
// 	reader := &cipher.StreamReader{S: stream, R: in_f}
// 	out_f, err := os.Create(outPath)
// 	if err != nil {
// 		in_f.Close()
// 		return err
// 	}
// 	defer out_f.Close()
// 	_, err = io.Copy(out_f, reader)
// 	if err != nil {
// 		in_f.Close()
// 		return err
// 	}
// 	in_f.Close()
// 	err = os.Remove(inPath)
// 	if err != nil {
// 		fmt.Println("Failed to delete encrypted track.")
// 	}
// 	return nil
// }

func pkcs5Trimming(data []byte) []byte {
	padding := data[len(data)-1]
	return data[:len(data)-int(padding)]
}

func decryptTrack(key, iv []byte) ([]byte, error) {
	encData, err := os.ReadFile("temp_enc.ts")
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	ecb := cipher.NewCBCDecrypter(block, iv)
	decrypted := make([]byte, len(encData))
	printInfo("Decrypting...")
	ecb.CryptBlocks(decrypted, encData)
	return decrypted, nil
}

func tsToAac(decData []byte, outPath, ffmpegNameStr string) error {
	var errBuffer bytes.Buffer
	cmd := exec.Command(ffmpegNameStr, "-i", "pipe:", "-c:a", "copy", outPath)
	cmd.Stdin = bytes.NewReader(decData)
	cmd.Stderr = &errBuffer
	err := cmd.Run()
	if err != nil {
		errString := fmt.Sprintf("%s\n%s", err, errBuffer.String())
		return errors.New(errString)
	}
	return nil
}

func hlsOnly(trackPath, manUrl, ffmpegNameStr string) error {
	req, err := client.Get(manUrl)
	if err != nil {
		return err
	}
	defer req.Body.Close()
	if req.StatusCode != http.StatusOK {
		return errors.New(req.Status)
	}
	playlist, _, err := m3u8.DecodeFrom(req.Body, true)
	if err != nil {
		return err
	}
	media := playlist.(*m3u8.MediaPlaylist)

	manBase, q, err := getManifestBase(manUrl)
	if err != nil {
		return err
	}
	tsUrl := manBase + media.Segments[0].URI + q

	key := media.Key
	keyBytes, err := getKey(manBase + key.URI)
	if err != nil {
		return err
	}

	iv, err := hex.DecodeString(key.IV[2:])
	if err != nil {
		return err
	}

	err = downloadTrack("temp_enc.ts", tsUrl)
	if err != nil {
		return err
	}
	decData, err := decryptTrack(keyBytes, iv)
	if err != nil {
		return err
	}
	err = os.Remove("temp_enc.ts")
	if err != nil {
		return err
	}
	err = tsToAac(decData, trackPath, ffmpegNameStr)
	return err
}

func checkIfHlsOnly(quals []*Quality) bool {
	for _, quality := range quals {
		if !strings.Contains(quality.URL, ".m3u8?") {
			return false
		}
	}
	return true
}

func processTrack(folPath string, trackNum, trackTotal int, cfg *Config, track *Track, streamParams *StreamParams) error {
	origWantFmt := cfg.Format
	wantFmt := origWantFmt
	var (
		quals      []*Quality
		chosenQual *Quality
	)
	// Call the stream meta endpoint four times to get all avail formats since the formats can shift.
	// This will ensure the right format's always chosen.
	for _, i := range [4]int{1, 4, 7, 10} {
		streamUrl, err := getStreamMeta(track.TrackID, 0, i, streamParams)
		if err != nil {
			printError("Failed to get track stream metadata")
			return err
		} else if streamUrl == "" {
			return errors.New("the api didn't return a track stream URL")
		}
		quality := queryQuality(streamUrl)
		if quality == nil {
			printError(fmt.Sprintf("The API returned an unsupported format: %s", streamUrl))
			continue
			//return errors.New("The API returned an unsupported format.")
		}
		quals = append(quals, quality)
		// if quality.Format == 6 {
		// 	isHlsOnly = true
		// 	break
		// }
	}

	if len(quals) == 0 {
		return errors.New("the api didn't return any formats")
	}

	isHlsOnly := checkIfHlsOnly(quals)

	if isHlsOnly {
		printInfo("HLS-only track. Only AAC is available, tags currently unsupported")
		chosenQual = quals[0]
		err := parseHlsMaster(chosenQual)
		if err != nil {
			return err
		}
	} else {
		for {
			chosenQual = getTrackQual(quals, wantFmt)
			if chosenQual != nil {
				break
			} else {
				// Fallback quality.
				wantFmt = trackFallback[wantFmt]
			}
		}
		// chosenQual is guaranteed non-nil after loop exit
		if wantFmt != origWantFmt && origWantFmt != 4 {
			printInfo("Unavailable in your chosen format")
		}
	}
	trackFname := fmt.Sprintf(
		"%02d. %s%s", trackNum, sanitise(track.SongTitle), chosenQual.Extension,
	)
	trackPath := filepath.Join(folPath, trackFname)
	exists, err := fileExists(trackPath)
	if err != nil {
		printError("Failed to check if track already exists locally")
		return err
	}
	if exists {
		printInfo(fmt.Sprintf("Track exists %s skipping", symbolArrow))
		return nil
	}
	printDownload(fmt.Sprintf("Track %d/%d: %s%s%s - %s",
		trackNum, trackTotal, colorBold, track.SongTitle, colorReset, chosenQual.Specs))
	if isHlsOnly {
		err = hlsOnly(trackPath, chosenQual.URL, cfg.FfmpegNameStr)
	} else {
		err = downloadTrack(trackPath, chosenQual.URL)
	}
	if err != nil {
		printError("Failed to download track")
		return err
	}
	return nil
}

// album downloads an album or show from Nugs.net using the provided albumID.
// If albumID is empty, uses the provided artResp metadata instead of fetching it.
// The function creates artist and album directories, downloads all tracks, and optionally
// uploads to rclone if configured. Skips download if the show already exists locally or on remote.
// Returns an error if metadata fetching, directory creation, or any track download fails.
func album(albumID string, cfg *Config, streamParams *StreamParams, artResp *AlbArtResp) error {
	var (
		meta   *AlbArtResp
		tracks []Track
	)
	if albumID == "" {
		meta = artResp
		tracks = meta.Songs
	} else {
		_meta, err := getAlbumMeta(albumID)
		if err != nil {
			printError("Failed to get metadata")
			return err
		}
		meta = _meta.Response
		tracks = meta.Tracks
	}

	trackTotal := len(tracks)

	skuID := getVideoSku(meta.Products)

	if skuID == 0 && trackTotal < 1 {
		return errors.New("release has no tracks or videos")
	}

	if skuID != 0 {
		if cfg.SkipVideos {
			printInfo("Video-only album, skipped")
			return nil
		}
		if cfg.ForceVideo || trackTotal < 1 {
			return video(albumID, "", cfg, streamParams, meta, false)
		}
	}
	// Create artist directory
	artistFolder := sanitise(meta.ArtistName)
	artistPath := filepath.Join(cfg.OutPath, artistFolder)
	err := makeDirs(artistPath)
	if err != nil {
		printError("Failed to make artist folder")
		return err
	}

	albumFolder := buildAlbumFolderName(meta.ArtistName, meta.ContainerInfo)
	fmt.Println(albumFolder)
	if len([]rune(meta.ArtistName+" - "+strings.TrimRight(meta.ContainerInfo, " "))) > 120 {
		fmt.Println(
			"Album folder name was chopped because it exceeds 120 characters.")
	}
	albumPath := filepath.Join(artistPath, albumFolder)

	// Check if show already exists locally
	if stat, err := os.Stat(albumPath); err == nil && stat.IsDir() {
		printInfo(fmt.Sprintf("Show already exists locally %s skipping", symbolArrow))
		return nil
	}

	// Check if show already exists on remote
	remoteShowPath := artistFolder + "/" + albumFolder
	printInfo(fmt.Sprintf("Checking remote for: %s%s%s", colorCyan, albumFolder, colorReset))
	exists, err := remotePathExists(remoteShowPath, cfg)
	if err != nil {
		printWarning(fmt.Sprintf("Failed to check remote: %v", err))
		// Continue with download even if remote check fails
	} else if exists {
		printSuccess(fmt.Sprintf("Show found on remote %s skipping", symbolArrow))
		return nil
	}

	err = makeDirs(albumPath)
	if err != nil {
		printError("Failed to make album folder")
		return err
	}
	for trackNum, track := range tracks {
		trackNum++
		err := processTrack(
			albumPath, trackNum, trackTotal, cfg, &track, streamParams)
		if err != nil {
			handleErr("Track failed.", err, false)
		}
	}

	// Upload to rclone if enabled
	if cfg.RcloneEnabled {
		err = uploadToRclone(albumPath, artistFolder, cfg)
		if err != nil {
			handleErr("Upload failed.", err, false)
		}
	}

	return nil
}

func getAlbumTotal(meta []*ArtistMeta) int {
	var total int
	for _, _meta := range meta {
		total += len(_meta.Response.Containers)
	}
	return total
}

func artist(artistId string, cfg *Config, streamParams *StreamParams) error {
	meta, err := getArtistMeta(artistId)
	if err != nil {
		printError("Failed to get artist metadata")
		return err
	}
	if len(meta) == 0 {
		return errors.New(
			"The API didn't return any artist metadata.")
	}
	fmt.Println(meta[0].Response.Containers[0].ArtistName)
	albumTotal := getAlbumTotal(meta)
	for _, _meta := range meta {
		for albumNum, container := range _meta.Response.Containers {
			fmt.Printf("Item %d of %d:\n", albumNum+1, albumTotal)
			if cfg.SkipVideos {
				err = album("", cfg, streamParams, container)
			} else {
				// Can't re-use this metadata as it doesn't have any product info for videos.
				err = album(strconv.Itoa(container.ContainerID), cfg, streamParams, nil)
			}
			if err != nil {
				handleErr("Item failed.", err, false)
			}
		}
	}
	return nil
}

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
		fmt.Sprintf("%snugs list artists%s - Browse all artists", colorCyan, colorReset),
		fmt.Sprintf("%snugs list 1125%s - View Billy Strings shows", colorCyan, colorReset),
		fmt.Sprintf("%snugs 1125 latest%s - Download latest shows", colorCyan, colorReset),
		fmt.Sprintf("%snugs list artists --json standard | jq%s - Export to JSON", colorCyan, colorReset),
		fmt.Sprintf("%snugs help%s - View all commands", colorCyan, colorReset),
	}
	printList(quickStartCommands, colorGreen)
	fmt.Println()

	return nil
}

// listArtistShows fetches and displays all shows for a specific artist identified by artistId.
// The output is sorted by date in reverse chronological order (newest first) and includes
// container ID, date, title, and venue for each show.
// Returns an error if the artist metadata cannot be fetched from the API.
func listArtistShows(artistId string, jsonLevel string) error {
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
	type containerWithDate struct {
		container *AlbArtResp
		dateStr   string
	}
	var allContainers []containerWithDate

	for _, meta := range allMeta {
		for _, container := range meta.Response.Containers {
			dateStr := container.PerformanceDateShortYearFirst
			if dateStr == "" {
				dateStr = container.PerformanceDate
			}
			allContainers = append(allContainers, containerWithDate{
				container: container,
				dateStr:   dateStr,
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
	// Empty dates go to the end
	sort.Slice(allContainers, func(i, j int) bool {
		dateI := allContainers[i].dateStr
		dateJ := allContainers[j].dateStr

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
				shows[i] = item.container
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
					ContainerID: item.container.ContainerID,
					Date:        item.dateStr,
					Title:       item.container.ContainerInfo,
					Venue:       item.container.VenueName,
				}

				// Standard level includes location details
				if jsonLevel == JSONLevelStandard {
					show.VenueCity = item.container.VenueCity
					show.VenueState = item.container.VenueState
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
		printSection(fmt.Sprintf("%s - %d shows", artistName, len(allContainers)))

		table := NewTable([]TableColumn{
			{Header: "ID", Width: 10, Align: "left"},
			{Header: "Date", Width: 12, Align: "left"},
			{Header: "Title", Width: 45, Align: "left"},
			{Header: "Venue", Width: 30, Align: "left"},
		})

		for _, item := range allContainers {
			container := item.container
			table.AddRow(
				strconv.Itoa(container.ContainerID),
				item.dateStr,
				container.ContainerInfo,
				container.VenueName,
			)
		}

		table.Print()
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
	type containerWithDate struct {
		container *AlbArtResp
		dateStr   string
	}
	var filteredContainers []containerWithDate
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
				filteredContainers = append(filteredContainers, containerWithDate{
					container: container,
					dateStr:   dateStr,
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
		dateI := filteredContainers[i].dateStr
		dateJ := filteredContainers[j].dateStr

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
				shows[i] = item.container
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
					ContainerID: item.container.ContainerID,
					Date:        item.dateStr,
					Title:       item.container.ContainerInfo,
					Venue:       item.container.VenueName,
				}

				// Standard level includes location details
				if jsonLevel == JSONLevelStandard {
					show.VenueCity = item.container.VenueCity
					show.VenueState = item.container.VenueState
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
			container := item.container
			table.AddRow(
				strconv.Itoa(container.ContainerID),
				item.dateStr,
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
	type containerWithDate struct {
		container *AlbArtResp
		dateStr   string
	}
	var allContainers []containerWithDate

	for _, meta := range allMeta {
		for _, container := range meta.Response.Containers {
			dateStr := container.PerformanceDateShortYearFirst
			if dateStr == "" {
				dateStr = container.PerformanceDate
			}
			allContainers = append(allContainers, containerWithDate{
				container: container,
				dateStr:   dateStr,
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
		dateI := allContainers[i].dateStr
		dateJ := allContainers[j].dateStr

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
				shows[i] = item.container
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
					ContainerID: item.container.ContainerID,
					Date:        item.dateStr,
					Title:       item.container.ContainerInfo,
					Venue:       item.container.VenueName,
				}

				// Standard level includes location details
				if jsonLevel == JSONLevelStandard {
					show.VenueCity = item.container.VenueCity
					show.VenueState = item.container.VenueState
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
				fmt.Sprintf("%d", item.container.ContainerID),
				item.dateStr,
				item.container.ContainerInfo,
				item.container.VenueName,
			)
		}

		table.Print()
		fmt.Printf("\n%s%s%s To download: %snugs <container_id>%s\n\n",
			colorCyan, symbolInfo, colorReset, colorBold, colorReset)
	}
	return nil
}

// getCacheDir returns the cache directory path, creating it if needed
func getCacheDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	cacheDir := filepath.Join(homeDir, ".cache", "nugs")
	err = os.MkdirAll(cacheDir, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}
	return cacheDir, nil
}

// readCacheMeta reads the cache metadata file
func readCacheMeta() (*CacheMeta, error) {
	cacheDir, err := getCacheDir()
	if err != nil {
		return nil, err
	}
	metaPath := filepath.Join(cacheDir, "catalog_meta.json")

	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No cache yet
		}
		return nil, fmt.Errorf("failed to read cache metadata: %w", err)
	}

	var meta CacheMeta
	err = json.Unmarshal(data, &meta)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cache metadata: %w", err)
	}
	return &meta, nil
}

// readCatalogCache reads the cached catalog data
func readCatalogCache() (*LatestCatalogResp, error) {
	cacheDir, err := getCacheDir()
	if err != nil {
		return nil, err
	}
	catalogPath := filepath.Join(cacheDir, "catalog.json")

	data, err := os.ReadFile(catalogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no cache found - run 'nugs catalog update' first")
		}
		return nil, fmt.Errorf("failed to read catalog cache: %w", err)
	}

	var catalog LatestCatalogResp
	err = json.Unmarshal(data, &catalog)
	if err != nil {
		return nil, fmt.Errorf("failed to parse catalog cache: %w", err)
	}
	return &catalog, nil
}

// writeCatalogCache writes the catalog and metadata to cache
// Uses file locking to prevent corruption from concurrent writes
func writeCatalogCache(catalog *LatestCatalogResp, updateDuration time.Duration) error {
	// Acquire lock for the entire cache write operation
	return WithCacheLock(func() error {
		cacheDir, err := getCacheDir()
		if err != nil {
			return err
		}

		// Write catalog.json atomically using temp file
		catalogPath := filepath.Join(cacheDir, "catalog.json")
		catalogData, err := json.MarshalIndent(catalog, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal catalog: %w", err)
		}

		// Write to temp file first
		tmpCatalogPath := catalogPath + ".tmp"
		err = os.WriteFile(tmpCatalogPath, catalogData, 0644)
		if err != nil {
			return fmt.Errorf("failed to write temp catalog: %w", err)
		}

		// Atomic rename
		err = os.Rename(tmpCatalogPath, catalogPath)
		if err != nil {
			_ = os.Remove(tmpCatalogPath) // Best effort cleanup
			return fmt.Errorf("failed to rename catalog: %w", err)
		}

		// Count unique artists
		artistSet := make(map[int]bool)
		for _, item := range catalog.Response.RecentItems {
			artistSet[item.ArtistID] = true
		}

		// Write catalog_meta.json atomically
		meta := CacheMeta{
			LastUpdated:    time.Now(),
			CacheVersion:   "v1.0.0",
			TotalShows:     len(catalog.Response.RecentItems),
			TotalArtists:   len(artistSet),
			ApiMethod:      "catalog.latest",
			UpdateDuration: formatDuration(updateDuration),
		}
		metaPath := filepath.Join(cacheDir, "catalog_meta.json")
		metaData, err := json.MarshalIndent(meta, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}

		tmpMetaPath := metaPath + ".tmp"
		err = os.WriteFile(tmpMetaPath, metaData, 0644)
		if err != nil {
			return fmt.Errorf("failed to write temp metadata: %w", err)
		}

		err = os.Rename(tmpMetaPath, metaPath)
		if err != nil {
			_ = os.Remove(tmpMetaPath) // Best effort cleanup
			return fmt.Errorf("failed to rename metadata: %w", err)
		}

		// Build indexes (also atomic)
		err = buildArtistIndex(catalog)
		if err != nil {
			return err
		}
		err = buildContainerIndex(catalog)
		if err != nil {
			return err
		}

		return nil
	})
}

// buildArtistIndex creates artist name → ID lookup index
// Uses atomic write (temp file + rename) to prevent corruption
func buildArtistIndex(catalog *LatestCatalogResp) error {
	cacheDir, err := getCacheDir()
	if err != nil {
		return err
	}

	index := make(map[string]int)
	for _, item := range catalog.Response.RecentItems {
		normalizedName := strings.ToLower(strings.TrimSpace(item.ArtistName))
		index[normalizedName] = item.ArtistID
	}

	artistIndex := ArtistsIndex{Index: index}
	indexPath := filepath.Join(cacheDir, "artists_index.json")
	indexData, err := json.MarshalIndent(artistIndex, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal artist index: %w", err)
	}

	// Atomic write via temp file
	tmpPath := indexPath + ".tmp"
	err = os.WriteFile(tmpPath, indexData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write temp artist index: %w", err)
	}

	err = os.Rename(tmpPath, indexPath)
	if err != nil {
		_ = os.Remove(tmpPath) // Best effort cleanup
		return fmt.Errorf("failed to rename artist index: %w", err)
	}

	return nil
}

// buildContainerIndex creates containerID → artist mapping
// Uses atomic write (temp file + rename) to prevent corruption
func buildContainerIndex(catalog *LatestCatalogResp) error {
	cacheDir, err := getCacheDir()
	if err != nil {
		return err
	}

	containers := make(map[int]ContainerIndexEntry)
	for _, item := range catalog.Response.RecentItems {
		containers[item.ContainerID] = ContainerIndexEntry{
			ArtistID:        item.ArtistID,
			ArtistName:      item.ArtistName,
			ContainerInfo:   item.ContainerInfo,
			PerformanceDate: item.PerformanceDateStr,
		}
	}

	containerIndex := ContainersIndex{Containers: containers}
	indexPath := filepath.Join(cacheDir, "containers_index.json")
	indexData, err := json.MarshalIndent(containerIndex, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal container index: %w", err)
	}

	// Atomic write via temp file
	tmpPath := indexPath + ".tmp"
	err = os.WriteFile(tmpPath, indexData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write temp container index: %w", err)
	}

	err = os.Rename(tmpPath, indexPath)
	if err != nil {
		_ = os.Remove(tmpPath) // Best effort cleanup
		return fmt.Errorf("failed to rename container index: %w", err)
	}

	return nil
}

// formatDuration formats a duration in human-readable form
func formatDuration(d time.Duration) string {
	seconds := d.Seconds()
	if seconds < 60 {
		return fmt.Sprintf("%.0f seconds", seconds)
	}
	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%.0f minutes", minutes)
	}
	hours := minutes / 60
	if hours < 24 {
		return fmt.Sprintf("%.1f hours", hours)
	}
	days := hours / 24
	return fmt.Sprintf("%.1f days", days)
}

func playlist(plistId, legacyToken string, cfg *Config, streamParams *StreamParams, cat bool) error {
	_meta, err := getPlistMeta(plistId, cfg.Email, legacyToken, cat)
	if err != nil {
		printError("Failed to get playlist metadata")
		return err
	}
	meta := _meta.Response
	plistName := meta.PlayListName
	fmt.Println(plistName)
	if len(plistName) > 120 {
		plistName = plistName[:120]
		fmt.Println(
			"Playlist folder name was chopped because it exceeds 120 characters.")
	}
	plistPath := filepath.Join(cfg.OutPath, sanitise(plistName))
	err = makeDirs(plistPath)
	if err != nil {
		printError("Failed to make playlist folder")
		return err
	}
	trackTotal := len(meta.Items)
	for trackNum, track := range meta.Items {
		trackNum++
		err := processTrack(
			plistPath, trackNum, trackTotal, cfg, &track.Track, streamParams)
		if err != nil {
			handleErr("Track failed.", err, false)
		}
	}

	// Upload to rclone if enabled
	if cfg.RcloneEnabled {
		// Playlists don't have artist folder structure
		err = uploadToRclone(plistPath, "", cfg)
		if err != nil {
			handleErr("Upload failed.", err, false)
		}
	}

	return nil
}

func getVideoSku(products []Product) int {
	for _, product := range products {
		formatStr := product.FormatStr
		if formatStr == "VIDEO ON DEMAND" || formatStr == "LIVE HD VIDEO" {
			return product.SkuID
		}
	}
	return 0
}

func getLstreamSku(products []*ProductFormatList) int {
	for _, product := range products {
		if product.FormatStr == "LIVE HD VIDEO" {
			return product.SkuID
		}
	}
	return 0
}

func getVidVariant(variants []*m3u8.Variant, wantRes string) *m3u8.Variant {
	for _, variant := range variants {
		if strings.HasSuffix(variant.Resolution, "x"+wantRes) {
			return variant
		}
	}
	return nil
}

func formatRes(res string) string {
	if res == "2160" {
		return "4K"
	} else {
		return res + "p"
	}
}

func chooseVariant(manifestUrl, wantRes string) (*m3u8.Variant, string, error) {
	origWantRes := wantRes
	var wantVariant *m3u8.Variant
	req, err := client.Get(manifestUrl)
	if err != nil {
		return nil, "", err
	}
	defer req.Body.Close()
	if req.StatusCode != http.StatusOK {
		return nil, "", errors.New(req.Status)
	}
	playlist, _, err := m3u8.DecodeFrom(req.Body, true)
	if err != nil {
		return nil, "", err
	}
	master := playlist.(*m3u8.MasterPlaylist)
	sort.Slice(master.Variants, func(x, y int) bool {
		return master.Variants[x].Bandwidth > master.Variants[y].Bandwidth
	})
	if wantRes == "2160" {
		variant := master.Variants[0]
		varRes := strings.SplitN(variant.Resolution, "x", 2)[1]
		varRes = formatRes(varRes)
		return variant, varRes, nil
	}
	for {
		wantVariant = getVidVariant(master.Variants, wantRes)
		if wantVariant != nil {
			break
		} else {
			wantRes = resFallback[wantRes]
		}
	}
	// wantVariant is guaranteed non-nil after loop exit
	if wantRes != origWantRes {
		printInfo("Unavailable in your chosen format")
	}
	wantRes = formatRes(wantRes)
	return wantVariant, wantRes, nil
}

func getManifestBase(manifestUrl string) (string, string, error) {
	u, err := url.Parse(manifestUrl)
	if err != nil {
		return "", "", err
	}
	path := u.Path
	lastPathIdx := strings.LastIndex(path, "/")
	base := u.Scheme + "://" + u.Host + path[:lastPathIdx+1]
	return base, "?" + u.RawQuery, nil
}

func getSegUrls(manifestUrl, query string) ([]string, error) {
	var segUrls []string
	req, err := client.Get(manifestUrl)
	if err != nil {
		return nil, err
	}
	defer req.Body.Close()
	if req.StatusCode != http.StatusOK {
		return nil, errors.New(req.Status)
	}
	playlist, _, err := m3u8.DecodeFrom(req.Body, true)
	if err != nil {
		return nil, err
	}
	media := playlist.(*m3u8.MediaPlaylist)
	for _, seg := range media.Segments {
		if seg == nil {
			break
		}
		segUrls = append(segUrls, seg.URI+query)
	}
	return segUrls, nil
}

func downloadVideo(videoPath, _url string) error {
	f, err := os.OpenFile(videoPath, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return err
	}
	startByte := stat.Size()

	req, err := http.NewRequest(http.MethodGet, _url, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Range", fmt.Sprintf("bytes=%d-", startByte))
	do, err := client.Do(req)
	if err != nil {
		return err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK && do.StatusCode != http.StatusPartialContent {
		return errors.New(do.Status)
	}

	if startByte > 0 {
		fmt.Printf("TS already exists locally, resuming from byte %d...\n", startByte)
		startByte = 0
	}

	totalBytes := do.ContentLength
	counter := &WriteCounter{
		Total:      totalBytes,
		TotalStr:   humanize.Bytes(uint64(totalBytes)),
		StartTime:  time.Now().UnixMilli(),
		Downloaded: startByte,
	}
	_, err = io.Copy(f, io.TeeReader(do.Body, counter))
	fmt.Println("")
	return err
}

func downloadLstream(videoPath, baseUrl string, segUrls []string) error {
	f, err := os.OpenFile(videoPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer f.Close()
	segTotal := len(segUrls)
	for segNum, segUrl := range segUrls {
		segNum++
		fmt.Printf("\rSegment %d of %d.", segNum, segTotal)
		req, err := http.NewRequest(http.MethodGet, baseUrl+segUrl, nil)
		if err != nil {
			return err
		}
		do, err := client.Do(req)
		if err != nil {
			return err
		}
		if do.StatusCode != http.StatusOK {
			do.Body.Close()
			return errors.New(do.Status)
		}
		_, err = io.Copy(f, do.Body)
		do.Body.Close()
		if err != nil {
			return err
		}
	}
	fmt.Println("")
	return err
}

func extractDuration(errStr string) string {
	regex := regexp.MustCompile(durRegex)
	match := regex.FindStringSubmatch(errStr)
	if match != nil {
		return match[1]
	}
	return ""
}

func parseDuration(dur string) (int, error) {
	dur = strings.Replace(dur, ":", "h", 1)
	dur = strings.Replace(dur, ":", "m", 1)
	dur = strings.Replace(dur, ".", "s", 1)
	dur += "ms"
	d, err := time.ParseDuration(dur)
	if err != nil {
		return 0, err
	}
	rounded := math.Round(d.Seconds())
	return int(rounded), nil
}

// Horrible, but best way without ffprobe.
// My native Go duration calculation's too slow. Is there a way without having to iterate over all the packets?
func getDuration(tsPath, ffmpegNameStr string) (int, error) {
	var errBuffer bytes.Buffer
	args := []string{"-hide_banner", "-i", tsPath}
	cmd := exec.Command(ffmpegNameStr, args...)
	cmd.Stderr = &errBuffer
	// Return code's always 1 as we're not providing any output files.
	err := cmd.Run()
	if err.Error() != "exit status 1" {
		return 0, err
	}
	errStr := errBuffer.String()
	ok := strings.HasSuffix(
		strings.TrimSpace(errStr), "At least one output file must be specified")
	if !ok {
		errString := fmt.Sprintf("%s\n%s", err, errStr)
		return 0, errors.New(errString)
	}
	dur := extractDuration(errStr)
	if dur == "" {
		return 0, errors.New("No regex match.")
	}
	durSecs, err := parseDuration(dur)
	if err != nil {
		return 0, err
	}
	return durSecs, nil
}

func getNextChapStart(chapters []any, idx int) float64 {
	for i, chapter := range chapters {
		if i == idx {
			m := chapter.(map[string]any)
			return m["chapterSeconds"].(float64)
		}
	}
	return 0
}

func writeChapsFile(chapters []any, dur int) error {
	f, err := os.OpenFile(chapsFileFname, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(";FFMETADATA1\n")
	if err != nil {
		return err
	}
	chaptersCount := len(chapters)

	var nextChapStart float64

	for i, chapter := range chapters {
		i++
		isLast := i == chaptersCount

		// casting to struct won't work.
		m := chapter.(map[string]any)
		start := m["chapterSeconds"].(float64)

		if !isLast {
			nextChapStart = getNextChapStart(chapters, i)
			if nextChapStart <= start {
				continue
			}
		}

		_, err := f.WriteString("\n[CHAPTER]\n")
		if err != nil {
			return err
		}
		_, err = f.WriteString("TIMEBASE=1/1\n")
		if err != nil {
			return err
		}

		startLine := fmt.Sprintf("START=%d\n", int(math.Round(start)))
		_, err = f.WriteString(startLine)
		if err != nil {
			return err
		}
		if isLast {
			endLine := fmt.Sprintf("END=%d\n", dur)
			_, err = f.WriteString(endLine)
			if err != nil {
				return err
			}
		} else {
			endLine := fmt.Sprintf("END=%d\n", int(math.Round(nextChapStart)-1))
			_, err = f.WriteString(endLine)
			if err != nil {
				return err
			}
		}
		_, err = f.WriteString("TITLE=" + m["chaptername"].(string) + "\n")
		if err != nil {
			return err
		}
	}
	return nil
}

// There are native MPEG demuxers and MP4 muxers for Go, but they're too slow.
func tsToMp4(VidPathTs, vidPath, ffmpegNameStr string, chapAvail bool) error {
	var (
		errBuffer bytes.Buffer
		args      []string
	)
	if chapAvail {
		args = []string{
			"-hide_banner", "-i", VidPathTs, "-f", "ffmetadata",
			"-i", chapsFileFname, "-map_metadata", "1", "-c", "copy", vidPath,
		}
	} else {
		args = []string{"-hide_banner", "-i", VidPathTs, "-c", "copy", vidPath}
	}
	cmd := exec.Command(ffmpegNameStr, args...)
	cmd.Stderr = &errBuffer
	err := cmd.Run()
	if err != nil {
		errString := fmt.Sprintf("%s\n%s", err, errBuffer.String())
		return errors.New(errString)
	}
	return nil
}

func getLstreamContainer(containers []*AlbArtResp) *AlbArtResp {
	for i := len(containers) - 1; i >= 0; i-- {
		c := containers[i]
		if c.AvailabilityTypeStr == "AVAILABLE" && c.ContainerTypeStr == "Show" {
			return c
		}
	}
	return nil
}

func parseLstreamMeta(_meta *ArtistMeta) *AlbumMeta {
	meta := getLstreamContainer(_meta.Response.Containers)
	parsed := &AlbumMeta{
		Response: &AlbArtResp{
			ArtistName:        meta.ArtistName,
			ContainerInfo:     meta.ContainerInfo,
			ContainerID:       meta.ContainerID,
			VideoChapters:     meta.VideoChapters,
			Products:          meta.Products,
			ProductFormatList: meta.ProductFormatList,
		},
	}
	return parsed
}

// video downloads a video from Nugs.net using the provided videoID.
// If _meta is provided, uses it instead of fetching metadata. uguID is used for purchased videos.
// The isLstream parameter indicates whether this is a livestream video.
// Downloads video in the highest quality matching cfg.VideoFormat, processes chapters if available,
// converts from TS to MP4 container, and optionally uploads to rclone if configured.
// Returns an error if metadata fetching, download, conversion, or upload fails.
func video(videoID, uguID string, cfg *Config, streamParams *StreamParams, _meta *AlbArtResp, isLstream bool) error {
	var (
		chapsAvail  bool
		skuID       int
		manifestUrl string
		meta        *AlbArtResp
		err         error
	)

	if _meta != nil {
		meta = _meta
	} else {
		m, err := getAlbumMeta(videoID)
		if err != nil {
			printError("Failed to get metadata")
			return err
		}
		meta = m.Response
	}

	if !cfg.SkipChapters {
		chapsAvail = !reflect.ValueOf(meta.VideoChapters).IsZero()
	}

	videoFname := buildAlbumFolderName(meta.ArtistName, meta.ContainerInfo, 110)
	fmt.Println(videoFname)
	if len([]rune(meta.ArtistName+" - "+strings.TrimRight(meta.ContainerInfo, " "))) > 110 {
		fmt.Println(
			"Video filename was chopped because it exceeds 110 characters.")
	}
	if isLstream {
		skuID = getLstreamSku(meta.ProductFormatList)
	} else {
		skuID = getVideoSku(meta.Products)
	}
	if skuID == 0 {
		return errors.New("no video available")
	}
	if uguID == "" {
		manifestUrl, err = getStreamMeta(
			meta.ContainerID, skuID, 0, streamParams)
	} else {
		manifestUrl, err = getPurchasedManUrl(skuID, videoID, streamParams.UserID, uguID)
	}

	if err != nil {
		printError("Failed to get video file metadata")
		return err
	} else if manifestUrl == "" {
		return errors.New("the api didn't return a video manifest url")
	}
	variant, retRes, err := chooseVariant(manifestUrl, cfg.WantRes)
	if err != nil {
		printError("Failed to get video master manifest")
		return err
	}

	// Create artist directory
	artistFolder := sanitise(meta.ArtistName)
	artistPath := filepath.Join(cfg.OutPath, artistFolder)
	err = makeDirs(artistPath)
	if err != nil {
		printError("Failed to make artist folder")
		return err
	}

	vidPathNoExt := filepath.Join(artistPath, sanitise(videoFname+"_"+retRes))
	VidPathTs := vidPathNoExt + ".ts"
	vidPath := vidPathNoExt + ".mp4"
	exists, err := fileExists(vidPath)
	if err != nil {
		printError("Failed to check if video already exists locally")
		return err
	}
	if exists {
		printInfo(fmt.Sprintf("Video exists %s skipping", symbolArrow))
		return nil
	}
	manBaseUrl, query, err := getManifestBase(manifestUrl)
	if err != nil {
		printError("Failed to get video manifest base URL")
		return err
	}

	segUrls, err := getSegUrls(manBaseUrl+variant.URI, query)
	if err != nil {
		printError("Failed to get video segment URLs")
		return err
	}

	// Player album page videos aren't always only the first seg for the entire vid.
	isLstream = segUrls[0] != segUrls[1]

	if !isLstream {
		fmt.Printf("%.3f FPS, ", variant.FrameRate)
	}
	fmt.Printf("%d Kbps, %s (%s)\n",
		variant.Bandwidth/1000, retRes, variant.Resolution)
	if isLstream {
		err = downloadLstream(VidPathTs, manBaseUrl, segUrls)
	} else {
		err = downloadVideo(VidPathTs, manBaseUrl+segUrls[0])
	}
	if err != nil {
		printError("Failed to download video segments")
		return err
	}
	if chapsAvail {
		dur, err := getDuration(VidPathTs, cfg.FfmpegNameStr)
		if err != nil {
			printError("Failed to get TS duration")
			return err
		}
		err = writeChapsFile(meta.VideoChapters, dur)
		if err != nil {
			printError("Failed to write chapters file")
			return err
		}
	}
	printInfo("Putting into MP4 container...")
	err = tsToMp4(VidPathTs, vidPath, cfg.FfmpegNameStr, chapsAvail)
	if err != nil {
		printError("Failed to put TS into MP4 container")
		return err
	}
	if chapsAvail {
		err = os.Remove(chapsFileFname)
		if err != nil {
			printError("Failed to delete chapters file")
		}
	}
	err = os.Remove(VidPathTs)
	if err != nil {
		printError("Failed to delete TS")
	}

	// Upload to rclone if enabled
	if cfg.RcloneEnabled {
		// Upload the video file to the artist folder on remote
		err = uploadToRclone(vidPath, artistFolder, cfg)
		if err != nil {
			handleErr("Upload failed.", err, false)
		}
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

func paidLstream(query, uguID string, cfg *Config, streamParams *StreamParams) error {
	q, err := url.ParseQuery(query)
	if err != nil {
		return err
	}
	showId := q["showID"][0]
	if showId == "" {
		return errors.New("url didn't contain a show id parameter")
	}
	err = video(showId, uguID, cfg, streamParams, nil, true)
	return err
}

func init() {
	// Check if --json flag is present, if so, suppress banner
	if slices.Contains(os.Args, "--json") {
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
	var token string
	scriptDir, err := getScriptDir()
	if err != nil {
		panic(err)
	}
	err = os.Chdir(scriptDir)
	if err != nil {
		panic(err)
	}

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
		err = promptForConfig()
		if err != nil {
			handleErr("Failed to create config.", err, true)
		}
	}

	// Check if first argument is "help" before parsing
	if len(os.Args) > 1 && os.Args[1] == "help" {
		// Replace with --help to trigger help display
		os.Args[1] = "--help"
	}

	// Check for --json flag with level parameter BEFORE parsing config
	// This removes it from os.Args so the arg parser doesn't complain
	jsonLevel := "" // empty = table output, "minimal"/"standard"/"extended"/"raw"
	for i := 0; i < len(os.Args); i++ {
		if os.Args[i] == "--json" {
			// Check if value is provided
			if i+1 >= len(os.Args) {
				fmt.Println("Error: --json flag requires a level argument (minimal, standard, extended, raw)")
				printInfo("Usage: nugs list artists --json <level>")
				return
			}
			jsonLevel = os.Args[i+1]
			// Remove --json and its value from args
			os.Args = append(os.Args[:i], os.Args[i+2:]...)
			break
		}
	}

	// Validate json level
	if jsonLevel != "" && jsonLevel != JSONLevelMinimal && jsonLevel != JSONLevelStandard && jsonLevel != JSONLevelExtended && jsonLevel != JSONLevelRaw {
		fmt.Printf("Invalid JSON level: %s. Valid options: %s, %s, %s, %s\n", jsonLevel, JSONLevelMinimal, JSONLevelStandard, JSONLevelExtended, JSONLevelRaw)
		return
	}

	cfg, err := parseCfg()
	if err != nil {
		handleErr("Failed to parse config/args.", err, true)
	}
	cfg.Urls = normalizeCliAliases(cfg.Urls)

	// Auto-refresh catalog cache if needed
	err = autoRefreshIfNeeded(cfg)
	if err != nil {
		// Log error but don't stop execution
		fmt.Fprintf(os.Stderr, "Auto-refresh warning: %v\n", err)
	}

	// Check if rclone is available when enabled
	if cfg.RcloneEnabled {
		err = checkRcloneAvailable(jsonLevel != "")
		if err != nil {
			handleErr("Rclone check failed.", err, true)
		}
	}

	// Show welcome screen if no arguments provided
	if len(cfg.Urls) == 0 {
		err := displayWelcome()
		if err != nil {
			fmt.Printf("Error displaying welcome screen: %v\n", err)
		}
		return
	}

	// Check if first argument is "list" command
	if len(cfg.Urls) > 0 && cfg.Urls[0] == "list" {
		if len(cfg.Urls) < 2 {
			printInfo("Usage: nugs list artists | list <artist_id> [shows \"venue\" | latest <N>]")
			fmt.Println("       list <show_count_filter>")
			fmt.Println("       list <artist_id> [\"venue\" | latest <N>]")
			return
		}

		subCmd := cfg.Urls[1]
		if subCmd == "artists" {
			// Check for show count filter: list artists shows >100
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
					return
				}
				showFilter = cfg.Urls[3]
			}

			err := listArtists(jsonLevel, showFilter)
			if err != nil {
				handleErr("List artists failed.", err, true)
			}
			return
		}

		// list <artist_id> subcommands
		artistId := subCmd

		// Check for venue filter: list <artist_id> shows "venue"
		if len(cfg.Urls) > 2 && cfg.Urls[2] == "shows" {
			if len(cfg.Urls) < 4 {
				printInfo("Usage: nugs list <artist_id> shows \"<venue_name>\"")
				fmt.Println("Or:    list <artist_id> shows \"<venue_name>\"")
				fmt.Println("Example: list 461 \"Red Rocks\"")
				return
			}
			// Join remaining args to support multi-word venue names without quotes
			// e.g., "list 461 shows Red Rocks" -> venueFilter = "Red Rocks"
			venueFilter := strings.Join(cfg.Urls[3:], " ")
			err := listArtistShowsByVenue(artistId, venueFilter, jsonLevel)
			if err != nil {
				handleErr("List shows by venue failed.", err, true)
			}
			return
		}

		// Check for latest N: list <artist_id> latest <N>
		if len(cfg.Urls) > 2 && cfg.Urls[2] == "latest" {
			limit := 10 // default
			if len(cfg.Urls) > 3 {
				if parsedLimit, parseErr := strconv.Atoi(cfg.Urls[3]); parseErr == nil {
					if parsedLimit < 1 {
						fmt.Println("Error: limit must be a positive number (got", parsedLimit, ")")
						return
					}
					limit = parsedLimit
				}
			}
			err := listArtistLatestShows(artistId, limit, jsonLevel)
			if err != nil {
				handleErr("List latest shows failed.", err, true)
			}
			return
		}

		// Default: list all shows for artist
		err := listArtistShows(artistId, jsonLevel)
		if err != nil {
			handleErr("List shows failed.", err, true)
		}
		return
	}

	// Catalog commands (except "catalog gaps ... fill" which requires auth)
	isCatalogGapsFill := len(cfg.Urls) >= 4 && cfg.Urls[0] == "catalog" && cfg.Urls[1] == "gaps" && cfg.Urls[len(cfg.Urls)-1] == "fill"
	if len(cfg.Urls) > 0 && cfg.Urls[0] == "catalog" && !isCatalogGapsFill {
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
			return
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
			limit := 15 // default
			if len(cfg.Urls) > 2 {
				if parsedLimit, err := strconv.Atoi(cfg.Urls[2]); err == nil {
					if parsedLimit < 1 {
						fmt.Println("Error: limit must be a positive number (got", parsedLimit, ")")
						return
					}
					limit = parsedLimit
				}
			}
			err := catalogLatest(limit, jsonLevel)
			if err != nil {
				handleErr("Catalog latest failed.", err, true)
			}
		case "gaps":
			if len(cfg.Urls) < 3 {
				printInfo("Usage: nugs catalog gaps <artist_id> [...] [fill]")
				fmt.Println("       catalog gaps <artist_id> [...] --ids-only")
				return
			}

			// Check for --ids-only flag and collect artist IDs
			idsOnly := false
			artistIds := []string{}
			for i := 2; i < len(cfg.Urls); i++ {
				if cfg.Urls[i] == "--ids-only" {
					idsOnly = true
					continue
				}
				artistIds = append(artistIds, cfg.Urls[i])
			}

			if len(artistIds) == 0 {
				fmt.Println("Error: No artist IDs provided")
				return
			}

			err := catalogGaps(artistIds, cfg, jsonLevel, idsOnly)
			if err != nil {
				handleErr("Catalog gaps failed.", err, true)
			}
		case "list":
			if len(cfg.Urls) < 3 {
				printInfo("Usage: nugs catalog coverage <artist_id> [...]")
				return
			}

			// Get artist IDs (everything after "list")
			artistIds := cfg.Urls[2:]

			err := catalogList(artistIds, cfg, jsonLevel)
			if err != nil {
				handleErr("Catalog list failed.", err, true)
			}
		case "coverage":
			// Get artist IDs (everything after "coverage")
			artistIds := []string{}
			if len(cfg.Urls) > 2 {
				artistIds = cfg.Urls[2:]
			}

			err := catalogCoverage(artistIds, cfg, jsonLevel)
			if err != nil {
				handleErr("Catalog coverage failed.", err, true)
			}
		case "config":
			if len(cfg.Urls) < 3 {
				printInfo("Usage: nugs catalog config enable|disable|set")
				return
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
		return
	}

	// Check for "<artistID> latest" or "<artistID> full" shorthand
	if len(cfg.Urls) == 2 {
		if artistID, err := strconv.Atoi(cfg.Urls[0]); err == nil {
			switch cfg.Urls[1] {
			case "latest":
				// Construct the artist latest URL and replace the args
				artistUrl := fmt.Sprintf("https://play.nugs.net/artist/%d/latest", artistID)
				cfg.Urls = []string{artistUrl}
				printMusic(fmt.Sprintf("Downloading latest shows from %sartist %d%s", colorBold, artistID, colorReset))
			case "full":
				// Construct the full artist catalog URL and replace the args
				artistUrl := fmt.Sprintf("https://play.nugs.net/#/artist/%d", artistID)
				cfg.Urls = []string{artistUrl}
				printMusic(fmt.Sprintf("Downloading entire catalog from %sartist %d%s", colorBold, artistID, colorReset))
			case "gaps", "update", "cache", "stats", "config", "coverage", "list":
				// User likely meant a catalog command
				fmt.Printf("%s✗ Invalid syntax%s\n\n", colorRed, colorReset)
				fmt.Printf("Did you mean: %snugs catalog %s %d%s\n\n", colorBold, cfg.Urls[1], artistID, colorReset)
				fmt.Printf("Valid artist shortcuts:\n")
				fmt.Printf("  • %snugs %d latest%s  - Download latest shows\n", colorBold, artistID, colorReset)
				fmt.Printf("  • %snugs %d full%s    - Download entire catalog\n\n", colorBold, artistID, colorReset)
				fmt.Printf("For catalog commands, use:\n")
				fmt.Printf("  • %snugs catalog %s %d%s\n", colorBold, cfg.Urls[1], artistID, colorReset)
				os.Exit(1)
			default:
				// Unknown subcommand after artist ID
				fmt.Printf("%s✗ Unknown command: %s%s\n\n", colorRed, cfg.Urls[1], colorReset)
				fmt.Printf("Valid artist shortcuts:\n")
				fmt.Printf("  • %snugs %d latest%s  - Download latest shows\n", colorBold, artistID, colorReset)
				fmt.Printf("  • %snugs %d full%s    - Download entire catalog\n\n", colorBold, artistID, colorReset)
				os.Exit(1)
			}
		}
	}

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

	// Handle "catalog gaps <artist_id> [...] fill" command (needs auth)
	if len(cfg.Urls) >= 4 && cfg.Urls[0] == "catalog" && cfg.Urls[1] == "gaps" && cfg.Urls[len(cfg.Urls)-1] == "fill" {
		// Extract all artist IDs between "gaps" and "fill"
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
		return
	}

	albumTotal := len(cfg.Urls)
	var itemErr error
	for albumNum, _url := range cfg.Urls {
		fmt.Printf("\n%s%s Item %d of %d%s\n", colorBold, symbolPackage, albumNum+1, albumTotal, colorReset)
		itemId, mediaType := checkUrl(_url)
		if itemId == "" {
			fmt.Println("Invalid URL:", _url)
			continue
		}
		switch mediaType {
		case 0:
			itemErr = album(itemId, cfg, streamParams, nil)
		case 1, 2:
			itemErr = playlist(itemId, legacyToken, cfg, streamParams, false)
		case 3:
			itemErr = catalogPlist(itemId, legacyToken, cfg, streamParams)
		case 4, 10:
			itemErr = video(itemId, "", cfg, streamParams, nil, false)
		case 5:
			itemErr = artist(itemId, cfg, streamParams)
		case 6, 7, 8:
			itemErr = video(itemId, "", cfg, streamParams, nil, true)
		case 9:
			itemErr = paidLstream(itemId, uguID, cfg, streamParams)
		case 11:
			itemErr = album(itemId, cfg, streamParams, nil)
		}
		if itemErr != nil {
			handleErr("Item failed.", itemErr, false)
		}
	}
}
