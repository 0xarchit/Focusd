package tui

import (
	"fmt"
	"focusd/system"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) handleDashboardKey(key string) (Model, tea.Cmd) {
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

func (m Model) renderDashboard(width, height int) string {
	inner := width - 2
	gap := " "
	colW := (inner - 2) / 3
	if colW < 20 {
		colW = 20
	}
	remainder := inner - 2 - (colW * 3)
	leftW := colW
	midW := colW + remainder
	rightW := colW

	todayPanel := m.renderTodayPanel(leftW)
	topAppsPanel := m.renderTopAppsPanel(midW)
	hourlyPanel := m.renderHourlyPanel(rightW)

	return lipgloss.JoinHorizontal(lipgloss.Top,
		todayPanel,
		gap,
		topAppsPanel,
		gap,
		hourlyPanel,
	)
}

func (m Model) renderTodayPanel(width int) string {
	rows := []string{
		rowKV("Screen Time", formatDuration(m.dashboard.Total), width-4),
		rowKV("Active Apps", fmt.Sprintf("%d", m.dashboard.ActiveApps), width-4),
		rowKV("Limits Hit", fmt.Sprintf("%d", m.dashboard.LimitsHit), width-4),
	}
	return panel("TODAY", "", width, "\n"+strings.Join(rows, "\n")+"\n", m.panelFocus == 0)
}

func (m Model) renderTopAppsPanel(width int) string {
	maxDuration := 0
	for _, a := range m.dashboard.Apps {
		maxDuration = max(maxDuration, a.Duration)
	}
	limits := system.GetAppTimeLimits()
	var lines []string
	if len(m.dashboard.Apps) == 0 {
		lines = append(lines, mutedStyle.Render("No app data yet. Start the daemon to begin tracking."))
	} else {
		for i, a := range m.dashboard.Apps[:min(10, len(m.dashboard.Apps))] {
			warn := "  "
			if _, ok := limits[a.Name]; ok {
				warn = amberStyle.Render("⚠ ")
			}
			nameW := max(10, width-24)
			line := mutedStyle.Render(fmt.Sprintf("%2d. ", i+1)) + warn +
				padRight(truncate(a.Name, nameW), nameW) +
				padLeft(formatDuration(a.Duration), 8) + "  " +
				cyanStyle.Render(bar(a.Duration, maxDuration, 3, "█"))
			if i == m.dashSelected && m.panelFocus == 1 {
				line = selectedRowStyle.Render(padRight(line, width-2))
			}
			lines = append(lines, line)
		}
	}
	refresh := " "
	if m.refreshing {
		refresh = cyanStyle.Render("↻")
	}
	return panel("TOP APPLICATIONS", refresh, width, "\n"+strings.Join(lines, "\n")+"\n", m.panelFocus == 1)
}

func (m Model) renderHourlyPanel(width int) string {
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
	hoursText := "00" + strings.Repeat(" ", blockWidth*6-2) + "06" + strings.Repeat(" ", blockWidth*6-2) + "12" + strings.Repeat(" ", blockWidth*6-2) + "18" + strings.Repeat(" ", blockWidth*6-2) + "23"
	if len(hoursText) > width-6 {
		hoursText = truncate(hoursText, width-6)
	}
	lines = append(lines, "   "+mutedStyle.Render(hoursText))
	lines = append(lines, "", "   "+mutedStyle.Render("Heatmap shows activity throughout"), "   "+mutedStyle.Render("the day in hourly blocks."), "")
	return panel("ACTIVITY HEATMAP", "", width, strings.Join(lines, "\n"), m.panelFocus == 2)
}
