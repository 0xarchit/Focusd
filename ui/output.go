package ui

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"
)

const (
	Reset = "\033[0m"
	bold  = "\033[1m"
	Dim   = "\033[2m"

	red    = "\033[31m"
	Green  = "\033[32m"
	yellow = "\033[33m"
	blue   = "\033[34m"
	Cyan   = "\033[36m"
	white  = "\033[37m"
	gray   = "\033[90m"
)

const (
	boxHorizontal   = "─"
	boxVertical     = "│"
	boxTopLeft      = "┌"
	boxTopRight     = "┐"
	boxBottomLeft   = "└"
	boxBottomRight  = "┘"
	boxMiddleLeft   = "├"
	boxMiddleRight  = "┤"
	boxCross        = "┼"
	boxTopMiddle    = "┬"
	boxBottomMiddle = "┴"

	arrow     = "❯❯❯"
	checkMark = "✓"
	crossMark = "✗"
	circle    = "○"
)

func PrintHeader() {
	fmt.Println()
	fmt.Printf("   %s%s%s focusd %s\n", yellow, arrow, Reset, Dim+"Local Focus Daemon"+Reset)
	fmt.Printf("   %s%s%s\n", gray, strings.Repeat("─", 50), Reset)
	fmt.Println()
}

func PrintSectionHeader(title string) {
	fmt.Println()
	repeatLen := 40 - len(title)
	if repeatLen < 0 {
		repeatLen = 0
	}
	fmt.Printf("   %s%s%s %s %s%s%s\n",
		yellow, arrow, Reset,
		bold+title+Reset,
		gray, strings.Repeat("─", repeatLen), Reset)
	fmt.Println()
}

func PrintOK(msg string) {
	fmt.Printf("   %s%s%s %s%s%s\n", Green, checkMark, Reset, Green, msg, Reset)
}

func PrintInfo(msg string) {
	fmt.Printf("   %s%s%s %s\n", blue, circle, Reset, msg)
}

func PrintWarn(msg string) {
	fmt.Printf("   %s%s WARNING%s %s\n", yellow, "⚠", Reset, msg)
}

func PrintError(msg string) {
	fmt.Printf("   %s%s%s %s%s%s\n", red, crossMark, Reset, red, msg, Reset)
}

func PrintStatus(label, value string, active bool) {
	c := gray
	if active {
		c = Green
	}
	fmt.Printf("   %s%-15s%s %s%s%s\n", Dim, label, Reset, c, value, Reset)
}

func FormatDuration(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60
	if hours > 0 {
		return fmt.Sprintf("%s%02dh%s %02dm %02ds", Cyan, hours, Reset, minutes, secs)
	}
	return fmt.Sprintf("%02dm %02ds", minutes, secs)
}

func FormatDurationShort(seconds int) string {
	if seconds < 60 {
		return "<1m"
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60
	if hours > 0 {
		return fmt.Sprintf("%02dh %02dm %02ds", hours, minutes, secs)
	}
	return fmt.Sprintf("%02dm %02ds", minutes, secs)
}

type TableColumn struct {
	Header string
	Width  int
}

func PrintTable(columns []TableColumn, rows [][]string) {
	printTableLine(columns, boxTopLeft, boxTopMiddle, boxTopRight)
	printTableHeader(columns)
	printTableLine(columns, boxMiddleLeft, boxCross, boxMiddleRight)

	for i, row := range rows {
		printTableRowStyled(columns, row, i == 0)
	}

	printTableLine(columns, boxBottomLeft, boxBottomMiddle, boxBottomRight)
}

func printTableHeader(columns []TableColumn) {
	fmt.Print(gray + boxVertical + Reset)
	for _, col := range columns {
		fmt.Printf(" %s%s%-*s%s %s%s%s", bold, yellow, col.Width, col.Header, Reset, gray, boxVertical, Reset)
	}
	fmt.Println()
}

func printTableRowStyled(columns []TableColumn, values []string, first bool) {
	fmt.Print(gray + boxVertical + Reset)
	for i, col := range columns {
		val := ""
		if i < len(values) {
			val = values[i]
		}
		val = TruncateString(val, col.Width)
		c := white
		if first && i == 0 {
			c = Green
		}
		if i > 0 {
			c = Cyan
		}
		fmt.Printf(" %s%-*s%s %s%s%s", c, col.Width, val, Reset, gray, boxVertical, Reset)
	}
	fmt.Println()
}

func printTableLine(columns []TableColumn, left, middle, right string) {
	fmt.Print(gray + left)
	for i, col := range columns {
		fmt.Print(strings.Repeat(boxHorizontal, col.Width+2))
		if i < len(columns)-1 {
			fmt.Print(middle)
		}
	}
	fmt.Println(right + Reset)
}

func TruncateString(s string, maxLen int) string {
	return runewidth.Truncate(s, maxLen, "...")
}
