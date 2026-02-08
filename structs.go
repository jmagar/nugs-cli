package main

import (
	"os"
	"strings"
)

// ANSI color codes
var (
	colorReset  = "\033[0m"
	colorRed    = "\033[91m"
	colorGreen  = "\033[92m"
	colorYellow = "\033[93m"
	colorBlue   = "\033[94m"
	colorPurple = "\033[95m"
	colorCyan   = "\033[96m"
	colorBold   = "\033[1m"
	activeTheme = "nordonedark"
)

func init() {
	initColorPalette()
}

func initColorPalette() {
	theme := strings.ToLower(strings.TrimSpace(os.Getenv("NUGS_THEME")))
	if theme != "" {
		activeTheme = theme
	}

	if activeTheme == "vivid" {
		initVividPalette()
		return
	}
	initNordOneDarkPalette()
}

func initVividPalette() {
	if supportsTruecolor() {
		colorRed = "\033[1;38;2;255;76;102m"
		colorGreen = "\033[1;38;2;80;250;123m"
		colorYellow = "\033[1;38;2;255;221;87m"
		colorBlue = "\033[1;38;2;110;196;255m"
		colorPurple = "\033[1;38;2;215;130;255m"
		colorCyan = "\033[1;38;2;0;245;255m"
		return
	}
	if supports256Color() {
		colorRed = "\033[1;38;5;203m"
		colorGreen = "\033[1;38;5;84m"
		colorYellow = "\033[1;38;5;227m"
		colorBlue = "\033[1;38;5;81m"
		colorPurple = "\033[1;38;5;177m"
		colorCyan = "\033[1;38;5;51m"
		return
	}
	// Basic ANSI fallback
	colorRed = "\033[1;91m"
	colorGreen = "\033[1;92m"
	colorYellow = "\033[1;93m"
	colorBlue = "\033[1;94m"
	colorPurple = "\033[1;95m"
	colorCyan = "\033[1;96m"
}

func initNordOneDarkPalette() {
	if supportsTruecolor() {
		// Vibrant nord + one-dark blend
		colorRed = "\033[1;38;2;224;108;117m"    // one-dark red
		colorGreen = "\033[1;38;2;152;195;121m"  // one-dark green
		colorYellow = "\033[1;38;2;229;192;123m" // one-dark yellow
		colorBlue = "\033[1;38;2;143;188;255m"   // brighter nord blue
		colorPurple = "\033[1;38;2;180;142;255m" // nord-ish purple accent
		colorCyan = "\033[1;38;2;136;220;255m"   // icy cyan
		return
	}
	if supports256Color() {
		colorRed = "\033[1;38;5;210m"
		colorGreen = "\033[1;38;5;114m"
		colorYellow = "\033[1;38;5;222m"
		colorBlue = "\033[1;38;5;111m"
		colorPurple = "\033[1;38;5;183m"
		colorCyan = "\033[1;38;5;159m"
	}
}

func supportsTruecolor() bool {
	term := strings.ToLower(os.Getenv("TERM"))
	colorTerm := strings.ToLower(os.Getenv("COLORTERM"))
	return strings.Contains(colorTerm, "truecolor") ||
		strings.Contains(colorTerm, "24bit") ||
		strings.Contains(term, "truecolor") ||
		strings.Contains(term, "24bit")
}

func supports256Color() bool {
	term := strings.ToLower(os.Getenv("TERM"))
	return strings.Contains(term, "256color")
}

// Unicode symbols
var (
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

	symbolAudio = "🎵" // audio only
	symbolVideo = "🎬" // video only
	symbolBoth  = "📹" // both available
)

// WriteCounter tracks download progress.
// Kept in root package because it has a Write() method defined in download.go.
type WriteCounter struct {
	Total      int64
	TotalStr   string
	Downloaded int64
	Percentage int
	StartTime  int64
	OnProgress func(downloaded, total, speed int64)
}
