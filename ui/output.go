package ui

import (
	"fmt"
	"strings"
)

const (
	Reset = "\033[0m"
	Bold  = "\033[1m"
	Dim   = "\033[2m"

	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"
	Gray    = "\033[90m"

	BrightRed     = "\033[91m"
	BrightGreen   = "\033[92m"
	BrightYellow  = "\033[93m"
	BrightBlue    = "\033[94m"
	BrightMagenta = "\033[95m"
	BrightCyan    = "\033[96m"
)

const (
	BoxHorizontal   = "─"
	BoxVertical     = "│"
	BoxTopLeft      = "┌"
	BoxTopRight     = "┐"
	BoxBottomLeft   = "└"
	BoxBottomRight  = "┘"
	BoxMiddleLeft   = "├"
	BoxMiddleRight  = "┤"
	BoxCross        = "┼"
	BoxTopMiddle    = "┬"
	BoxBottomMiddle = "┴"

	Arrow        = "❯❯❯"
	Bullet       = "•"
	CheckMark    = "✓"
	CrossMark    = "✗"
	Star         = "★"
	Circle       = "○"
	FilledCircle = "●"
)

func PrintLogo() {
	fmt.Println()
	fmt.Printf("%s%s   ┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓%s\n", Bold, Cyan, Reset)
	fmt.Printf("%s%s   ┃    █▀▀ █▀█ █▀▀ █ █ █▀ █▀▄                       ┃%s\n", Bold, Cyan, Reset)
	fmt.Printf("%s%s   ┃    █▀  █▄█ █▄▄ █▄█ ▄█ █▄▀   %s%sFocus Daemon%s%s        ┃%s\n", Bold, Cyan, Yellow, Bold, Reset, Cyan, Reset)
	fmt.Printf("%s%s   ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛%s\n", Bold, Cyan, Reset)
	fmt.Println()
}

func PrintHeader() {
	fmt.Println()
	fmt.Printf("   %s%s%s focusd %s\n", Yellow, Arrow, Reset, Dim+"Local Focus Daemon"+Reset)
	fmt.Printf("   %s%s%s\n", Gray, strings.Repeat("─", 50), Reset)
	fmt.Println()
}

func PrintMenuHeader() {
	PrintLogo()
	fmt.Printf("   %sPrivate%s %s•%s %sOffline%s %s•%s %sLocal-first%s\n",
		Green, Reset, Gray, Reset,
		Blue, Reset, Gray, Reset,
		Magenta, Reset)
	fmt.Println()
}

func PrintSectionHeader(title string) {
	fmt.Println()
	fmt.Printf("   %s%s%s %s %s%s%s\n",
		Yellow, Arrow, Reset,
		Bold+title+Reset,
		Gray, strings.Repeat("─", 40-len(title)), Reset)
	fmt.Println()
}

func PrintSubHeader(title string) {
	fmt.Printf("   %s%s%s\n", Dim, title, Reset)
}

func PrintOK(msg string) {
	fmt.Printf("   %s%s%s %s%s%s\n", Green, CheckMark, Reset, Green, msg, Reset)
}

func PrintInfo(msg string) {
	fmt.Printf("   %s%s%s %s\n", Blue, Circle, Reset, msg)
}

func PrintWarn(msg string) {
	fmt.Printf("   %s%s WARNING%s %s\n", Yellow, "⚠", Reset, msg)
}

func PrintError(msg string) {
	fmt.Printf("   %s%s%s %s%s%s\n", Red, CrossMark, Reset, Red, msg, Reset)
}

func PrintStatus(label, value string, active bool) {
	color := Gray
	if active {
		color = Green
	}
	fmt.Printf("   %s%-15s%s %s%s%s\n", Dim, label, Reset, color, value, Reset)
}

func PrintKeyValue(key, value string) {
	fmt.Printf("   %s%-20s%s %s%s%s\n", Dim, key, Reset, White, value, Reset)
}

func PrintStatRow(name string, time string, highlight bool) {
	color := White
	if highlight {
		color = Green
	}
	fmt.Printf("   %s%-30s%s %s%s%s\n", color, name, Reset, Cyan, time, Reset)
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

func FormatDurationStyled(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60
	return fmt.Sprintf("%s%02dh%s %02dm %02ds", Cyan, hours, Reset, minutes, secs)
}

type TableColumn struct {
	Header string
	Width  int
}

func PrintTable(columns []TableColumn, rows [][]string) {
	printTableLine(columns, BoxTopLeft, BoxTopMiddle, BoxTopRight)
	printTableHeader(columns)
	printTableLine(columns, BoxMiddleLeft, BoxCross, BoxMiddleRight)

	for i, row := range rows {
		printTableRowStyled(columns, row, i == 0)
	}

	printTableLine(columns, BoxBottomLeft, BoxBottomMiddle, BoxBottomRight)
}

func printTableHeader(columns []TableColumn) {
	fmt.Print(Gray + BoxVertical + Reset)
	for _, col := range columns {
		fmt.Printf(" %s%s%-*s%s %s%s%s", Bold, Yellow, col.Width, col.Header, Reset, Gray, BoxVertical, Reset)
	}
	fmt.Println()
}

func printTableRowStyled(columns []TableColumn, values []string, first bool) {
	fmt.Print(Gray + BoxVertical + Reset)
	for i, col := range columns {
		val := ""
		if i < len(values) {
			val = values[i]
		}
		val = TruncateString(val, col.Width)
		color := White
		if first && i == 0 {
			color = Green
		}
		if i > 0 {
			color = Cyan
		}
		fmt.Printf(" %s%-*s%s %s%s%s", color, col.Width, val, Reset, Gray, BoxVertical, Reset)
	}
	fmt.Println()
}

func printTableLine(columns []TableColumn, left, middle, right string) {
	fmt.Print(Gray + left)
	for i, col := range columns {
		fmt.Print(strings.Repeat(BoxHorizontal, col.Width+2))
		if i < len(columns)-1 {
			fmt.Print(middle)
		}
	}
	fmt.Println(right + Reset)
}

func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func Confirm(prompt string) bool {
	fmt.Printf("   %s%s%s [y/N]: ", Yellow, prompt, Reset)
	var response string
	fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

func ConfirmDestructive(action string) bool {
	fmt.Println()
	PrintWarn("This action is irreversible!")
	fmt.Println()
	return Confirm(fmt.Sprintf("Are you sure you want to %s?", action))
}

func PrintMenuItem(num string, label string, active bool) {
	indicator := " "
	color := White
	if active {
		indicator = FilledCircle
		color = Green
	}
	fmt.Printf("   %s%s%s  %s%s.%s %s%s%s\n", Green, indicator, Reset, Dim, num, Reset, color, label, Reset)
}

func PrintMenuDivider() {
	fmt.Printf("   %s%s%s\n", Gray, strings.Repeat("─", 45), Reset)
}

func ClearLine() {
	fmt.Print("\033[2K\r")
}

func ClearScreen() {
	fmt.Print("\033[2J\033[3J\033[H")
}
