package ui

import (
	"os"
	"strings"
)

// ANSI color codes - exported for use across packages.
var (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[91m"
	ColorGreen  = "\033[92m"
	ColorYellow = "\033[93m"
	ColorBlue   = "\033[94m"
	ColorPurple = "\033[95m"
	ColorCyan   = "\033[96m"
	ColorBold   = "\033[1m"
	ActiveTheme = "nordonedark"
)

// Unicode symbols
var (
	SymbolCheck    = "✓"
	SymbolCross    = "✗"
	SymbolArrow    = "→"
	SymbolMusic    = "♪"
	SymbolUpload   = "⬆"
	SymbolDownload = "⬇"
	SymbolInfo     = "ℹ"
	SymbolWarning  = "⚠"
	SymbolGear     = "⚙"
	SymbolPackage  = "📦"
	SymbolRocket   = "🚀"

	SymbolAudio = "🎵" // audio only
	SymbolVideo = "🎬" // video only
	SymbolBoth  = "📹" // both available
)

func init() {
	InitColorPalette()
}

// InitColorPalette selects the color theme based on NUGS_THEME env var.
func InitColorPalette() {
	theme := strings.ToLower(strings.TrimSpace(os.Getenv("NUGS_THEME")))
	if theme != "" {
		ActiveTheme = theme
	}

	if ActiveTheme == "vivid" {
		initVividPalette()
		return
	}
	initNordOneDarkPalette()
}

func initVividPalette() {
	if SupportsTruecolor() {
		ColorRed = "\033[1;38;2;255;76;102m"
		ColorGreen = "\033[1;38;2;80;250;123m"
		ColorYellow = "\033[1;38;2;255;221;87m"
		ColorBlue = "\033[1;38;2;110;196;255m"
		ColorPurple = "\033[1;38;2;215;130;255m"
		ColorCyan = "\033[1;38;2;0;245;255m"
		return
	}
	if Supports256Color() {
		ColorRed = "\033[1;38;5;203m"
		ColorGreen = "\033[1;38;5;84m"
		ColorYellow = "\033[1;38;5;227m"
		ColorBlue = "\033[1;38;5;81m"
		ColorPurple = "\033[1;38;5;177m"
		ColorCyan = "\033[1;38;5;51m"
		return
	}
	// Basic ANSI fallback
	ColorRed = "\033[1;91m"
	ColorGreen = "\033[1;92m"
	ColorYellow = "\033[1;93m"
	ColorBlue = "\033[1;94m"
	ColorPurple = "\033[1;95m"
	ColorCyan = "\033[1;96m"
}

func initNordOneDarkPalette() {
	if SupportsTruecolor() {
		ColorRed = "\033[1;38;2;224;108;117m"
		ColorGreen = "\033[1;38;2;152;195;121m"
		ColorYellow = "\033[1;38;2;229;192;123m"
		ColorBlue = "\033[1;38;2;143;188;255m"
		ColorPurple = "\033[1;38;2;180;142;255m"
		ColorCyan = "\033[1;38;2;136;220;255m"
		return
	}
	if Supports256Color() {
		ColorRed = "\033[1;38;5;210m"
		ColorGreen = "\033[1;38;5;114m"
		ColorYellow = "\033[1;38;5;222m"
		ColorBlue = "\033[1;38;5;111m"
		ColorPurple = "\033[1;38;5;183m"
		ColorCyan = "\033[1;38;5;159m"
	}
}

// SupportsTruecolor checks if the terminal supports 24-bit color.
func SupportsTruecolor() bool {
	term := strings.ToLower(os.Getenv("TERM"))
	colorTerm := strings.ToLower(os.Getenv("COLORTERM"))
	return strings.Contains(colorTerm, "truecolor") ||
		strings.Contains(colorTerm, "24bit") ||
		strings.Contains(term, "truecolor") ||
		strings.Contains(term, "24bit")
}

// Supports256Color checks if the terminal supports 256 colors.
func Supports256Color() bool {
	term := strings.ToLower(os.Getenv("TERM"))
	return strings.Contains(term, "256color")
}
