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

func (m *Model) statsVisibleRows(isBrowser bool) int {
	inner := m.width - 2
	contentHeight := m.contentHeight()
	if inner >= 120 {
		return max(3, min(10, contentHeight-15))
	}
	if contentHeight < 30 {
		hide := m.panelFocus != 2
		if !isBrowser {
			hide = m.panelFocus == 2 || m.panelFocus == 3
		}
		if hide {
			return 0
		}
		return max(4, contentHeight-12)
	}
	return max(3, min(6, (contentHeight-15)/2))
}

func (m Model) loadStatsCmd() tea.Cmd {
	return loadStats(m.statsRange, m.statsCustomFrom, m.statsCustomTo)
}

func (m Model) handleCustomRangeModalKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()
	m.modal.Error = ""
	switch key {
	case "esc":
		m.modal = modal{}
	case "tab", "shift+tab", "left", "right", "h", "l":
		m.statsCustomField = (m.statsCustomField + 1) % 2
	case "backspace":
		draft := &m.statsCustomDraftFrom
		if m.statsCustomField == 1 {
			draft = &m.statsCustomDraftTo
		}
		if n := len(*draft); n > 0 {
			if n == 6 || n == 9 {
				*draft = (*draft)[:n-2]
			} else {
				*draft = (*draft)[:n-1]
			}
		}
	case "enter":
		from, to, err := validateCustomRange(m.statsCustomDraftFrom, m.statsCustomDraftTo)
		if err != nil {
			m.modal.Error = err.Error()
			return m, nil
		}
		m.statsCustomFrom = from
		m.statsCustomTo = to
		m.modal = modal{}
		m.refreshing = true
		return m, m.loadStatsCmd()
	default:
		for _, r := range msg.Runes {
			if r < '0' || r > '9' {
				continue
			}
			draft := &m.statsCustomDraftFrom
			if m.statsCustomField == 1 {
				draft = &m.statsCustomDraftTo
			}
			n := len(*draft)
			if n >= 10 {
				continue
			}
			if n == 4 || n == 7 {
				*draft += "-"
			}
			*draft += string(r)
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
			appsLen := len(m.sortedStatsApps())
			if appsLen > 0 {
				m.statsSelected = min(m.statsSelected+1, appsLen-1)
				visibleRows := m.statsVisibleRows(false)
				if m.statsSelected >= m.statsUsageOffset+visibleRows {
					m.statsUsageOffset = clampScrollOffset(m.statsSelected-visibleRows+1, visibleRows, appsLen)
				}
			}
		case 2:
			browserLen := len(m.stats.Browsers)
			if browserLen > 0 {
				m.browserSelected = min(m.browserSelected+1, browserLen-1)
				visibleRows := m.statsVisibleRows(true)
				if m.browserSelected >= m.statsBrowserOffset+visibleRows {
					m.statsBrowserOffset = clampScrollOffset(m.browserSelected-visibleRows+1, visibleRows, browserLen)
				}
			}
		}
	case "k", "up":
		switch m.panelFocus {
		case 1:
			appsLen := len(m.sortedStatsApps())
			if appsLen > 0 {
				m.statsSelected = max(0, m.statsSelected-1)
				if m.statsSelected < m.statsUsageOffset {
					m.statsUsageOffset = m.statsSelected
				}
			}
		case 2:
			browserLen := len(m.stats.Browsers)
			if browserLen > 0 {
				m.browserSelected = max(0, m.browserSelected-1)
				if m.browserSelected < m.statsBrowserOffset {
					m.statsBrowserOffset = m.browserSelected
				}
			}
		}
	case "pgdown":
		switch m.panelFocus {
		case 1:
			appsLen := len(m.sortedStatsApps())
			if appsLen > 0 {
				visibleRows := m.statsVisibleRows(false)
				m.statsSelected = min(m.statsSelected+visibleRows, appsLen-1)
				if m.statsSelected >= m.statsUsageOffset+visibleRows {
					m.statsUsageOffset = clampScrollOffset(m.statsSelected-visibleRows+1, visibleRows, appsLen)
				}
			}
		case 2:
			browserLen := len(m.stats.Browsers)
			if browserLen > 0 {
				visibleRows := m.statsVisibleRows(true)
				m.browserSelected = min(m.browserSelected+visibleRows, browserLen-1)
				if m.browserSelected >= m.statsBrowserOffset+visibleRows {
					m.statsBrowserOffset = clampScrollOffset(m.browserSelected-visibleRows+1, visibleRows, browserLen)
				}
			}
		}
	case "pgup":
		switch m.panelFocus {
		case 1:
			appsLen := len(m.sortedStatsApps())
			if appsLen > 0 {
				visibleRows := m.statsVisibleRows(false)
				m.statsSelected = max(0, m.statsSelected-visibleRows)
				if m.statsSelected < m.statsUsageOffset {
					m.statsUsageOffset = max(0, m.statsSelected)
				}
			}
		case 2:
			browserLen := len(m.stats.Browsers)
			if browserLen > 0 {
				visibleRows := m.statsVisibleRows(true)
				m.browserSelected = max(0, m.browserSelected-visibleRows)
				if m.browserSelected < m.statsBrowserOffset {
					m.statsBrowserOffset = max(0, m.browserSelected)
				}
			}
		}
	}
	return m, nil
}

func validateCustomRange(from, to string) (string, string, error) {
	fromTime, err := time.Parse("2006-01-02", from)
	if err != nil {
		return "", "", fmt.Errorf("Invalid date format. Use YYYY-MM-DD")
	}
	toTime, err := time.Parse("2006-01-02", to)
	if err != nil {
		return "", "", fmt.Errorf("Invalid date format. Use YYYY-MM-DD")
	}
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

func (m *Model) renderStats(width, height int) string {
	inner := width - 2
	rangeSelector := m.renderRangeSelector(inner - 4)
	topPanel := panelWithHover("TIME RANGE", "", inner, "\n"+rangeSelector+"\n", m.panelFocus == 0, m.hoveredPanel == 0)

	topH := lipgloss.Height(topPanel)
	contentHeight := height - topH - 2
	if contentHeight < 6 {
		contentHeight = 6
	}

	m.clickableRegions = append(m.clickableRegions, clickableRegion{
		X1: 1, Y1: 5, X2: inner + 1, Y2: 5 + topH,
		ID: "panel-0", Kind: "panel",
	})

	var middleRow string
	topY := 5 + topH + 1
	var middleHeight int

	if inner >= 120 {
		leftW := (inner - 2) / 2
		rightW := inner - 2 - leftW
		middleHeight = (contentHeight * 60) / 100

		m.clickableRegions = append(m.clickableRegions, clickableRegion{
			X1: 1, Y1: topY, X2: leftW + 1, Y2: topY + middleHeight,
			ID: "panel-1", Kind: "panel",
		})
		m.clickableRegions = append(m.clickableRegions, clickableRegion{
			X1: leftW + 3, Y1: topY, X2: width - 1, Y2: topY + middleHeight,
			ID: "panel-2", Kind: "panel",
		})

		usageRows := m.statsVisibleRows(false)
		browserRows := m.statsVisibleRows(true)
		appsPanel := panelWithHover("APPLICATION BREAKDOWN", "", leftW, "\n"+m.renderUsageBreakdown(leftW, usageRows)+"\n", m.panelFocus == 1, m.hoveredPanel == 1)
		browsersPanel := panelWithHover("BROWSER USAGE", "", rightW, "\n"+m.renderBrowserUsage(rightW, browserRows)+"\n", m.panelFocus == 2, m.hoveredPanel == 2)
		middleRow = lipgloss.JoinHorizontal(lipgloss.Top, appsPanel, " ", browsersPanel)
	} else {
		middleHeight = contentHeight - 6
		if m.panelFocus == 0 || m.panelFocus == 1 {
			m.clickableRegions = append(m.clickableRegions, clickableRegion{
				X1: 1, Y1: topY, X2: inner + 1, Y2: topY + middleHeight,
				ID: "panel-1", Kind: "panel",
			})
			usageRows := m.statsVisibleRows(false)
			middleRow = panelWithHover("APPLICATION BREAKDOWN", "", inner, "\n"+m.renderUsageBreakdown(inner, usageRows)+"\n", m.panelFocus == 1, m.hoveredPanel == 1)
		} else {
			m.clickableRegions = append(m.clickableRegions, clickableRegion{
				X1: 1, Y1: topY, X2: inner + 1, Y2: topY + middleHeight,
				ID: "panel-2", Kind: "panel",
			})
			browserRows := m.statsVisibleRows(true)
			middleRow = panelWithHover("BROWSER USAGE", "", inner, "\n"+m.renderBrowserUsage(inner, browserRows)+"\n", m.panelFocus == 2, m.hoveredPanel == 2)
		}
	}

	historyY := topY + middleHeight + 1
	historyHeight := height - historyY
	if historyHeight < 3 {
		historyHeight = 3
	}
	m.clickableRegions = append(m.clickableRegions, clickableRegion{
		X1: 1, Y1: historyY, X2: inner + 1, Y2: height - 1,
		ID: "panel-3", Kind: "panel",
	})

	historyPanel := m.renderDailyHistory(inner)

	return lipgloss.JoinVertical(lipgloss.Left, topPanel, middleRow, historyPanel)
}

func (m *Model) renderRangeSelector(width int) string {
	ranges := []string{"TODAY", "LAST 7 DAYS", "LAST 30 DAYS", "CUSTOM RANGE"}
	var parts []string

	selectorText := "[ TODAY ]   [ LAST 7 DAYS ]   [ LAST 30 DAYS ]   [ CUSTOM RANGE ]"
	if m.statsRange == 3 {
		customInfo := fmt.Sprintf("   Range: %s to %s", m.statsCustomFrom, m.statsCustomTo)
		selectorText += customInfo
	}
	selectorLen := len(selectorText)
	startOffset := 3 + max(0, (width-selectorLen)/2)

	btnWidths := []int{11, 17, 18, 18}
	offsets := []int{0, 14, 34, 55}

	for i, r := range ranges {
		startX := startOffset + offsets[i]
		m.clickableRegions = append(m.clickableRegions, clickableRegion{
			X1: startX, Y1: 8, X2: startX + btnWidths[i], Y2: 9,
			ID: fmt.Sprintf("range-%d", i), Kind: "button",
		})

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

func (m *Model) renderUsageBreakdown(width, visibleRows int) string {
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
	lines = append(lines, header, mutedStyle.Render(strings.Repeat("─", max(0, width-4))))

	if len(sorted) > 0 {
		start := m.statsUsageOffset
		end := min(len(sorted), start+visibleRows)
		for i := start; i < end; i++ {
			a := sorted[i]
			color := cyanStyle
			if i == m.statsSelected && m.panelFocus == 1 {
				color = lipgloss.NewStyle().Foreground(cWhite)
			}
			filled := 0
			if maxDuration > 0 {
				filled = a.Duration * 3 / maxDuration
				if filled == 0 && a.Duration > 0 {
					filled = 1
				}
				if filled > 3 {
					filled = 3
				}
			}
			barStr := strings.Repeat("█", filled) + strings.Repeat("░", 3-filled)
			line := mutedStyle.Render(fmt.Sprintf("%2d. ", i+1)) +
				color.Render(padRight(truncate(a.Name, max(8, width-36)), max(8, width-36))) +
				padLeft(formatDuration(a.Duration), 8) +
				padLeft(strconv.Itoa(a.Opens), 8) + " " +
				color.Render(barStr)

			if i == m.statsSelected && m.panelFocus == 1 {
				line = selectedRowStyle.Render(padRight(line, width-4))
			} else if m.hoveredPanel == 1 && m.statsSelected == i {
				line = hoverRowStyle.Render(padRight(line, width-4))
			}
			lines = append(lines, line)
		}
	} else {
		lines = append(lines, mutedStyle.Render("No usage recorded for this range."))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) renderBrowserUsage(width, visibleRows int) string {
	var lines []string
	titleW := max(8, width-16)
	lines = append(lines, boldStyle.Render(fitLine(padRight("Tab Title", titleW)+padLeft("Time", 10), width-4)), mutedStyle.Render(strings.Repeat("─", max(0, width-4))))

	if len(m.stats.Browsers) > 0 {
		start := m.statsBrowserOffset
		end := min(len(m.stats.Browsers), start+visibleRows)
		for i := start; i < end; i++ {
			b := m.stats.Browsers[i]
			prefix := mutedStyle.Render("▶ ")
			line := prefix + boldStyle.Render(padRight(truncate(b.Name, titleW), titleW)) + padLeft(formatDuration(b.Duration), 10)
			if i == m.browserSelected && m.panelFocus == 2 {
				line = selectedRowStyle.Render(padRight(line, width-4))
			} else if m.hoveredPanel == 2 && m.browserSelected == i {
				line = hoverRowStyle.Render(padRight(line, width-4))
			}
			lines = append(lines, line)
		}
	} else {
		lines = append(lines, mutedStyle.Render("No browser activity recorded."))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) renderDailyHistory(width int) string {
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
	return panelWithHover(title, "", width, "\n"+strings.Join(lines, "\n")+"\n", m.panelFocus == 3, m.hoveredPanel == 3)
}

func (m *Model) renderCustomRangeModal() string {
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
	return panelWithHover("CUSTOM RANGE", "", 50, strings.Join(msg, "\n"), true, false)
}

func (m *Model) sortedStatsApps() []appUsage {
	var firstItems string
	if len(m.stats.Apps) > 0 {
		firstItems = m.stats.Apps[0].Name + fmt.Sprintf("-%d-%d", m.stats.Apps[0].Duration, m.stats.Apps[0].Opens)
	}
	if len(m.stats.Apps) > 1 {
		firstItems += m.stats.Apps[1].Name + fmt.Sprintf("-%d-%d", m.stats.Apps[1].Duration, m.stats.Apps[1].Opens)
	}
	key := fmt.Sprintf("%d-%d-%t-%d-%s", len(m.stats.Apps), m.statsSort, m.statsAsc, m.stats.Total, firstItems)
	if m.sortedAppsCache != nil && m.sortedAppsCacheKey == key {
		return m.sortedAppsCache
	}
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
	m.sortedAppsCache = apps
	m.sortedAppsCacheKey = key
	return apps
}
