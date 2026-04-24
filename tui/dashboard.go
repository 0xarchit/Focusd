package tui

import (
	"fmt"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

func (m mainModel) renderDashboard() string {
	leftWidth := int(float64(m.width) * 0.65)
	rightWidth := m.width - leftWidth - 4

	m.appTable.SetWidth(leftWidth)
	m.appTable.SetHeight(m.height - 10)

	leftPane := cardStyle.Width(leftWidth).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("Top Applications"),
			m.appTable.View(),
		),
	)

	totalDur := fmt.Sprintf("%dh %dm", m.totalTimeSecs/3600, (m.totalTimeSecs%3600)/60)
	limitDur := "6h 00m" // Could be fetched from config
	progress := float64(m.totalTimeSecs) / (6 * 3600)

	box1 := cardStyle.Width(rightWidth).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("Daily Progress"),
			fmt.Sprintf("Today: %s", totalDur),
			fmt.Sprintf("Limit: %s", limitDur),
			fmt.Sprintf("\n%s", renderProgressBar(progress, rightWidth-4)),
		),
	)

	box2 := cardStyle.Width(rightWidth).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("Daemon Health"),
			fmt.Sprintf("PID: %d", m.daemonPID),
			"Status: Connected",
			"Mode: Stealth",
		),
	)

	rightPane := lipgloss.JoinVertical(lipgloss.Left, box1, box2)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
}

func renderProgressBar(percent float64, width int) string {
	filled := int(float64(width) * percent)
	if filled > width {
		filled = width
	}
	bar := ""
	for i := 0; i < filled; i++ {
		bar += "█"
	}
	for i := filled; i < width; i++ {
		bar += "░"
	}
	return lipgloss.NewStyle().Foreground(accentColor).Render(bar)
}

func (m *mainModel) initTable() {
	columns := []table.Column{
		{Title: "App Name", Width: 30},
		{Title: "Duration", Width: 15},
		{Title: "Opens", Width: 10},
	}

	rows := []table.Row{
		{"Code.exe", "2h 45m", "12"},
		{"chrome.exe", "1h 10m", "45"},
		{"discord.exe", "0h 30m", "8"},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	s := table.DefaultStyles()
	s.Header = tableHeaderStyle
	s.Selected = tableSelectedStyle
	t.SetStyles(s)
	m.appTable = t
}
