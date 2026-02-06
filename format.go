package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"
)

// Box drawing characters for beautiful tables
const (
	boxTopLeft     = "┌"
	boxTopRight    = "┐"
	boxBottomLeft  = "└"
	boxBottomRight = "┘"
	boxVertical    = "│"
	boxHorizontal  = "─"
	boxTeeLeft     = "├"
	boxTeeRight    = "┤"
	boxTeeTop      = "┬"
	boxTeeBottom   = "┴"
	boxCross       = "┼"

	// Double line variants for emphasis (used in headers and emphasis areas)
	boxDoubleHorizontal  = "═"
	boxDoubleTopLeft     = "╔"
	boxDoubleTopRight    = "╗"
	boxDoubleBottomLeft  = "╚"
	boxDoubleBottomRight = "╝"

	// Bullets and decorations
	bulletSquare  = "▪"
	bulletCircle  = "•"
	bulletArrow   = "▸"
	bulletDiamond = "◆"
)

// ansiRegex is compiled once at package init for performance
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// getTermWidth returns the terminal width, defaulting to 80 if not detectable
func getTermWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width == 0 {
		return 80 // Default fallback
	}
	return width
}

// stripAnsiCodes removes ANSI escape sequences from a string
func stripAnsiCodes(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// visibleLength returns the visible length of a string (excluding ANSI codes)
// Uses UTF-8 rune counting to handle multi-byte characters correctly
func visibleLength(s string) int {
	return utf8.RuneCountInString(stripAnsiCodes(s))
}

// truncateWithEllipsis truncates a string to maxLen with ellipsis if needed
// Handles ANSI color codes and multi-byte UTF-8 characters properly
func truncateWithEllipsis(s string, maxLen int) string {
	visibleLen := visibleLength(s)
	if visibleLen <= maxLen {
		return s
	}
	if maxLen <= 3 {
		// For very short limits, just strip and truncate using runes
		stripped := stripAnsiCodes(s)
		runes := []rune(stripped)
		if len(runes) <= maxLen {
			return stripped
		}
		return string(runes[:maxLen])
	}

	// Extract ANSI codes and visible text
	codes := ansiRegex.FindAllString(s, -1)
	stripped := stripAnsiCodes(s)

	// Truncate the visible text using runes for proper UTF-8 handling
	runes := []rune(stripped)
	truncated := string(runes[:maxLen-3]) + "..."

	// If there were color codes, try to preserve the first one
	if len(codes) > 0 {
		// Apply first color code and reset at the end
		return codes[0] + truncated + colorReset
	}

	return truncated
}

// padRight pads a string to the specified width using visible length (ANSI-aware)
func padRight(s string, width int) string {
	visLen := visibleLength(s)
	if visLen >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visLen)
}

// padCenter centers a string in the specified width using visible length (ANSI-aware)
func padCenter(s string, width int) string {
	visLen := visibleLength(s)
	if visLen >= width {
		return s
	}
	padding := width - visLen
	leftPad := padding / 2
	rightPad := padding - leftPad
	return strings.Repeat(" ", leftPad) + s + strings.Repeat(" ", rightPad)
}

// printHeader prints a styled header with box drawing
func printHeader(title string) {
	width := getTermWidth()
	titleLen := len(title) + 4 // 2 spaces on each side

	// Ensure we don't exceed terminal width
	if titleLen > width-4 {
		title = truncateWithEllipsis(title, width-10)
		titleLen = len(title) + 4
	}

	lineLen := width - 2

	fmt.Printf("\n%s%s%s%s%s\n",
		colorCyan, boxDoubleTopLeft,
		strings.Repeat(boxDoubleHorizontal, lineLen),
		boxDoubleTopRight, colorReset)

	fmt.Printf("%s%s%s %s %s%s%s\n",
		colorCyan, boxVertical, colorReset,
		colorBold+padCenter(title, lineLen-2)+colorReset,
		colorCyan, boxVertical, colorReset)

	fmt.Printf("%s%s%s%s%s\n\n",
		colorCyan, boxDoubleBottomLeft,
		strings.Repeat(boxDoubleHorizontal, lineLen),
		boxDoubleBottomRight, colorReset)
}

// printSection prints a section title with underline
func printSection(title string) {
	fmt.Printf("\n%s%s %s%s\n", colorBold, bulletDiamond, title, colorReset)
	fmt.Printf("%s%s%s\n\n", colorCyan, strings.Repeat(boxHorizontal, len(title)+2), colorReset)
}

// TableColumn represents a column in a table
type TableColumn struct {
	Header string
	Width  int
	Align  string // "left", "right", "center"
}

// Table represents a formatted table
type Table struct {
	Columns []TableColumn
	Rows    [][]string
}

// NewTable creates a new table
func NewTable(columns []TableColumn) *Table {
	return &Table{
		Columns: columns,
		Rows:    make([][]string, 0),
	}
}

// AddRow adds a row to the table
func (t *Table) AddRow(cells ...string) {
	if len(cells) != len(t.Columns) {
		// Pad or truncate to match column count
		row := make([]string, len(t.Columns))
		copy(row, cells)
		t.Rows = append(t.Rows, row)
	} else {
		t.Rows = append(t.Rows, cells)
	}
}

// Print renders the table to stdout
func (t *Table) Print() {
	if len(t.Columns) == 0 {
		return
	}

	// Auto-adjust column widths to fit terminal
	termWidth := getTermWidth()
	totalBorders := len(t.Columns) + 1                                // +1 for edges
	availableWidth := termWidth - totalBorders - (len(t.Columns) * 2) // -2 for padding per column

	// Calculate proportional widths
	totalRequestedWidth := 0
	for _, col := range t.Columns {
		totalRequestedWidth += col.Width
	}

	adjustedColumns := make([]TableColumn, len(t.Columns))
	for i, col := range t.Columns {
		if totalRequestedWidth > availableWidth {
			// Scale down proportionally
			adjustedColumns[i] = col
			adjustedColumns[i].Width = (col.Width * availableWidth) / totalRequestedWidth
		} else {
			adjustedColumns[i] = col
		}
	}

	// Print top border
	fmt.Print(colorCyan + boxTopLeft)
	for i, col := range adjustedColumns {
		fmt.Print(strings.Repeat(boxHorizontal, col.Width+2))
		if i < len(adjustedColumns)-1 {
			fmt.Print(boxTeeTop)
		}
	}
	fmt.Println(boxTopRight + colorReset)

	// Print header
	fmt.Print(colorCyan + boxVertical + colorReset)
	for _, col := range adjustedColumns {
		header := truncateWithEllipsis(col.Header, col.Width)
		fmt.Printf(" %s%s%s ", colorBold, padCenter(header, col.Width), colorReset)
		fmt.Print(colorCyan + boxVertical + colorReset)
	}
	fmt.Println()

	// Print header separator
	fmt.Print(colorCyan + boxTeeLeft)
	for i, col := range adjustedColumns {
		fmt.Print(strings.Repeat(boxHorizontal, col.Width+2))
		if i < len(adjustedColumns)-1 {
			fmt.Print(boxCross)
		}
	}
	fmt.Println(boxTeeRight + colorReset)

	// Print rows
	for _, row := range t.Rows {
		fmt.Print(colorCyan + boxVertical + colorReset)
		for colIdx, cell := range row {
			if colIdx >= len(adjustedColumns) {
				break
			}
			col := adjustedColumns[colIdx]
			truncated := truncateWithEllipsis(cell, col.Width)

			var formatted string
			switch col.Align {
			case "right":
				// Use visible length for ANSI-aware right-alignment
				visLen := visibleLength(truncated)
				if visLen < col.Width {
					formatted = strings.Repeat(" ", col.Width-visLen) + truncated
				} else {
					formatted = truncated
				}
			case "center":
				formatted = padCenter(truncated, col.Width)
			default: // "left"
				formatted = padRight(truncated, col.Width)
			}

			fmt.Printf(" %s ", formatted)
			fmt.Print(colorCyan + boxVertical + colorReset)
		}
		fmt.Println()
	}

	// Print bottom border
	fmt.Print(colorCyan + boxBottomLeft)
	for i, col := range adjustedColumns {
		fmt.Print(strings.Repeat(boxHorizontal, col.Width+2))
		if i < len(adjustedColumns)-1 {
			fmt.Print(boxTeeBottom)
		}
	}
	fmt.Println(boxBottomRight + colorReset)
}

// printList prints a styled bullet list
func printList(items []string, color string) {
	for _, item := range items {
		fmt.Printf("  %s%s%s %s\n", color, bulletCircle, colorReset, item)
	}
}

// printKeyValue prints a key-value pair with styling
func printKeyValue(key, value, valueColor string) {
	width := getTermWidth()
	maxValueWidth := width - len(key) - 10 // Leave room for key and spacing

	if len(value) > maxValueWidth {
		value = truncateWithEllipsis(value, maxValueWidth)
	}

	fmt.Printf("  %s%-20s%s %s%s%s\n",
		colorCyan, key+":", colorReset,
		valueColor, value, colorReset)
}

// printDivider prints a horizontal divider
func printDivider() {
	width := getTermWidth()
	fmt.Printf("%s%s%s\n", colorCyan, strings.Repeat(boxHorizontal, width-1), colorReset)
}

// printBox prints text in a box
func printBox(text string, borderColor string) {
	width := getTermWidth()
	maxTextWidth := width - 6 // Account for borders and padding

	lines := strings.Split(text, "\n")
	boxWidth := 0

	// Find longest line
	for _, line := range lines {
		if len(line) > boxWidth {
			boxWidth = len(line)
		}
	}

	// Cap at maxTextWidth
	if boxWidth > maxTextWidth {
		boxWidth = maxTextWidth
	}

	// Top border
	fmt.Printf("%s%s%s%s%s\n",
		borderColor, boxTopLeft,
		strings.Repeat(boxHorizontal, boxWidth+2),
		boxTopRight, colorReset)

	// Content
	for _, line := range lines {
		truncated := truncateWithEllipsis(line, boxWidth)
		fmt.Printf("%s%s%s %s %s%s%s\n",
			borderColor, boxVertical, colorReset,
			padRight(truncated, boxWidth),
			borderColor, boxVertical, colorReset)
	}

	// Bottom border
	fmt.Printf("%s%s%s%s%s\n",
		borderColor, boxBottomLeft,
		strings.Repeat(boxHorizontal, boxWidth+2),
		boxBottomRight, colorReset)
}
