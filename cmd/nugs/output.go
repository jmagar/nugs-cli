package main

// Output wrappers delegating to internal/ui during migration.
// These will be removed in Phase 12 when all callers move to internal packages.

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/jmagar/nugs-cli/internal/ui"
)

// runErrorCount and runWarningCount track errors/warnings during a run.
// These are atomic because PrintError/PrintWarning can be called from download goroutines.
var runErrorCount atomic.Int64
var runWarningCount atomic.Int64

func printSuccess(msg string) { ui.PrintSuccess(msg) }
func printError(msg string) {
	runErrorCount.Add(1)
	ui.PrintError(msg)
}
func printInfo(msg string) { ui.PrintInfo(msg) }
func printWarning(msg string) {
	runWarningCount.Add(1)
	ui.PrintWarning(msg)
}
func printDownload(msg string) { ui.PrintDownload(msg) }
func printUpload(msg string)   { ui.PrintUpload(msg) }
func printMusic(msg string)    { ui.PrintMusic(msg) }

func getMediaTypeIndicator(mediaType MediaType) string {
	return ui.GetMediaTypeIndicator(mediaType)
}

func describeAudioFormat(format int) string       { return ui.DescribeAudioFormat(format) }
func describeVideoFormat(videoFormat int) string   { return ui.DescribeVideoFormat(videoFormat) }
func describeAuthStatus(cfg *Config) string        { return ui.DescribeAuthStatus(cfg) }

func printStartupEnvironment(cfg *Config, jsonLevel string) {
	if jsonLevel != "" {
		return
	}
	printSection("Environment")
	configPath := getLoadedConfigPath()
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
