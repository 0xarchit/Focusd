package tui

import (
	"fmt"
	"focusd/storage"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"math"
	"strings"
)

type dashboardModel struct {
	table        table.Model
	width        int
	height       int
	totalSecs    int
	limitTotal   int
	daemonPID    uint32
	daemonMemory string
	daemonUptime string
	pomodoroOn   bool
	pomodoroLeft int
	pomodoroMins int
}

func newDashboardModel() dashboardModel {
	columns := []table.Column{
		{Title: "App Name", Width: 28},
		{Title: "Duration", Width: 14},
		{Title: "Opens", Width: 8},
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	tStyles := table.DefaultStyles()
	tStyles.Header = lipgloss.NewStyle().Foreground(colorGold).Bold(true)
	tStyles.Selected = lipgloss.NewStyle().Foreground(colorWhite).Background(colorNavy).Bold(true)
	t.SetStyles(tStyles)
	return dashboardModel{table: t}
}

func (d *dashboardModel) SetSize(width, height int) {
	d.width = width
	d.height = height
	leftWidth := int(float64(width) * 0.65)
	if leftWidth < 30 {
		leftWidth = width
	}
	tableHeight := height - 6
	if tableHeight < 6 {
		tableHeight = 6
	}
	d.table.SetWidth(leftWidth - 8)
	d.table.SetHeight(tableHeight)
	cols := d.table.Columns()
	if len(cols) == 3 {
		cols[0].Width = maxInt(20, leftWidth-32)
		cols[1].Width = 12
		cols[2].Width = 8
		d.table.SetColumns(cols)
	}
}

func (d *dashboardModel) SetData(state appSnapshot) {
	rows := make([]table.Row, 0, len(state.appStats))
	for _, stat := range state.appStats {
		rows = append(rows, table.Row{displayName(stat), fmtDuration(stat.TotalDurationSecs), fmt.Sprintf("%d", stat.OpenCount)})
	}
	d.table.SetRows(rows)
	d.totalSecs = state.totalScreenSecs
	limitTotal := 0
	for _, minutes := range state.appLimits {
		if minutes > 0 {
			limitTotal += minutes * 60
		}
	}
	d.limitTotal = limitTotal
	d.daemonPID = state.daemonPID
	d.daemonMemory = state.daemonMemory
	d.daemonUptime = state.daemonUptime
	d.pomodoroOn = state.pomodoroActive
	d.pomodoroLeft = state.pomodoroRemSecs
	d.pomodoroMins = state.pomodoroTotal
}

func (d dashboardModel) Update(msg tea.Msg) (dashboardModel, tea.Cmd) {
	var cmd tea.Cmd
	d.table, cmd = d.table.Update(msg)
	return d, cmd
}

func (d dashboardModel) HandleKey(msg tea.KeyMsg) (dashboardModel, tea.Cmd, bool) {
	switch msg.String() {
	case "s":
		return d, startDaemonCmd(), true
	case "x":
		return d, stopDaemonCmd(), true
	case " ":
		return d, toggleTrackingCmd(), true
	}
	var cmd tea.Cmd
	d.table, cmd = d.table.Update(msg)
	return d, cmd, true
}

func (d dashboardModel) View() string {
	leftWidth := int(float64(d.width) * 0.65)
	if leftWidth < 30 {
		leftWidth = d.width
	}
	rightWidth := d.width - leftWidth - 1
	if rightWidth < 28 {
		rightWidth = 28
	}
	leftPane := cardStyle.Width(leftWidth).Height(d.height - 1).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			sectionTitleStyle.Render("Executive Dashboard"),
			d.table.View(),
		),
	)
	limitText := "No app limits configured"
	progress := 0.0
	if d.limitTotal > 0 {
		limitText = fmtDuration(d.limitTotal)
		progress = math.Min(float64(d.totalSecs)/float64(d.limitTotal), 1)
	}
	statusText := "Inactive"
	if d.pomodoroOn {
		statusText = "Active"
	}
	box1 := cardSoftStyle.Width(rightWidth).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			sectionTitleStyle.Render("Screen Time vs Limits"),
			"Today: "+fmtDuration(d.totalSecs),
			"Configured limits: "+limitText,
			renderBar(progress, rightWidth-8),
		),
	)
	box2 := cardSoftStyle.Width(rightWidth).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			sectionTitleStyle.Render("Daemon Health"),
			fmt.Sprintf("PID: %d", d.daemonPID),
			"Memory: "+d.daemonMemory,
			"Uptime: "+d.daemonUptime,
		),
	)
	box3 := cardSoftStyle.Width(rightWidth).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			sectionTitleStyle.Render("Pomodoro"),
			"Status: "+statusText,
			"Remaining: "+fmtDuration(d.pomodoroLeft),
			fmt.Sprintf("Session: %d min", d.pomodoroMins),
		),
	)
	rightPane := lipgloss.JoinVertical(lipgloss.Left, box1, box2, box3)
	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
}

func renderBar(progress float64, width int) string {
	if width < 10 {
		width = 10
	}
	filled := int(progress * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	bar := lipgloss.NewStyle().Foreground(colorGold).Render(stringsRepeat("█", filled))
	pad := lipgloss.NewStyle().Foreground(colorSlate).Render(stringsRepeat("░", width-filled))
	return bar + pad
}

func displayName(stat storage.AppDailyStat) string {
	if strings.TrimSpace(stat.AppName) != "" {
		return stat.AppName
	}
	return stat.ExeName
}

func fmtDuration(seconds int) string {
	if seconds < 0 {
		seconds = 0
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	if h > 0 {
		return fmt.Sprintf("%dh %02dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func stringsRepeat(value string, count int) string {
	if count <= 0 {
		return ""
	}
	return strings.Repeat(value, count)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
