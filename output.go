package main

import (
	"fmt"
	"strings"
)

var runErrorCount int
var runWarningCount int

// Colorized print functions
func printSuccess(msg string) {
	fmt.Printf("%s%s%s %s%s\n", colorGreen, symbolCheck, colorReset, msg, colorReset)
}

func printError(msg string) {
	runErrorCount++
	fmt.Printf("%s%s%s %s%s\n", colorRed, symbolCross, colorReset, msg, colorReset)
}

func printInfo(msg string) {
	fmt.Printf("%s%s%s %s%s\n", colorBlue, symbolInfo, colorReset, msg, colorReset)
}

func printWarning(msg string) {
	runWarningCount++
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

// getMediaTypeIndicator returns the emoji symbol for a media type
func getMediaTypeIndicator(mediaType MediaType) string {
	switch mediaType {
	case MediaTypeAudio:
		return symbolAudio // 🎵
	case MediaTypeVideo:
		return symbolVideo // 🎬
	case MediaTypeBoth:
		return symbolBoth // 📹
	default:
		return ""
	}
}

func describeAudioFormat(format int) string {
	switch format {
	case 1:
		return "ALAC 16-bit/44.1kHz"
	case 2:
		return "FLAC 16-bit/44.1kHz"
	case 3:
		return "MQA 24-bit/48kHz"
	case 4:
		return "360 Reality Audio"
	case 5:
		return "AAC 150kbps"
	default:
		return fmt.Sprintf("Unknown (%d)", format)
	}
}

func describeVideoFormat(videoFormat int) string {
	switch videoFormat {
	case 1:
		return "480p"
	case 2:
		return "720p"
	case 3:
		return "1080p"
	case 4:
		return "1440p"
	case 5:
		return "4K / best"
	default:
		return fmt.Sprintf("Unknown (%d)", videoFormat)
	}
}

func describeAuthStatus(cfg *Config) string {
	if strings.TrimSpace(cfg.Token) != "" {
		return "Configured (token)"
	}
	if strings.TrimSpace(cfg.Email) != "" && strings.TrimSpace(cfg.Password) != "" {
		return "Configured (email/password)"
	}
	if strings.TrimSpace(cfg.Email) != "" {
		return "Partial (email only)"
	}
	return "Not configured"
}

func printStartupEnvironment(cfg *Config, jsonLevel string) {
	if jsonLevel != "" {
		return
	}
	printSection("Environment")
	configPath := loadedConfigPath
	if strings.TrimSpace(configPath) == "" {
		configPath = "(unknown)"
	}
	printKeyValue("Config File", configPath, colorCyan)
	printKeyValue("Auth", describeAuthStatus(cfg), colorYellow)
	printKeyValue("Audio Format", describeAudioFormat(cfg.Format), colorYellow)
	printKeyValue("Video Format", describeVideoFormat(cfg.VideoFormat), colorYellow)
	printKeyValue("FFmpeg Binary", cfg.FfmpegNameStr, colorCyan)
	printKeyValue("Audio Output", cfg.OutPath, colorCyan)
	printKeyValue("Video Output", getVideoOutPath(cfg), colorCyan)
	rcloneAudioPath := "Disabled"
	rcloneVideoPath := "Disabled"
	if cfg.RcloneEnabled {
		rcloneAudioPath = fmt.Sprintf("%s:%s", cfg.RcloneRemote, cfg.RclonePath)
		rcloneVideoPath = fmt.Sprintf("%s:%s", cfg.RcloneRemote, getRcloneBasePath(cfg, true))
	}
	printKeyValue("Rclone Audio Path", rcloneAudioPath, colorCyan)
	printKeyValue("Rclone Video Path", rcloneVideoPath, colorCyan)
	printKeyValue("Rclone Status", checkRclonePathOnline(cfg), colorYellow)
	fmt.Println("")
}
