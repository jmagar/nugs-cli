package main

// Command rendering aliases are synchronized from the selected UI theme.

import "github.com/jmagar/nugs-cli/internal/ui"

// Color values are snapshotted from ui after theme selection.
var (
	colorReset  string
	colorRed    string
	colorGreen  string
	colorYellow string
	colorBlue   string
	colorPurple string
	colorCyan   string
	colorBold   string
)

// Symbol variable wrappers
var (
	symbolCheck    string
	symbolCross    string
	symbolDownload string
	symbolInfo     string
	symbolWarning  string
	symbolPackage  string
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

	symbolCheck = ui.SymbolCheck
	symbolCross = ui.SymbolCross
	symbolDownload = ui.SymbolDownload
	symbolInfo = ui.SymbolInfo
	symbolWarning = ui.SymbolWarning
	symbolPackage = ui.SymbolPackage
}
