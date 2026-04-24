package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type settingsMode int

const (
	settingsIdle settingsMode = iota
	settingsInput
)

type categoryItem struct {
	title string
	desc  string
}

func (i categoryItem) Title() string       { return i.title }
func (i categoryItem) Description() string { return i.desc }
func (i categoryItem) FilterValue() string { return i.title }

type valueItem struct {
	name string
}

func (i valueItem) Title() string       { return i.name }
func (i valueItem) Description() string { return "" }
func (i valueItem) FilterValue() string { return i.name }

type systemModel struct {
	width        int
	height       int
	focusPane    int
	mode         settingsMode
	inputPurpose string
	categoryList list.Model
	valueList    list.Model
	input        textinput.Model
	state        appSnapshot
}

func newSystemModel() systemModel {
	categoryItems := []list.Item{
		categoryItem{title: "Retention Policy", desc: "Data lifespan in days"},
		categoryItem{title: "Autostart", desc: "Background startup behavior"},
		categoryItem{title: "PATH Integration", desc: "Terminal command availability"},
		categoryItem{title: "Whitelist Apps", desc: "Apps excluded from tracking"},
		categoryItem{title: "Custom Browsers", desc: "Additional browser executables"},
	}
	left := list.New(categoryItems, list.NewDefaultDelegate(), 20, 10)
	left.Title = "System & Configuration"
	left.SetFilteringEnabled(false)
	left.SetShowHelp(false)
	left.SetShowStatusBar(false)

	right := list.New([]list.Item{}, list.NewDefaultDelegate(), 20, 10)
	right.Title = "Details"
	right.SetFilteringEnabled(false)
	right.SetShowHelp(false)
	right.SetShowStatusBar(false)

	input := textinput.New()
	input.CharLimit = 128
	input.Width = 28

	return systemModel{categoryList: left, valueList: right, input: input}
}

func (s *systemModel) SetSize(width, height int) {
	s.width = width
	s.height = height
	leftW := width / 3
	if leftW < 22 {
		leftW = 22
	}
	if leftW > width-20 {
		leftW = width - 20
	}
	rightW := width - leftW - 2
	if rightW < 16 {
		rightW = 16
	}
	categoryWidth := leftW - 4
	if categoryWidth < 14 {
		categoryWidth = 14
	}
	valueWidth := rightW - 4
	if valueWidth < 12 {
		valueWidth = 12
	}
	s.categoryList.SetSize(categoryWidth, maxInt(8, height-4))
	s.valueList.SetSize(valueWidth, maxInt(8, height-11))
}

func (s *systemModel) SetData(state appSnapshot) {
	s.state = state
	s.refreshValueList()
}

func (s *systemModel) refreshValueList() {
	category := s.selectedCategory()
	items := make([]list.Item, 0)
	if category == "Whitelist Apps" {
		for _, entry := range s.state.whitelistApps {
			items = append(items, valueItem{name: entry})
		}
	}
	if category == "Custom Browsers" {
		for _, entry := range s.state.customBrowsers {
			items = append(items, valueItem{name: entry})
		}
	}
	if len(items) == 0 {
		items = []list.Item{valueItem{name: "No entries"}}
	}
	s.valueList.SetItems(items)
}

func (s *systemModel) selectedCategory() string {
	item, ok := s.categoryList.SelectedItem().(categoryItem)
	if !ok {
		return "Retention Policy"
	}
	return item.title
}

func (s *systemModel) HandleKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	if s.mode == settingsInput {
		switch msg.String() {
		case "esc":
			s.mode = settingsIdle
			s.input.Blur()
			return nil, true
		case "enter":
			cmd := s.confirmInput()
			s.mode = settingsIdle
			s.input.Blur()
			s.input.SetValue("")
			return cmd, true
		}
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)
		return cmd, true
	}

	switch msg.String() {
	case "f":
		s.focusPane = (s.focusPane + 1) % 2
		return nil, true
	case "a":
		if s.selectedCategory() == "Whitelist Apps" {
			s.mode = settingsInput
			s.inputPurpose = "whitelist_add"
			s.input.Placeholder = "app.exe"
			s.input.SetValue("")
			s.input.Focus()
			return textinput.Blink, true
		}
		if s.selectedCategory() == "Custom Browsers" {
			s.mode = settingsInput
			s.inputPurpose = "browser_add"
			s.input.Placeholder = "browser.exe"
			s.input.SetValue("")
			s.input.Focus()
			return textinput.Blink, true
		}
	case "enter":
		category := s.selectedCategory()
		if category == "Retention Policy" {
			s.mode = settingsInput
			s.inputPurpose = "retention"
			s.input.Placeholder = "1-30"
			s.input.SetValue(fmt.Sprintf("%d", s.state.retentionDays))
			s.input.Focus()
			return textinput.Blink, true
		}
		if category == "Autostart" {
			return toggleAutostartCmd(s.state.autostartOn), true
		}
		if category == "PATH Integration" {
			return togglePathCmd(s.state.pathOn), true
		}
	case "del", "backspace":
		category := s.selectedCategory()
		selected, ok := s.valueList.SelectedItem().(valueItem)
		if !ok || strings.EqualFold(selected.name, "No entries") {
			return nil, true
		}
		if category == "Whitelist Apps" {
			return removeWhitelistCmd(selected.name), true
		}
		if category == "Custom Browsers" {
			return removeBrowserCmd(selected.name), true
		}
	}

	var cmd tea.Cmd
	if s.focusPane == 0 {
		s.categoryList, cmd = s.categoryList.Update(msg)
		s.refreshValueList()
	} else {
		s.valueList, cmd = s.valueList.Update(msg)
	}
	return cmd, true
}

func (s *systemModel) confirmInput() tea.Cmd {
	value := strings.TrimSpace(s.input.Value())
	if s.inputPurpose == "retention" {
		days, err := parsePositiveInt(value)
		if err != nil {
			return func() tea.Msg { return opDoneMsg{err: err} }
		}
		return setRetentionCmd(days)
	}
	if s.inputPurpose == "whitelist_add" {
		if value == "" {
			return func() tea.Msg { return opDoneMsg{err: fmt.Errorf("application name is required")} }
		}
		return addWhitelistCmd(value)
	}
	if s.inputPurpose == "browser_add" {
		if value == "" {
			return func() tea.Msg { return opDoneMsg{err: fmt.Errorf("browser name is required")} }
		}
		return addBrowserCmd(value)
	}
	return nil
}

func (s systemModel) View() string {
	leftW := s.width / 3
	if leftW < 22 {
		leftW = 22
	}
	if leftW > s.width-20 {
		leftW = s.width - 20
	}
	rightW := s.width - leftW - 1
	if rightW < 16 {
		rightW = 16
	}
	leftBox := cardStyle.Width(leftW).Height(s.height - 1).Render(s.categoryList.View())
	rightContent := s.contextView()
	rightBox := cardSoftStyle.Width(rightW).Height(s.height - 1).Render(rightContent)
	return lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)
}

func (s systemModel) contextView() string {
	category := s.selectedCategory()
	if s.mode == settingsInput {
		return lipgloss.JoinVertical(lipgloss.Left,
			sectionTitleStyle.Render(category),
			s.input.View(),
			"enter: confirm • esc: cancel",
		)
	}
	if category == "Retention Policy" {
		return lipgloss.JoinVertical(lipgloss.Left,
			sectionTitleStyle.Render("Retention Policy"),
			fmt.Sprintf("Current: %d days", s.state.retentionDays),
			"enter: set value",
		)
	}
	if category == "Autostart" {
		status := "Disabled"
		if s.state.autostartOn {
			status = "Enabled"
		}
		return lipgloss.JoinVertical(lipgloss.Left,
			sectionTitleStyle.Render("Autostart"),
			"Current: "+status,
			"enter: toggle",
		)
	}
	if category == "PATH Integration" {
		status := "Disabled"
		if s.state.pathOn {
			status = "Enabled"
		}
		return lipgloss.JoinVertical(lipgloss.Left,
			sectionTitleStyle.Render("PATH Integration"),
			"Current: "+status,
			"enter: toggle",
		)
	}
	if category == "Whitelist Apps" {
		return lipgloss.JoinVertical(lipgloss.Left,
			sectionTitleStyle.Render("Whitelist Apps"),
			s.valueList.View(),
			"a: add • del: remove",
		)
	}
	if category == "Custom Browsers" {
		return lipgloss.JoinVertical(lipgloss.Left,
			sectionTitleStyle.Render("Custom Browsers"),
			s.valueList.View(),
			"a: add • del: remove",
		)
	}
	return ""
}
