package tui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m mainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.state = (m.state + 1) % 3
		case "1":
			m.state = dashboardView
		case "2":
			m.state = toolsView
		case "3":
			m.state = settingsView
		}

	case fullDataMsg:
		m.daemonActive = msg.daemonActive
		m.daemonPID = msg.daemonPID
		m.totalTimeSecs = msg.totalTimeSecs
		m.pomodoroActive = msg.pomodoroActive
		m.pomodoroRem = msg.pomodoroRem
		m.pomodoroTotal = msg.pomodoroTotal
		m.whitelist = msg.whitelist
		m.browsers = msg.browsers
		m.appLimits = msg.appLimits

		var rows []table.Row
		for _, s := range msg.stats {
			dur := fmt.Sprintf("%dh %dm", s.TotalDurationSecs/3600, (s.TotalDurationSecs%3600)/60)
			rows = append(rows, table.Row{s.ExeName, dur, fmt.Sprintf("%d", s.OpenCount)})
		}
		m.appTable.SetRows(rows)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.settingsList.SetSize(m.width/3, m.height-10)
	}

	switch m.state {
	case dashboardView:
		m.appTable, cmd = m.appTable.Update(msg)
	case toolsView:
		m.pomodoroIn, cmd = m.pomodoroIn.Update(msg)
	case settingsView:
		m.settingsList, cmd = m.settingsList.Update(msg)
	}

	return m, cmd
}

func (m mainModel) View() string {
	if !m.ready {
		return "\n  Initializing focusd..."
	}

	if m.width < 80 || m.height < 24 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			"Please enlarge terminal for optimal viewing",
		)
	}

	header := m.renderHeader()
	content := m.renderContent()
	footer := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		lipgloss.NewStyle().Height(m.height-6).Render(content),
		footer,
	)
}

func (m mainModel) renderHeader() string {
	tabs := []string{"Dashboard", "Focus Tools", "Settings"}
	var renderedTabs []string

	for i, t := range tabs {
		var style lipgloss.Style
		if int(m.state) == i {
			style = activeTabStyle
		} else {
			style = tabStyle
		}
		renderedTabs = append(renderedTabs, style.Render(fmt.Sprintf("%d. %s", i+1, t)))
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
	
	status := " STOPPED "
	statusStyle := statusStopped
	if m.daemonActive {
		status = " RUNNING "
		statusStyle = statusRunning
	}

	statusBlock := statusStyle.Copy().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(slateColor).
		Render(status)

	header := lipgloss.JoinHorizontal(lipgloss.Center,
		headerStyle.Render(" FOCUSD "),
		row,
		lipgloss.NewStyle().Width(m.width-lipgloss.Width(row)-20).Render(""),
		statusBlock,
	)

	return lipgloss.NewStyle().
		MarginBottom(1).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(slateColor).
		Render(header)
}

func (m mainModel) renderFooter() string {
	help := "↑/↓: navigate • tab: switch tab • 1-3: jump • q: quit"
	return footerStyle.Render(help)
}

func (m mainModel) renderContent() string {
	switch m.state {
	case dashboardView:
		return m.renderDashboard()
	case toolsView:
		return m.renderTools()
	case settingsView:
		return m.renderSettings()
	default:
		return "View not implemented"
	}
}
