package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type toolsMode int

const (
	toolsIdle toolsMode = iota
	toolsPomodoroInput
	toolsAddLimitInput
	toolsEditLimitInput
)

type limitItem struct {
	app     string
	minutes int
}

func (i limitItem) Title() string       { return i.app }
func (i limitItem) Description() string { return fmt.Sprintf("%d min/day", i.minutes) }
func (i limitItem) FilterValue() string { return i.app }

type toolsModel struct {
	width         int
	height        int
	mode          toolsMode
	limitsList    list.Model
	pomodoroInput textinput.Model
	appInput      textinput.Model
	minutesInput  textinput.Model
	inputField    int
	pomodoroOn    bool
	pomodoroLeft  int
	pomodoroTotal int
}

func newToolsModel() toolsModel {
	listModel := list.New([]list.Item{}, list.NewDefaultDelegate(), 10, 10)
	listModel.Title = "Active App Limits"
	listModel.SetShowStatusBar(false)
	listModel.SetFilteringEnabled(false)
	listModel.SetShowHelp(false)

	pomodoroInput := textinput.New()
	pomodoroInput.Placeholder = "minutes"
	pomodoroInput.CharLimit = 4
	pomodoroInput.Width = 12

	appInput := textinput.New()
	appInput.Placeholder = "example.exe"
	appInput.CharLimit = 64
	appInput.Width = 20

	minutesInput := textinput.New()
	minutesInput.Placeholder = "minutes"
	minutesInput.CharLimit = 4
	minutesInput.Width = 12

	return toolsModel{
		limitsList:    listModel,
		pomodoroInput: pomodoroInput,
		appInput:      appInput,
		minutesInput:  minutesInput,
	}
}

func (t *toolsModel) SetSize(width, height int) {
	t.width = width
	t.height = height
	t.limitsList.SetSize(maxInt(32, width/2), maxInt(8, height/2))
}

func (t *toolsModel) SetData(state appSnapshot) {
	t.pomodoroOn = state.pomodoroActive
	t.pomodoroLeft = state.pomodoroRemSecs
	t.pomodoroTotal = state.pomodoroTotal
	keys := make([]string, 0, len(state.appLimits))
	for app, mins := range state.appLimits {
		if mins > 0 {
			keys = append(keys, app)
		}
	}
	sort.Strings(keys)
	items := make([]list.Item, 0, len(keys))
	for _, app := range keys {
		items = append(items, limitItem{app: app, minutes: state.appLimits[app]})
	}
	if len(items) == 0 {
		items = []list.Item{limitItem{app: "No limits configured", minutes: 0}}
	}
	t.limitsList.SetItems(items)
}

func (t *toolsModel) HandleKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	if t.mode != toolsIdle {
		switch msg.String() {
		case "esc":
			t.resetInputs()
			return nil, true
		case "left", "right":
			if t.mode == toolsAddLimitInput {
				t.toggleInputField()
				return nil, true
			}
		case "enter":
			cmd := t.confirmInput()
			t.resetInputs()
			return cmd, true
		}
		var cmd tea.Cmd
		switch t.mode {
		case toolsPomodoroInput:
			t.pomodoroInput, cmd = t.pomodoroInput.Update(msg)
		case toolsAddLimitInput, toolsEditLimitInput:
			if t.inputField == 0 {
				t.appInput, cmd = t.appInput.Update(msg)
			} else {
				t.minutesInput, cmd = t.minutesInput.Update(msg)
			}
		}
		return cmd, true
	}

	switch msg.String() {
	case "p":
		t.mode = toolsPomodoroInput
		t.pomodoroInput.SetValue("25")
		t.pomodoroInput.Focus()
		return textinput.Blink, true
	case "s":
		return stopPomodoroCmd(), true
	case "a":
		t.mode = toolsAddLimitInput
		t.appInput.SetValue("")
		t.minutesInput.SetValue("")
		t.inputField = 0
		t.appInput.Focus()
		t.minutesInput.Blur()
		return textinput.Blink, true
	case "enter":
		selected, ok := t.limitsList.SelectedItem().(limitItem)
		if !ok || selected.minutes <= 0 {
			return nil, true
		}
		t.mode = toolsEditLimitInput
		t.appInput.SetValue(selected.app)
		t.minutesInput.SetValue(fmt.Sprintf("%d", selected.minutes))
		t.inputField = 1
		t.appInput.Blur()
		t.minutesInput.Focus()
		return textinput.Blink, true
	case "del", "backspace":
		selected, ok := t.limitsList.SelectedItem().(limitItem)
		if !ok || selected.minutes <= 0 {
			return nil, true
		}
		return setLimitCmd(selected.app, 0), true
	}

	var cmd tea.Cmd
	t.limitsList, cmd = t.limitsList.Update(msg)
	return cmd, true
}

func (t *toolsModel) toggleInputField() {
	if t.inputField == 0 {
		t.inputField = 1
		t.appInput.Blur()
		t.minutesInput.Focus()
		return
	}
	t.inputField = 0
	t.minutesInput.Blur()
	t.appInput.Focus()
}

func (t *toolsModel) confirmInput() tea.Cmd {
	switch t.mode {
	case toolsPomodoroInput:
		minutes, err := parsePositiveInt(t.pomodoroInput.Value())
		if err != nil {
			return func() tea.Msg { return opDoneMsg{err: err} }
		}
		return startPomodoroCmd(minutes)
	case toolsAddLimitInput:
		app := strings.TrimSpace(t.appInput.Value())
		minutes, err := parsePositiveInt(t.minutesInput.Value())
		if err != nil {
			return func() tea.Msg { return opDoneMsg{err: err} }
		}
		if app == "" {
			return func() tea.Msg { return opDoneMsg{err: fmt.Errorf("application name is required")} }
		}
		return setLimitCmd(app, minutes)
	case toolsEditLimitInput:
		app := strings.TrimSpace(t.appInput.Value())
		minutes, err := strconv.Atoi(strings.TrimSpace(t.minutesInput.Value()))
		if err != nil || minutes < 0 {
			return func() tea.Msg { return opDoneMsg{err: fmt.Errorf("invalid minutes value")} }
		}
		if app == "" {
			return func() tea.Msg { return opDoneMsg{err: fmt.Errorf("application name is required")} }
		}
		return setLimitCmd(app, minutes)
	}
	return nil
}

func (t *toolsModel) resetInputs() {
	t.mode = toolsIdle
	t.inputField = 0
	t.pomodoroInput.Blur()
	t.appInput.Blur()
	t.minutesInput.Blur()
}

func (t toolsModel) View() string {
	pStatus := "Inactive"
	if t.pomodoroOn {
		pStatus = "Active"
	}
	header := cardStyle.Width(maxInt(42, t.width/2)).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			sectionTitleStyle.Render("Focus Tools"),
			"Pomodoro: "+pStatus,
			"Remaining: "+fmtDuration(t.pomodoroLeft),
			fmt.Sprintf("Session: %d min", t.pomodoroTotal),
		),
	)
	modeBox := "p: start pomodoro • s: stop pomodoro • a: add limit • del: remove limit • enter: edit"
	if t.mode == toolsPomodoroInput {
		modeBox = "Set Pomodoro Minutes\n" + t.pomodoroInput.View() + "\nenter: confirm • esc: cancel"
	}
	if t.mode == toolsAddLimitInput {
		inputTitle := "Add App Limit"
		if t.inputField == 0 {
			inputTitle += " (app)"
		} else {
			inputTitle += " (minutes)"
		}
		modeBox = inputTitle + "\nApp: " + t.appInput.View() + "\nMinutes: " + t.minutesInput.View() + "\nenter: save • left/right: switch • esc: cancel"
	}
	if t.mode == toolsEditLimitInput {
		modeBox = "Edit App Limit\nApp: " + t.appInput.View() + "\nMinutes (0 removes): " + t.minutesInput.View() + "\nenter: save • esc: cancel"
	}
	inputPanel := cardSoftStyle.Width(maxInt(42, t.width/2)).Render(modeBox)
	limitsPanel := cardStyle.Width(maxInt(42, t.width/2)).Render(t.limitsList.View())
	content := lipgloss.JoinVertical(lipgloss.Left, header, inputPanel, limitsPanel)
	return lipgloss.Place(t.width, t.height, lipgloss.Center, lipgloss.Top, content)
}
