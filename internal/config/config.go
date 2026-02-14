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
	"syscall"

	"github.com/alexflint/go-arg"
	"github.com/jmagar/nugs-cli/internal/helpers"
	"github.com/jmagar/nugs-cli/internal/model"
	"github.com/jmagar/nugs-cli/internal/ui"
	"golang.org/x/term"
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

// scanLine reads a line from the scanner, returning the trimmed text or a scan error.
func scanLine(scanner *bufio.Scanner) (string, error) {
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("failed to read input: %w", err)
		}
		return "", errors.New("unexpected end of input")
	}
	return strings.TrimSpace(scanner.Text()), nil
}

// promptEmail prompts for and validates the user's email.
func promptEmail(scanner *bufio.Scanner) (string, error) {
	fmt.Printf("%s%s%s Enter your Nugs.net email: ", ui.ColorCyan, ui.BulletArrow, ui.ColorReset)
	email, err := scanLine(scanner)
	if err != nil {
		return "", err
	}
	if email == "" {
		return "", errors.New("email is required")
	}
	return email, nil
}

// promptPassword securely reads the password without echoing to the terminal.
func promptPassword() (string, error) {
	fmt.Printf("%s%s%s Enter your Nugs.net password: ", ui.ColorCyan, ui.BulletArrow, ui.ColorReset)
	pwBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println() // newline after hidden input
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}
	password := strings.TrimSpace(string(pwBytes))
	if password == "" {
		return "", errors.New("password is required")
	}
	return password, nil
}

// promptFormat asks the user to select an audio quality format (1-5, default 4).
func promptFormat(scanner *bufio.Scanner) (int, error) {
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
	formatStr, err := scanLine(scanner)
	if err != nil {
		return 0, err
	}
	if formatStr == "" {
		return 4, nil
	}
	format, err := strconv.Atoi(formatStr)
	if err != nil || format < 1 || format > 5 {
		return 0, errors.New("format must be between 1 and 5")
	}
	return format, nil
}

// promptVideoFormat asks the user to select a video resolution (1-5, default 5).
func promptVideoFormat(scanner *bufio.Scanner) (int, error) {
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
	videoFormatStr, err := scanLine(scanner)
	if err != nil {
		return 0, err
	}
	if videoFormatStr == "" {
		return 5, nil
	}
	videoFormat, err := strconv.Atoi(videoFormatStr)
	if err != nil || videoFormat < 1 || videoFormat > 5 {
		return 0, errors.New("video format must be between 1 and 5")
	}
	return videoFormat, nil
}

// promptOutPaths asks for audio and video download directories.
func promptOutPaths(scanner *bufio.Scanner) (outPath, videoOutPath string, err error) {
	fmt.Printf("\n%s%s%s Enter download directory (default: Nugs downloads): ", ui.ColorCyan, ui.BulletArrow, ui.ColorReset)
	outPath, err = scanLine(scanner)
	if err != nil {
		return "", "", err
	}
	if outPath == "" {
		outPath = "Nugs downloads"
	}

	fmt.Printf("%s%s%s Enter video download directory (default: same as download directory): ", ui.ColorCyan, ui.BulletArrow, ui.ColorReset)
	videoOutPath, err = scanLine(scanner)
	if err != nil {
		return "", "", err
	}
	if videoOutPath == "" {
		videoOutPath = outPath
	}
	return outPath, videoOutPath, nil
}

// promptFfmpegFlag asks whether to use the system FFmpeg.
func promptFfmpegFlag(scanner *bufio.Scanner) (bool, error) {
	fmt.Printf("\n%s%s%s Use FFmpeg from system PATH? [y/N] (default: N): ", ui.ColorCyan, ui.BulletArrow, ui.ColorReset)
	answer, err := scanLine(scanner)
	if err != nil {
		return false, err
	}
	answer = strings.ToLower(answer)
	return answer == "y" || answer == "yes", nil
}

// rcloneSettings holds all rclone-related configuration from the setup prompt.
type rcloneSettings struct {
	enabled          bool
	remote           string
	path             string
	videoPath        string
	transfers        int
	deleteAfterUpload bool
}

// promptRclone asks for rclone upload configuration.
func promptRclone(scanner *bufio.Scanner) (rcloneSettings, error) {
	fmt.Printf("\n%s%s%s Upload to remote using rclone? [y/N] (default: N): ", ui.ColorCyan, ui.BulletArrow, ui.ColorReset)
	answer, err := scanLine(scanner)
	if err != nil {
		return rcloneSettings{}, err
	}
	answer = strings.ToLower(answer)
	if answer != "y" && answer != "yes" {
		return rcloneSettings{}, nil
	}

	fmt.Print("Enter rclone remote name (e.g., tootie): ")
	remote, err := scanLine(scanner)
	if err != nil {
		return rcloneSettings{}, err
	}
	if remote == "" {
		return rcloneSettings{}, errors.New("rclone remote name is required")
	}

	fmt.Print("Enter remote path (e.g., /mnt/user/data/media/music): ")
	path, err := scanLine(scanner)
	if err != nil {
		return rcloneSettings{}, err
	}
	if path == "" {
		return rcloneSettings{}, errors.New("rclone remote path is required")
	}

	fmt.Print("Enter remote video path (default: same as remote path): ")
	videoPath, err := scanLine(scanner)
	if err != nil {
		return rcloneSettings{}, err
	}
	if videoPath == "" {
		videoPath = path
	}

	fmt.Print("Enter number of parallel transfers (default: 4): ")
	transfersStr, err := scanLine(scanner)
	if err != nil {
		return rcloneSettings{}, err
	}
	transfers := 4
	if transfersStr != "" {
		transfers, err = strconv.Atoi(transfersStr)
		if err != nil || transfers < 1 {
			return rcloneSettings{}, errors.New("transfers must be a positive integer")
		}
	}

	fmt.Print("Delete local files after upload? [Y/n] (default: Y): ")
	deleteStr, err := scanLine(scanner)
	if err != nil {
		return rcloneSettings{}, err
	}
	deleteStr = strings.ToLower(deleteStr)
	deleteAfterUpload := deleteStr != "n" && deleteStr != "no"

	return rcloneSettings{
		enabled:          true,
		remote:           remote,
		path:             path,
		videoPath:        videoPath,
		transfers:        transfers,
		deleteAfterUpload: deleteAfterUpload,
	}, nil
}

// atomicWriteFile writes data to a temp file then atomically renames it to the target path.
func atomicWriteFile(targetPath string, data []byte, perm os.FileMode) error {
	tmpPath := targetPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, perm); err != nil {
		return fmt.Errorf("failed to write temp file %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		// Clean up temp file on rename failure
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename %s to %s: %w", tmpPath, targetPath, err)
	}
	return nil
}

// PromptForConfig runs the interactive first-time setup flow.
func PromptForConfig() error {
	scanner := bufio.NewScanner(os.Stdin)
	ui.PrintHeader("First Time Setup")
	ui.PrintInfo("No config.json found. Let's create one!")
	fmt.Println()

	email, err := promptEmail(scanner)
	if err != nil {
		return err
	}
	password, err := promptPassword()
	if err != nil {
		return err
	}
	format, err := promptFormat(scanner)
	if err != nil {
		return err
	}
	videoFormat, err := promptVideoFormat(scanner)
	if err != nil {
		return err
	}
	outPath, videoOutPath, err := promptOutPaths(scanner)
	if err != nil {
		return err
	}
	useFfmpegEnvVar, err := promptFfmpegFlag(scanner)
	if err != nil {
		return err
	}
	rclone, err := promptRclone(scanner)
	if err != nil {
		return err
	}

	cfg := model.Config{
		Email:                  email,
		Password:               password,
		Format:                 format,
		VideoFormat:            videoFormat,
		OutPath:                outPath,
		VideoOutPath:           videoOutPath,
		Token:                  "",
		UseFfmpegEnvVar:        useFfmpegEnvVar,
		RcloneEnabled:          rclone.enabled,
		RcloneRemote:           rclone.remote,
		RclonePath:             rclone.path,
		RcloneVideoPath:        rclone.videoPath,
		DeleteAfterUpload:      rclone.deleteAfterUpload,
		RcloneTransfers:        rclone.transfers,
		CatalogAutoRefresh:     true,
		CatalogRefreshTime:     "05:00",
		CatalogRefreshTimezone: "America/New_York",
		CatalogRefreshInterval: "daily",
	}

	configPath, err := writeNewConfigFile(cfg)
	if err != nil {
		return err
	}

	fmt.Println()
	ui.PrintSuccess(fmt.Sprintf("Config created at %s", configPath))
	ui.PrintInfo("You can edit this file later to change settings.")
	fmt.Println()
	return nil
}

// writeNewConfigFile creates the config directory and atomically writes the config file.
func writeNewConfigFile(cfg model.Config) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	configDir := filepath.Join(homeDir, ".nugs")
	if mkErr := os.MkdirAll(configDir, 0700); mkErr != nil {
		return "", fmt.Errorf("failed to create config directory: %w", mkErr)
	}
	configPath := filepath.Join(configDir, "config.json")

	data, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := atomicWriteFile(configPath, data, 0600); err != nil {
		return "", err
	}
	return configPath, nil
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

var showCountFilterRegex = regexp.MustCompile(`^(>=|<=|>|<|=)\d+$`)

// IsShowCountFilterToken returns true if the string matches a show count filter pattern.
func IsShowCountFilterToken(s string) bool {
	return showCountFilterRegex.MatchString(s)
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
		"./config.json", // Local config (highest priority)
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
		return nil, fmt.Errorf("config file not found in any location (~/.nugs/config.json, ~/.config/nugs/config.json): %w", lastErr)
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
// Uses atomic write (temp file + rename) to prevent corruption on crash/interrupt.
func WriteConfig(cfg *model.Config) error {
	configData, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to the same file that was loaded by ReadConfig
	targetPath := LoadedConfigPath
	if targetPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		targetPath = filepath.Join(homeDir, ".nugs", "config.json")
	}

	// Ensure parent directory exists
	dir := filepath.Dir(targetPath)
	if mkErr := os.MkdirAll(dir, 0700); mkErr != nil {
		return fmt.Errorf("failed to create config directory %s: %w", dir, mkErr)
	}

	if err := atomicWriteFile(targetPath, configData, 0600); err != nil {
		return fmt.Errorf("failed to write config to %s: %w", targetPath, err)
	}

	return nil
}
