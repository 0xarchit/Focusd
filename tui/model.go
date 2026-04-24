package tui

import (
	"focusd/system"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type keyMap struct {
	Tab      key.Binding
	ShiftTab key.Binding
	Left     key.Binding
	Right    key.Binding
	Quit     key.Binding
}

var keys = keyMap{
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next tab"),
	),
	ShiftTab: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "prev tab"),
	),
	Left: key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("←", "prev tab"),
	),
	Right: key.NewBinding(
		key.WithKeys("right"),
		key.WithHelp("→", "next tab"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}

type Model struct {
	activeTab int
	width     int
	height    int
	dashboard dashboardModel
	focus     focusModel
	settings  settingsModel
	help      help.Model
	running   bool
}

func NewModel() Model {
	return Model{
		activeTab: 0,
		dashboard: newDashboard(),
		focus:     newFocus(),
		settings:  newSettings(),
		help:      help.New(),
	}
}

type statusMsg bool

func checkDaemon() tea.Cmd {
	return func() tea.Msg {
		count := system.GetProcessCount(system.DaemonProcessName)
		return statusMsg(count > 1)
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		checkDaemon(),
		m.dashboard.Init(),
		m.focus.Init(),
		m.settings.Init(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case statusMsg:
		m.running = bool(msg)
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Tab), key.Matches(msg, keys.Right):
			m.activeTab = (m.activeTab + 1) % 3
		case key.Matches(msg, keys.ShiftTab), key.Matches(msg, keys.Left):
			m.activeTab = (m.activeTab + 2) % 3
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.dashboard, cmd = m.dashboard.Update(msg)
		cmds = append(cmds, cmd)
		m.focus, cmd = m.focus.Update(msg)
		cmds = append(cmds, cmd)
		m.settings, cmd = m.settings.Update(msg)
		cmds = append(cmds, cmd)
	}

	switch m.activeTab {
	case 0:
		m.dashboard, cmd = m.dashboard.Update(msg)
	case 1:
		m.focus, cmd = m.focus.Update(msg)
	case 2:
		m.settings, cmd = m.settings.Update(msg)
	}
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.width < 80 || m.height < 24 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, "Terminal too small. Please expand.")
	}

	var header strings.Builder
	logo := logoStyle.Render("focusd")
	
	tabs := []string{"Dashboard", "Focus Tools", "Settings"}
	var renderedTabs []string
	for i, t := range tabs {
		if i == m.activeTab {
			renderedTabs = append(renderedTabs, activeTabStyle.Render("[ "+t+" ]"))
		} else {
			renderedTabs = append(renderedTabs, tabStyle.Render(t))
		}
	}
	
	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
	
	status := statusRunningStyle.Render("● RUNNING")
	if !m.running {
		status = statusStoppedStyle.Render("○ STOPPED")
	}

	headerContent := lipgloss.JoinHorizontal(lipgloss.Center,
		logo,
		lipgloss.PlaceHorizontal(m.width-lipgloss.Width(logo)-lipgloss.Width(status)-4, lipgloss.Center, tabBar),
		status,
	)
	
	header.WriteString(headerStyle.Width(m.width).Render(headerContent))

	var body string
	switch m.activeTab {
	case 0:
		body = m.dashboard.View()
	case 1:
		body = m.focus.View()
	case 2:
		body = m.settings.View()
	}

	viewport := mainViewportStyle.Height(m.height - 4).Width(m.width).Render(body)
	
	footer := footerStyle.Width(m.width).Render(m.help.View(keys))

	return lipgloss.JoinVertical(lipgloss.Left, header.String(), viewport, footer)
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Tab, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Tab, k.ShiftTab, k.Left, k.Right},
		{k.Quit},
	}
}
