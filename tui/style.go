package tui

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

var (
	cBg      = lipgloss.Color("#0D0D0D")
	cCyan    = lipgloss.Color("#00D7FF")
	cViolet  = lipgloss.Color("#7C3AED")
	cGreen   = lipgloss.Color("#22C55E")
	cAmber   = lipgloss.Color("#F59E0B")
	cRed     = lipgloss.Color("#EF4444")
	cMuted   = lipgloss.Color("#6B7280")
	cWhite   = lipgloss.Color("#F8FAFC")
	cPanel   = lipgloss.Color("#111827")
	cRow     = lipgloss.Color("#1E293B")
	cDanger  = lipgloss.Color("#2D1B1B")
	cOverlay = lipgloss.Color("#3F3F3F")

	appStyle = lipgloss.NewStyle().
			Background(cBg).
			Foreground(cWhite)

	logoStyle = lipgloss.NewStyle().
			Foreground(cCyan).
			Bold(true)

	mutedStyle = lipgloss.NewStyle().
			Foreground(cMuted)

	boldStyle = lipgloss.NewStyle().
			Foreground(cWhite).
			Bold(true)

	cyanStyle = lipgloss.NewStyle().
			Foreground(cCyan)

	greenStyle = lipgloss.NewStyle().
			Foreground(cGreen)

	amberStyle = lipgloss.NewStyle().
			Foreground(cAmber)

	redStyle = lipgloss.NewStyle().
			Foreground(cRed)

	violetStyle = lipgloss.NewStyle().
			Foreground(cViolet)

	selectedRowStyle = lipgloss.NewStyle().
				Background(cRow).
				Foreground(cWhite)

	dangerRowStyle = lipgloss.NewStyle().
			Background(cDanger).
			Foreground(cRed)

	buttonStyle = lipgloss.NewStyle().
			Foreground(cMuted)

	activeButtonStyle = lipgloss.NewStyle().
				Foreground(cCyan).
				Bold(true)

	hoverRowStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1B2230")).
			Foreground(cWhite)
)

func borderColor(focused bool) lipgloss.Color {
	return borderColorWithHover(focused, false)
}

func borderColorWithHover(focused bool, hovered bool) lipgloss.Color {
	if focused {
		return cCyan
	}
	if hovered {
		// Hover highlight: desaturated/reduced intensity cyan or grey highlight
		return lipgloss.Color("#008FAD")
	}
	return cMuted
}

func panel(title string, right string, width int, body string, focused bool) string {
	return panelWithHover(title, right, width, body, focused, false)
}

func panelWithHover(title string, right string, width int, body string, focused bool, hovered bool) string {
	if width < 8 {
		width = 8
	}
	inner := width - 2
	right = strings.TrimSpace(right)
	if lipgloss.Width(right) > inner/3 {
		right = truncate(right, inner/3)
	}
	rightWidth := lipgloss.Width(right)
	titleLimit := max(1, inner-rightWidth-4)
	titleText := " " + truncate(title, max(1, titleLimit-2)) + " "
	topFill := inner - 1 - lipgloss.Width(titleText) - rightWidth
	if topFill < 1 {
		topFill = 1
	}
	edge := lipgloss.NewStyle().Foreground(borderColorWithHover(focused, hovered))
	top := edge.Render("╭─") + titleStyle().Render(titleText) + edge.Render(strings.Repeat("─", topFill))
	if right != "" {
		top += mutedStyle.Render(right)
	}
	top += edge.Render("╮")
	lines := normalizeLines(body, inner)
	for i := range lines {
		lines[i] = edge.Render("│") + lines[i] + edge.Render("│")
	}
	bottom := edge.Render("╰" + strings.Repeat("─", inner) + "╯")
	box := lipgloss.JoinVertical(lipgloss.Left, append(append([]string{top}, lines...), bottom)...)
	return lipgloss.NewStyle().BorderForeground(borderColorWithHover(focused, hovered)).Render(box)
}

func titleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(cWhite).Bold(true)
}

func normalizeLines(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	raw := strings.Split(s, "\n")
	if len(raw) == 0 {
		raw = []string{""}
	}
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		if lipgloss.Width(line) > width {
			line = truncate(line, width)
		}
		out = append(out, line+strings.Repeat(" ", max(0, width-lipgloss.Width(line))))
	}
	return out
}

func fitLine(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) > width {
		return truncate(s, width)
	}
	return s + strings.Repeat(" ", width-lipgloss.Width(s))
}

func clipLines(s string, maxLines, width int) string {
	if maxLines <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	for i := range lines {
		if lipgloss.Width(lines[i]) > width {
			lines[i] = truncate(lines[i], width)
		}
	}
	return strings.Join(lines, "\n")
}

func scrollStart(selected, visible, total int) int {
	if visible <= 0 || total <= visible {
		return 0
	}
	if selected < 0 {
		selected = 0
	}
	if selected >= total {
		selected = total - 1
	}
	start := selected - visible + 1
	if start < 0 {
		start = 0
	}
	if start+visible > total {
		start = total - visible
	}
	return start
}

func clampScrollOffset(offset, visible, total int) int {
	if total <= 0 || visible <= 0 {
		return 0
	}
	maxOffset := max(0, total-visible)
	if offset < 0 {
		return 0
	}
	if offset > maxOffset {
		return maxOffset
	}
	return offset
}

func padRight(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) > width {
		return truncate(s, width)
	}
	return s + strings.Repeat(" ", width-lipgloss.Width(s))
}

func padLeft(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) > width {
		return truncate(s, width)
	}
	return strings.Repeat(" ", width-lipgloss.Width(s)) + s
}

func center(s string, width int) string {
	if lipgloss.Width(s) >= width {
		return truncate(s, width)
	}
	left := (width - lipgloss.Width(s)) / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", width-lipgloss.Width(s)-left)
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}

	var result strings.Builder
	currentWidth := 0
	inEscape := false
	targetWidth := width - 1

	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			inEscape = true
			result.WriteByte(s[i])
			i++
			continue
		}
		if inEscape {
			result.WriteByte(s[i])
			if s[i] >= 0x40 && s[i] <= 0x7E {
				inEscape = false
			}
			i++
			continue
		}

		r, size := utf8.DecodeRuneInString(s[i:])
		w := runewidth.RuneWidth(r)
		if currentWidth+w > targetWidth {
			break
		}
		result.WriteRune(r)
		currentWidth += w
		i += size
	}

	if strings.Contains(s, "\x1b") {
		result.WriteString("\x1b[0m")
	}
	result.WriteString("…")
	return result.String()
}

func fill(width int, ch string) string {
	if width <= 0 {
		return ""
	}
	return strings.Repeat(ch, width)
}

func bar(value, maxValue, width int, fillChar string) string {
	if width <= 0 {
		return ""
	}
	if maxValue <= 0 {
		return strings.Repeat("░", width)
	}
	filled := value * width / maxValue
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	if filled == 0 && value > 0 {
		filled = 1
	}
	return strings.Repeat(fillChar, filled) + strings.Repeat("░", width-filled)
}
