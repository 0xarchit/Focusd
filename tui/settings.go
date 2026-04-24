package tui

import (
	"fmt"
	"focusd/storage"
	"focusd/system"
	"strconv"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type settingsModel struct {
	width, height int
	menu          list.Model
	activePanel   int
	retentionDays int
	whitelist     list.Model
	browsers      list.Model
	input         textinput.Model
	adding        bool
	autoStart     bool
	pathEnabled   bool
	subSelection  int
}

type settingsItem struct {
	title string
	id    int
}

func (i settingsItem) Title() string       { return i.title }
func (i settingsItem) Description() string { return "" }
func (i settingsItem) FilterValue() string { return i.title }

type listItem struct {
	value string
}

func (i listItem) Title() string       { return i.value }
func (i listItem) Description() string { return "" }
func (i listItem) FilterValue() string { return i.value }

func newSettings() settingsModel {
	m := list.New([]list.Item{
		settingsItem{"1. Privacy/Retention", 0},
		settingsItem{"2. Whitelist", 1},
		settingsItem{"3. Custom Browsers", 2},
		settingsItem{"4. System Integration", 3},
	}, list.NewDefaultDelegate(), 0, 0)
	m.Title = "Settings"
	m.SetShowStatusBar(false)
	m.SetFilteringEnabled(false)
	m.Styles.Title = titleStyle

	w := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	w.SetShowHelp(false)
	w.SetShowTitle(false)
	w.SetShowStatusBar(false)

	b := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	b.SetShowHelp(false)
	b.SetShowTitle(false)
	b.SetShowStatusBar(false)

	ti := textinput.New()
	ti.Placeholder = "exe_name.exe"

	return settingsModel{
		menu:      m,
		whitelist: w,
		browsers:  b,
		input:     ti,
	}
}

func (m settingsModel) Init() tea.Cmd {
	return func() tea.Msg {
		daysStr, _ := storage.GetConfig("retention_days")
		days, _ := strconv.Atoi(daysStr)
		if days == 0 {
			days = 30
		}
		auto, _, _ := system.GetAutoStartEnabled()
		path, _ := system.GetPathEnabled()
		return settingsDataMsg{
			retention: days,
			autoStart: auto,
			path:      path,
		}
	}
}

type settingsDataMsg struct {
	retention int
	autoStart bool
	path      bool
}

func (m settingsModel) Update(msg tea.Msg) (settingsModel, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case settingsDataMsg:
		m.retentionDays = msg.retention
		m.autoStart = msg.autoStart
		m.pathEnabled = msg.path
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.menu.SetSize(int(float64(msg.Width)*0.3), msg.Height-6)
		m.whitelist.SetSize(int(float64(msg.Width)*0.6), msg.Height-10)
		m.browsers.SetSize(int(float64(msg.Width)*0.6), msg.Height-10)
	case tea.KeyMsg:
		if m.adding {
			switch msg.String() {
			case "esc":
				m.adding = false
			case "enter":
				val := m.input.Value()
				switch m.activePanel {
				case 1:
					system.AddWhitelistApp(val)
				case 2:
					storage.AddCustomBrowser(val)
				}
				m.adding = false
				m.input.Reset()
			}
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "up", "down":
			if m.activePanel == 3 {
				if msg.String() == "up" {
					m.subSelection = (m.subSelection + 1) % 2
				} else {
					m.subSelection = (m.subSelection + 1) % 2
				}
				return m, nil
			}
			m.menu, cmd = m.menu.Update(msg)
			if item, ok := m.menu.SelectedItem().(settingsItem); ok {
				m.activePanel = item.id
			}
			return m, cmd
		case "left", "right":
			if m.activePanel == 0 {
				if msg.String() == "left" && m.retentionDays > 1 {
					m.retentionDays--
				} else if msg.String() == "right" {
					m.retentionDays++
				}
				storage.SetConfig("retention_days", strconv.Itoa(m.retentionDays))
			}
		case "a":
			if m.activePanel == 1 || m.activePanel == 2 {
				m.adding = true
				m.input.Focus()
			}
		case "delete", "backspace":
			switch m.activePanel {
			case 1:
				if item, ok := m.whitelist.SelectedItem().(listItem); ok {
					system.RemoveWhitelistApp(item.value)
				}
			case 2:
				if item, ok := m.browsers.SelectedItem().(listItem); ok {
					storage.RemoveCustomBrowser(item.value)
				}
			}
		case "enter":
			if m.activePanel == 3 {
				if m.subSelection == 0 {
					if m.autoStart {
						system.DisableAutoStart()
					} else {
						system.EnableAutoStart()
					}
					m.autoStart = !m.autoStart
				} else {
					if m.pathEnabled {
						system.DisablePath()
					} else {
						system.EnablePath()
					}
					m.pathEnabled = !m.pathEnabled
				}
			}
		}
	}

	switch m.activePanel {
	case 1:
		apps := system.GetWhitelistApps()
		var items []list.Item
		for _, a := range apps {
			items = append(items, listItem{a})
		}
		m.whitelist.SetItems(items)
		m.whitelist, cmd = m.whitelist.Update(msg)
		cmds = append(cmds, cmd)
	case 2:
		browserList := storage.GetCustomBrowsersList()
		var items []list.Item
		for _, b := range browserList {
			items = append(items, listItem{b})
		}
		m.browsers.SetItems(items)
		m.browsers, cmd = m.browsers.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m settingsModel) View() string {
	leftCol := boxStyle.Width(int(float64(m.width) * 0.3)).Render(m.menu.View())

	var rightView string
	switch m.activePanel {
	case 0:
		rightView = lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("Privacy & Retention"),
			fmt.Sprintf("Retention Days: %d", m.retentionDays),
			"\nUse ← / → to change.",
		)
	case 1:
		rightView = lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("Whitelist"),
			m.whitelist.View(),
			"\n'a' to add • 'del' to remove",
		)
		if m.adding {
			rightView += "\n\nAdd: " + m.input.View()
		}
	case 2:
		rightView = lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("Custom Browsers"),
			m.browsers.View(),
			"\n'a' to add • 'del' to remove",
		)
		if m.adding {
			rightView += "\n\nAdd: " + m.input.View()
		}
	case 3:
		autoStatus := "DISABLED"
		if m.autoStart {
			autoStatus = "ENABLED"
		}
		pathStatus := "DISABLED"
		if m.pathEnabled {
			pathStatus = "ENABLED"
		}

		autoLine := "  Auto-Start: " + autoStatus
		pathLine := "  PATH:       " + pathStatus

		if m.subSelection == 0 {
			autoLine = selectedRowStyle.Render("> Auto-Start: " + autoStatus)
		} else {
			pathLine = selectedRowStyle.Render("> PATH:       " + pathStatus)
		}

		rightView = lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("System Integration"),
			autoLine,
			pathLine,
			"\n↑ / ↓ to select • Enter to toggle",
		)
	}

	rightCol := boxStyle.Width(int(float64(m.width) * 0.65)).Render(rightView)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftCol, rightCol)
}
