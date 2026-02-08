package config

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/jmagar/nugs-cli/internal/helpers"
	"github.com/jmagar/nugs-cli/internal/model"
	"github.com/jmagar/nugs-cli/internal/ui"
)

// LoadedConfigPath tracks which config file was loaded so WriteConfig can save to the same location.
var LoadedConfigPath string

// ResolveRes maps video format codes to resolution strings.
var ResolveRes = map[int]string{
	1: "480",
	2: "720",
	3: "1080",
	4: "1440",
	5: "2160",
}

// PromptForConfig runs the interactive first-time setup flow.
func PromptForConfig() error {
	scanner := bufio.NewScanner(os.Stdin)
	ui.PrintHeader("First Time Setup")
	ui.PrintInfo("No config.json found. Let's create one!")
	fmt.Println()

	// Email
	fmt.Printf("%s%s%s Enter your Nugs.net email: ", ui.ColorCyan, ui.BulletArrow, ui.ColorReset)
	scanner.Scan()
	email := strings.TrimSpace(scanner.Text())
	if email == "" {
		return errors.New("email is required")
	}

	// Password
	fmt.Printf("%s%s%s Enter your Nugs.net password: ", ui.ColorCyan, ui.BulletArrow, ui.ColorReset)
	scanner.Scan()
	password := strings.TrimSpace(scanner.Text())
	if password == "" {
		return errors.New("password is required")
	}

	// Format
	fmt.Println()
	ui.PrintSection("Track Download Quality")
	qualityOptions := []string{
		"1 = 16-bit / 44.1 kHz ALAC",
		"2 = 16-bit / 44.1 kHz FLAC",
		"3 = 24-bit / 48 kHz MQA",
		fmt.Sprintf("4 = 360 Reality Audio / best available %s(recommended)%s", ui.ColorGreen, ui.ColorReset),
		"5 = 150 Kbps AAC",
	}
	ui.PrintList(qualityOptions, ui.ColorYellow)
	fmt.Printf("\n%s%s%s Enter format choice [1-5] (default: 4): ", ui.ColorCyan, ui.BulletArrow, ui.ColorReset)
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
	ui.PrintSection("Video Download Format")
	videoOptions := []string{
		"1 = 480p",
		"2 = 720p",
		"3 = 1080p",
		"4 = 1440p",
		fmt.Sprintf("5 = 4K / best available %s(recommended)%s", ui.ColorGreen, ui.ColorReset),
	}
	ui.PrintList(videoOptions, ui.ColorYellow)
	fmt.Printf("\n%s%s%s Enter video format choice [1-5] (default: 5): ", ui.ColorCyan, ui.BulletArrow, ui.ColorReset)
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
	fmt.Printf("\n%s%s%s Enter download directory (default: Nugs downloads): ", ui.ColorCyan, ui.BulletArrow, ui.ColorReset)
	scanner.Scan()
	outPath := strings.TrimSpace(scanner.Text())
	if outPath == "" {
		outPath = "Nugs downloads"
	}

	// Video Output Path
	fmt.Printf("%s%s%s Enter video download directory (default: same as download directory): ", ui.ColorCyan, ui.BulletArrow, ui.ColorReset)
	scanner.Scan()
	videoOutPath := strings.TrimSpace(scanner.Text())
	if videoOutPath == "" {
		videoOutPath = outPath
	}

	// FFmpeg
	fmt.Printf("\n%s%s%s Use FFmpeg from system PATH? [y/N] (default: N): ", ui.ColorCyan, ui.BulletArrow, ui.ColorReset)
	scanner.Scan()
	useFfmpegEnvVarStr := strings.ToLower(strings.TrimSpace(scanner.Text()))
	useFfmpegEnvVar := useFfmpegEnvVarStr == "y" || useFfmpegEnvVarStr == "yes"

	// Rclone
	fmt.Printf("\n%s%s%s Upload to remote using rclone? [y/N] (default: N): ", ui.ColorCyan, ui.BulletArrow, ui.ColorReset)
	scanner.Scan()
	rcloneEnabledStr := strings.ToLower(strings.TrimSpace(scanner.Text()))
	rcloneEnabled := rcloneEnabledStr == "y" || rcloneEnabledStr == "yes"

	var rcloneRemote, rclonePath, rcloneVideoPath string
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

		fmt.Print("Enter remote video path (default: same as remote path): ")
		scanner.Scan()
		rcloneVideoPath = strings.TrimSpace(scanner.Text())
		if rcloneVideoPath == "" {
			rcloneVideoPath = rclonePath
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
	cfg := model.Config{
		Email:                  email,
		Password:               password,
		Format:                 format,
		VideoFormat:            videoFormat,
		OutPath:                outPath,
		VideoOutPath:           videoOutPath,
		Token:                  "",
		UseFfmpegEnvVar:        useFfmpegEnvVar,
		RcloneEnabled:          rcloneEnabled,
		RcloneRemote:           rcloneRemote,
		RclonePath:             rclonePath,
		RcloneVideoPath:        rcloneVideoPath,
		DeleteAfterUpload:      deleteAfterUpload,
		RcloneTransfers:        rcloneTransfers,
		CatalogAutoRefresh:     true,
		CatalogRefreshTime:     "05:00",
		CatalogRefreshTimezone: "America/New_York",
		CatalogRefreshInterval: "daily",
	}

	// Write to file
	data, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return err
	}

	err = os.WriteFile("config.json", data, 0600)
	if err != nil {
		return err
	}

	fmt.Println()
	ui.PrintSuccess("config.json created successfully!")
	ui.PrintInfo("You can edit config.json later to change these settings.")
	fmt.Println()
	return nil
}

// ParseCfg reads config, parses CLI args, and returns the resolved Config.
func ParseCfg() (*model.Config, error) {
	cfg, err := ReadConfig()
	if err != nil {
		return nil, err
	}
	args := ParseArgs()
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

	// Validate and set defaultOutputs
	if cfg.DefaultOutputs == "" {
		cfg.DefaultOutputs = "audio"
	}
	validOutputs := map[string]bool{"audio": true, "video": true, "both": true}
	if !validOutputs[cfg.DefaultOutputs] {
		return nil, fmt.Errorf("invalid defaultOutputs: %q (must be audio, video, or both)", cfg.DefaultOutputs)
	}

	cfg.WantRes = ResolveRes[cfg.VideoFormat]
	cfg.OutPath = strings.TrimSpace(cfg.OutPath)
	cfg.VideoOutPath = strings.TrimSpace(cfg.VideoOutPath)
	cfg.RclonePath = strings.TrimSpace(cfg.RclonePath)
	cfg.RcloneVideoPath = strings.TrimSpace(cfg.RcloneVideoPath)
	if args.OutPath != "" {
		cfg.OutPath = args.OutPath
	}
	if cfg.OutPath == "" {
		cfg.OutPath = "Nugs downloads"
	}
	if strings.TrimSpace(cfg.VideoOutPath) == "" {
		cfg.VideoOutPath = cfg.OutPath
	}
	if strings.TrimSpace(cfg.RcloneVideoPath) == "" {
		cfg.RcloneVideoPath = cfg.RclonePath
	}
	if cfg.Token != "" {
		cfg.Token = strings.TrimPrefix(cfg.Token, "Bearer ")
	}
	ffmpegName, err := ResolveFfmpegBinary(cfg)
	if err != nil {
		return nil, err
	}
	cfg.FfmpegNameStr = ffmpegName
	cfg.Urls, err = helpers.ProcessUrls(args.Urls)
	if err != nil {
		ui.PrintError("Failed to process URLs")
		return nil, err
	}
	cfg.ForceVideo = args.ForceVideo
	cfg.SkipVideos = args.SkipVideos
	if cfg.ForceVideo {
		ui.PrintWarning("--force-video is deprecated. Use 'nugs grab <id> video' or set defaultOutputs: \"video\" in config.json")
	}
	if cfg.SkipVideos {
		ui.PrintWarning("--skip-videos is deprecated. Use 'nugs grab <id> audio' or set defaultOutputs: \"audio\" in config.json")
	}
	cfg.SkipChapters = args.SkipChapters
	return cfg, nil
}

// ResolveFfmpegBinary locates the ffmpeg binary based on config settings.
func ResolveFfmpegBinary(cfg *model.Config) (string, error) {
	preferred := strings.TrimSpace(cfg.FfmpegNameStr)

	// Respect explicit non-default binary names/paths from config.
	if preferred != "" && preferred != "./ffmpeg" && preferred != "ffmpeg" {
		if resolved, err := exec.LookPath(preferred); err == nil {
			return resolved, nil
		}
		if info, err := os.Stat(preferred); err == nil && !info.IsDir() {
			return preferred, nil
		}
		return "", fmt.Errorf("configured ffmpeg binary not found: %s", preferred)
	}

	if cfg.UseFfmpegEnvVar || preferred == "ffmpeg" {
		if resolved, err := exec.LookPath("ffmpeg"); err == nil {
			return resolved, nil
		}
		return "", errors.New("ffmpeg not found in PATH (install ffmpeg or set ffmpegNameStr to an absolute/local binary path)")
	}

	// Backward-compatible default: local ./ffmpeg first.
	candidates := []string{"./ffmpeg"}
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		exeLocal := filepath.Join(exeDir, "ffmpeg")
		if exeLocal != "./ffmpeg" {
			candidates = append(candidates, exeLocal)
		}
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	// Fallback: use system ffmpeg if available.
	if resolved, err := exec.LookPath("ffmpeg"); err == nil {
		return resolved, nil
	}

	return "", errors.New("ffmpeg binary not found (checked ./ffmpeg and PATH)")
}

// IsShowCountFilterToken returns true if the string matches a show count filter pattern.
func IsShowCountFilterToken(s string) bool {
	re := regexp.MustCompile(`^(>=|<=|>|<|=)\d+$`)
	return re.MatchString(s)
}

// IsMediaModifier returns true if the string is a media type modifier keyword.
func IsMediaModifier(s string) bool {
	switch strings.ToLower(s) {
	case "audio", "video", "both":
		return true
	}
	return false
}

// NormalizeCliAliases maps the updated short command syntax to internal command routing.
func NormalizeCliAliases(urls []string) []string {
	if len(urls) == 0 {
		return urls
	}

	switch urls[0] {
	case "list":
		// nugs list -> nugs list artists
		if len(urls) == 1 {
			return []string{"list", "artists"}
		}
		// nugs list audio/video/both -> nugs list artists (media modifier preserved)
		if len(urls) == 2 && IsMediaModifier(urls[1]) {
			return []string{"list", "artists", urls[1]}
		}
		// nugs list >100 -> nugs list artists shows >100
		if len(urls) == 2 && IsShowCountFilterToken(urls[1]) {
			return []string{"list", "artists", "shows", urls[1]}
		}
		// nugs list <artist_id> ... -> handle media modifiers before venue rewrite
		if len(urls) >= 3 {
			if _, err := strconv.Atoi(urls[1]); err == nil && urls[2] != "shows" && urls[2] != "latest" {
				// Check if urls[2] is a media modifier - don't treat as venue
				if IsMediaModifier(urls[2]) {
					// nugs list 1125 video -> list 1125 video (no rewrite needed)
					return urls
				}
				// nugs list 1125 "Red Rocks" -> nugs list 1125 shows "Red Rocks"
				normalized := []string{"list", urls[1], "shows"}
				normalized = append(normalized, urls[2:]...)
				return normalized
			}
		}
	case "grab":
		// nugs grab <args...> -> nugs <args...>
		if len(urls) >= 2 {
			return urls[1:]
		}
	case "update", "cache", "stats", "latest", "gaps", "coverage":
		// Top-level catalog aliases
		return append([]string{"catalog", urls[0]}, urls[1:]...)
	case "refresh":
		// nugs refresh enable|disable|set -> nugs catalog config enable|disable|set
		return append([]string{"catalog", "config"}, urls[1:]...)
	}

	return urls
}

// ReadConfig reads the config file from known locations.
func ReadConfig() (*model.Config, error) {
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

	var obj model.Config
	err = json.Unmarshal(data, &obj)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config at %s: %w", configPath, err)
	}

	// Track which config file was loaded for WriteConfig
	LoadedConfigPath = configPath

	// Security warning for config file permissions
	fileInfo, err := os.Stat(configPath)
	if err == nil {
		mode := fileInfo.Mode()
		if mode.Perm()&0077 != 0 {
			fmt.Fprintf(os.Stderr, "%s WARNING: Config file has insecure permissions (%04o)\n", ui.ColorYellow+ui.SymbolWarning+ui.ColorReset, mode.Perm())
			fmt.Fprintf(os.Stderr, "   File: %s\n", configPath)
			fmt.Fprintf(os.Stderr, "   Risk: Config contains credentials and should only be readable by you\n")
			if runtime.GOOS != "windows" {
				if chmodErr := os.Chmod(configPath, 0600); chmodErr != nil {
					fmt.Fprintf(os.Stderr, "   Auto-fix failed: %v\n", chmodErr)
					fmt.Fprintf(os.Stderr, "   Fix manually: chmod 600 %s\n\n", configPath)
				} else {
					fmt.Fprintf(os.Stderr, "   Auto-fix applied: chmod 600 %s\n\n", configPath)
				}
			} else {
				fmt.Fprintf(os.Stderr, "   Windows ACLs in use; skipping chmod auto-fix\n\n")
			}
		}
	}

	return &obj, nil
}

// ParseArgs parses CLI arguments using go-arg.
func ParseArgs() *model.Args {
	var args model.Args
	arg.MustParse(&args)
	return &args
}

// WriteConfig writes the config to the same file that was loaded by ReadConfig.
func WriteConfig(cfg *model.Config) error {
	configData, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to the same file that was loaded by ReadConfig
	targetPath := LoadedConfigPath
	if targetPath == "" {
		targetPath = "config.json" // fallback to current directory
	}

	// Ensure parent directory exists
	dir := filepath.Dir(targetPath)
	if dir != "." {
		if mkErr := os.MkdirAll(dir, 0755); mkErr != nil {
			return fmt.Errorf("failed to create config directory %s: %w", dir, mkErr)
		}
	}

	err = os.WriteFile(targetPath, configData, 0600)
	if err != nil {
		return fmt.Errorf("failed to write config to %s: %w", targetPath, err)
	}

	return nil
}
