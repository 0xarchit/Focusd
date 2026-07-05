package tui

import (
	"fmt"
	"focusd/storage"
	"focusd/system"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) handleDashboardKey(key string) (Model, tea.Cmd) {
	switch key {
	case "s":
		m.activeTab = tabFocus
		return m, nil
	case "n":
		m.activeTab = tabLimits
		m.limitForm = limitForm{Visible: true}
		m.panelFocus = 1 // Focus form panel
		return m, nil
	case "p":
		paused := storage.IsPaused()
		if err := storage.SetPaused(!paused); err != nil {
			m.addToast("Failed to toggle tracking: "+err.Error(), toastError)
		} else {
			if !paused {
				m.addToast("Tracking paused", toastWarning)
			} else {
				m.addToast("Tracking resumed", toastSuccess)
			}
		}
		return m, nil
	}

	if m.panelFocus == 1 {
		switch key {
		case "j", "down":
			m.dashSelected = min(m.dashSelected+1, max(0, len(m.dashboard.Apps)-1))
		case "k", "up":
			m.dashSelected = max(0, m.dashSelected-1)
		}
	}
	return m, nil
}

func (m *Model) renderDashboard(width, height int) string {
	inner := width

	topHeight := (height * 55) / 100
	bottomHeight := height - topHeight

	leftW := (inner * 25) / 100
	midW := (inner * 45) / 100
	rightW := inner - leftW - midW - 2

	weeklyH := (bottomHeight * 70) / 100
	quickH := bottomHeight - weeklyH - 1
	weeklyW := (inner * 70) / 100
	quickW := inner - weeklyW - 1

	var bottomPart string
	if width < 100 {
		weeklyPanel := m.renderWeeklyPanel(inner, weeklyH)
		quickPanel := m.renderQuickActionsPanel(inner, quickH)
		bottomPart = lipgloss.JoinVertical(lipgloss.Left, weeklyPanel, "", quickPanel)
	} else {
		weeklyPanel := m.renderWeeklyPanel(weeklyW, bottomHeight)
		quickPanel := m.renderQuickActionsPanel(quickW, bottomHeight)
		bottomPart = lipgloss.JoinHorizontal(lipgloss.Top, weeklyPanel, " ", quickPanel)
	}

	topY := 5
	m.clickableRegions = append(m.clickableRegions, clickableRegion{X1: 1, Y1: topY, X2: leftW, Y2: topY + topHeight, ID: "panel-0", Kind: "panel"})
	m.clickableRegions = append(m.clickableRegions, clickableRegion{X1: leftW + 2, Y1: topY, X2: leftW + 2 + midW, Y2: topY + topHeight, ID: "panel-1", Kind: "panel"})
	m.clickableRegions = append(m.clickableRegions, clickableRegion{X1: leftW + midW + 4, Y1: topY, X2: width - 1, Y2: topY + topHeight, ID: "panel-2", Kind: "panel"})

	bottomY := topY + topHeight + 1
	if width < 100 {
		m.clickableRegions = append(m.clickableRegions, clickableRegion{X1: 1, Y1: bottomY, X2: width - 1, Y2: bottomY + weeklyH, ID: "panel-3", Kind: "panel"})
		m.clickableRegions = append(m.clickableRegions, clickableRegion{X1: 1, Y1: bottomY + weeklyH + 1, X2: width - 1, Y2: bottomY + weeklyH + 1 + quickH, ID: "panel-4", Kind: "panel"})
	} else {
		m.clickableRegions = append(m.clickableRegions, clickableRegion{X1: 1, Y1: bottomY, X2: weeklyW, Y2: bottomY + bottomHeight, ID: "panel-3", Kind: "panel"})
		m.clickableRegions = append(m.clickableRegions, clickableRegion{X1: weeklyW + 2, Y1: bottomY, X2: width - 1, Y2: bottomY + bottomHeight, ID: "panel-4", Kind: "panel"})
	}

	todayPanel := m.renderTodayPanel(leftW, topHeight)
	topAppsPanel := m.renderTopAppsPanel(midW, topHeight)
	hourlyPanel := m.renderHourlyPanel(rightW, topHeight)

	topPart := lipgloss.JoinHorizontal(lipgloss.Top, todayPanel, " ", topAppsPanel, " ", hourlyPanel)

	return lipgloss.JoinVertical(lipgloss.Left, topPart, " ", bottomPart)
}

func (m *Model) renderTodayPanel(width, height int) string {
	rows := []string{
		rowKV("Screen Time", formatDuration(m.dashboard.Total), width-4),
		rowKV("Active Apps", fmt.Sprintf("%d", m.dashboard.ActiveApps), width-4),
		rowKV("Limits Hit", fmt.Sprintf("%d", m.dashboard.LimitsHit), width-4),
	}
	var trendText string
	if m.dashboard.YesterdayTotal == 0 {
		if m.dashboard.Total > 0 {
			trendText = mutedStyle.Render("N/A")
		} else {
			trendText = mutedStyle.Render("0%")
		}
	} else {
		diff := m.dashboard.Total - m.dashboard.YesterdayTotal
		pct := int(float64(diff) / float64(m.dashboard.YesterdayTotal) * 100)
		if pct > 0 {
			trendText = redStyle.Render(fmt.Sprintf("▲ %d%%", pct))
		} else if pct < 0 {
			trendText = greenStyle.Render(fmt.Sprintf("▼ %d%%", -pct))
		} else {
			trendText = mutedStyle.Render("0%")
		}
	}
	rows = append(rows, rowKV("Vs Yesterday", trendText, width-4))

	paddingLines := max(0, height-2-len(rows)-2)
	body := "\n" + strings.Join(rows, "\n") + strings.Repeat("\n", paddingLines)
	return panelWithHover("TODAY", "", width, body, m.panelFocus == 0, m.hoveredPanel == 0)
}

func (m *Model) renderTopAppsPanel(width, height int) string {
	maxDuration := 0
	for _, a := range m.dashboard.Apps {
		maxDuration = max(maxDuration, a.Duration)
	}
	limits := system.GetAppTimeLimits()
	var lines []string

	maxRows := max(1, height-4)
	if len(m.dashboard.Apps) == 0 {
		lines = append(lines, mutedStyle.Render("No app data yet. Start tracking."))
	} else {
		start := 0
		if m.dashSelected >= maxRows {
			start = m.dashSelected - maxRows + 1
		}
		end := min(start+maxRows, len(m.dashboard.Apps))
		if end-start < maxRows {
			start = max(0, end-maxRows)
		}

		for idx := start; idx < end; idx++ {
			a := m.dashboard.Apps[idx]
			warn := "  "
			if _, ok := limits[a.Name]; ok {
				warn = amberStyle.Render("⚠ ")
			}
			nameW := max(10, width-24)
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
			line := mutedStyle.Render(fmt.Sprintf("%2d. ", idx+1)) + warn +
				padRight(truncate(a.Name, nameW), nameW) +
				padLeft(formatDuration(a.Duration), 8) + "  " +
				cyanStyle.Render(barStr)

			if idx == m.dashSelected && m.panelFocus == 1 {
				line = selectedRowStyle.Render(padRight(line, width-2))
			} else if m.hoveredPanel == 1 && m.dashSelected == idx {
				line = hoverRowStyle.Render(padRight(line, width-2))
			}
			lines = append(lines, line)
		}
	}
	refresh := " "
	if m.refreshing {
		refresh = cyanStyle.Render("↻")
	}
	return panelWithHover("TOP APPLICATIONS", refresh, width, "\n"+strings.Join(lines, "\n")+"\n", m.panelFocus == 1, m.hoveredPanel == 1)
}

func (m *Model) renderHourlyPanel(width, height int) string {
	var lines []string
	lines = append(lines, "")
	maxVal := 1
	for _, v := range m.dashboard.Hourly {
		if v > maxVal {
			maxVal = v
		}
	}
	blockWidth := (width - 6) / 24
	if blockWidth < 1 {
		blockWidth = 1
	}
	var blocks []string
	for h := 0; h < 24; h++ {
		block := "░"
		val := m.dashboard.Hourly[h]
		if val == 0 {
			block = mutedStyle.Render("·")
		} else {
			ratio := float64(val) / float64(maxVal)
			if ratio > 0.75 {
				block = "█"
			} else if ratio > 0.5 {
				block = "▓"
			} else if ratio > 0.25 {
				block = "▒"
			}
			block = cyanStyle.Render(block)
		}
		blocks = append(blocks, strings.Repeat(block, blockWidth))
	}
	prefix := "   "
	suffix := "   "
	lines = append(lines, prefix+strings.Join(blocks, "")+suffix)
	gap := strings.Repeat(" ", blockWidth*6-2)
	hoursText := "00" + gap + "06" + gap + "12" + gap + "18" + gap + "23"
	if len(hoursText) > width-6 {
		hoursText = truncate(hoursText, width-6)
	}
	lines = append(lines, "   "+mutedStyle.Render(hoursText))

	// Add legend for usability, splitting into 2 lines on narrow panels
	if width >= 52 {
		legend := "Legend: " + mutedStyle.Render("·") + " 0m  " +
			cyanStyle.Render("░") + " <15m  " +
			cyanStyle.Render("▒") + " <30m  " +
			cyanStyle.Render("▓") + " <45m  " +
			cyanStyle.Render("█") + " >=45m"
		lines = append(lines, "")
		lines = append(lines, "   "+legend)
	} else if width >= 38 {
		legend1 := "Legend: " + mutedStyle.Render("·") + " 0m  " +
			cyanStyle.Render("░") + " <15m  " +
			cyanStyle.Render("▒") + " <30m"
		legend2 := "        " + cyanStyle.Render("▓") + " <45m  " +
			cyanStyle.Render("█") + " >=45m"
		lines = append(lines, "")
		lines = append(lines, "   "+legend1)
		lines = append(lines, "   "+legend2)
	}

	extraLines := max(0, height-2-len(lines)-2)
	for i := 0; i < extraLines; i++ {
		lines = append(lines, "")
	}
	return panelWithHover("ACTIVITY HEATMAP", "", width, strings.Join(lines, "\n"), m.panelFocus == 2, m.hoveredPanel == 2)
}

func (m *Model) renderWeeklyPanel(width, height int) string {
	var lines []string
	maxVal := 1
	for _, day := range m.dashboard.Days {
		if day.Duration > maxVal {
			maxVal = day.Duration
		}
	}

	maxRows := max(1, height-4-2) // Subtract 2 for headers
	visibleDays := m.dashboard.Days
	if len(visibleDays) > maxRows {
		visibleDays = visibleDays[len(visibleDays)-maxRows:]
	}

	if len(visibleDays) == 0 {
		lines = append(lines, mutedStyle.Render("No historical weekly trends yet."))
	} else {
		// Aligned Headers
		barWidth := max(5, width-29)
		headerLine := boldStyle.Render(padRight("DAY", 6) + padRight("DATE", 12) + padRight("WEEKLY TREND", barWidth) + " " + padLeft("DURATION", 8))
		lines = append(lines, headerLine)
		lines = append(lines, mutedStyle.Render(strings.Repeat("─", width-4)))

		for _, day := range visibleDays {
			labelStyle := mutedStyle
			if day.Today {
				labelStyle = boldStyle
			} else if day.Weekend {
				labelStyle = amberStyle
			}
			filled := 0
			if maxVal > 0 {
				filled = day.Duration * barWidth / maxVal
				if filled == 0 && day.Duration > 0 {
					filled = 1
				}
				if filled > barWidth {
					filled = barWidth
				}
			}
			barStr := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
			line := labelStyle.Render(padRight(day.Label, 6)) +
				padRight(day.Date, 12) +
				cyanStyle.Render(barStr) + " " +
				padLeft(formatDuration(day.Duration), 8)
			lines = append(lines, line)
		}
	}
	padding := max(0, height-2-len(lines)-2)
	body := "\n" + strings.Join(lines, "\n") + strings.Repeat("\n", padding)
	return panelWithHover("WEEKLY TREND", "", width, body, m.panelFocus == 3, m.hoveredPanel == 3)
}

func (m *Model) renderQuickActionsPanel(width, height int) string {
	pauseLabel := "[p] Pause Tracking"
	if storage.IsPaused() {
		pauseLabel = "[p] Resume"
	}
	actions := []string{
		"[s] Start Focus",
		"[n] Add App Limit",
		pauseLabel,
		"[q] Quit App",
	}
	var lines []string
	lines = append(lines, "")
	for _, act := range actions {
		line := "  " + cyanStyle.Render(act[:3]) + mutedStyle.Render(act[3:])
		lines = append(lines, line)
	}
	padding := max(0, height-2-len(lines)-2)
	body := strings.Join(lines, "\n") + strings.Repeat("\n", padding)
	return panelWithHover("QUICK ACTIONS", "", width, body, m.panelFocus == 4, m.hoveredPanel == 4)
}
