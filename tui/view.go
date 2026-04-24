package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m mainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = typed.Width
		m.height = typed.Height
		m.ready = true
		mainHeight := m.mainHeight()
		m.dashboard.SetSize(m.width, mainHeight)
		m.tools.SetSize(m.width, mainHeight)
		m.systemPane.SetSize(m.width, mainHeight)

	case tickMsg:
		return m, tea.Batch(fetchSnapshotCmd(m.repo), tickCmd())

	case snapshotMsg:
		if typed.err != nil {
			m.errText = typed.err.Error()
			return m, nil
		}
		m.data = typed.state
		m.errText = ""
		m.notice = typed.warning
		m.dashboard.SetData(typed.state)
		m.tools.SetData(typed.state)
		m.systemPane.SetData(typed.state)

	case opDoneMsg:
		if typed.err != nil {
			m.errText = typed.err.Error()
		} else {
			m.notice = typed.detail
			m.errText = ""
		}
		return m, fetchSnapshotCmd(m.repo)

	case externalDoneMsg:
		if typed.err != nil {
			m.errText = typed.err.Error()
		} else {
			m.notice = typed.name + " command completed"
		}
		return m, fetchSnapshotCmd(m.repo)

	case tea.KeyMsg:
		if m.pending != "" {
			switch typed.String() {
			case "y", "enter":
				action := m.pending
				m.pending = ""
				m.errText = ""
				if action == "update" {
					m.notice = "Starting update..."
					return m, runExternalCmd("update", "update")
				}
				m.notice = "Starting uninstall..."
				return m, runExternalCmd("uninstall", "uninstall")
			case "n", "esc":
				m.pending = ""
				m.notice = "Action cancelled"
				return m, nil
			default:
				return m, nil
			}
		}
		switch typed.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.activeTab = (m.activeTab + 1) % 3
			return m, nil
		case "1":
			m.activeTab = tabDashboard
			return m, nil
		case "2":
			m.activeTab = tabTools
			return m, nil
		case "3":
			m.activeTab = tabSystem
			return m, nil
		case "r":
			return m, fetchSnapshotCmd(m.repo)
		case "u":
			m.pending = "update"
			m.notice = "Confirm update: press y to continue, n to cancel"
			return m, nil
		case "U":
			m.pending = "uninstall"
			m.notice = "Confirm uninstall: press y to continue, n to cancel"
			return m, nil
		}
		if m.activeTab == tabDashboard {
			var consumed bool
			m.dashboard, cmd, consumed = m.dashboard.HandleKey(typed)
			if consumed {
				return m, cmd
			}
		}
		if m.activeTab == tabTools {
			cmd, _ = m.tools.HandleKey(typed)
			return m, cmd
		}
		if m.activeTab == tabSystem {
			cmd, _ = m.systemPane.HandleKey(typed)
			return m, cmd
		}
	}

	if m.activeTab == tabDashboard {
		m.dashboard, cmd = m.dashboard.Update(msg)
	}
	return m, cmd
}

func (m mainModel) View() string {
	if !m.ready {
		return "Initializing focusd..."
	}
	if m.width < 80 || m.height < 24 {
		warn := lipgloss.NewStyle().Foreground(colorGold).Bold(true).Render("Please enlarge terminal for optimal viewing")
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, warn)
	}
	headerHeight := m.headerHeight()
	footerHeight := m.footerHeight()
	mainHeight := m.mainHeight()
	header := lipgloss.NewStyle().Width(m.width).Height(headerHeight).Render(m.renderHeader())
	mainContent := lipgloss.NewStyle().Width(m.width).Height(mainHeight).Render(m.renderMain())
	footer := lipgloss.NewStyle().Width(m.width).Height(footerHeight).Render(m.renderFooter())
	return appFrameStyle.Render(lipgloss.JoinVertical(lipgloss.Left, header, mainContent, footer))
}

func (m mainModel) headerHeight() int {
	h := int(float64(m.height) * 0.10)
	if h < 3 {
		h = 3
	}
	return h
}

func (m mainModel) footerHeight() int {
	h := int(float64(m.height) * 0.05)
	if h < 2 {
		h = 2
	}
	return h
}

func (m mainModel) mainHeight() int {
	h := m.height - m.headerHeight() - m.footerHeight()
	if h < 10 {
		return 10
	}
	return h
}

func (m mainModel) renderHeader() string {
	tabs := []string{"Dashboard", "Focus Tools", "System"}
	renderedTabs := make([]string, 0, len(tabs))
	for i, label := range tabs {
		if int(m.activeTab) == i {
			renderedTabs = append(renderedTabs, activeTabStyle.Render("[ "+label+" ]"))
		} else {
			renderedTabs = append(renderedTabs, inactiveTabStyle.Render(label))
		}
	}
	left := lipgloss.JoinHorizontal(lipgloss.Left, renderedTabs...)
	statusText := "Stopped"
	statusBlock := statusOffStyle.Render("STOPPED")
	if m.data.daemonRunning {
		statusText = "Running"
		statusBlock = statusOnStyle.Render("RUNNING")
	}
	meta := fmt.Sprintf("daemon: %s", statusText)
	line := lipgloss.JoinHorizontal(lipgloss.Top,
		left,
		lipgloss.NewStyle().Width(maxInt(2, m.width-lipgloss.Width(left)-lipgloss.Width(statusBlock)-len(meta)-6)).Render(""),
		lipgloss.NewStyle().Foreground(colorSlate).Render(meta),
		" ",
		statusBlock,
	)
	if m.errText != "" {
		line = lipgloss.JoinVertical(lipgloss.Left, line, lipgloss.NewStyle().Foreground(colorRed).Render(m.errText))
	} else if m.notice != "" {
		line = lipgloss.JoinVertical(lipgloss.Left, line, lipgloss.NewStyle().Foreground(colorGold).Render(m.notice))
	}
	return headerBoxStyle.Width(m.width).Render(line)
}

func (m mainModel) renderMain() string {
	if m.activeTab == tabDashboard {
		return m.dashboard.View()
	}
	if m.activeTab == tabTools {
		return m.tools.View()
	}
	return m.systemPane.View()
}

func (m mainModel) renderFooter() string {
	help := "tab: next tab • 1/2/3: jump • r: refresh • u: update • shift+u: uninstall • y/n: confirm"
	if m.activeTab == tabDashboard {
		help = "up/down: table • s: start daemon • x: stop daemon • space: pause/resume • tab: switch • q: quit"
	}
	if m.activeTab == tabTools {
		help = "p: pomodoro • s: stop pomodoro • a: add limit • enter: edit • del: remove • left/right: switch field"
	}
	if m.activeTab == tabSystem {
		help = "f: switch pane • enter: apply/toggle • a: add entry • del: remove entry • u: update • shift+u: uninstall"
	}
	return footerBoxStyle.Width(m.width).Render(help)
}
