package tui

import (
	"focusd/core"
	"focusd/storage"
	"focusd/system"
	"fmt"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"time"
)

type sessionState int

const (
	dashboardView sessionState = iota
	toolsView
	settingsView
)

type mainModel struct {
	state          sessionState
	width          int
	height         int
	ready          bool
	appTable       table.Model
	pomodoroIn     textinput.Model
	settingsList   list.Model
	daemonActive   bool
	daemonPID      uint32
	totalTimeSecs  int
	timeLimitMins  int
	pomodoroActive bool
	pomodoroRem    int
	pomodoroTotal  int
	whitelist      []string
	browsers       []string
	appLimits      map[string]int
}

func NewModel() mainModel {
	ti := textinput.New()
	ti.Placeholder = "Minutes"
	ti.CharLimit = 3
	ti.Width = 10

	items := []list.Item{
		item{title: "Retention Policy", desc: "Manage how long data is stored"},
		item{title: "Autostart", desc: "Manage background daemon autostart"},
		item{title: "PATH Integration", desc: "Enable focusd command anywhere"},
		item{title: "Whitelist Apps", desc: "Apps that will not be tracked"},
		item{title: "Custom Browsers", desc: "Add non-standard browsers"},
	}
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Configuration"
	l.SetShowHelp(false)

	m := mainModel{
		state:        dashboardView,
		pomodoroIn:   ti,
		settingsList: l,
	}
	m.initTable()
	return m
}

func (m mainModel) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.fetchInitialData(),
	)
}

func (m mainModel) fetchInitialData() tea.Cmd {
	return func() tea.Msg {
		today := time.Now().Format("2006-01-02")
		stats, _ := storage.GetAppStatsForDate(today)
		
		pid, _ := system.GetPIDByName(system.DaemonProcessName)
		isRunning := system.GetProcessCount(system.DaemonProcessName) > 1
		
		pActive, pRem, pTotal := core.GetPomodoroStatus()
		
		return fullDataMsg{
			stats:          stats,
			daemonActive:   isRunning,
			daemonPID:      pid,
			totalTimeSecs:  storage.GetTotalScreenTimeTodaySecs(),
			pomodoroActive: pActive,
			pomodoroRem:    int(pRem.Minutes()),
			pomodoroTotal:  pTotal,
			whitelist:      system.GetWhitelistApps(),
			browsers:       storage.GetCustomBrowsersList(),
			appLimits:      system.GetAppTimeLimits(),
		}
	}
}

type fullDataMsg struct {
	stats          []storage.AppDailyStat
	daemonActive   bool
	daemonPID      uint32
	totalTimeSecs  int
	pomodoroActive bool
	pomodoroRem    int
	pomodoroTotal  int
	whitelist      []string
	browsers       []string
	appLimits      map[string]int
}

type item struct {
	title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type errorMsg error
