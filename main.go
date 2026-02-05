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
	"io/ioutil"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/dustin/go-humanize"
	"github.com/grafov/m3u8"
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
	`^https://www.nugs.net/on/demandware.store/Sites-NugsNet-Site/d`+
		`efault/(?:Stash-QueueVideo|NugsVideo-GetStashVideo)\?([a-zA-Z0-9=%&-]+$)`,
	`^https://play.nugs.net/library/webcast/(\d+)$`,
	`^(\d+)$`,
}

var qualityMap = map[string]Quality{
	".alac16/": {Specs: "16-bit / 44.1 kHz ALAC", Extension: ".m4a", Format: 1},
	".flac16/": {Specs: "16-bit / 44.1 kHz FLAC", Extension: ".flac", Format: 2},
	// .mqa24/ must be above .flac?
	".mqa24/":  {Specs: "24-bit / 48 kHz MQA", Extension: ".flac", Format: 3},
	".flac?": {Specs: "FLAC", Extension: ".flac", Format: 2},
	".s360/":   {Specs: "360 Reality Audio", Extension: ".mp4", Format: 4},
	".aac150/": {Specs: "150 Kbps AAC", Extension: ".m4a", Format: 5},
	".m4a?": {Specs: "AAC", Extension: ".m4a", Format: 5},
	".m3u8?":	{Extension: ".m4a", Format: 6},
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
	fmt.Printf("\r%d%% @ %s/s, %s/%s ", wc.Percentage, humanize.Bytes(uint64(speed)),
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
	fmt.Println("\n=== First Time Setup ===")
	fmt.Println("No config.json found. Let's create one!\n")

	// Email
	fmt.Print("Enter your Nugs.net email: ")
	scanner.Scan()
	email := strings.TrimSpace(scanner.Text())
	if email == "" {
		return errors.New("email is required")
	}

	// Password
	fmt.Print("Enter your Nugs.net password: ")
	scanner.Scan()
	password := strings.TrimSpace(scanner.Text())
	if password == "" {
		return errors.New("password is required")
	}

	// Format
	fmt.Println("\nTrack download quality:")
	fmt.Println("  1 = 16-bit / 44.1 kHz ALAC")
	fmt.Println("  2 = 16-bit / 44.1 kHz FLAC")
	fmt.Println("  3 = 24-bit / 48 kHz MQA")
	fmt.Println("  4 = 360 Reality Audio / best available (recommended)")
	fmt.Println("  5 = 150 Kbps AAC")
	fmt.Print("Enter format choice [1-5] (default: 4): ")
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
	fmt.Println("\nVideo download format:")
	fmt.Println("  1 = 480p")
	fmt.Println("  2 = 720p")
	fmt.Println("  3 = 1080p")
	fmt.Println("  4 = 1440p")
	fmt.Println("  5 = 4K / best available (recommended)")
	fmt.Print("Enter video format choice [1-5] (default: 5): ")
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
	fmt.Print("\nEnter download directory (default: Nugs downloads): ")
	scanner.Scan()
	outPath := strings.TrimSpace(scanner.Text())
	if outPath == "" {
		outPath = "Nugs downloads"
	}

	// FFmpeg
	fmt.Print("\nUse FFmpeg from system PATH? [y/N] (default: N): ")
	scanner.Scan()
	useFfmpegEnvVarStr := strings.ToLower(strings.TrimSpace(scanner.Text()))
	useFfmpegEnvVar := useFfmpegEnvVarStr == "y" || useFfmpegEnvVarStr == "yes"

	// Rclone
	fmt.Print("\nUpload to remote using rclone? [y/N] (default: N): ")
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
		Email:             email,
		Password:          password,
		Format:            format,
		VideoFormat:       videoFormat,
		OutPath:           outPath,
		Token:             "",
		UseFfmpegEnvVar:   useFfmpegEnvVar,
		RcloneEnabled:     rcloneEnabled,
		RcloneRemote:      rcloneRemote,
		RclonePath:        rclonePath,
		DeleteAfterUpload: deleteAfterUpload,
		RcloneTransfers:   rcloneTransfers,
	}

	// Write to file
	data, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return err
	}

	err = ioutil.WriteFile("config.json", data, 0600)
	if err != nil {
		return err
	}

	fmt.Println("\n✓ config.json created successfully!")
	fmt.Println("You can edit config.json later to change these settings.\n")
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
func uploadToRclone(localPath string, cfg *Config) error {
	if !cfg.RcloneEnabled {
		return nil
	}

	// Validate paths before executing rclone command
	if err := validatePath(localPath); err != nil {
		return fmt.Errorf("invalid local path: %w", err)
	}

	remoteDest := cfg.RcloneRemote + ":" + cfg.RclonePath
	printUpload(fmt.Sprintf("Uploading to %s%s%s...", colorBold, remoteDest, colorReset))

	// Default to 4 transfers if not set
	transfers := cfg.RcloneTransfers
	if transfers == 0 {
		transfers = 4
	}

	// Use rclone copy to upload without deleting remote files
	transfersFlag := fmt.Sprintf("--transfers=%d", transfers)
	cmd := exec.Command("rclone", "copy", localPath, remoteDest+"/"+filepath.Base(localPath),
		"-P", transfersFlag)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("rclone upload failed: %w", err)
	}

	printSuccess("Upload complete!")

	if cfg.DeleteAfterUpload {
		// Verify upload before deleting local files
		printInfo("Verifying upload integrity...")
		verifyCmd := exec.Command("rclone", "check", "--one-way", localPath, remoteDest+"/"+filepath.Base(localPath))
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
	if !(cfg.Format >= 1 && cfg.Format <= 5) {
		return nil, errors.New("track Format must be between 1 and 5")
	}
	if !(cfg.VideoFormat >= 1 && cfg.VideoFormat <= 5) {
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
		fmt.Println("Failed to process URLs.")
		return nil, err
	}
	cfg.ForceVideo = args.ForceVideo
	cfg.SkipVideos = args.SkipVideos
	cfg.SkipChapters = args.SkipChapters
	return cfg, nil
}

func readConfig() (*Config, error) {
	data, err := ioutil.ReadFile("config.json")
	if err != nil {
		return nil, err
	}
	var obj Config
	err = json.Unmarshal(data, &obj)
	if err != nil {
		return nil, err
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
	return strings.TrimSuffix(san, "\t")
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
// 	fmt.Println("Decrypting...")
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
	fmt.Println("Decrypting...")
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
			fmt.Println("failed to get track stream metadata")
			return err
		} else if streamUrl == "" {
			return errors.New("the api didn't return a track stream URL")
		}
		quality := queryQuality(streamUrl)
		if quality == nil {
			fmt.Println("The API returned an unsupported format, URL:", streamUrl)
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
		fmt.Println("HLS-only track. Only AAC is available, tags currently unsupported.")
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
		if chosenQual == nil {
			return errors.New("no track format was chosen")
		}
		if wantFmt != origWantFmt && origWantFmt != 4 {
			fmt.Println("Unavailable in your chosen format.")
		}
	}
	trackFname := fmt.Sprintf(
		"%02d. %s%s", trackNum, sanitise(track.SongTitle), chosenQual.Extension,
	)
	trackPath := filepath.Join(folPath, trackFname)
	exists, err := fileExists(trackPath)
	if err != nil {
		fmt.Println("Failed to check if track already exists locally.")
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
		fmt.Println("Failed to download track.")
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
			fmt.Println("Failed to get metadata.")
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
			fmt.Println("Video-only album, skipped.")
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
		fmt.Println("Failed to make artist folder.")
		return err
	}

	albumFolder := meta.ArtistName + " - " + strings.TrimRight(meta.ContainerInfo, " ")
	fmt.Println(albumFolder)
	if len(albumFolder) > 120 {
		albumFolder = albumFolder[:120]
		fmt.Println(
			"Album folder name was chopped because it exceeds 120 characters.")
	}
	albumPath := filepath.Join(artistPath, sanitise(albumFolder))

	// Check if show already exists locally
	if stat, err := os.Stat(albumPath); err == nil && stat.IsDir() {
		printInfo(fmt.Sprintf("Show already exists locally %s skipping", symbolArrow))
		return nil
	}

	// Check if show already exists on remote
	remoteShowPath := artistFolder + "/" + sanitise(albumFolder)
	printInfo(fmt.Sprintf("Checking remote for: %s%s%s", colorCyan, sanitise(albumFolder), colorReset))
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
		fmt.Println("Failed to make album folder.")
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
		err = uploadToRclone(albumPath, cfg)
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
		fmt.Println("Failed to get artist metadata.")
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

// listArtists fetches and displays a formatted list of all artists available on Nugs.net.
// The output includes artist ID, name, number of shows, and number of albums.
// Returns an error if the artist list cannot be fetched from the API.
func listArtists(jsonLevel string) error {
	if jsonLevel == "" {
		fmt.Println("Fetching artist catalog...")
	}
	artistList, err := getArtistList()
	if err != nil {
		fmt.Println("Failed to get artist list.")
		return err
	}

	artists := artistList.Response.Artists
	if len(artists) == 0 {
		if jsonLevel != "" {
			emptyOutput := ArtistListOutput{Artists: []ArtistOutput{}, Total: 0}
			jsonData, _ := json.MarshalIndent(emptyOutput, "", "  ")
			fmt.Println(string(jsonData))
		} else {
			fmt.Println("No artists found.")
		}
		return nil
	}

	if jsonLevel != "" {
		// Raw mode: output full API response as-is
		if jsonLevel == "raw" {
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
		// Existing table output
		fmt.Printf("\nFound %d artists:\n\n", len(artists))
		fmt.Printf("%-8s %-60s %-10s %-10s\n", "ID", "Name", "Shows", "Albums")
		fmt.Println(strings.Repeat("-", 90))

		for _, artist := range artists {
			name := artist.ArtistName
			if len(name) > 58 {
				name = name[:58] + ".."
			}
			fmt.Printf("%-8d %-60s %-10d %-10d\n",
				artist.ArtistID,
				name,
				artist.NumShows,
				artist.NumAlbums)
		}

		fmt.Printf("\nTo list shows for an artist, use: list <artist_id>\n")
	}
	return nil
}

// listArtistShows fetches and displays all shows for a specific artist identified by artistId.
// The output is sorted by date in reverse chronological order (newest first) and includes
// container ID, date, title, and venue for each show.
// Returns an error if the artist metadata cannot be fetched from the API.
func listArtistShows(artistId string, jsonLevel string) error {
	if jsonLevel == "" {
		fmt.Println("Fetching artist shows...")
	}
	allMeta, err := getArtistMeta(artistId)
	if err != nil {
		fmt.Println("Failed to get artist metadata.")
		return err
	}

	if len(allMeta) == 0 {
		fmt.Println("No metadata found for this artist.")
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
			jsonData, _ := json.MarshalIndent(emptyOutput, "", "  ")
			fmt.Println(string(jsonData))
		} else {
			fmt.Printf("No shows found for %s\n", artistName)
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
		if jsonLevel == "raw" {
			jsonData, err := json.MarshalIndent(allMeta, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Println(string(jsonData))
			return nil
		}

		// Build structured JSON output for minimal/standard/extended
		artistIdInt, _ := strconv.Atoi(artistId)

		if jsonLevel == "extended" {
			// Extended: output full container structs with all fields
			shows := make([]*AlbArtResp, len(allContainers))
			for i, item := range allContainers {
				shows[i] = item.container
			}
			output := map[string]interface{}{
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
				if jsonLevel == "standard" {
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
		// Existing table output
		fmt.Printf("\n%s - %d shows:\n\n", artistName, len(allContainers))
		fmt.Printf("%-10s %-12s %-50s %-30s\n", "ID", "Date", "Title", "Venue")
		fmt.Println(strings.Repeat("-", 105))

		for _, item := range allContainers {
			container := item.container
			title := container.ContainerInfo
			if len(title) > 48 {
				title = title[:48] + ".."
			}
			venue := container.VenueName
			if len(venue) > 28 {
				venue = venue[:28] + ".."
			}

			fmt.Printf("%-10d %-12s %-50s %-30s\n",
				container.ContainerID,
				item.dateStr,
				title,
				venue)
		}

		fmt.Printf("\nTo download a show, use: https://play.nugs.net/release/<container_id>\n")
	}
	return nil
}

func playlist(plistId, legacyToken string, cfg *Config, streamParams *StreamParams, cat bool) error {
	_meta, err := getPlistMeta(plistId, cfg.Email, legacyToken, cat)
	if err != nil {
		fmt.Println("Failed to get playlist metadata.")
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
		fmt.Println("Failed to make playlist folder.")
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
		err = uploadToRclone(plistPath, cfg)
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
	if wantVariant == nil {
		return nil, "", errors.New("No variant was chosen.")
	}
	if wantRes != origWantRes {
		fmt.Println("Unavailable in your chosen format.")
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
		Total:     totalBytes,
		TotalStr:  humanize.Bytes(uint64(totalBytes)),
		StartTime: time.Now().UnixMilli(),
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
		if err != nil {
			return err
		}
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

func getNextChapStart(chapters []interface{}, idx int) float64 {
	for i, chapter := range chapters {
		if i == idx {
			m := chapter.(map[string]interface{})
			return m["chapterSeconds"].(float64)
		}
	}
	return 0
}


func writeChapsFile(chapters []interface{}, dur int) error {
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
		m := chapter.(map[string]interface{})
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
		chapsAvail bool
		skuID int
		manifestUrl string
		meta *AlbArtResp
		err error
	)

	if _meta != nil {
		meta = _meta
	} else {
		m, err := getAlbumMeta(videoID)
		if err != nil {
			fmt.Println("Failed to get metadata.")
			return err
		}
		meta = m.Response
	}

	if !cfg.SkipChapters {
		chapsAvail = !reflect.ValueOf(meta.VideoChapters).IsZero()
	}
	
	videoFname := meta.ArtistName + " - " + strings.TrimRight(meta.ContainerInfo, " ")
	fmt.Println(videoFname)
	if len(videoFname) > 110 {
		videoFname = videoFname[:110]
		fmt.Println(
			"Video filename was chopped because it exceeds 120 characters.")
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
		fmt.Println("Failed to get video file metadata.")
		return err
	} else if manifestUrl == "" {
		return errors.New("the api didn't return a video manifest url")
	}
	variant, retRes, err := chooseVariant(manifestUrl, cfg.WantRes)
	if err != nil {
		fmt.Println("Failed to get video master manifest.")
		return err
	}

	// Create artist directory
	artistFolder := sanitise(meta.ArtistName)
	artistPath := filepath.Join(cfg.OutPath, artistFolder)
	err = makeDirs(artistPath)
	if err != nil {
		fmt.Println("Failed to make artist folder.")
		return err
	}

	vidPathNoExt := filepath.Join(artistPath, sanitise(videoFname+"_"+retRes))
	VidPathTs := vidPathNoExt + ".ts"
	vidPath := vidPathNoExt + ".mp4"
	exists, err := fileExists(vidPath)
	if err != nil {
		fmt.Println("Failed to check if video already exists locally.")
		return err
	}
	if exists {
		printInfo(fmt.Sprintf("Video exists %s skipping", symbolArrow))
		return nil
	}
	manBaseUrl, query, err := getManifestBase(manifestUrl)
	if err != nil {
		fmt.Println("Failed to get video manifest base URL.")
		return err
	}

	segUrls, err := getSegUrls(manBaseUrl+variant.URI, query)
	if err != nil {
		fmt.Println("Failed to get video segment URLs.")
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
		fmt.Println("Failed to download video segments.")
		return err
	}
	if chapsAvail {
		dur, err := getDuration(VidPathTs, cfg.FfmpegNameStr)
		if err != nil {
			fmt.Println("Failed to get TS duration.")
			return err
		}
		err = writeChapsFile(meta.VideoChapters, dur)
		if err != nil {
			fmt.Println("Failed to write chapters file.")
			return err
		}
	}
	fmt.Println("Putting into MP4 container...")
	err = tsToMp4(VidPathTs, vidPath, cfg.FfmpegNameStr, chapsAvail)
	if err != nil {
		fmt.Println("Failed to put TS into MP4 container.")
		return err
	}
	if chapsAvail {
		err = os.Remove(chapsFileFname)
		if err != nil {
			fmt.Println("Failed to delete chapters file.")
		}
	}
	err = os.Remove(VidPathTs)
	if err != nil {
		fmt.Println("Failed to delete TS.")
	}

	// Upload to rclone if enabled
	if cfg.RcloneEnabled {
		err = uploadToRclone(artistPath, cfg)
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
	for _, arg := range os.Args {
		if arg == "--json" {
			return
		}
	}
	fmt.Println(`
 _____                ____                _           _
|   | |_ _ ___ ___   |    \ ___ _ _ _ ___| |___ ___ _| |___ ___
| | | | | | . |_ -|  |  |  | . | | | |   | | . | .'| . | -_|  _|
|_|___|___|_  |___|  |____/|___|_____|_|_|_|___|__,|___|___|_|
	  |___|
`)
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

	// Check if config.json exists, if not, prompt to create it
	if _, err := os.Stat("config.json"); os.IsNotExist(err) {
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
				fmt.Println("Usage: list artists --json <level>")
				return
			}
			jsonLevel = os.Args[i+1]
			// Remove --json and its value from args
			os.Args = append(os.Args[:i], os.Args[i+2:]...)
			break
		}
	}

	// Validate json level
	if jsonLevel != "" && jsonLevel != "minimal" && jsonLevel != "standard" && jsonLevel != "extended" && jsonLevel != "raw" {
		fmt.Printf("Invalid JSON level: %s. Valid options: minimal, standard, extended, raw\n", jsonLevel)
		return
	}

	cfg, err := parseCfg()
	if err != nil {
		handleErr("Failed to parse config/args.", err, true)
	}

	// Check if rclone is available when enabled
	if cfg.RcloneEnabled {
		err = checkRcloneAvailable(jsonLevel != "")
		if err != nil {
			handleErr("Rclone check failed.", err, true)
		}
	}

	// Check if first argument is "list" command
	if len(cfg.Urls) > 0 && cfg.Urls[0] == "list" {
		if len(cfg.Urls) < 2 {
			fmt.Println("Usage: list artists | list <artist_id>")
			return
		}

		subCmd := cfg.Urls[1]
		if subCmd == "artists" {
			err := listArtists(jsonLevel)
			if err != nil {
				handleErr("List artists failed.", err, true)
			}
			return
		}

		// Try to parse as artist ID
		if _, err := strconv.Atoi(subCmd); err == nil {
			err := listArtistShows(subCmd, jsonLevel)
			if err != nil {
				handleErr("List shows failed.", err, true)
			}
			return
		}

		fmt.Printf("Invalid list command: %s\n", subCmd)
		fmt.Println("Usage: list artists | list <artist_id>")
		return
	}

	// Check for "<artistID> latest" shorthand
	if len(cfg.Urls) == 2 {
		if artistID, err := strconv.Atoi(cfg.Urls[0]); err == nil {
			if cfg.Urls[1] == "latest" {
				// Construct the artist latest URL and replace the args
				artistUrl := fmt.Sprintf("https://play.nugs.net/artist/%d/latest", artistID)
				cfg.Urls = []string{artistUrl}
				printMusic(fmt.Sprintf("Downloading latest shows from %sartist %d%s", colorBold, artistID, colorReset))
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
