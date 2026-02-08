package main

// Format wrappers delegating to internal/ui during migration.
// These will be removed in Phase 12 when all callers move to internal packages.
// ProgressBoxState and related rendering code remain here because they depend on
// root-level runtime functions (updateRuntimeProgress) and color variables.

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/jmagar/nugs-cli/internal/ui"
)

const defaultProgressRenderInterval = 100 * time.Millisecond

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

func getTermWidth() int                          { return ui.GetTermWidth() }
func stripAnsiCodes(s string) string             { return ui.StripAnsiCodes(s) }
func visibleLength(s string) int                 { return ui.VisibleLength(s) }
func truncateWithEllipsis(s string, maxLen int) string { return ui.TruncateWithEllipsis(s, maxLen) }
func padRight(s string, width int) string        { return ui.PadRight(s, width) }
func padCenter(s string, width int) string       { return ui.PadCenter(s, width) }
func printHeader(title string)                   { ui.PrintHeader(title) }
func printSection(title string)                  { ui.PrintSection(title) }
func printList(items []string, color string)      { ui.PrintList(items, color) }
func printKeyValue(key, value, valueColor string) { ui.PrintKeyValue(key, value, valueColor) }
func printDivider()                              { ui.PrintDivider() }
func printBox(text string, borderColor string)   { ui.PrintBox(text, borderColor) }

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

// ProgressBoxState tracks the state of the dual progress box display
type ProgressBoxState struct {
	mu sync.Mutex // Protects all fields from concurrent access

	ShowTitle         string
	ShowNumber        string
	TrackNumber       int
	TrackTotal        int
	TrackName         string
	TrackFormat       string
	DownloadPercent   int
	DownloadSpeed     string
	Downloaded        string
	DownloadTotal     string
	UploadPercent     int
	UploadSpeed       string
	Uploaded          string
	UploadTotal       string
	ShowPercent       int
	ShowDownloaded    string
	ShowTotal         string
	AccumulatedBytes  int64 // Accumulated download across all tracks
	AccumulatedTracks int   // Number of completed tracks
	RcloneEnabled     bool
	LinesDrawn        int

	// ETA tracking
	DownloadETA        string        // Estimated time remaining for download
	UploadETA          string        // Estimated time remaining for upload
	SpeedHistory       []float64     // Last 10 download speed samples for smoothing
	UploadSpeedHistory []float64     // Last 10 upload speed samples for smoothing (thread-safe)
	LastUpdateTime     time.Time     // Last time the progress box was rendered
	RenderInterval     time.Duration // Minimum interval between redraws (defaults to 100ms)

	// Completion tracking
	IsComplete     bool          // Whether all tracks have completed
	CompletionTime time.Time     // When the download completed
	TotalDuration  time.Duration // Total time taken for the show
	SkippedTracks  int           // Number of tracks skipped (already exist)
	ErrorTracks    int           // Number of tracks that failed
	StartTime      time.Time     // When the download started

	// Phase tracking (Tier 1 enhancement)
	CurrentPhase string // Current operation phase (download, upload, verify, paused, error)
	StatusColor  string // ANSI color code for current phase

	// Message display (Tier 3 enhancement)
	ErrorMessage    string    // Current error message to display
	WarningMessage  string    // Current warning message to display
	StatusMessage   string    // Current status message to display
	MessageExpiry   time.Time // When the current message should expire
	MessagePriority int       // Priority of current message (1=status, 2=warning, 3=error)

	// State indicators (Tier 3 enhancement)
	IsPaused    bool // Whether the download/upload is paused
	IsCancelled bool // Whether the operation was cancelled

	// Batch progress (Tier 4 enhancement)
	BatchState *BatchProgressState // Optional batch context for multi-album operations

	// Render-state tracking for smart redraw decisions
	LastRenderedTrackNumber     int
	LastRenderedMessagePriority int
	LastRenderedPaused          bool
	LastRenderedCancelled       bool
	forceRender                 bool
}

// SetPhase sets the current operation phase and corresponding color (Tier 1 enhancement)
func (s *ProgressBoxState) SetPhase(phase string) {
	s.CurrentPhase = phase
	switch phase {
	case "download":
		s.StatusColor = colorGreen
	case "upload":
		s.StatusColor = colorBlue
	case "verify":
		s.StatusColor = colorYellow
	case "paused":
		s.StatusColor = colorYellow
	case "error":
		s.StatusColor = colorRed
	default:
		s.StatusColor = colorReset
	}
}

func (s *ProgressBoxState) getRenderIntervalLocked() time.Duration {
	if s.RenderInterval <= 0 {
		return defaultProgressRenderInterval
	}
	return s.RenderInterval
}

func (s *ProgressBoxState) hasCriticalRenderChangeLocked() bool {
	return s.TrackNumber != s.LastRenderedTrackNumber ||
		s.MessagePriority != s.LastRenderedMessagePriority ||
		s.IsPaused != s.LastRenderedPaused ||
		s.IsCancelled != s.LastRenderedCancelled
}

func (s *ProgressBoxState) shouldRenderLocked(now time.Time) bool {
	force := s.forceRender || s.hasCriticalRenderChangeLocked()
	if !force && !s.LastUpdateTime.IsZero() &&
		now.Sub(s.LastUpdateTime) < s.getRenderIntervalLocked() {
		return false
	}

	s.LastUpdateTime = now
	s.forceRender = false
	s.LastRenderedTrackNumber = s.TrackNumber
	s.LastRenderedMessagePriority = s.MessagePriority
	s.LastRenderedPaused = s.IsPaused
	s.LastRenderedCancelled = s.IsCancelled
	return true
}

func (s *ProgressBoxState) shouldRender(now time.Time) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.shouldRenderLocked(now)
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

// renderProgressBox draws the complete progress box with dual progress bars
func renderProgressBox(state *ProgressBoxState) {
	if state == nil {
		return
	}

	// Lock for reading state (released after we copy all values we need)
	state.mu.Lock()
	if !state.shouldRenderLocked(time.Now()) {
		state.mu.Unlock()
		return
	}

	width := calculateBoxWidth() // Dynamic width based on terminal size (Tier 1 enhancement)

	// Clear previous box (move up and clear lines)
	linesToClear := state.LinesDrawn
	state.mu.Unlock() // Release lock before I/O operations

	if linesToClear > 0 {
		for i := 0; i < linesToClear; i++ {
			fmt.Print("\033[A\033[2K") // Move up one line and clear it
		}
	}

	// Re-acquire lock to read all state for rendering
	state.mu.Lock()
	defer state.mu.Unlock()

	lineCount := 0

	// Top border with double lines
	fmt.Printf("%s%s%s%s%s\n",
		colorCyan, boxDoubleTopLeft,
		strings.Repeat(boxDoubleHorizontal, width),
		boxDoubleTopRight, colorReset)
	lineCount++

	// Batch header (Tier 4 enhancement) - only show if we're in a batch operation
	if state.BatchState != nil {
		batch := state.BatchState
		batch.Validate() // Ensure batch counters are consistent before rendering
		elapsed := time.Since(batch.StartTime)
		batchHeader := fmt.Sprintf("  📦 Batch Progress: %d/%d albums │ Complete: %d │ Failed: %d │ Time: %s",
			batch.CurrentAlbum, batch.TotalAlbums, batch.Complete, batch.Failed, formatDuration(elapsed))
		fmt.Printf("%s%s%s %s %s%s%s\n",
			colorPurple, boxVertical, colorReset,
			padRight(truncateWithEllipsis(batchHeader, width-2), width-2),
			colorPurple, boxVertical, colorReset)
		lineCount++

		// Separator line between batch and album
		fmt.Printf("%s%s%s%s%s\n",
			colorCyan, boxTeeLeft,
			strings.Repeat(boxHorizontal, width),
			boxTeeRight, colorReset)
		lineCount++
	}

	// Show header line (download number and date)
	header := fmt.Sprintf("  %s %s", symbolDownload, state.ShowNumber)
	fmt.Printf("%s%s%s %s %s%s%s\n",
		colorCyan, boxVertical, colorReset,
		padRight(truncateWithEllipsis(header, width-2), width-2),
		colorCyan, boxVertical, colorReset)
	lineCount++

	// Show title line (artist - venue, location)
	titleLine := fmt.Sprintf("  %s", state.ShowTitle)
	fmt.Printf("%s%s%s %s %s%s%s\n",
		colorCyan, boxVertical, colorReset,
		padRight(truncateWithEllipsis(titleLine, width-2), width-2),
		colorCyan, boxVertical, colorReset)
	lineCount++

	// Middle separator with double lines
	fmt.Printf("%s%s%s%s%s\n",
		colorCyan, "╠",
		strings.Repeat(boxDoubleHorizontal, width),
		"╣", colorReset)
	lineCount++

	// Empty line
	fmt.Printf("%s%s%s %s %s%s%s\n",
		colorCyan, boxVertical, colorReset,
		strings.Repeat(" ", width-2),
		colorCyan, boxVertical, colorReset)
	lineCount++

	// Track info line
	trackInfo := fmt.Sprintf("  %s Track %d/%d: %s - %s",
		symbolDownload, state.TrackNumber, state.TrackTotal, state.TrackName, state.TrackFormat)
	fmt.Printf("%s%s%s %s %s%s%s\n",
		colorCyan, boxVertical, colorReset,
		padRight(truncateWithEllipsis(trackInfo, width-2), width-2),
		colorCyan, boxVertical, colorReset)
	lineCount++

	// Empty line
	fmt.Printf("%s%s%s %s %s%s%s\n",
		colorCyan, boxVertical, colorReset,
		strings.Repeat(" ", width-2),
		colorCyan, boxVertical, colorReset)
	lineCount++

	// Download progress bar with ETA and sparkline
	dlBar := buildProgressBar(state.DownloadPercent, 30, colorGreen)
	sparkline := generateSparkline(state.SpeedHistory, 7)
	dlLine := ""
	if state.DownloadETA != "" {
		if sparkline != "" {
			dlLine = fmt.Sprintf("  Download [%s] %3d%% @ %s/s %s │ ETA %s",
				dlBar, state.DownloadPercent, state.DownloadSpeed, sparkline, state.DownloadETA)
		} else {
			dlLine = fmt.Sprintf("  Download [%s] %3d%% @ %s/s │ ETA %s",
				dlBar, state.DownloadPercent, state.DownloadSpeed, state.DownloadETA)
		}
	} else {
		if sparkline != "" {
			dlLine = fmt.Sprintf("  Download [%s] %3d%% @ %s/s %s │ %s/%s",
				dlBar, state.DownloadPercent, state.DownloadSpeed, sparkline, state.Downloaded, state.DownloadTotal)
		} else {
			dlLine = fmt.Sprintf("  Download [%s] %3d%% @ %s/s │ %s/%s",
				dlBar, state.DownloadPercent, state.DownloadSpeed, state.Downloaded, state.DownloadTotal)
		}
	}
	fmt.Printf("%s%s%s %s %s%s%s\n",
		colorCyan, boxVertical, colorReset,
		padRight(dlLine, width-2),
		colorCyan, boxVertical, colorReset)
	lineCount++

	// Upload progress bar (only if rclone enabled) with ETA
	// Note: sparkline not shown for upload as it would show stale download speeds
	if state.RcloneEnabled {
		ulBar := buildProgressBar(state.UploadPercent, 30, colorBlue)
		ulLine := ""
		if state.UploadETA != "" {
			ulLine = fmt.Sprintf("  Upload   [%s] %3d%% @ %s/s │ ETA %s",
				ulBar, state.UploadPercent, state.UploadSpeed, state.UploadETA)
		} else {
			ulLine = fmt.Sprintf("  Upload   [%s] %3d%% @ %s/s │ %s/%s",
				ulBar, state.UploadPercent, state.UploadSpeed, state.Uploaded, state.UploadTotal)
		}
		fmt.Printf("%s%s%s %s %s%s%s\n",
			colorCyan, boxVertical, colorReset,
			padRight(ulLine, width-2),
			colorCyan, boxVertical, colorReset)
		lineCount++
	}

	// Empty line
	fmt.Printf("%s%s%s %s %s%s%s\n",
		colorCyan, boxVertical, colorReset,
		strings.Repeat(" ", width-2),
		colorCyan, boxVertical, colorReset)
	lineCount++

	// Message line (errors, warnings, status) - Tier 3 enhancement
	if msg := state.GetDisplayMessage(); msg != "" {
		msgLine := fmt.Sprintf("  %s", msg)
		fmt.Printf("%s%s%s %s %s%s%s\n",
			colorCyan, boxVertical, colorReset,
			padRight(truncateWithEllipsis(msgLine, width-2), width-2),
			colorCyan, boxVertical, colorReset)
		lineCount++

		// Empty line after message for spacing
		fmt.Printf("%s%s%s %s %s%s%s\n",
			colorCyan, boxVertical, colorReset,
			strings.Repeat(" ", width-2),
			colorCyan, boxVertical, colorReset)
		lineCount++
	}

	// Show progress line
	showLine := fmt.Sprintf("  Show Progress: Track %02d/%02d │ %s/%s total (%d%%)",
		state.TrackNumber, state.TrackTotal, state.ShowDownloaded, state.ShowTotal, state.ShowPercent)
	fmt.Printf("%s%s%s %s %s%s%s\n",
		colorCyan, boxVertical, colorReset,
		padRight(showLine, width-2),
		colorCyan, boxVertical, colorReset)
	lineCount++

	// Empty line
	fmt.Printf("%s%s%s %s %s%s%s\n",
		colorCyan, boxVertical, colorReset,
		strings.Repeat(" ", width-2),
		colorCyan, boxVertical, colorReset)
	lineCount++

	// Bottom border with double lines
	fmt.Printf("%s%s%s%s%s\n",
		colorCyan, boxDoubleBottomLeft,
		strings.Repeat(boxDoubleHorizontal, width),
		boxDoubleBottomRight, colorReset)
	lineCount++

	state.LinesDrawn = lineCount
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
// Returns formatted ETA string (e.g., "2m 34s", "calculating...")
// Returns empty string for invalid/edge cases to avoid showing misleading ETAs
func calculateETA(speedHistory []float64, remaining int64) string {
	// Guard against negative remaining (progress calculation errors)
	if remaining <= 0 {
		return "" // Already complete or invalid state
	}

	// Show ETA even with just 1 speed sample (don't require full history)
	if len(speedHistory) == 0 {
		return "" // No data yet
	}

	// Calculate average speed from available samples
	var totalSpeed float64
	for _, speed := range speedHistory {
		totalSpeed += speed
	}
	avgSpeed := totalSpeed / float64(len(speedHistory))

	// Avoid division by zero - use threshold for float precision
	if avgSpeed < 0.001 { // Effectively zero (< 1 byte/sec)
		return ""
	}

	// Calculate ETA in seconds
	etaSeconds := float64(remaining) / avgSpeed

	// Sanity check: don't show ETA > 24 hours (likely calculation error)
	if etaSeconds > 86400 {
		return ""
	}

	// Don't show ETA for very small remaining amounts (< 1 second)
	if etaSeconds < 1 {
		return ""
	}

	return formatDuration(time.Duration(etaSeconds * float64(time.Second)))
}

// formatDuration formats a duration into a human-readable string
// Examples: "2m 34s", "1h 23m", "45s"
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
// Uses Unicode block characters: ▁▂▃▄▅▆▇█
func generateSparkline(values []float64, maxWidth int) string {
	if len(values) == 0 {
		return ""
	}

	// Limit to maxWidth samples (use most recent)
	if len(values) > maxWidth {
		values = values[len(values)-maxWidth:]
	}

	// Find min/max for normalization
	minVal, maxVal := values[0], values[0]
	for _, v := range values {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	// Handle edge case: all values the same
	if maxVal == minVal {
		if maxVal == 0 {
			// All zeros - show lowest blocks
			return strings.Repeat("▁", len(values))
		}
		// Constant non-zero speed - show full blocks
		return strings.Repeat("█", len(values))
	}

	// Unicode blocks from lowest to highest
	blocks := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

	var sparkline strings.Builder
	for _, v := range values {
		// Normalize to 0-7 range
		normalized := ((v - minVal) / (maxVal - minVal)) * 7
		index := int(normalized)
		if index > 7 {
			index = 7
		}
		sparkline.WriteRune(blocks[index])
	}

	return sparkline.String()
}

// renderCompletionSummary displays final summary when all tracks complete
func renderCompletionSummary(state *ProgressBoxState) {
	// Lock to read final state
	state.mu.Lock()
	defer state.mu.Unlock()

	width := calculateBoxWidth() // Match progress box width

	// Clear previous box
	if state.LinesDrawn > 0 {
		for i := 0; i < state.LinesDrawn; i++ {
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

	// Header: COMPLETED
	header := fmt.Sprintf("  %s DOWNLOAD COMPLETE", symbolCheck)
	fmt.Printf("%s%s%s %s %s%s%s\n",
		colorCyan, boxVertical, colorReset,
		padRight(truncateWithEllipsis(header, width-2), width-2),
		colorCyan, boxVertical, colorReset)
	lineCount++

	// Show title
	titleLine := fmt.Sprintf("  %s", state.ShowTitle)
	fmt.Printf("%s%s%s %s %s%s%s\n",
		colorCyan, boxVertical, colorReset,
		padRight(truncateWithEllipsis(titleLine, width-2), width-2),
		colorCyan, boxVertical, colorReset)
	lineCount++

	// Separator
	fmt.Printf("%s%s%s%s%s\n",
		colorCyan, "╠",
		strings.Repeat(boxDoubleHorizontal, width),
		"╣", colorReset)
	lineCount++

	// Empty line
	fmt.Printf("%s%s%s %s %s%s%s\n",
		colorCyan, boxVertical, colorReset,
		strings.Repeat(" ", width-2),
		colorCyan, boxVertical, colorReset)
	lineCount++

	// Stats lines
	completedTracks := state.AccumulatedTracks
	stats := []string{
		fmt.Sprintf("  Tracks Downloaded:  %d/%d", completedTracks, state.TrackTotal),
		fmt.Sprintf("  Total Size:         %s", state.ShowTotal),
		fmt.Sprintf("  Duration:           %s", formatDuration(state.TotalDuration)),
	}

	// Add skipped/error stats if any
	if state.SkippedTracks > 0 {
		stats = append(stats, fmt.Sprintf("  Skipped:            %d", state.SkippedTracks))
	}
	if state.ErrorTracks > 0 {
		stats = append(stats, fmt.Sprintf("  Errors:             %d", state.ErrorTracks))
	}

	// Calculate average speed if duration > 0
	if state.TotalDuration.Seconds() > 0 {
		avgSpeed := float64(state.AccumulatedBytes) / state.TotalDuration.Seconds()
		stats = append(stats, fmt.Sprintf("  Avg Speed:          %s/s", humanize.Bytes(uint64(avgSpeed))))
	}

	for _, stat := range stats {
		fmt.Printf("%s%s%s %s %s%s%s\n",
			colorCyan, boxVertical, colorReset,
			padRight(truncateWithEllipsis(stat, width-2), width-2),
			colorCyan, boxVertical, colorReset)
		lineCount++
	}

	// Empty line
	fmt.Printf("%s%s%s %s %s%s%s\n",
		colorCyan, boxVertical, colorReset,
		strings.Repeat(" ", width-2),
		colorCyan, boxVertical, colorReset)
	lineCount++

	// Bottom border
	fmt.Printf("%s%s%s%s%s\n",
		colorGreen, boxDoubleBottomLeft,
		strings.Repeat(boxDoubleHorizontal, width),
		boxDoubleBottomRight, colorReset)
	lineCount++

	state.LinesDrawn = lineCount
}
