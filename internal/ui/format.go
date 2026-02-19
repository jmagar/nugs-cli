package ui

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/term"
)

// Box drawing characters
const (
	BoxTopLeft     = "┌"
	BoxTopRight    = "┐"
	BoxBottomLeft  = "└"
	BoxBottomRight = "┘"
	BoxVertical    = "│"
	BoxHorizontal  = "─"
	BoxTeeLeft     = "├"
	BoxTeeRight    = "┤"
	BoxTeeTop      = "┬"
	BoxTeeBottom   = "┴"
	BoxCross       = "┼"

	BoxDoubleHorizontal  = "═"
	BoxDoubleTopLeft     = "╔"
	BoxDoubleTopRight    = "╗"
	BoxDoubleBottomLeft  = "╚"
	BoxDoubleBottomRight = "╝"

	BulletSquare  = "▪"
	BulletCircle  = "•"
	BulletArrow   = "▸"
	BulletDiamond = "◆"
)

// AnsiRegex is compiled once for performance.
var AnsiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

const termWidthCacheTTL = 500 * time.Millisecond

var (
	termWidthMu         sync.Mutex
	cachedTermWidth     = 80
	cachedTermWidthTime time.Time
)

// GetTermWidth returns the terminal width, defaulting to 80.
func GetTermWidth() int {
	termWidthMu.Lock()
	if time.Since(cachedTermWidthTime) <= termWidthCacheTTL && cachedTermWidth > 0 {
		width := cachedTermWidth
		termWidthMu.Unlock()
		return width
	}
	termWidthMu.Unlock()

	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width == 0 {
		width = 80
	}

	termWidthMu.Lock()
	cachedTermWidth = width
	cachedTermWidthTime = time.Now()
	termWidthMu.Unlock()

	return width
}

// StripAnsiCodes removes ANSI escape sequences from a string.
func StripAnsiCodes(s string) string {
	return AnsiRegex.ReplaceAllString(s, "")
}

// VisibleLength returns the visible length of a string (excluding ANSI codes).
func VisibleLength(s string) int {
	return utf8.RuneCountInString(StripAnsiCodes(s))
}

// TruncateWithEllipsis truncates a string to maxLen with ellipsis if needed.
func TruncateWithEllipsis(s string, maxLen int) string {
	visibleLen := VisibleLength(s)
	if visibleLen <= maxLen {
		return s
	}
	if maxLen <= 3 {
		stripped := StripAnsiCodes(s)
		runes := []rune(stripped)
		if len(runes) <= maxLen {
			return stripped
		}
		return string(runes[:maxLen])
	}

	codes := AnsiRegex.FindAllString(s, -1)
	stripped := StripAnsiCodes(s)
	runes := []rune(stripped)
	truncated := string(runes[:maxLen-3]) + "..."

	if len(codes) > 0 {
		return codes[0] + truncated + ColorReset
	}

	return truncated
}

// PadRight pads a string to the specified width using visible length.
func PadRight(s string, width int) string {
	visLen := VisibleLength(s)
	if visLen >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visLen)
}

// PadCenter centers a string in the specified width using visible length.
func PadCenter(s string, width int) string {
	visLen := VisibleLength(s)
	if visLen >= width {
		return s
	}
	padding := width - visLen
	leftPad := padding / 2
	rightPad := padding - leftPad
	return strings.Repeat(" ", leftPad) + s + strings.Repeat(" ", rightPad)
}

// PrintHeader prints a styled header with box drawing.
func PrintHeader(title string) {
	width := GetTermWidth()
	titleLen := VisibleLength(title) + 4

	if titleLen > width-4 {
		title = TruncateWithEllipsis(title, width-10)
	}

	lineLen := width - 2

	fmt.Printf("\n%s%s%s%s%s\n",
		ColorCyan, BoxDoubleTopLeft,
		strings.Repeat(BoxDoubleHorizontal, lineLen),
		BoxDoubleTopRight, ColorReset)

	fmt.Printf("%s%s%s %s %s%s%s\n",
		ColorCyan, BoxVertical, ColorReset,
		ColorBold+PadCenter(title, lineLen-2)+ColorReset,
		ColorCyan, BoxVertical, ColorReset)

	fmt.Printf("%s%s%s%s%s\n\n",
		ColorCyan, BoxDoubleBottomLeft,
		strings.Repeat(BoxDoubleHorizontal, lineLen),
		BoxDoubleBottomRight, ColorReset)
}

// PrintSection prints a section title with underline.
func PrintSection(title string) {
	fmt.Printf("\n%s%s %s%s\n", ColorBold, BulletDiamond, title, ColorReset)
	fmt.Printf("%s%s%s\n\n", ColorCyan, strings.Repeat(BoxHorizontal, len(title)+2), ColorReset)
}

// TableColumn represents a column in a table.
type TableColumn struct {
	Header string
	Width  int
	Align  string // "left", "right", "center"
}

// Table represents a formatted table.
type Table struct {
	Columns []TableColumn
	Rows    [][]string
}

// NewTable creates a new table.
func NewTable(columns []TableColumn) *Table {
	return &Table{
		Columns: columns,
		Rows:    make([][]string, 0),
	}
}

// AddRow adds a row to the table.
func (t *Table) AddRow(cells ...string) {
	if len(cells) != len(t.Columns) {
		row := make([]string, len(t.Columns))
		copy(row, cells)
		t.Rows = append(t.Rows, row)
	} else {
		t.Rows = append(t.Rows, cells)
	}
}

// Print renders the table to stdout.
func (t *Table) Print() {
	if len(t.Columns) == 0 {
		return
	}

	termWidth := GetTermWidth()
	totalBorders := len(t.Columns) + 1
	availableWidth := termWidth - totalBorders - (len(t.Columns) * 2)

	totalRequestedWidth := 0
	for _, col := range t.Columns {
		totalRequestedWidth += col.Width
	}

	adjustedColumns := make([]TableColumn, len(t.Columns))
	for i, col := range t.Columns {
		if totalRequestedWidth > availableWidth {
			adjustedColumns[i] = col
			adjustedColumns[i].Width = (col.Width * availableWidth) / totalRequestedWidth
		} else {
			adjustedColumns[i] = col
		}
	}

	// Top border
	fmt.Print(ColorCyan + BoxTopLeft)
	for i, col := range adjustedColumns {
		fmt.Print(strings.Repeat(BoxHorizontal, col.Width+2))
		if i < len(adjustedColumns)-1 {
			fmt.Print(BoxTeeTop)
		}
	}
	fmt.Println(BoxTopRight + ColorReset)

	// Header
	fmt.Print(ColorCyan + BoxVertical + ColorReset)
	for _, col := range adjustedColumns {
		header := TruncateWithEllipsis(col.Header, col.Width)
		fmt.Printf(" %s%s%s ", ColorBold, PadCenter(header, col.Width), ColorReset)
		fmt.Print(ColorCyan + BoxVertical + ColorReset)
	}
	fmt.Println()

	// Header separator
	fmt.Print(ColorCyan + BoxTeeLeft)
	for i, col := range adjustedColumns {
		fmt.Print(strings.Repeat(BoxHorizontal, col.Width+2))
		if i < len(adjustedColumns)-1 {
			fmt.Print(BoxCross)
		}
	}
	fmt.Println(BoxTeeRight + ColorReset)

	// Rows
	for _, row := range t.Rows {
		fmt.Print(ColorCyan + BoxVertical + ColorReset)
		for colIdx, cell := range row {
			if colIdx >= len(adjustedColumns) {
				break
			}
			col := adjustedColumns[colIdx]
			truncated := TruncateWithEllipsis(cell, col.Width)

			var formatted string
			switch col.Align {
			case "right":
				visLen := VisibleLength(truncated)
				if visLen < col.Width {
					formatted = strings.Repeat(" ", col.Width-visLen) + truncated
				} else {
					formatted = truncated
				}
			case "center":
				formatted = PadCenter(truncated, col.Width)
			default:
				formatted = PadRight(truncated, col.Width)
			}

			fmt.Printf(" %s ", formatted)
			fmt.Print(ColorCyan + BoxVertical + ColorReset)
		}
		fmt.Println()
	}

	// Bottom border
	fmt.Print(ColorCyan + BoxBottomLeft)
	for i, col := range adjustedColumns {
		fmt.Print(strings.Repeat(BoxHorizontal, col.Width+2))
		if i < len(adjustedColumns)-1 {
			fmt.Print(BoxTeeBottom)
		}
	}
	fmt.Println(BoxBottomRight + ColorReset)
}

// PrintList prints a styled bullet list.
func PrintList(items []string, color string) {
	for _, item := range items {
		fmt.Printf("  %s%s%s %s\n", color, BulletCircle, ColorReset, item)
	}
}

// PrintKeyValue prints a key-value pair with styling.
func PrintKeyValue(key, value, valueColor string) {
	width := GetTermWidth()
	maxValueWidth := width - len(key) - 10

	if len(value) > maxValueWidth {
		value = TruncateWithEllipsis(value, maxValueWidth)
	}

	fmt.Printf("  %s%-20s%s %s%s%s\n",
		ColorCyan, key+":", ColorReset,
		valueColor, value, ColorReset)
}

// PrintDivider prints a horizontal divider.
func PrintDivider() {
	width := GetTermWidth()
	fmt.Printf("%s%s%s\n", ColorCyan, strings.Repeat(BoxHorizontal, width-1), ColorReset)
}

// PrintBox prints text in a box.
func PrintBox(text string, borderColor string) {
	width := GetTermWidth()
	maxTextWidth := width - 6

	lines := strings.Split(text, "\n")
	boxWidth := 0

	for _, line := range lines {
		if len(line) > boxWidth {
			boxWidth = len(line)
		}
	}

	if boxWidth > maxTextWidth {
		boxWidth = maxTextWidth
	}

	fmt.Printf("%s%s%s%s%s\n",
		borderColor, BoxTopLeft,
		strings.Repeat(BoxHorizontal, boxWidth+2),
		BoxTopRight, ColorReset)

	for _, line := range lines {
		truncated := TruncateWithEllipsis(line, boxWidth)
		fmt.Printf("%s%s%s %s %s%s%s\n",
			borderColor, BoxVertical, ColorReset,
			PadRight(truncated, boxWidth),
			borderColor, BoxVertical, ColorReset)
	}

	fmt.Printf("%s%s%s%s%s\n",
		borderColor, BoxBottomLeft,
		strings.Repeat(BoxHorizontal, boxWidth+2),
		BoxBottomRight, ColorReset)
}

// RenderProgress displays a styled progress bar.
// The onUpdate callback is called with (label, percentage, speed, downloaded, total) for runtime status updates.
func RenderProgress(label string, percentage int, speed, downloaded, total, fillColor string, alignRight bool, onUpdate func(string, int, string, string, string)) {
	if percentage < 0 {
		percentage = 0
	}
	if percentage > 100 {
		percentage = 100
	}

	barWidth := 30
	filled := (percentage * barWidth) / 100
	empty := barWidth - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)

	line := fmt.Sprintf("%s%s%s %s[%s%s%s]%s %s%3d%%%s @ %s/s, %s/%s ",
		ColorBold, label, ColorReset,
		ColorCyan, fillColor, bar, ColorCyan, ColorReset,
		ColorBold, percentage, ColorReset,
		speed, downloaded, total)
	if alignRight {
		width := GetTermWidth()
		padding := width - VisibleLength(line) - 1
		if padding > 0 {
			line = strings.Repeat(" ", padding) + line
		}
	}
	fmt.Printf("\r%s", line)
	if onUpdate != nil {
		onUpdate(label, percentage, speed, downloaded, total)
	}
}
