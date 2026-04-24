package tui

import (
	"fmt"
	"focusd/core"
	"focusd/storage"
	"focusd/system"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type dashboardModel struct {
	width, height int
	table         table.Model
	totalToday    string
	totalYesterday string
	pomodoroStatus string
	dbSize        string
	version       string
}

func newDashboard() dashboardModel {
	columns := []table.Column{
		{Title: "Application", Width: 30},
		{Title: "Time", Width: 12},
		{Title: "Visits", Width: 8},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorSlate).
		BorderBottom(true).
		Bold(false)
	s.Selected = selectedRowStyle
	t.SetStyles(s)

	return dashboardModel{
		table:   t,
		version: system.Version,
	}
}

type statsMsg struct {
	rows           []table.Row
	totalToday     string
	totalYesterday string
	pomodoro       string
	dbSize         string
}

func fetchStats() tea.Cmd {
	return func() tea.Msg {
		today := time.Now().Format("2006-01-02")
		yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

		stats, _ := storage.GetAppStatsForDate(today)
		var rows []table.Row
		totalSecs := 0
		for _, s := range stats {
			rows = append(rows, table.Row{
				s.AppName,
				formatDuration(s.TotalDurationSecs),
				fmt.Sprintf("%d", s.OpenCount),
			})
			totalSecs += s.TotalDurationSecs
		}

		yStats, _ := storage.GetAppStatsForDate(yesterday)
		yTotalSecs := 0
		for _, s := range yStats {
			yTotalSecs += s.TotalDurationSecs
		}

		active, rem, _ := core.GetPomodoroStatus()
		pomo := "Idle"
		if active {
			pomo = fmt.Sprintf("Active - %02d:%02d", int(rem.Minutes()), int(rem.Seconds())%60)
		}

		dbPath, _ := storage.GetDBPath()
		fi, _ := os.Stat(dbPath)
		size := "0 KB"
		if fi != nil {
			size = fmt.Sprintf("%.1f MB", float64(fi.Size())/1024/1024)
		}

		return statsMsg{
			rows:           rows,
			totalToday:     formatDuration(totalSecs),
			totalYesterday: formatDuration(yTotalSecs),
			pomodoro:       pomo,
			dbSize:         size,
		}
	}
}

func formatDuration(secs int) string {
	h := secs / 3600
	m := (secs % 3600) / 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func (m dashboardModel) Init() tea.Cmd {
	return fetchStats()
}

func (m dashboardModel) Update(msg tea.Msg) (dashboardModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case statsMsg:
		m.table.SetRows(msg.rows)
		m.totalToday = msg.totalToday
		m.totalYesterday = msg.totalYesterday
		m.pomodoroStatus = msg.pomodoro
		m.dbSize = msg.dbSize
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetWidth(int(float64(msg.Width) * 0.6))
	}
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m dashboardModel) View() string {
	leftWidth := int(float64(m.width) * 0.65)
	rightWidth := m.width - leftWidth - 4

	leftCol := boxStyle.Width(leftWidth).Render(m.table.View())

	summaryBox := boxStyle.Width(rightWidth).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("Summary"),
			fmt.Sprintf("Today: %s", m.totalToday),
			fmt.Sprintf("Yesterday: %s", m.totalYesterday),
		),
	)

	pomoBox := boxStyle.Width(rightWidth).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("Pomodoro Health"),
			m.pomodoroStatus,
		),
	)

	sysBox := boxStyle.Width(rightWidth).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("System"),
			fmt.Sprintf("DB Size: %s", m.dbSize),
			fmt.Sprintf("Version: %s", m.version),
		),
	)

	rightCol := lipgloss.JoinVertical(lipgloss.Left, summaryBox, pomoBox, sysBox)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftCol, rightCol)
}
