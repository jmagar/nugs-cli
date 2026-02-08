package main

// Theme wrappers delegating to internal/ui during migration.
// These will be removed in Phase 12 when all callers move to internal packages.
// The ui package init() runs first (Go import ordering), setting colors before
// these vars are initialized.

import "github.com/jmagar/nugs-cli/internal/ui"

// Color variable wrappers - these are value copies from ui after its init runs.
var (
	colorReset  string
	colorRed    string
	colorGreen  string
	colorYellow string
	colorBlue   string
	colorPurple string
	colorCyan   string
	colorBold   string
	activeTheme string
)

// Symbol variable wrappers
var (
	symbolCheck    string
	symbolCross    string
	symbolArrow    string
	symbolMusic    string
	symbolUpload   string
	symbolDownload string
	symbolInfo     string
	symbolWarning  string
	symbolGear     string
	symbolPackage  string
	symbolRocket   string
	symbolAudio    string
	symbolVideo    string
	symbolBoth     string
)

func init() {
	syncFromUI()
}

func syncFromUI() {
	colorReset = ui.ColorReset
	colorRed = ui.ColorRed
	colorGreen = ui.ColorGreen
	colorYellow = ui.ColorYellow
	colorBlue = ui.ColorBlue
	colorPurple = ui.ColorPurple
	colorCyan = ui.ColorCyan
	colorBold = ui.ColorBold
	activeTheme = ui.ActiveTheme

	symbolCheck = ui.SymbolCheck
	symbolCross = ui.SymbolCross
	symbolArrow = ui.SymbolArrow
	symbolMusic = ui.SymbolMusic
	symbolUpload = ui.SymbolUpload
	symbolDownload = ui.SymbolDownload
	symbolInfo = ui.SymbolInfo
	symbolWarning = ui.SymbolWarning
	symbolGear = ui.SymbolGear
	symbolPackage = ui.SymbolPackage
	symbolRocket = ui.SymbolRocket
	symbolAudio = ui.SymbolAudio
	symbolVideo = ui.SymbolVideo
	symbolBoth = ui.SymbolBoth
}

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
