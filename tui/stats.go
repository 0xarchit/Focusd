package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) statsVisibleUsageRows() int {
	inner := m.width - 2
	contentHeight := m.contentHeight()
	if inner >= 120 {
		return max(3, min(10, contentHeight-15))
	}
	if contentHeight < 30 {
		if m.panelFocus == 2 || m.panelFocus == 3 {
			return 0
		}
		return max(4, contentHeight-12)
	}
	return max(3, min(6, (contentHeight-15)/2))
}

func (m Model) statsVisibleBrowserRows() int {
	inner := m.width - 2
	contentHeight := m.contentHeight()
	if inner >= 120 {
		return max(3, min(10, contentHeight-15))
	}
	if contentHeight < 30 {
		if m.panelFocus != 2 {
			return 0
		}
		return max(4, contentHeight-12)
	}
	return max(3, min(6, (contentHeight-15)/2))
}

func (m Model) statsPanelAt(x, y int) int {
	if m.width <= 0 || m.height <= 0 {
		return -1
	}
	inner := m.width - 2
	if inner <= 0 {
		return -1
	}
	contentHeight := m.contentHeight()
	contentX := 1
	contentY := lipgloss.Height(m.renderHeader()) + lipgloss.Height(m.renderTabs()) + 1
	if x < contentX || x >= contentX+inner {
		return -1
	}
	if y < contentY {
		return -1
	}
	relY := y - contentY
	rangePanel := panel("TIME RANGE", "", inner, "\n"+m.renderRangeSelector(inner-4)+"\n", m.panelFocus == 0)
	rangeH := lipgloss.Height(rangePanel)
	if relY < rangeH {
		return 0
	}
	curY := rangeH
	if inner >= 120 {
		tableRows := max(3, min(10, contentHeight-15))
		tbl := panel("APPLICATION BREAKDOWN", "", (inner-2)/2, "\n"+m.renderUsageBreakdown((inner-2)/2, tableRows)+"\n", m.panelFocus == 1)
		tblH := lipgloss.Height(tbl)
		if relY < curY+tblH {
			if x < contentX+inner/2 {
				return 1
			}
			return 2
		}
		return 3
	}
	if m.panelFocus == 0 || m.panelFocus == 1 {
		tbl := panel("APPLICATION BREAKDOWN", "", inner, "\n"+m.renderUsageBreakdown(inner, m.statsVisibleUsageRows())+"\n", m.panelFocus == 1)
		tblH := lipgloss.Height(tbl)
		if relY < curY+tblH {
			return 1
		}
		curY += tblH
	} else if m.panelFocus == 2 {
		tbl := panel("BROWSER USAGE", "", inner, "\n"+m.renderBrowserUsage(inner, m.statsVisibleBrowserRows())+"\n", m.panelFocus == 2)
		tblH := lipgloss.Height(tbl)
		if relY < curY+tblH {
			return 2
		}
		curY += tblH
	}
	return 3
}

func (m Model) handleMouse(msg tea.MouseMsg) Model {
	if m.splash || m.showHelp || m.modal.Active || m.activeTab != tabStats {
		return m
	}
	delta := 0
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		delta = -1
	case tea.MouseButtonWheelDown:
		delta = 1
	default:
		return m
	}
	panel := m.statsPanelAt(msg.X, msg.Y)
	switch panel {
	case 1:
		appsLen := len(m.sortedStatsApps())
		m.statsUsageOffset = clampScrollOffset(m.statsUsageOffset+delta, m.statsVisibleUsageRows(), appsLen)
		m.statsSelected = min(m.statsUsageOffset, max(0, appsLen-1))
		m.panelFocus = 1
	case 2:
		browserLen := len(m.stats.Browsers)
		m.statsBrowserOffset = clampScrollOffset(m.statsBrowserOffset+delta, m.statsVisibleBrowserRows(), browserLen)
		m.browserSelected = min(m.statsBrowserOffset, max(0, browserLen-1))
		m.panelFocus = 2
	case 3:
		m.panelFocus = 3
	}
	return m
}

func (m Model) loadStatsCmd() tea.Cmd {
	if m.statsRange == 3 {
		return loadCustomStats(m.statsCustomFrom, m.statsCustomTo)
	}
	return loadStats(m.statsRange)
}

func (m Model) handleCustomRangeModalKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()
	m.modal.Error = ""
	switch key {
	case "esc":
		m.modal = modal{}
		m.statsCustomEditing = false
	case "tab", "right", "l":
		m.statsCustomField = (m.statsCustomField + 1) % 2
	case "shift+tab", "left", "h":
		m.statsCustomField = (m.statsCustomField + 1) % 2
	case "backspace":
		if m.statsCustomField == 0 {
			m.statsCustomDraftFrom = removeDateChar(m.statsCustomDraftFrom)
		} else {
			m.statsCustomDraftTo = removeDateChar(m.statsCustomDraftTo)
		}
	case "enter":
		from, to, err := validateCustomRange(m.statsCustomDraftFrom, m.statsCustomDraftTo)
		if err != nil {
			m.modal.Error = err.Error()
			return m, nil
		}
		m.statsCustomFrom = from
		m.statsCustomTo = to
		m.statsCustomEditing = false
		m.modal = modal{}
		m.refreshing = true
		return m, m.loadStatsCmd()
	default:
		for _, r := range msg.Runes {
			if !isDateInputRune(r) {
				continue
			}
			if m.statsCustomField == 0 {
				m.statsCustomDraftFrom = appendDateChar(m.statsCustomDraftFrom, r)
			} else {
				m.statsCustomDraftTo = appendDateChar(m.statsCustomDraftTo, r)
			}
		}
	}
	return m, nil
}

func (m Model) handleStatsKey(key string) (Model, tea.Cmd) {
	switch key {
	case "h", "left":
		m.statsRange = (m.statsRange + 3) % 4
		m.refreshing = true
		return m, m.loadStatsCmd()
	case "l", "right":
		m.statsRange = (m.statsRange + 1) % 4
		m.refreshing = true
		return m, m.loadStatsCmd()
	case "enter":
		if m.panelFocus == 0 && m.statsRange == 3 {
			m.statsCustomDraftFrom = m.statsCustomFrom
			m.statsCustomDraftTo = m.statsCustomTo
			m.statsCustomField = 0
			m.statsCustomEditing = true
			m.modal = modal{
				Active:  true,
				Title:   "CUSTOM RANGE",
				Message: "Enter a date range for Stats.",
			}
			return m, nil
		}
	case "s":
		if m.panelFocus == 1 {
			m.statsSort = (m.statsSort + 1) % 3
			if m.statsSort == 0 || m.statsSort == 2 {
				m.statsAsc = false
			} else {
				m.statsAsc = !m.statsAsc
			}
		}
	case "j", "down":
		switch m.panelFocus {
		case 1:
			m.statsUsageOffset = min(m.statsUsageOffset+1, max(0, len(m.sortedStatsApps())-1))
			m.statsSelected = m.statsUsageOffset
		case 2:
			m.statsBrowserOffset = min(m.statsBrowserOffset+1, max(0, len(m.stats.Browsers)-1))
			m.browserSelected = m.statsBrowserOffset
		}
	case "k", "up":
		switch m.panelFocus {
		case 1:
			m.statsUsageOffset = max(0, m.statsUsageOffset-1)
			m.statsSelected = m.statsUsageOffset
		case 2:
			m.statsBrowserOffset = max(0, m.statsBrowserOffset-1)
			m.browserSelected = m.statsBrowserOffset
		}
	case "pgdown":
		switch m.panelFocus {
		case 1:
			m.statsUsageOffset = min(m.statsUsageOffset+5, max(0, len(m.sortedStatsApps())-1))
			m.statsSelected = m.statsUsageOffset
		case 2:
			m.statsBrowserOffset = min(m.statsBrowserOffset+5, max(0, len(m.stats.Browsers)-1))
			m.browserSelected = m.statsBrowserOffset
		}
	case "pgup":
		switch m.panelFocus {
		case 1:
			m.statsUsageOffset = max(0, m.statsUsageOffset-5)
			m.statsSelected = m.statsUsageOffset
		case 2:
			m.statsBrowserOffset = max(0, m.statsBrowserOffset-5)
			m.browserSelected = m.statsBrowserOffset
		}
	}
	return m, nil
}

func validDateInput(value string) bool {
	_, err := time.Parse("2006-01-02", value)
	return err == nil
}

func isDateInputRune(r rune) bool {
	return r == '-' || (r >= '0' && r <= '9')
}

func appendDateChar(current string, r rune) string {
	digits := dateDigits(current)
	if r >= '0' && r <= '9' && len(digits) < 8 {
		digits += string(r)
	}
	return formatDateDigits(digits)
}

func removeDateChar(current string) string {
	digits := dateDigits(current)
	if len(digits) > 0 {
		digits = digits[:len(digits)-1]
	}
	return formatDateDigits(digits)
}

func dateDigits(value string) string {
	var out []rune
	for _, r := range value {
		if r >= '0' && r <= '9' {
			out = append(out, r)
		}
	}
	return string(out)
}

func formatDateDigits(digits string) string {
	n := len(digits)
	if n == 0 {
		return ""
	}
	if n <= 4 {
		return digits
	}
	if n <= 6 {
		return digits[:4] + "-" + digits[4:]
	}
	return digits[:4] + "-" + digits[4:6] + "-" + digits[6:]
}

func validateCustomRange(from, to string) (string, string, error) {
	if !validDateInput(from) || !validDateInput(to) {
		return "", "", fmt.Errorf("Invalid date format. Use YYYY-MM-DD")
	}
	fromTime, _ := time.Parse("2006-01-02", from)
	toTime, _ := time.Parse("2006-01-02", to)
	if fromTime.After(toTime) {
		return "", "", fmt.Errorf("From date must be before To date")
	}
	if toTime.After(time.Now()) {
		return "", "", fmt.Errorf("To date cannot be in the future")
	}
	if int(toTime.Sub(fromTime).Hours()/24)+1 > 366 {
		return "", "", fmt.Errorf("Range must be 366 days or less")
	}
	return from, to, nil
}

func (m Model) renderStats(width, height int) string {
	inner := width - 2
	rangeSelector := m.renderRangeSelector(inner - 4)
	topPanel := panel("TIME RANGE", "", inner, "\n"+rangeSelector+"\n", m.panelFocus == 0)

	contentHeight := height - lipgloss.Height(topPanel) - 2
	if contentHeight < 6 {
		contentHeight = 6
	}

	if inner >= 120 {
		leftW := (inner - 2) / 2
		rightW := inner - 2 - leftW
		tableRows := max(3, min(10, contentHeight-15))
		appsPanel := panel("APPLICATION BREAKDOWN", "", leftW, "\n"+m.renderUsageBreakdown(leftW, tableRows)+"\n", m.panelFocus == 1)
		browsersPanel := panel("BROWSER USAGE", "", rightW, "\n"+m.renderBrowserUsage(rightW, tableRows)+"\n", m.panelFocus == 2)
		middleRow := lipgloss.JoinHorizontal(lipgloss.Top, appsPanel, " ", browsersPanel)
		historyPanel := m.renderDailyHistory(inner)

		return lipgloss.JoinVertical(lipgloss.Left, topPanel, middleRow, historyPanel)
	}

	var visible []string
	visible = append(visible, topPanel)

	if m.panelFocus == 0 || m.panelFocus == 1 {
		tblW := inner
		visibleRows := m.statsVisibleUsageRows()
		if visibleRows > 0 {
			appsPanel := panel("APPLICATION BREAKDOWN", "", tblW, "\n"+m.renderUsageBreakdown(tblW, visibleRows)+"\n", m.panelFocus == 1)
			visible = append(visible, appsPanel)
		}
	} else if m.panelFocus == 2 {
		tblW := inner
		visibleRows := m.statsVisibleBrowserRows()
		if visibleRows > 0 {
			browsersPanel := panel("BROWSER USAGE", "", tblW, "\n"+m.renderBrowserUsage(tblW, visibleRows)+"\n", m.panelFocus == 2)
			visible = append(visible, browsersPanel)
		}
	}

	historyPanel := m.renderDailyHistory(inner)
	visible = append(visible, historyPanel)

	return lipgloss.JoinVertical(lipgloss.Left, visible...)
}

func (m Model) renderRangeSelector(width int) string {
	ranges := []string{"TODAY", "LAST 7 DAYS", "LAST 30 DAYS", "CUSTOM RANGE"}
	var parts []string
	for i, r := range ranges {
		if i == m.statsRange {
			parts = append(parts, cyanStyle.Bold(true).Render("[ "+r+" ]"))
		} else {
			parts = append(parts, mutedStyle.Render("[ "+r+" ]"))
		}
	}
	selector := strings.Join(parts, "   ")
	if m.statsRange == 3 {
		customInfo := fmt.Sprintf("   Range: %s to %s", m.statsCustomFrom, m.statsCustomTo)
		selector += mutedStyle.Render(customInfo)
	}
	return center(selector, width)
}

func (m Model) renderUsageBreakdown(width, visibleRows int) string {
	var lines []string
	sorted := m.sortedStatsApps()
	maxDuration := 1
	for _, a := range sorted {
		if a.Duration > maxDuration {
			maxDuration = a.Duration
		}
	}

	headerPlain := padRight("Application", max(8, width-32)) + padLeft("Time", 8) + padLeft("Opens", 8)
	header := boldStyle.Render(fitLine(headerPlain, width-4))
	lines = append(lines, header, mutedStyle.Render(fill(max(1, width-4), "─")))

	if len(sorted) > 0 {
		start := m.statsUsageOffset
		end := min(len(sorted), start+visibleRows)
		for i := start; i < end; i++ {
			a := sorted[i]
			color := cyanStyle
			if i == m.statsSelected && m.panelFocus == 1 {
				color = lipgloss.NewStyle().Foreground(cWhite)
			}
			line := mutedStyle.Render(fmt.Sprintf("%2d. ", i+1)) +
				color.Render(padRight(truncate(a.Name, max(8, width-36)), max(8, width-36))) +
				padLeft(formatDuration(a.Duration), 8) +
				padLeft(strconv.Itoa(a.Opens), 8) + " " +
				color.Render(bar(a.Duration, maxDuration, 3, "█"))
			if i == m.statsSelected && m.panelFocus == 1 {
				line = selectedRowStyle.Render(padRight(line, width-4))
			}
			lines = append(lines, line)
		}
	} else {
		lines = append(lines, mutedStyle.Render("No usage recorded for this range."))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderBrowserUsage(width, visibleRows int) string {
	var lines []string
	lines = append(lines, boldStyle.Render(fitLine(padRight("Tab Title", max(8, width-16))+padLeft("Time", 8), width-4)), mutedStyle.Render(fill(max(1, width-4), "─")))

	if len(m.stats.Browsers) > 0 {
		start := m.statsBrowserOffset
		end := min(len(m.stats.Browsers), start+visibleRows)
		for i := start; i < end; i++ {
			b := m.stats.Browsers[i]
			prefix := mutedStyle.Render("▶ ")
			line := prefix + boldStyle.Render(padRight(truncate(b.Name, max(8, width-18)), max(8, width-18))) + padLeft(formatDuration(b.Duration), 8)
			if i == m.browserSelected && m.panelFocus == 2 {
				line = selectedRowStyle.Render(padRight(line, width-4))
			}
			lines = append(lines, line)
		}
	} else {
		lines = append(lines, mutedStyle.Render("No browser activity recorded."))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderDailyHistory(width int) string {
	maxDuration := 1
	for _, d := range m.stats.Days {
		if d.Duration > maxDuration {
			maxDuration = d.Duration
		}
	}
	var lines []string
	var labels []string
	var bars []string
	var hours []string

	maxH := 4
	for h := maxH; h >= 1; h-- {
		var row []string
		for _, d := range m.stats.Days {
			val := d.Duration
			pct := val * 100 / maxDuration
			threshold := h * 100 / maxH
			ch := "░"
			if pct >= threshold {
				ch = "█"
				if d.Weekend {
					ch = violetStyle.Render(ch)
				} else {
					ch = cyanStyle.Render(ch)
				}
			} else {
				ch = mutedStyle.Render("·")
			}
			row = append(row, ch)
		}
		bars = append(bars, "  "+strings.Join(row, "   ")+"  ")
	}

	for _, d := range m.stats.Days {
		style := lipgloss.NewStyle().Foreground(cWhite)
		if d.Today {
			style = cyanStyle
		} else if d.Weekend {
			style = violetStyle
		}
		labels = append(labels, style.Render(d.Label))
		hours = append(hours, mutedStyle.Render(fmt.Sprintf("%2.1fh", float64(d.Duration)/3600.0)))
	}

	lines = append(lines, bars...)
	lines = append(lines, "  "+strings.Join(labels, "   ")+"  ")
	lines = append(lines, "  "+strings.Join(hours, "  ")+" ")

	title := "DAILY HISTORY"
	if m.statsRange == 1 {
		title = "DAILY HISTORY (LAST 7 DAYS)"
	} else if m.statsRange == 2 {
		title = "DAILY HISTORY (LAST 30 DAYS)"
	} else if m.statsRange == 3 {
		title = "DAILY HISTORY (CUSTOM RANGE)"
	}
	return panel(title, "", width, "\n"+strings.Join(lines, "\n")+"\n", m.panelFocus == 3)
}

func (m Model) renderCustomRangeModal() string {
	from := padRight(m.statsCustomDraftFrom, 10)
	to := padRight(m.statsCustomDraftTo, 10)
	if m.statsCustomField == 0 {
		from = cyanStyle.Bold(true).Render(from)
	} else {
		to = cyanStyle.Bold(true).Render(to)
	}
	errorLine := mutedStyle.Render("Digits are auto-formatted as YYYY-MM-DD")
	if m.modal.Error != "" {
		errorLine = redStyle.Render(m.modal.Error)
	}
	msg := []string{
		"",
		mutedStyle.Render(m.modal.Message),
		"",
		"From: [ " + from + " ]",
		"To:   [ " + to + " ]",
		"",
		errorLine,
		"",
		center(cyanStyle.Render("[ ENTER APPLY ]")+"   "+buttonStyle.Render("[ ESC CANCEL ]"), 42),
		"",
		mutedStyle.Render("Tab / ← → switch fields   Backspace deletes"),
	}
	return panel("CUSTOM RANGE", "", 50, strings.Join(msg, "\n"), true)
}

func (m Model) sortedStatsApps() []appUsage {
	apps := append([]appUsage(nil), m.stats.Apps...)
	sort.Slice(apps, func(i, j int) bool {
		a, b := i, j
		if !m.statsAsc {
			a, b = j, i
		}
		switch m.statsSort {
		case 0:
			return apps[a].Duration < apps[b].Duration
		case 1:
			return apps[a].Name < apps[b].Name
		case 2:
			return apps[a].Opens < apps[b].Opens
		}
		return false
	})
	return apps
}
