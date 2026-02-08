package ui

import (
	"fmt"
	"strings"

	"github.com/jmagar/nugs-cli/internal/model"
)

// RunErrorCount and RunWarningCount track errors/warnings during a run.
var RunErrorCount int
var RunWarningCount int

// PrintSuccess prints a success message.
func PrintSuccess(msg string) {
	fmt.Printf("%s%s%s %s%s\n", ColorGreen, SymbolCheck, ColorReset, msg, ColorReset)
}

// PrintError prints an error message and increments the error counter.
func PrintError(msg string) {
	RunErrorCount++
	fmt.Printf("%s%s%s %s%s\n", ColorRed, SymbolCross, ColorReset, msg, ColorReset)
}

// PrintInfo prints an info message.
func PrintInfo(msg string) {
	fmt.Printf("%s%s%s %s%s\n", ColorBlue, SymbolInfo, ColorReset, msg, ColorReset)
}

// PrintWarning prints a warning message and increments the warning counter.
func PrintWarning(msg string) {
	RunWarningCount++
	fmt.Printf("%s%s%s %s%s\n", ColorYellow, SymbolWarning, ColorReset, msg, ColorReset)
}

// PrintDownload prints a download message.
func PrintDownload(msg string) {
	fmt.Printf("%s%s%s %s%s\n", ColorCyan, SymbolDownload, ColorReset, msg, ColorReset)
}

// PrintUpload prints an upload message.
func PrintUpload(msg string) {
	fmt.Printf("%s%s%s %s%s\n", ColorPurple, SymbolUpload, ColorReset, msg, ColorReset)
}

// PrintMusic prints a music message.
func PrintMusic(msg string) {
	fmt.Printf("%s%s%s %s%s\n", ColorGreen, SymbolMusic, ColorReset, msg, ColorReset)
}

// GetMediaTypeIndicator returns the emoji symbol for a media type.
func GetMediaTypeIndicator(mediaType model.MediaType) string {
	switch mediaType {
	case model.MediaTypeAudio:
		return SymbolAudio
	case model.MediaTypeVideo:
		return SymbolVideo
	case model.MediaTypeBoth:
		return SymbolBoth
	default:
		return ""
	}
}

// DescribeAudioFormat returns a human-readable audio format description.
func DescribeAudioFormat(format int) string {
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

// DescribeVideoFormat returns a human-readable video format description.
func DescribeVideoFormat(videoFormat int) string {
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

// DescribeAuthStatus returns a human-readable authentication status.
func DescribeAuthStatus(cfg *model.Config) string {
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
