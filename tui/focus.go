package tui

import (
	"fmt"
	"focusd/core"
	"focusd/system"
	"strconv"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type focusModel struct {
	width, height int
	pomoInput     textinput.Model
	limitList     list.Model
	addingLimit   bool
	addAppInput   textinput.Model
	addMinInput   textinput.Model
	inputStep     int
}

type limitItem struct {
	app     string
	minutes int
}

func (i limitItem) Title() string       { return i.app }
func (i limitItem) Description() string { return fmt.Sprintf("%d minutes/day", i.minutes) }
func (i limitItem) FilterValue() string { return i.app }

func newFocus() focusModel {
	pi := textinput.New()
	pi.Placeholder = "25"
	pi.CharLimit = 3
	pi.Width = 5

	ai := textinput.New()
	ai.Placeholder = "app.exe"
	
	mi := textinput.New()
	mi.Placeholder = "60"

	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "App Limits"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle

	return focusModel{
		pomoInput:   pi,
		limitList:   l,
		addAppInput: ai,
		addMinInput: mi,
	}
}

type limitsMsg []list.Item

func fetchLimits() tea.Cmd {
	return func() tea.Msg {
		limits := system.GetAppTimeLimits()
		var items []list.Item
		for app, min := range limits {
			items = append(items, limitItem{app: app, minutes: min})
		}
		return limitsMsg(items)
	}
}

func (m focusModel) Init() tea.Cmd {
	return fetchLimits()
}

func (m focusModel) Update(msg tea.Msg) (focusModel, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case limitsMsg:
		cmds = append(cmds, m.limitList.SetItems(msg))
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.limitList.SetSize(msg.Width-4, msg.Height/2)
	case tea.KeyMsg:
		if m.addingLimit {
			switch msg.String() {
			case "esc":
				m.addingLimit = false
				return m, nil
			case "enter":
				if m.inputStep == 0 {
					m.inputStep = 1
					m.addAppInput.Blur()
					m.addMinInput.Focus()
				} else {
					mins, _ := strconv.Atoi(m.addMinInput.Value())
					system.SetAppTimeLimit(m.addAppInput.Value(), mins)
					m.addingLimit = false
					m.inputStep = 0
					m.addAppInput.Reset()
					m.addMinInput.Reset()
					return m, fetchLimits()
				}
			}
			if m.inputStep == 0 {
				m.addAppInput, cmd = m.addAppInput.Update(msg)
			} else {
				m.addMinInput, cmd = m.addMinInput.Update(msg)
			}
			return m, cmd
		}

		switch msg.String() {
		case "a":
			m.addingLimit = true
			m.inputStep = 0
			m.addAppInput.Focus()
			return m, nil
		case "delete", "backspace":
			if item, ok := m.limitList.SelectedItem().(limitItem); ok {
				system.RemoveAppTimeLimit(item.app)
				return m, fetchLimits()
			}
		case "enter":
			if m.pomoInput.Focused() {
				mins, _ := strconv.Atoi(m.pomoInput.Value())
				core.StartPomodoro(mins)
				m.pomoInput.Blur()
			}
		}
	}

	if !m.addingLimit {
		m.pomoInput, cmd = m.pomoInput.Update(msg)
		cmds = append(cmds, cmd)
		m.limitList, cmd = m.limitList.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m focusModel) View() string {
	pomoView := boxStyle.Width(m.width - 4).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("Pomodoro Control"),
			lipgloss.JoinHorizontal(lipgloss.Center,
				"Minutes: ", m.pomoInput.View(),
			),
			"\n[ Press Enter to START ]",
		),
	)

	listView := lipgloss.NewStyle().Padding(1, 0).Render(m.limitList.View())

	if m.addingLimit {
		addBox := boxStyle.Width(m.width - 10).Render(
			lipgloss.JoinVertical(lipgloss.Left,
				titleStyle.Render("Add App Limit"),
				"App EXE: "+m.addAppInput.View(),
				"Minutes: "+m.addMinInput.View(),
				"\n(esc to cancel)",
			),
		)
		return lipgloss.JoinVertical(lipgloss.Left, pomoView, addBox)
	}

	return lipgloss.JoinVertical(lipgloss.Left, pomoView, listView)
}
