package tui

import (
	"errors"
	"fmt"
	"focusd/core"
	"focusd/storage"
	"focusd/system"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type tabID int

const (
	tabDashboard tabID = iota
	tabTools
	tabSystem
)

type appSnapshot struct {
	date            string
	appStats        []storage.AppDailyStat
	totalScreenSecs int
	daemonRunning   bool
	daemonPID       uint32
	daemonMemory    string
	daemonUptime    string
	trackingPaused  bool
	pomodoroActive  bool
	pomodoroRemSecs int
	pomodoroTotal   int
	retentionDays   int
	autostartOn     bool
	pathOn          bool
	whitelistApps   []string
	customBrowsers  []string
	appLimits       map[string]int
}

type tickMsg time.Time

type snapshotMsg struct {
	state   appSnapshot
	warning string
	err     error
}

type opDoneMsg struct {
	detail string
	err    error
}

type externalDoneMsg struct {
	name string
	err  error
}

type mainModel struct {
	activeTab  tabID
	width      int
	height     int
	ready      bool
	repo       storage.Repository
	data       appSnapshot
	notice     string
	errText    string
	pending    string
	dashboard  dashboardModel
	tools      toolsModel
	systemPane systemModel
}

func NewModel() mainModel {
	repo := storage.GetRepository()
	return mainModel{
		activeTab:  tabDashboard,
		repo:       repo,
		dashboard:  newDashboardModel(),
		tools:      newToolsModel(),
		systemPane: newSystemModel(),
	}
}

func (m mainModel) Init() tea.Cmd {
	return tea.Batch(fetchSnapshotCmd(m.repo), tickCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func fetchSnapshotCmd(repo storage.Repository) tea.Cmd {
	return func() tea.Msg {
		date := time.Now().Format("2006-01-02")
		stats, warning, err := getAppStatsWithRetry(repo, date)
		if err != nil {
			return snapshotMsg{err: err}
		}
		pid, _ := system.GetPIDByName(system.DaemonProcessName)
		running := system.GetProcessCount(system.DaemonProcessName) > 1
		mem := "Unavailable"
		uptime := "Unavailable"
		if running && pid > 0 {
			mem, uptime = readDaemonVitals(pid)
		}
		pomActive, pomRem, pomTotal := core.GetPomodoroStatus()
		autoOn, _, _ := system.GetAutoStartEnabled()
		pathOn, _ := system.GetPathEnabled()
		limitMap := make(map[string]int)
		for app, mins := range system.GetAppTimeLimits() {
			limitMap[app] = mins
		}
		white := append([]string{}, system.GetWhitelistApps()...)
		browsers := append([]string{}, storage.GetCustomBrowsersList()...)
		sort.Strings(white)
		sort.Strings(browsers)
		state := appSnapshot{
			date:            date,
			appStats:        stats,
			totalScreenSecs: storage.GetTotalScreenTimeTodaySecs(),
			daemonRunning:   running,
			daemonPID:       pid,
			daemonMemory:    mem,
			daemonUptime:    uptime,
			trackingPaused:  storage.IsPaused(),
			pomodoroActive:  pomActive,
			pomodoroRemSecs: int(pomRem.Seconds()),
			pomodoroTotal:   pomTotal,
			retentionDays:   storage.GetRetentionDays(),
			autostartOn:     autoOn,
			pathOn:          pathOn,
			whitelistApps:   white,
			customBrowsers:  browsers,
			appLimits:       limitMap,
		}
		return snapshotMsg{state: state, warning: warning}
	}
}

func getAppStatsWithRetry(repo storage.Repository, date string) ([]storage.AppDailyStat, string, error) {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		stats, err := repo.GetAppStatsForDate(date)
		if err == nil {
			return stats, "", nil
		}
		lastErr = err
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "locked") || strings.Contains(msg, "busy") {
			time.Sleep(time.Duration(90*(attempt+1)) * time.Millisecond)
			continue
		}
		return nil, "", err
	}
	if lastErr != nil {
		msg := strings.ToLower(lastErr.Error())
		if strings.Contains(msg, "locked") || strings.Contains(msg, "busy") {
			return []storage.AppDailyStat{}, "Database temporarily busy, values may lag briefly", nil
		}
	}
	return nil, "", lastErr
}

func readDaemonVitals(pid uint32) (string, string) {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", fmt.Sprintf("$p=Get-Process -Id %d -ErrorAction SilentlyContinue; if ($null -eq $p) {''} else {$mins=[math]::Round(((Get-Date)-$p.StartTime).TotalMinutes); $mb=[math]::Round($p.WorkingSet64/1MB); Write-Output (\"$mb MB|$mins min\")}", pid))
	out, err := cmd.Output()
	if err != nil {
		return "Unavailable", "Unavailable"
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "|")
	if len(parts) != 2 {
		return "Unavailable", "Unavailable"
	}
	mem := strings.TrimSpace(parts[0])
	uptime := strings.TrimSpace(parts[1])
	if mem == "" {
		mem = "Unavailable"
	}
	if uptime == "" {
		uptime = "Unavailable"
	}
	return mem, uptime
}

func startDaemonCmd() tea.Cmd {
	return func() tea.Msg {
		if system.GetProcessCount(system.DaemonProcessName) > 1 {
			return opDoneMsg{detail: "Daemon already running"}
		}
		pid, err := system.StartDaemon()
		if err != nil {
			return opDoneMsg{err: err}
		}
		return opDoneMsg{detail: fmt.Sprintf("Daemon started (PID %d)", pid)}
	}
}

func stopDaemonCmd() tea.Cmd {
	return func() tea.Msg {
		if system.GetProcessCount(system.DaemonProcessName) <= 1 {
			return opDoneMsg{detail: "Daemon already stopped"}
		}
		err := system.KillOtherInstances(system.DaemonProcessName)
		if err != nil {
			return opDoneMsg{err: err}
		}
		return opDoneMsg{detail: "Daemon stopped"}
	}
}

func toggleTrackingCmd() tea.Cmd {
	return func() tea.Msg {
		err := storage.SetPaused(!storage.IsPaused())
		if err != nil {
			return opDoneMsg{err: err}
		}
		if storage.IsPaused() {
			return opDoneMsg{detail: "Tracking paused"}
		}
		return opDoneMsg{detail: "Tracking resumed"}
	}
}

func startPomodoroCmd(minutes int) tea.Cmd {
	return func() tea.Msg {
		if minutes <= 0 {
			return opDoneMsg{err: errors.New("minutes must be greater than zero")}
		}
		err := core.StartPomodoro(minutes)
		if err != nil {
			return opDoneMsg{err: err}
		}
		return opDoneMsg{detail: fmt.Sprintf("Pomodoro started for %d minutes", minutes)}
	}
}

func stopPomodoroCmd() tea.Cmd {
	return func() tea.Msg {
		err := core.StopPomodoro()
		if err != nil {
			return opDoneMsg{err: err}
		}
		return opDoneMsg{detail: "Pomodoro stopped"}
	}
}

func setLimitCmd(app string, minutes int) tea.Cmd {
	return func() tea.Msg {
		normalized := strings.TrimSpace(strings.ToLower(app))
		if normalized == "" {
			return opDoneMsg{err: errors.New("application name is required")}
		}
		err := system.SetAppTimeLimit(normalized, minutes)
		if err != nil {
			return opDoneMsg{err: err}
		}
		if minutes > 0 {
			return opDoneMsg{detail: fmt.Sprintf("Updated limit: %s -> %d min", normalized, minutes)}
		}
		return opDoneMsg{detail: fmt.Sprintf("Removed limit: %s", normalized)}
	}
}

func setRetentionCmd(days int) tea.Cmd {
	return func() tea.Msg {
		if days < 1 {
			return opDoneMsg{err: errors.New("retention must be at least 1 day")}
		}
		err := storage.SetRetentionDays(days)
		if err != nil {
			return opDoneMsg{err: err}
		}
		return opDoneMsg{detail: fmt.Sprintf("Retention set to %d days", days)}
	}
}

func toggleAutostartCmd(enabled bool) tea.Cmd {
	return func() tea.Msg {
		var err error
		if enabled {
			err = system.DisableAutoStart()
		} else {
			err = system.EnableAutoStart()
		}
		if err != nil {
			return opDoneMsg{err: err}
		}
		if enabled {
			return opDoneMsg{detail: "Autostart disabled"}
		}
		return opDoneMsg{detail: "Autostart enabled"}
	}
}

func togglePathCmd(enabled bool) tea.Cmd {
	return func() tea.Msg {
		var err error
		if enabled {
			err = system.DisablePath()
		} else {
			err = system.EnablePath()
		}
		if err != nil {
			return opDoneMsg{err: err}
		}
		if enabled {
			return opDoneMsg{detail: "PATH integration disabled"}
		}
		return opDoneMsg{detail: "PATH integration enabled"}
	}
}

func addWhitelistCmd(app string) tea.Cmd {
	return func() tea.Msg {
		err := system.AddWhitelistApp(app)
		if err != nil {
			return opDoneMsg{err: err}
		}
		return opDoneMsg{detail: fmt.Sprintf("Added whitelist app: %s", app)}
	}
}

func removeWhitelistCmd(app string) tea.Cmd {
	return func() tea.Msg {
		err := system.RemoveWhitelistApp(app)
		if err != nil {
			return opDoneMsg{err: err}
		}
		return opDoneMsg{detail: fmt.Sprintf("Removed whitelist app: %s", app)}
	}
}

func addBrowserCmd(app string) tea.Cmd {
	return func() tea.Msg {
		err := storage.AddCustomBrowser(app)
		if err != nil {
			return opDoneMsg{err: err}
		}
		return opDoneMsg{detail: fmt.Sprintf("Added custom browser: %s", app)}
	}
}

func removeBrowserCmd(app string) tea.Cmd {
	return func() tea.Msg {
		err := storage.RemoveCustomBrowser(app)
		if err != nil {
			return opDoneMsg{err: err}
		}
		return opDoneMsg{detail: fmt.Sprintf("Removed custom browser: %s", app)}
	}
}

func runExternalCmd(name string, args ...string) tea.Cmd {
	cmd := exec.Command(os.Args[0], args...)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return externalDoneMsg{name: name, err: err}
	})
}

func parsePositiveInt(raw string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, err
	}
	if n <= 0 {
		return 0, errors.New("value must be greater than zero")
	}
	return n, nil
}
