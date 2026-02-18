package main

// Format wrappers delegating to internal/ui during migration.
// Migration plan: Phase 12 will move renderProgressBox and renderCompletionSummary
// into internal/ui, eliminating the aliases below. These functions remain here
// because they depend on root-level runtime callbacks (updateRuntimeProgress)
// and package-level color variables from structs.go.

import (
	"fmt"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/jmagar/nugs-cli/internal/model"
	"github.com/jmagar/nugs-cli/internal/ui"
)

const defaultProgressRenderInterval = model.DefaultProgressRenderInterval

// Box drawing character wrappers
var (
	boxTopLeft     = ui.BoxTopLeft
	boxTopRight    = ui.BoxTopRight
	boxBottomLeft  = ui.BoxBottomLeft
	boxBottomRight = ui.BoxBottomRight
	boxVertical    = ui.BoxVertical
	boxHorizontal  = ui.BoxHorizontal
	boxTeeLeft     = ui.BoxTeeLeft
	boxTeeRight    = ui.BoxTeeRight
	boxTeeTop      = ui.BoxTeeTop
	boxTeeBottom   = ui.BoxTeeBottom
	boxCross       = ui.BoxCross

	boxDoubleHorizontal  = ui.BoxDoubleHorizontal
	boxDoubleTopLeft     = ui.BoxDoubleTopLeft
	boxDoubleTopRight    = ui.BoxDoubleTopRight
	boxDoubleBottomLeft  = ui.BoxDoubleBottomLeft
	boxDoubleBottomRight = ui.BoxDoubleBottomRight

	bulletSquare  = ui.BulletSquare
	bulletCircle  = ui.BulletCircle
	bulletArrow   = ui.BulletArrow
	bulletDiamond = ui.BulletDiamond
)

// ansiRegex delegates to ui.AnsiRegex
var ansiRegex = ui.AnsiRegex

func getTermWidth() int                                { return ui.GetTermWidth() }
func stripAnsiCodes(s string) string                   { return ui.StripAnsiCodes(s) }
func visibleLength(s string) int                       { return ui.VisibleLength(s) }
func truncateWithEllipsis(s string, maxLen int) string { return ui.TruncateWithEllipsis(s, maxLen) }
func padRight(s string, width int) string              { return ui.PadRight(s, width) }
func padCenter(s string, width int) string             { return ui.PadCenter(s, width) }
func printHeader(title string)                         { ui.PrintHeader(title) }
func printSection(title string)                        { ui.PrintSection(title) }
func printList(items []string, color string)           { ui.PrintList(items, color) }
func printKeyValue(key, value, valueColor string)      { ui.PrintKeyValue(key, value, valueColor) }
func printDivider()                                    { ui.PrintDivider() }
func printBox(text string, borderColor string)         { ui.PrintBox(text, borderColor) }

// Table type aliases
type TableColumn = ui.TableColumn
type Table = ui.Table

func NewTable(columns []TableColumn) *Table { return ui.NewTable(columns) }

// renderProgress uses the ui.RenderProgress with a callback that updates runtime status.
func renderProgress(label string, percentage int, speed, downloaded, total, fillColor string, alignRight bool) {
	ui.RenderProgress(label, percentage, speed, downloaded, total, fillColor, alignRight,
		func(l string, p int, s, d, t string) {
			updateRuntimeProgress(l, p, s, d, t)
		})
}

func printProgress(percentage int, speed, downloaded, total string) {
	renderProgress("DL", percentage, speed, downloaded, total, colorGreen, false)
}

func printUploadProgress(percentage int, speed, uploaded, total string) {
	renderProgress("UP", percentage, speed, uploaded, total, colorBlue, true)
}

// getQualityName delegates to model.GetQualityName.
func getQualityName(format int) string {
	return model.GetQualityName(format)
}

// calculateBoxWidth determines optimal box width based on terminal size (Tier 1 enhancement)
// Returns a width between 79 (minimum) and 120 (maximum) characters
func calculateBoxWidth() int {
	termWidth := getTermWidth()

	// Minimum width for readability
	const minWidth = 79
	// Maximum width to prevent overly wide boxes
	const maxWidth = 120

	// Use 95% of terminal width to leave some breathing room
	boxWidth := int(float64(termWidth) * 0.95)

	// Clamp to min/max bounds
	if boxWidth < minWidth {
		return minWidth
	}
	if boxWidth > maxWidth {
		return maxWidth
	}

	return boxWidth
}

// progressSnapshot holds all state needed to render the progress box.
type progressSnapshot struct {
	ShowNumber      string
	ShowTitle       string
	CurrentPhase    string
	TrackNumber     int
	TrackTotal      int
	TrackName       string
	TrackFormat     string
	DownloadPercent int
	DownloadSpeed   string
	DownloadETA     string
	Downloaded      string
	DownloadTotal   string
	SpeedHistory    []float64
	UploadPercent   int
	UploadSpeed     string
	UploadETA       string
	Uploaded        string
	UploadTotal     string
	RcloneEnabled   bool
	ShowDownloaded  string
	ShowTotal       string
	ShowPercent     int
	DisplayMessage  string
	LinesDrawn      int
	// Batch state (nil if not in batch)
	BatchCurrentAlbum int
	BatchTotalAlbums  int
	BatchComplete     int
	BatchFailed       int
	BatchElapsed      time.Duration
	HasBatch          bool
}

// snapshotProgressState copies all fields from ProgressBoxState under lock.
func snapshotProgressState(state *ProgressBoxState) (progressSnapshot, bool) {
	state.Mu.Lock()
	defer state.Mu.Unlock()

	if !state.ShouldRenderLocked(time.Now()) {
		return progressSnapshot{}, false
	}

	snap := progressSnapshot{
		ShowNumber:      state.ShowNumber,
		ShowTitle:       state.ShowTitle,
		CurrentPhase:    state.CurrentPhase,
		TrackNumber:     state.TrackNumber,
		TrackTotal:      state.TrackTotal,
		TrackName:       state.TrackName,
		TrackFormat:     state.TrackFormat,
		DownloadPercent: state.DownloadPercent,
		DownloadSpeed:   state.DownloadSpeed,
		DownloadETA:     state.DownloadETA,
		Downloaded:      state.Downloaded,
		DownloadTotal:   state.DownloadTotal,
		UploadPercent:   state.UploadPercent,
		UploadSpeed:     state.UploadSpeed,
		UploadETA:       state.UploadETA,
		Uploaded:        state.Uploaded,
		UploadTotal:     state.UploadTotal,
		RcloneEnabled:   state.RcloneEnabled,
		ShowDownloaded:  state.ShowDownloaded,
		ShowTotal:       state.ShowTotal,
		ShowPercent:     state.ShowPercent,
		DisplayMessage:  state.GetDisplayMessage(colorRed, colorYellow, colorCyan, colorReset, symbolCross, symbolWarning, symbolInfo),
		LinesDrawn:      state.LinesDrawn,
	}
	// Copy speed history slice to avoid data race
	if len(state.SpeedHistory) > 0 {
		snap.SpeedHistory = make([]float64, len(state.SpeedHistory))
		copy(snap.SpeedHistory, state.SpeedHistory)
	}
	if state.BatchState != nil {
		state.BatchState.Validate()
		snap.HasBatch = true
		snap.BatchCurrentAlbum = state.BatchState.CurrentAlbum
		snap.BatchTotalAlbums = state.BatchState.TotalAlbums
		snap.BatchComplete = state.BatchState.Complete
		snap.BatchFailed = state.BatchState.Failed
		snap.BatchElapsed = time.Since(state.BatchState.StartTime)
	}
	return snap, true
}

// renderProgressBox draws the complete progress box with dual progress bars.
func renderProgressBox(state *ProgressBoxState) {
	if state == nil {
		return
	}

	snap, ok := snapshotProgressState(state)
	if !ok {
		return
	}

	// All I/O below uses snap -- no lock held
	width := calculateBoxWidth()
	lineCount := renderProgressBoxFromSnapshot(snap, width)

	state.Mu.Lock()
	state.LinesDrawn = lineCount
	state.Mu.Unlock()
}

// renderProgressBoxFromSnapshot renders the progress box from a snapshot and returns the line count.
func renderProgressBoxFromSnapshot(snap progressSnapshot, width int) int {
	innerWidth := width - 2
	contentWidth := width - 4
	if contentWidth < 1 {
		contentWidth = 1
	}
	fitLine := func(line string) string {
		return fitLineForWidth(line, contentWidth, innerWidth)
	}

	// Clear previous box
	if snap.LinesDrawn > 0 {
		for i := 0; i < snap.LinesDrawn; i++ {
			fmt.Print("\033[A\033[2K")
		}
	}

	lineCount := 0

	// Top border
	fmt.Printf("%s%s%s%s%s\n", colorCyan, boxDoubleTopLeft, strings.Repeat(boxDoubleHorizontal, width), boxDoubleTopRight, colorReset)
	lineCount++

	// Batch header
	if snap.HasBatch {
		batchHeader := fmt.Sprintf("  \U0001f4e6 Batch Progress: %d/%d albums │ Complete: %d │ Failed: %d │ Time: %s",
			snap.BatchCurrentAlbum, snap.BatchTotalAlbums, snap.BatchComplete, snap.BatchFailed, formatDuration(snap.BatchElapsed))
		lineCount += printBoxLine(fitLine(batchHeader), colorPurple, innerWidth)
		fmt.Printf("%s%s%s%s%s\n", colorCyan, boxTeeLeft, strings.Repeat(boxHorizontal, width), boxTeeRight, colorReset)
		lineCount++
	}

	// Show header and title
	lineCount += printBoxLine(fitLine(fmt.Sprintf("  %s %s", symbolDownload, snap.ShowNumber)), colorCyan, innerWidth)
	lineCount += printBoxLine(fitLine(fmt.Sprintf("  %s", snap.ShowTitle)), colorCyan, innerWidth)

	// Separator
	fmt.Printf("%s\u2560%s\u2563%s\n", colorCyan, strings.Repeat(boxDoubleHorizontal, width), colorReset)
	lineCount++

	lineCount += printEmptyBoxLine(innerWidth)

	// Track info or upload phase
	if snap.CurrentPhase == model.PhaseUpload {
		lineCount += printBoxLine(fitLine("  \u2b06 Uploading to remote storage..."), colorCyan, innerWidth)
	} else {
		lineCount += printBoxLine(fitLine(fmt.Sprintf("  %s Track %d/%d: %s - %s",
			symbolDownload, snap.TrackNumber, snap.TrackTotal, snap.TrackName, snap.TrackFormat)), colorCyan, innerWidth)
	}

	lineCount += printEmptyBoxLine(innerWidth)

	// Download progress bar
	lineCount += printBoxLine(fitLine(buildDownloadLine(snap)), colorCyan, innerWidth)

	// Upload progress bar (only if rclone enabled)
	if snap.RcloneEnabled {
		lineCount += printBoxLine(fitLine(buildUploadLine(snap)), colorCyan, innerWidth)
	}

	lineCount += printEmptyBoxLine(innerWidth)

	// Message line
	if snap.DisplayMessage != "" {
		lineCount += printBoxLine(fitLine(fmt.Sprintf("  %s", snap.DisplayMessage)), colorCyan, innerWidth)
		lineCount += printEmptyBoxLine(innerWidth)
	}

	// Show progress
	showLine := fmt.Sprintf("  Show Progress: Track %02d/%02d │ %s/%s total (%d%%)",
		snap.TrackNumber, snap.TrackTotal, snap.ShowDownloaded, snap.ShowTotal, snap.ShowPercent)
	lineCount += printBoxLine(fitLine(showLine), colorCyan, innerWidth)

	lineCount += printEmptyBoxLine(innerWidth)

	// Bottom border
	fmt.Printf("%s%s%s%s%s\n", colorCyan, boxDoubleBottomLeft, strings.Repeat(boxDoubleHorizontal, width), boxDoubleBottomRight, colorReset)
	lineCount++

	return lineCount
}

// printBoxLine prints a single content line inside the box, returns 1.
func printBoxLine(content, borderColor string, innerWidth int) int {
	fmt.Printf("%s%s%s %s %s%s%s\n",
		borderColor, boxVertical, colorReset,
		content,
		colorCyan, boxVertical, colorReset)
	return 1
}

// buildDownloadLine constructs the download progress bar string.
func buildDownloadLine(snap progressSnapshot) string {
	dlBar := buildProgressBar(snap.DownloadPercent, 30, colorGreen)
	sparkline := generateSparkline(snap.SpeedHistory, 7)

	sparkPart := ""
	if sparkline != "" {
		sparkPart = " " + sparkline
	}

	if snap.DownloadETA != "" {
		return fmt.Sprintf("  Download [%s] %3d%% @ %s/s%s │ ETA %s",
			dlBar, snap.DownloadPercent, snap.DownloadSpeed, sparkPart, snap.DownloadETA)
	}
	return fmt.Sprintf("  Download [%s] %3d%% @ %s/s%s │ %s/%s",
		dlBar, snap.DownloadPercent, snap.DownloadSpeed, sparkPart, snap.Downloaded, snap.DownloadTotal)
}

// buildUploadLine constructs the upload progress bar string.
func buildUploadLine(snap progressSnapshot) string {
	ulBar := buildProgressBar(snap.UploadPercent, 30, colorBlue)
	if snap.UploadETA != "" {
		return fmt.Sprintf("  Upload   [%s] %3d%% @ %s/s │ ETA %s",
			ulBar, snap.UploadPercent, snap.UploadSpeed, snap.UploadETA)
	}
	return fmt.Sprintf("  Upload   [%s] %3d%% @ %s/s │ %s/%s",
		ulBar, snap.UploadPercent, snap.UploadSpeed, snap.Uploaded, snap.UploadTotal)
}

// buildProgressBar creates a colored progress bar string
func buildProgressBar(percentage int, width int, fillColor string) string {
	if percentage < 0 {
		percentage = 0
	}
	if percentage > 100 {
		percentage = 100
	}

	filled := (percentage * width) / 100
	empty := width - filled

	bar := fillColor + strings.Repeat("█", filled) + colorReset +
		strings.Repeat("░", empty)

	return bar
}

// calculateETA calculates estimated time remaining based on speed history and remaining bytes
func calculateETA(speedHistory []float64, remaining int64) string {
	if remaining <= 0 {
		return ""
	}
	if len(speedHistory) == 0 {
		return ""
	}
	var totalSpeed float64
	for _, speed := range speedHistory {
		totalSpeed += speed
	}
	avgSpeed := totalSpeed / float64(len(speedHistory))
	if avgSpeed < 0.001 {
		return ""
	}
	etaSeconds := float64(remaining) / avgSpeed
	if etaSeconds > 86400 {
		return ""
	}
	if etaSeconds < 1 {
		return ""
	}
	return formatDuration(time.Duration(etaSeconds * float64(time.Second)))
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return "0s"
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// updateSpeedHistory adds a new speed sample and maintains last 10 samples
func updateSpeedHistory(history []float64, newSpeed float64) []float64 {
	history = append(history, newSpeed)
	if len(history) > 10 {
		history = history[1:] // Keep only last 10
	}
	return history
}

// generateSparkline creates an ASCII sparkline from speed history
func generateSparkline(values []float64, maxWidth int) string {
	if len(values) == 0 {
		return ""
	}
	if len(values) > maxWidth {
		values = values[len(values)-maxWidth:]
	}
	minVal, maxVal := values[0], values[0]
	for _, v := range values {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}
	if maxVal == minVal {
		if maxVal == 0 {
			return strings.Repeat("▁", len(values))
		}
		return strings.Repeat("█", len(values))
	}
	blocks := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	var sparkline strings.Builder
	for _, v := range values {
		normalized := ((v - minVal) / (maxVal - minVal)) * 7
		index := int(normalized)
		if index > 7 {
			index = 7
		}
		sparkline.WriteRune(blocks[index])
	}
	return sparkline.String()
}

// completionSnapshot holds all state needed to render the completion summary.
type completionSnapshot struct {
	ShowTitle         string
	AccumulatedTracks int
	TrackTotal        int
	ShowTotal         string
	TotalDuration     time.Duration
	AccumulatedBytes  int64
	SkippedTracks     int
	ErrorTracks       int
	RcloneEnabled     bool
	UploadDuration    time.Duration
	UploadTotal       string
	LinesDrawn        int
}

// renderCompletionSummary displays final summary when all tracks complete
func renderCompletionSummary(state *ProgressBoxState) {
	// Snapshot all state under a single lock acquisition
	state.Mu.Lock()
	snap := completionSnapshot{
		ShowTitle:         state.ShowTitle,
		AccumulatedTracks: state.AccumulatedTracks,
		TrackTotal:        state.TrackTotal,
		ShowTotal:         state.ShowTotal,
		TotalDuration:     state.TotalDuration,
		AccumulatedBytes:  state.AccumulatedBytes,
		SkippedTracks:     state.SkippedTracks,
		ErrorTracks:       state.ErrorTracks,
		RcloneEnabled:     state.RcloneEnabled,
		UploadDuration:    state.UploadDuration,
		UploadTotal:       state.UploadTotal,
		LinesDrawn:        state.LinesDrawn,
	}
	state.Mu.Unlock()

	// All I/O below uses snap -- no lock held
	width := calculateBoxWidth()
	lineCount := renderCompletionBox(snap, width)

	state.Mu.Lock()
	state.LinesDrawn = lineCount
	state.Mu.Unlock()
}

// renderCompletionBox renders the completion summary box and returns the line count.
func renderCompletionBox(snap completionSnapshot, width int) int {
	innerWidth := width - 2
	contentWidth := width - 4
	if contentWidth < 1 {
		contentWidth = 1
	}

	// Clear previous box
	if snap.LinesDrawn > 0 {
		for i := 0; i < snap.LinesDrawn; i++ {
			fmt.Print("\033[A\033[2K")
		}
	}

	lineCount := 0

	// Top border with double lines
	fmt.Printf("%s%s%s%s%s\n",
		colorGreen, boxDoubleTopLeft,
		strings.Repeat(boxDoubleHorizontal, width),
		boxDoubleTopRight, colorReset)
	lineCount++

	// Header
	headerText := "DOWNLOAD COMPLETE"
	if snap.RcloneEnabled && snap.UploadDuration > 0 {
		headerText = "COMPLETE"
	}
	header := fmt.Sprintf("  %s %s", symbolCheck, headerText)
	fmt.Printf("%s%s%s %s %s%s%s\n",
		colorCyan, boxVertical, colorReset,
		fitLineForWidth(header, contentWidth, innerWidth),
		colorCyan, boxVertical, colorReset)
	lineCount++

	// Show title
	titleLine := fmt.Sprintf("  %s", snap.ShowTitle)
	fmt.Printf("%s%s%s %s %s%s%s\n",
		colorCyan, boxVertical, colorReset,
		fitLineForWidth(titleLine, contentWidth, innerWidth),
		colorCyan, boxVertical, colorReset)
	lineCount++

	// Separator
	fmt.Printf("%s%s%s%s%s\n",
		colorCyan, "\u2560",
		strings.Repeat(boxDoubleHorizontal, width),
		"\u2563", colorReset)
	lineCount++

	lineCount += printEmptyBoxLine(innerWidth)

	// Stats lines
	stats := buildCompletionStats(snap)
	for _, stat := range stats {
		fmt.Printf("%s%s%s %s %s%s%s\n",
			colorCyan, boxVertical, colorReset,
			fitLineForWidth(stat, contentWidth, innerWidth),
			colorCyan, boxVertical, colorReset)
		lineCount++
	}

	lineCount += printEmptyBoxLine(innerWidth)

	// Bottom border
	fmt.Printf("%s%s%s%s%s\n",
		colorGreen, boxDoubleBottomLeft,
		strings.Repeat(boxDoubleHorizontal, width),
		boxDoubleBottomRight, colorReset)
	lineCount++

	return lineCount
}

// buildCompletionStats constructs the stats lines for the completion summary.
func buildCompletionStats(snap completionSnapshot) []string {
	stats := []string{
		fmt.Sprintf("  Tracks Downloaded:  %d/%d", snap.AccumulatedTracks, snap.TrackTotal),
		fmt.Sprintf("  Total Size:         %s", snap.ShowTotal),
		fmt.Sprintf("  Duration:           %s", formatDuration(snap.TotalDuration)),
	}

	if snap.SkippedTracks > 0 {
		stats = append(stats, fmt.Sprintf("  Skipped:            %d", snap.SkippedTracks))
	}
	if snap.ErrorTracks > 0 {
		stats = append(stats, fmt.Sprintf("  Errors:             %d", snap.ErrorTracks))
	}
	if shouldCalculateSpeed(snap.TotalDuration, snap.AccumulatedBytes) {
		avgSpeed := float64(snap.AccumulatedBytes) / snap.TotalDuration.Seconds()
		stats = append(stats, fmt.Sprintf("  Avg Speed:          %s/s", humanize.Bytes(uint64(avgSpeed))))
	}

	if snap.RcloneEnabled && snap.UploadDuration > 0 {
		stats = append(stats, "")
		stats = append(stats, fmt.Sprintf("  Upload Size:        %s", snap.UploadTotal))
		stats = append(stats, fmt.Sprintf("  Upload Duration:    %s", formatDuration(snap.UploadDuration)))
		if snap.UploadDuration.Seconds() >= 0.001 {
			uploadBytes := parseHumanizedBytes(snap.UploadTotal)
			if uploadBytes > 0 {
				uploadAvgSpeed := float64(uploadBytes) / snap.UploadDuration.Seconds()
				stats = append(stats, fmt.Sprintf("  Upload Avg Speed:   %s/s", humanize.Bytes(uint64(uploadAvgSpeed))))
			}
		}
	}

	return stats
}

// fitLineForWidth truncates and pads a line for box rendering.
func fitLineForWidth(line string, contentWidth, innerWidth int) string {
	return padRight(truncateWithEllipsis(line, contentWidth), innerWidth)
}

// printEmptyBoxLine prints an empty line inside the box and returns 1.
func printEmptyBoxLine(innerWidth int) int {
	fmt.Printf("%s%s%s %s %s%s%s\n",
		colorCyan, boxVertical, colorReset,
		strings.Repeat(" ", innerWidth),
		colorCyan, boxVertical, colorReset)
	return 1
}

// shouldCalculateSpeed returns true if there is enough data for a meaningful speed calculation.
func shouldCalculateSpeed(duration time.Duration, bytes int64) bool {
	return duration.Seconds() >= 0.001 && bytes > 0
}
