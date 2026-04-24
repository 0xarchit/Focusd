package tui

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"focusd/core"
	"focusd/storage"
	"focusd/system"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	tabDashboard = iota
	tabStats
	tabFocus
	tabLimits
	tabSettings
)

var tabNames = []string{"DASHBOARD", "STATS", "FOCUS", "LIMITS", "SETTINGS"}

type appUsage struct {
	Name     string
	Exe      string
	Duration int
	Opens    int
}

type dailyUsage struct {
	Date     string
	Label    string
	Duration int
	Today    bool
	Weekend  bool
}

type tuiData struct {
	Apps       []appUsage
	Browsers   []appUsage
	Days       []dailyUsage
	Hourly     [24]int
	Total      int
	ActiveApps int
	LimitsHit  int
	DBPath     string
	Err        error
}

type dashboardLoadedMsg tuiData
type statsLoadedMsg tuiData
type secondTickMsg time.Time
type splashDoneMsg struct{}
type statusMsg bool
type toastKind int

const (
	toastInfo toastKind = iota
	toastSuccess
	toastWarning
	toastError
)

type toast struct {
	Text      string
	Kind      toastKind
	CreatedAt time.Time
}

type limitForm struct {
	Visible bool
	Editing bool
	App     string
	Hours   int
	Minutes int
	Field   int
}

type modal struct {
	Active   bool
	Title    string
	Message  string
	Required string
	Input    string
	Danger   bool
	Error    string
}

type Model struct {
	width  int
	height int

	activeTab    int
	panelFocus   int
	showHelp     bool
	splash       bool
	splashFrame  int
	clock        time.Time
	daemonActive bool
	refreshing   bool

	dashboard            tuiData
	stats                tuiData
	statsRange           int
	statsSort            int
	statsAsc             bool
	statsCustomEditing   bool
	statsCustomFrom      string
	statsCustomTo        string
	statsCustomDraftFrom string
	statsCustomDraftTo   string
	statsCustomField     int
	statsUsageOffset     int
	statsBrowserOffset   int

	dashSelected            int
	statsSelected           int
	browserSelected         int
	limitsSelected          int
	settingsSelected        int
	settingsBrowserSelected int
	focusButton             int
	focusDuration           int
	focusBreak              int
	focusPaused             bool
	settingsAddingBrowser   bool
	settingsBrowserInput    string

	limitForm limitForm
	modal     modal
	toasts    []toast
}

func NewModel() Model {
	return Model{
		activeTab:        tabDashboard,
		splash:           true,
		clock:            time.Now(),
		focusDuration:    system.GetPomodoroMinutes(),
		focusBreak:       5,
		statsRange:       0,
		statsSort:        0,
		statsCustomFrom:  time.Now().Format("2006-01-02"),
		statsCustomTo:    time.Now().Format("2006-01-02"),
		settingsSelected: 0,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		secondTick(),
		splashDone(),
		checkDaemon(),
		loadDashboard(),
		loadStats(m.statsRange),
	)
}

func secondTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return secondTickMsg(t) })
}

func splashDone() tea.Cmd {
	return tea.Tick(1500*time.Millisecond, func(time.Time) tea.Msg { return splashDoneMsg{} })
}

func checkDaemon() tea.Cmd {
	return func() tea.Msg {
		return statusMsg(system.GetProcessCount(system.DaemonProcessName) > 1)
	}
}

func loadDashboard() tea.Cmd {
	return func() tea.Msg {
		today := storage.Today()
		return dashboardLoadedMsg(readTUIData(today, today, 7, true))
	}
}

func loadStats(rangeIndex int) tea.Cmd {
	return func() tea.Msg {
		start, end, historyDays := statsRangeDates(rangeIndex, "", "")
		return statsLoadedMsg(readTUIData(start, end, historyDays, false))
	}
}

func loadCustomStats(from, to string) tea.Cmd {
	return func() tea.Msg {
		start, end, historyDays := statsRangeDates(3, from, to)
		return statsLoadedMsg(readTUIData(start, end, historyDays, false))
	}
}

func statsRangeDates(rangeIndex int, customFrom, customTo string) (string, string, int) {
	today := storage.Today()
	now := time.Now()
	switch rangeIndex {
	case 1:
		return now.AddDate(0, 0, -6).Format("2006-01-02"), today, 7
	case 2:
		return now.AddDate(0, 0, -29).Format("2006-01-02"), today, 30
	case 3:
		if customFrom == "" {
			customFrom = today
		}
		if customTo == "" {
			customTo = customFrom
		}
		if customFrom > customTo {
			customFrom, customTo = customTo, customFrom
		}
		fromTime, err := time.Parse("2006-01-02", customFrom)
		toTime, toErr := time.Parse("2006-01-02", customTo)
		if err != nil || toErr != nil {
			return today, today, 1
		}
		days := int(toTime.Sub(fromTime).Hours()/24) + 1
		return customFrom, customTo, min(max(days, 1), 30)
	default:
		return today, today, 1
	}
}

func readTUIData(startDate, endDate string, historyDays int, includeHourly bool) tuiData {
	today := storage.Today()
	var data tuiData

	apps, err := aggregateAppStats(startDate, endDate)
	if err != nil {
		data.Err = err
	}
	for _, s := range apps {
		name := s.ExeName
		if name == "" {
			name = s.AppName
		}
		data.Apps = append(data.Apps, appUsage{Name: name, Exe: s.ExeName, Duration: s.TotalDurationSecs, Opens: s.OpenCount})
		data.Total += s.TotalDurationSecs
	}
	data.ActiveApps = len(data.Apps)

	browsers, _ := aggregateBrowserStats(startDate, endDate)
	for _, s := range browsers {
		data.Browsers = append(data.Browsers, appUsage{Name: s.AppName, Duration: s.TotalDurationSecs, Opens: s.OpenCount})
	}

	limits := system.GetAppTimeLimits()
	for app, mins := range limits {
		if storage.GetAppUsageTodayMinutes(app) >= mins {
			data.LimitsHit++
		}
	}

	if includeHourly {
		sessions, _, _ := storage.GetSessionsPaginated(500, 0, today, today)
		for _, s := range sessions {
			h := s.StartTime.Hour()
			if h >= 0 && h < 24 {
				data.Hourly[h] += s.DurationSecs
			}
		}
	}

	historyEnd, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		historyEnd = time.Now()
	}
	for i := historyDays - 1; i >= 0; i-- {
		d := historyEnd.AddDate(0, 0, -i)
		date := d.Format("2006-01-02")
		dayApps, _ := storage.GetAppStatsForDate(date)
		total := 0
		for _, s := range dayApps {
			total += s.TotalDurationSecs
		}
		data.Days = append(data.Days, dailyUsage{
			Date:     date,
			Label:    d.Format("Mon"),
			Duration: total,
			Today:    date == today,
			Weekend:  d.Weekday() == time.Saturday || d.Weekday() == time.Sunday,
		})
	}

	if dbPath, err := storage.GetDBPath(); err == nil {
		data.DBPath = dbPath
	}
	return data
}

func aggregateAppStats(startDate, endDate string) ([]storage.AppDailyStat, error) {
	stats, err := storage.GetAllAppStats()
	if err != nil {
		return nil, err
	}
	combined := map[string]storage.AppDailyStat{}
	for _, s := range stats {
		if s.Date < startDate || s.Date > endDate {
			continue
		}
		key := s.ExeName
		if key == "" {
			key = s.AppName
		}
		item := combined[key]
		if item.AppName == "" {
			item.AppName = s.AppName
			item.ExeName = s.ExeName
		}
		item.TotalDurationSecs += s.TotalDurationSecs
		item.OpenCount += s.OpenCount
		combined[key] = item
	}
	out := make([]storage.AppDailyStat, 0, len(combined))
	for _, item := range combined {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].TotalDurationSecs > out[j].TotalDurationSecs
	})
	return out, nil
}

func aggregateBrowserStats(startDate, endDate string) ([]storage.AppDailyStat, error) {
	stats, err := storage.GetAllBrowserStats()
	if err != nil {
		return nil, err
	}
	combined := map[string]storage.AppDailyStat{}
	for _, s := range stats {
		if s.Date < startDate || s.Date > endDate {
			continue
		}
		item := combined[s.AppName]
		item.AppName = s.AppName
		item.TotalDurationSecs += s.TotalDurationSecs
		item.OpenCount += s.OpenCount
		combined[s.AppName] = item
	}
	out := make([]storage.AppDailyStat, 0, len(combined))
	for _, item := range combined {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].TotalDurationSecs > out[j].TotalDurationSecs
	})
	return out, nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case splashDoneMsg:
		m.splash = false
	case secondTickMsg:
		m.clock = time.Time(msg)
		m.splashFrame++
		m.pruneToasts()
		cmds = append(cmds, secondTick(), checkDaemon())
		if m.clock.Second()%5 == 0 {
			m.refreshing = true
			cmds = append(cmds, loadDashboard())
		}
	case statusMsg:
		m.daemonActive = bool(msg)
	case dashboardLoadedMsg:
		m.dashboard = tuiData(msg)
		m.dashSelected = min(m.dashSelected, max(0, len(m.dashboard.Apps)-1))
		m.limitsSelected = min(m.limitsSelected, max(0, len(m.limitRows())-1))
		m.refreshing = false
	case statsLoadedMsg:
		m.stats = tuiData(msg)
		m.statsSelected = min(m.statsSelected, max(0, len(m.stats.Apps)-1))
		m.browserSelected = min(m.browserSelected, max(0, len(m.stats.Browsers)-1))
		m.statsUsageOffset = min(m.statsUsageOffset, max(0, len(m.stats.Apps)-1))
		m.statsBrowserOffset = min(m.statsBrowserOffset, max(0, len(m.stats.Browsers)-1))
		m.refreshing = false
	case tea.KeyMsg:
		if m.splash {
			if msg.String() == "ctrl+c" || msg.String() == "q" {
				return m, tea.Quit
			}
			return m, tea.Batch(cmds...)
		}
		next, cmd := m.handleKey(msg)
		m = next
		cmds = append(cmds, cmd)
	case tea.MouseMsg:
		m = m.handleMouse(msg)
	}
	return m, tea.Batch(cmds...)
}

func (m Model) handleMouse(msg tea.MouseMsg) Model {
	if m.splash || m.showHelp || m.modal.Active || m.activeTab != tabStats {
		return m
	}
	delta := 0
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		delta = -1
	case tea.MouseButtonWheelDown:
		delta = 1
	default:
		return m
	}
	panel := m.statsPanelAt(msg.X, msg.Y)
	switch panel {
	case 1:
		appsLen := len(m.sortedStatsApps())
		m.statsUsageOffset = clampScrollOffset(m.statsUsageOffset+delta, m.statsVisibleUsageRows(), appsLen)
		m.statsSelected = min(m.statsUsageOffset, max(0, appsLen-1))
		m.panelFocus = 1
	case 2:
		browserLen := len(m.stats.Browsers)
		m.statsBrowserOffset = clampScrollOffset(m.statsBrowserOffset+delta, m.statsVisibleBrowserRows(), browserLen)
		m.browserSelected = min(m.statsBrowserOffset, max(0, browserLen-1))
		m.panelFocus = 2
	case 3:
		m.panelFocus = 3
	}
	return m
}

func (m Model) statsVisibleUsageRows() int {
	inner := m.width - 2
	contentHeight := m.contentHeight()
	if inner >= 120 {
		return max(3, min(10, contentHeight-15))
	}
	if contentHeight < 30 {
		if m.panelFocus == 2 || m.panelFocus == 3 {
			return 0
		}
		return max(4, contentHeight-12)
	}
	return max(3, min(6, (contentHeight-15)/2))
}

func (m Model) statsVisibleBrowserRows() int {
	inner := m.width - 2
	contentHeight := m.contentHeight()
	if inner >= 120 {
		return max(3, min(10, contentHeight-15))
	}
	if contentHeight < 30 {
		if m.panelFocus != 2 {
			return 0
		}
		return max(4, contentHeight-12)
	}
	return max(3, min(6, (contentHeight-15)/2))
}

func (m Model) contentHeight() int {
	contentHeight := m.height - lipgloss.Height(m.renderHeader()) - lipgloss.Height(m.renderTabs()) - 2
	if contentHeight < 1 {
		return 1
	}
	return contentHeight
}

func (m Model) statsPanelAt(x, y int) int {
	if m.width <= 0 || m.height <= 0 {
		return -1
	}
	inner := m.width - 2
	if inner <= 0 {
		return -1
	}
	contentHeight := m.contentHeight()
	contentX := 1
	contentY := lipgloss.Height(m.renderHeader()) + lipgloss.Height(m.renderTabs()) + 1
	if x < contentX || x >= contentX+inner {
		return -1
	}
	if y < contentY {
		return -1
	}
	relY := y - contentY
	rangePanel := panel("TIME RANGE", "", inner, "\n"+m.renderRangeSelector(inner-4)+"\n", m.panelFocus == 0)
	rangeH := lipgloss.Height(rangePanel)
	if relY < rangeH {
		return 0
	}
	curY := rangeH
	if inner >= 120 {
		tableRows := max(3, min(10, contentHeight-15))
		leftW := (inner - 1) / 2
		usageH := lipgloss.Height(m.renderUsageBreakdown(leftW, tableRows))
		browserW := inner - leftW - 1
		browserH := lipgloss.Height(m.renderBrowserUsage(browserW, tableRows))
		tablesH := max(usageH, browserH)
		if relY < curY+tablesH {
			relX := x - contentX
			if relX < leftW {
				return 1
			}
			if relX > leftW {
				return 2
			}
			return -1
		}
		curY += tablesH
		dailyH := lipgloss.Height(m.renderDailyHistory(inner))
		if relY < curY+dailyH {
			return 3
		}
		return -1
	}
	if contentHeight < 30 {
		switch m.panelFocus {
		case 2:
			if relY >= curY {
				return 2
			}
		case 3:
			if relY >= curY {
				return 3
			}
		default:
			if relY >= curY {
				return 1
			}
		}
		return -1
	}
	tableRows := max(3, min(6, (contentHeight-15)/2))
	usageH := lipgloss.Height(m.renderUsageBreakdown(inner, tableRows))
	if relY < curY+usageH {
		return 1
	}
	curY += usageH
	browserH := lipgloss.Height(m.renderBrowserUsage(inner, tableRows))
	if relY < curY+browserH {
		return 2
	}
	curY += browserH
	dailyH := lipgloss.Height(m.renderDailyHistory(inner))
	if relY < curY+dailyH {
		return 3
	}
	return -1
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()
	if m.showHelp {
		switch key {
		case "?", "esc", "q":
			m.showHelp = false
		}
		return m, nil
	}
	if m.modal.Active {
		return m.handleModalKey(msg)
	}

	switch key {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "?":
		m.showHelp = true
		return m, nil
	case "1", "2", "3", "4", "5":
		n, _ := strconv.Atoi(key)
		m.activeTab = n - 1
		m.panelFocus = 0
		if m.activeTab == tabStats {
			m.refreshing = true
			return m, m.loadStatsCmd()
		}
		return m, nil
	case "tab":
		m.panelFocus = (m.panelFocus + 1) % m.panelCount()
		return m, nil
	case "shift+tab":
		m.panelFocus = (m.panelFocus + m.panelCount() - 1) % m.panelCount()
		return m, nil
	case "r":
		if m.activeTab == tabStats || m.activeTab == tabDashboard {
			m.refreshing = true
			return m, tea.Batch(m.loadStatsCmd(), loadDashboard())
		}
	}

	switch m.activeTab {
	case tabDashboard:
		return m.handleDashboardKey(key)
	case tabStats:
		return m.handleStatsKey(key)
	case tabFocus:
		return m.handleFocusKey(key)
	case tabLimits:
		return m.handleLimitsKey(key)
	case tabSettings:
		return m.handleSettingsKey(key)
	}
	return m, nil
}

func (m Model) loadStatsCmd() tea.Cmd {
	if m.statsRange == 3 {
		return loadCustomStats(m.statsCustomFrom, m.statsCustomTo)
	}
	return loadStats(m.statsRange)
}

func (m Model) handleModalKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()
	if m.modal.Title == "CUSTOM RANGE" {
		return m.handleCustomRangeModalKey(msg)
	}

	switch key {
	case "esc":
		m.modal = modal{}
	case "backspace":
		if len(m.modal.Input) > 0 {
			m.modal.Input = m.modal.Input[:len(m.modal.Input)-1]
		}
	case "enter":
		if m.modal.Required == "" || m.modal.Input == m.modal.Required {
			if m.modal.Title == "WIPE DATA" {
				if err := storage.ClearAllTrackingData(); err != nil {
					m.addToast("Failed to wipe data", toastError)
				} else {
					m.addToast("Tracking data wiped", toastSuccess)
				}
				m.modal = modal{}
				return m, loadDashboard()
			}
			m.modal = modal{}
		}
	default:
		if len(key) == 1 && len(m.modal.Input) < 32 {
			m.modal.Input += key
		}
	}
	return m, nil
}

func (m Model) handleCustomRangeModalKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()
	m.modal.Error = ""
	switch key {
	case "esc":
		m.modal = modal{}
		m.statsCustomEditing = false
	case "tab", "right", "l":
		m.statsCustomField = (m.statsCustomField + 1) % 2
	case "shift+tab", "left", "h":
		m.statsCustomField = (m.statsCustomField + 1) % 2
	case "backspace":
		if m.statsCustomField == 0 {
			m.statsCustomDraftFrom = removeDateChar(m.statsCustomDraftFrom)
		} else {
			m.statsCustomDraftTo = removeDateChar(m.statsCustomDraftTo)
		}
	case "enter":
		from, to, err := validateCustomRange(m.statsCustomDraftFrom, m.statsCustomDraftTo)
		if err != nil {
			m.modal.Error = err.Error()
			return m, nil
		}
		m.statsCustomFrom = from
		m.statsCustomTo = to
		m.statsCustomEditing = false
		m.modal = modal{}
		m.refreshing = true
		return m, m.loadStatsCmd()
	default:
		for _, r := range msg.Runes {
			if !isDateInputRune(r) {
				continue
			}
			if m.statsCustomField == 0 {
				m.statsCustomDraftFrom = appendDateChar(m.statsCustomDraftFrom, r)
			} else {
				m.statsCustomDraftTo = appendDateChar(m.statsCustomDraftTo, r)
			}
		}
	}
	return m, nil
}

func (m Model) handleDashboardKey(key string) (Model, tea.Cmd) {
	if m.panelFocus == 1 {
		switch key {
		case "j", "down":
			m.dashSelected = min(m.dashSelected+1, max(0, len(m.dashboard.Apps)-1))
		case "k", "up":
			m.dashSelected = max(0, m.dashSelected-1)
		}
	}
	return m, nil
}

func (m Model) handleStatsKey(key string) (Model, tea.Cmd) {
	switch key {
	case "h", "left":
		m.statsRange = (m.statsRange + 3) % 4
		m.refreshing = true
		return m, m.loadStatsCmd()
	case "l", "right":
		m.statsRange = (m.statsRange + 1) % 4
		m.refreshing = true
		return m, m.loadStatsCmd()
	case "enter":
		if m.panelFocus == 0 && m.statsRange == 3 {
			m.statsCustomDraftFrom = m.statsCustomFrom
			m.statsCustomDraftTo = m.statsCustomTo
			m.statsCustomField = 0
			m.statsCustomEditing = true
			m.modal = modal{
				Active:  true,
				Title:   "CUSTOM RANGE",
				Message: "Enter a date range for Stats.",
			}
			return m, nil
		}
	case "s":
		if m.panelFocus == 1 {
			m.statsSort = (m.statsSort + 1) % 3
			if m.statsSort == 0 || m.statsSort == 2 {
				m.statsAsc = false
			} else {
				m.statsAsc = !m.statsAsc
			}
		}
	case "j", "down":
		switch m.panelFocus {
		case 1:
			m.statsUsageOffset = min(m.statsUsageOffset+1, max(0, len(m.sortedStatsApps())-1))
			m.statsSelected = m.statsUsageOffset
		case 2:
			m.statsBrowserOffset = min(m.statsBrowserOffset+1, max(0, len(m.stats.Browsers)-1))
			m.browserSelected = m.statsBrowserOffset
		}
	case "k", "up":
		switch m.panelFocus {
		case 1:
			m.statsUsageOffset = max(0, m.statsUsageOffset-1)
			m.statsSelected = m.statsUsageOffset
		case 2:
			m.statsBrowserOffset = max(0, m.statsBrowserOffset-1)
			m.browserSelected = m.statsBrowserOffset
		}
	case "pgdown":
		switch m.panelFocus {
		case 1:
			m.statsUsageOffset = min(m.statsUsageOffset+5, max(0, len(m.sortedStatsApps())-1))
			m.statsSelected = m.statsUsageOffset
		case 2:
			m.statsBrowserOffset = min(m.statsBrowserOffset+5, max(0, len(m.stats.Browsers)-1))
			m.browserSelected = m.statsBrowserOffset
		}
	case "pgup":
		switch m.panelFocus {
		case 1:
			m.statsUsageOffset = max(0, m.statsUsageOffset-5)
			m.statsSelected = m.statsUsageOffset
		case 2:
			m.statsBrowserOffset = max(0, m.statsBrowserOffset-5)
			m.browserSelected = m.statsBrowserOffset
		}
	}
	return m, nil
}

func validDateInput(value string) bool {
	_, err := time.Parse("2006-01-02", value)
	return err == nil
}

func isDateInputRune(r rune) bool {
	return r == '-' || (r >= '0' && r <= '9')
}

func appendDateChar(current string, r rune) string {
	digits := dateDigits(current)
	if r >= '0' && r <= '9' && len(digits) < 8 {
		digits += string(r)
	}
	return formatDateDigits(digits)
}

func removeDateChar(current string) string {
	digits := dateDigits(current)
	if len(digits) == 0 {
		return ""
	}
	return formatDateDigits(digits[:len(digits)-1])
}

func dateDigits(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
		if b.Len() >= 8 {
			break
		}
	}
	return b.String()
}

func formatDateDigits(digits string) string {
	if len(digits) > 8 {
		digits = digits[:8]
	}
	switch {
	case len(digits) > 4:
		out := digits[:4] + "-" + digits[4:]
		if len(digits) > 6 {
			out = digits[:4] + "-" + digits[4:6] + "-" + digits[6:]
		}
		return out
	default:
		return digits
	}
}

func validateCustomRange(from, to string) (string, string, error) {
	from = formatDateDigits(dateDigits(from))
	to = formatDateDigits(dateDigits(to))
	if len(from) != 10 || len(to) != 10 {
		return "", "", fmt.Errorf("Use YYYY-MM-DD for both dates")
	}
	fromTime, err := time.Parse("2006-01-02", from)
	if err != nil || fromTime.Format("2006-01-02") != from {
		return "", "", fmt.Errorf("From date is invalid")
	}
	toTime, err := time.Parse("2006-01-02", to)
	if err != nil || toTime.Format("2006-01-02") != to {
		return "", "", fmt.Errorf("To date is invalid")
	}
	if fromTime.After(toTime) {
		return "", "", fmt.Errorf("From date must be before To date")
	}
	if toTime.After(time.Now()) {
		return "", "", fmt.Errorf("To date cannot be in the future")
	}
	if int(toTime.Sub(fromTime).Hours()/24)+1 > 366 {
		return "", "", fmt.Errorf("Range must be 366 days or less")
	}
	return from, to, nil
}

func (m Model) handleFocusKey(key string) (Model, tea.Cmd) {
	switch key {
	case "h", "left":
		m.focusButton = max(0, m.focusButton-1)
	case "l", "right":
		m.focusButton = min(3, m.focusButton+1)
	case "s":
		return m.startFocus()
	case "p":
		m.focusPaused = !m.focusPaused
	case "x":
		core.StopPomodoro()
		m.focusPaused = false
		m.addToast("Focus timer stopped", toastWarning)
	case "r":
		core.StopPomodoro()
		m.focusPaused = false
		m.addToast("Focus timer reset", toastInfo)
	case "enter":
		switch m.focusButton {
		case 0:
			return m.startFocus()
		case 1:
			m.focusPaused = !m.focusPaused
		case 2:
			core.StopPomodoro()
			m.addToast("Focus timer stopped", toastWarning)
		case 3:
			core.StopPomodoro()
			m.addToast("Focus timer reset", toastInfo)
		}
	case "up":
		m.focusDuration = min(180, m.focusDuration+1)
	case "down":
		m.focusDuration = max(1, m.focusDuration-1)
	}
	return m, nil
}

func (m Model) startFocus() (Model, tea.Cmd) {
	if err := core.StartPomodoro(m.focusDuration); err != nil {
		m.addToast("Could not start focus timer", toastError)
	} else {
		system.SetPomodoroMinutes(m.focusDuration)
		m.focusPaused = false
		m.addToast("Focus timer started", toastSuccess)
	}
	return m, nil
}

func (m Model) handleLimitsKey(key string) (Model, tea.Cmd) {
	if m.limitForm.Visible {
		switch key {
		case "esc":
			m.limitForm = limitForm{}
		case "tab":
			m.limitForm.Field = (m.limitForm.Field + 1) % 4
		case "shift+tab":
			m.limitForm.Field = (m.limitForm.Field + 3) % 4
		case "left":
			m.limitForm.Field = max(0, m.limitForm.Field-1)
		case "right":
			m.limitForm.Field = min(3, m.limitForm.Field+1)
		case "up":
			switch m.limitForm.Field {
			case 1:
				m.limitForm.Hours = min(24, m.limitForm.Hours+1)
			case 2:
				m.limitForm.Minutes = min(59, m.limitForm.Minutes+1)
			}
		case "down":
			switch m.limitForm.Field {
			case 1:
				m.limitForm.Hours = max(0, m.limitForm.Hours-1)
			case 2:
				m.limitForm.Minutes = max(0, m.limitForm.Minutes-1)
			}
		case "backspace":
			if m.limitForm.Field == 0 && len(m.limitForm.App) > 0 {
				m.limitForm.App = m.limitForm.App[:len(m.limitForm.App)-1]
			}
		case "enter":
			if m.limitForm.Field == 3 {
				mins := m.limitForm.Hours*60 + m.limitForm.Minutes
				if m.limitForm.App != "" && mins > 0 {
					system.SetAppTimeLimit(m.limitForm.App, mins)
					m.addToast("Limit saved for "+m.limitForm.App, toastSuccess)
					m.limitForm = limitForm{}
					return m, loadDashboard()
				}
				m.addToast("App and limit are required", toastError)
			}
		default:
			if len(key) == 1 {
				if m.limitForm.Field == 0 && len(m.limitForm.App) < 32 {
					m.limitForm.App += key
				} else if m.limitForm.Field == 1 || m.limitForm.Field == 2 {
					n, err := strconv.Atoi(key)
					if err == nil {
						if m.limitForm.Field == 1 {
							m.limitForm.Hours = (m.limitForm.Hours*10 + n) % 25
						} else {
							m.limitForm.Minutes = (m.limitForm.Minutes*10 + n) % 60
						}
					}
				}
			}
		}
		return m, nil
	}

	switch key {
	case "j", "down":
		m.limitsSelected = min(m.limitsSelected+1, max(0, len(m.limitRows())-1))
	case "k", "up":
		m.limitsSelected = max(0, m.limitsSelected-1)
	case "n":
		m.limitForm = limitForm{Visible: true}
	case "e":
		rows := m.limitRows()
		if len(rows) > 0 {
			row := rows[m.limitsSelected]
			m.limitForm = limitForm{Visible: true, Editing: true, App: row.Name, Hours: row.Opens / 60, Minutes: row.Opens % 60}
		}
	case "d":
		rows := m.limitRows()
		if len(rows) > 0 {
			system.RemoveAppTimeLimit(rows[m.limitsSelected].Name)
			m.addToast("Limit deleted for "+rows[m.limitsSelected].Name, toastWarning)
			return m, loadDashboard()
		}
	}
	return m, nil
}

func (m Model) handleSettingsKey(key string) (Model, tea.Cmd) {
	if m.settingsAddingBrowser {
		switch key {
		case "esc":
			m.settingsAddingBrowser = false
			m.settingsBrowserInput = ""
		case "backspace":
			if len(m.settingsBrowserInput) > 0 {
				m.settingsBrowserInput = m.settingsBrowserInput[:len(m.settingsBrowserInput)-1]
			}
		case "enter":
			if strings.TrimSpace(m.settingsBrowserInput) == "" {
				m.addToast("Browser name is required", toastError)
				return m, nil
			}
			if err := storage.AddCustomBrowser(m.settingsBrowserInput); err != nil {
				m.addToast(err.Error(), toastError)
			} else {
				m.addToast("Added "+m.settingsBrowserInput, toastSuccess)
			}
			m.settingsAddingBrowser = false
			m.settingsBrowserInput = ""
		default:
			if len(key) == 1 && len(m.settingsBrowserInput) < 32 {
				m.settingsBrowserInput += key
			}
		}
		return m, nil
	}

	switch key {
	case "j", "down":
		m.settingsSelected = min(m.settingsSelected+1, 12)
	case "k", "up":
		m.settingsSelected = max(0, m.settingsSelected-1)
	case "h", "left":
		switch m.settingsSelected {
		case 8:
			m.settingsBrowserSelected = max(0, m.settingsBrowserSelected-1)
		case 4:
			current := storage.GetTrackingIntervalSeconds()
			next := max(storage.MinTrackingIntervalSeconds, current-1)
			if next != current {
				storage.SetTrackingIntervalSeconds(next)
				m.addToast(fmt.Sprintf("Tracking interval: %ds", next), toastSuccess)
			}
		case 5:
			current := storage.GetIdleThresholdSeconds()
			next := max(storage.MinIdleThresholdSeconds, current-5)
			if next != current {
				storage.SetIdleThresholdSeconds(next)
				m.addToast(fmt.Sprintf("Idle threshold: %ds", next), toastSuccess)
			}
		case 7:
			current := storage.GetWarningThresholdPercent()
			next := max(storage.MinWarningThresholdPercent, current-5)
			if next != current {
				storage.SetWarningThresholdPercent(next)
				m.addToast(fmt.Sprintf("Warning threshold: %d%%", next), toastSuccess)
			}
		}
	case "l", "right":
		switch m.settingsSelected {
		case 8:
			browsers := storage.GetCustomBrowsersList()
			m.settingsBrowserSelected = min(m.settingsBrowserSelected+1, max(0, len(browsers)-1))
		case 4:
			current := storage.GetTrackingIntervalSeconds()
			next := min(storage.MaxTrackingIntervalSeconds, current+1)
			if next != current {
				storage.SetTrackingIntervalSeconds(next)
				m.addToast(fmt.Sprintf("Tracking interval: %ds", next), toastSuccess)
			}
		case 5:
			current := storage.GetIdleThresholdSeconds()
			next := min(storage.MaxIdleThresholdSeconds, current+5)
			if next != current {
				storage.SetIdleThresholdSeconds(next)
				m.addToast(fmt.Sprintf("Idle threshold: %ds", next), toastSuccess)
			}
		case 7:
			current := storage.GetWarningThresholdPercent()
			next := min(storage.MaxWarningThresholdPercent, current+5)
			if next != current {
				storage.SetWarningThresholdPercent(next)
				m.addToast(fmt.Sprintf("Warning threshold: %d%%", next), toastSuccess)
			}
		}
	case "space", "enter":
		switch m.settingsSelected {
		case 0:
			if m.daemonActive {
				exec.Command("cmd", "/c", "taskkill", "/IM", system.DaemonProcessName, "/F").Start()
				m.addToast("Daemon stop requested", toastWarning)
			} else {
				m.addToast("Run focusd start to launch daemon", toastInfo)
			}
		case 1:
			enabled, _, _ := system.GetAutoStartEnabled()
			if enabled {
				system.DisableAutoStart()
				m.addToast("Auto-start disabled", toastSuccess)
			} else {
				system.EnableAutoStart()
				m.addToast("Auto-start enabled", toastSuccess)
			}
		case 2:
			paused := storage.IsPaused()
			storage.SetPaused(!paused)
			m.addToast("Browser tracking toggled", toastSuccess)
		case 3:
			m.addToast("Smart grouping toggled", toastInfo)
		case 4:
			current := storage.GetTrackingIntervalSeconds()
			next := min(storage.MaxTrackingIntervalSeconds, current+1)
			if next != current {
				storage.SetTrackingIntervalSeconds(next)
			}
			m.addToast(fmt.Sprintf("Tracking interval: %ds", next), toastSuccess)
		case 5:
			current := storage.GetIdleThresholdSeconds()
			next := min(storage.MaxIdleThresholdSeconds, current+5)
			if next != current {
				storage.SetIdleThresholdSeconds(next)
			}
			m.addToast(fmt.Sprintf("Idle threshold: %ds", next), toastSuccess)
		case 6:
			enabled := system.GetBreakReminderEnabled()
			minutes := system.GetBreakReminderMinutes()
			if err := system.SetBreakReminder(!enabled, minutes); err != nil {
				m.addToast("Failed to toggle focus alerts", toastError)
			} else {
				m.addToast("Focus alerts toggled", toastSuccess)
			}
		case 7:
			current := storage.GetWarningThresholdPercent()
			next := min(storage.MaxWarningThresholdPercent, current+5)
			if next != current {
				storage.SetWarningThresholdPercent(next)
			}
			m.addToast(fmt.Sprintf("Warning threshold: %d%%", next), toastSuccess)
		case 8:
			m.settingsAddingBrowser = true
			m.settingsBrowserInput = ""
		case 9:
			openDBFolder()
		case 10:
			path, err := exportData(false)
			if err != nil {
				m.addToast("Export failed", toastError)
			} else {
				m.addToast("Exported to "+filepath.Base(path), toastSuccess)
			}
		case 11:
			if err := launchGitHubUpdate(); err != nil {
				m.addToast("Failed to launch updater", toastError)
			} else {
				m.addToast("Updater opened in new window", toastSuccess)
			}
		case 12:
			m.modal = modal{
				Active: true, Title: "WIPE DATA", Required: "DELETE", Danger: true,
				Message: "This will delete all tracking data and cannot be undone.",
			}
		}
	case "d":
		if m.settingsSelected == 8 {
			browsers := storage.GetCustomBrowsersList()
			if len(browsers) > 0 {
				i := min(m.settingsBrowserSelected, len(browsers)-1)
				storage.RemoveCustomBrowser(browsers[i])
				m.addToast("Removed "+browsers[i], toastWarning)
			}
		}
	}
	return m, nil
}

func (m Model) panelCount() int {
	switch m.activeTab {
	case tabDashboard:
		return 3
	case tabStats:
		return 4
	case tabFocus:
		return 2
	case tabLimits:
		if m.limitForm.Visible {
			return 2
		}
		return 1
	case tabSettings:
		return 1
	default:
		return 1
	}
}

func (m *Model) addToast(text string, kind toastKind) {
	m.toasts = append(m.toasts, toast{Text: text, Kind: kind, CreatedAt: time.Now()})
	if len(m.toasts) > 3 {
		m.toasts = m.toasts[len(m.toasts)-3:]
	}
}

func (m *Model) pruneToasts() {
	now := time.Now()
	var kept []toast
	for _, t := range m.toasts {
		if now.Sub(t.CreatedAt) < 3*time.Second {
			kept = append(kept, t)
		}
	}
	m.toasts = kept
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	if m.width < 80 || m.height < 24 {
		return appStyle.Width(m.width).Height(m.height).Render(
			lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
				amberStyle.Render("⚠ Terminal too small. Resize to at least 80×24.")),
		)
	}
	if m.splash {
		return appStyle.Width(m.width).Height(m.height).Render(m.renderSplash())
	}

	header := m.renderHeader()
	nav := m.renderTabs()
	contentHeight := m.height - lipgloss.Height(header) - lipgloss.Height(nav) - 2
	if contentHeight < 1 {
		contentHeight = 1
	}

	body := m.renderActiveTab(m.width, contentHeight)
	footer := m.renderFooter()
	screen := lipgloss.JoinVertical(lipgloss.Left, header, nav, body, footer)
	if m.showHelp {
		screen = m.overlay(screen, m.renderHelp())
	}
	if m.modal.Active {
		screen = m.overlay(screen, m.renderModal())
	}
	if len(m.toasts) > 0 {
		screen = m.placeToasts(screen)
	}
	return appStyle.Width(m.width).Height(m.height).Render(screen)
}

func (m Model) renderSplash() string {
	frames := []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}
	logo := `                   __                           _
                  / _|                         | |
                 | |_ ___   ___ _   _ ___  ____| |
                 |  _/ _ \ / __| | | / __|/ _  | |
                 | || (_) | (__| |_| \__ \ (_| |_|
                 |_| \___/ \___|\__,_|___/\____(_)`
	content := lipgloss.JoinVertical(lipgloss.Center,
		logoStyle.Render(logo),
		mutedStyle.Render("Privacy-First Screen Time Tracker"),
		boldStyle.Render("v"+system.Version+"  ·  Windows x64"),
		"",
		mutedStyle.Render("Connecting to daemon...  ")+cyanStyle.Render(frames[m.splashFrame%len(frames)]),
	)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) renderHeader() string {
	left := logoStyle.Render("⬡ FOCUSD")
	version := mutedStyle.Render("v" + system.Version)
	status := greenStyle.Render("● RUNNING")
	if !m.daemonActive {
		status = redStyle.Render("○ STOPPED")
	}
	right := "[" + mutedStyle.Render("daemon: ") + status + "]   " + boldStyle.Render(m.clock.Format("15:04:05"))
	midWidth := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 4
	if midWidth < 1 {
		midWidth = 1
	}
	line := "  " + left + center(version, midWidth) + right
	return lipgloss.JoinVertical(lipgloss.Left, padRight(line, m.width), mutedStyle.Render(fill(m.width, "─")))
}

func (m Model) renderTabs() string {
	var labels []string
	var underlines []string
	for i, name := range tabNames {
		raw := fmt.Sprintf("[%d] %s", i+1, name)
		cellWidth := lipgloss.Width(raw)
		if i == m.activeTab {
			labels = append(labels, cyanStyle.Bold(true).Render(raw))
			underlines = append(underlines, cyanStyle.Render(center(strings.Repeat("▔", min(10, cellWidth)), cellWidth)))
		} else {
			labels = append(labels, mutedStyle.Render(raw))
			underlines = append(underlines, strings.Repeat(" ", cellWidth))
		}
	}
	labelLine := strings.Join(labels, mutedStyle.Render(" │ "))
	underlineLine := strings.Join(underlines, "   ")
	contentWidth := min(max(lipgloss.Width(labelLine), lipgloss.Width(underlineLine)), max(1, m.width-8))
	bar := center(labelLine, contentWidth) + "\n" + center(underlineLine, contentWidth)
	return lipgloss.NewStyle().
		Width(m.width-2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cMuted).
		Padding(0, 1).
		Render(bar)
}

func (m Model) renderFooter() string {
	pairs := []string{"q Quit", "? Help"}
	switch m.activeTab {
	case tabDashboard:
		pairs = append([]string{"↑↓/jk Navigate", "Tab Switch Panel", "r Refresh"}, pairs...)
	case tabStats:
		pairs = append([]string{"←→ Range", "Enter Edit Custom Range", "s Sort", "r Refresh"}, pairs...)
	case tabFocus:
		pairs = append([]string{"s Start", "p Pause", "x Stop", "r Reset"}, pairs...)
	case tabLimits:
		pairs = append([]string{"n New", "e Edit", "d Delete", "Enter Save"}, pairs...)
	case tabSettings:
		pairs = append([]string{"↑↓ Select Row", "Enter Activate / Add Browser", "Esc Cancel Input", "d Delete"}, pairs...)
	}
	if !m.daemonActive {
		pairs = append(pairs, amberStyle.Render("⚠ Daemon not running - run focusd start"))
	}
	return mutedStyle.Render(fill(m.width, "─")) + "\n  " + mutedStyle.Render(truncate(strings.Join(pairs, "  ·  "), m.width-2))
}

func (m Model) renderActiveTab(width, height int) string {
	var body string
	switch m.activeTab {
	case tabDashboard:
		body = m.renderDashboard(width, height)
	case tabStats:
		body = m.renderStats(width, height)
	case tabFocus:
		body = m.renderFocus(width, height)
	case tabLimits:
		body = m.renderLimits(width, height)
	case tabSettings:
		body = m.renderSettings(width, height)
	}
	body = clipLines(body, max(1, height-2), max(1, width-2))
	return lipgloss.NewStyle().Width(width).Height(height).Padding(1, 1).Render(body)
}

func (m Model) renderDashboard(width, height int) string {
	inner := width - 2
	gap := " "
	if inner >= 120 {
		col := (inner - 1) / 2
		top := lipgloss.JoinHorizontal(lipgloss.Top,
			m.renderTodayPanel(col),
			gap,
			m.renderTopAppsPanel(inner-col-1),
		)
		return lipgloss.JoinVertical(lipgloss.Left, top, m.renderHourlyPanel(inner))
	}
	if height < 24 {
		switch m.panelFocus {
		case 1:
			return m.renderTopAppsPanel(inner)
		case 2:
			return m.renderHourlyPanel(inner)
		default:
			return m.renderTodayPanel(inner)
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderTodayPanel(inner),
		m.renderTopAppsPanel(inner),
		m.renderHourlyPanel(inner),
	)
}

func (m Model) renderTodayPanel(width int) string {
	active, _, _ := core.GetPomodoroStatus()
	focusText := "0 / 5 goal"
	if active {
		focusText = amberStyle.Render("1 / 5 goal")
	}
	lines := []string{
		rowKV("Screen Time", formatDuration(m.dashboard.Total), width-4),
		rowKV("Active Apps", fmt.Sprintf("%d", m.dashboard.ActiveApps), width-4),
		rowKV("Focus Sessions", focusText, width-4),
		rowKV("Limits Hit", fmt.Sprintf("%d", m.dashboard.LimitsHit), width-4),
	}
	return panel("TODAY", "", width, "\n"+strings.Join(lines, "\n")+"\n", m.panelFocus == 0)
}

func (m Model) renderTopAppsPanel(width int) string {
	maxDuration := 0
	for _, a := range m.dashboard.Apps {
		maxDuration = max(maxDuration, a.Duration)
	}
	limits := system.GetAppTimeLimits()
	var lines []string
	if len(m.dashboard.Apps) == 0 {
		lines = append(lines, mutedStyle.Render("No app data yet. Start the daemon to begin tracking."))
	} else {
		for i, a := range m.dashboard.Apps[:min(10, len(m.dashboard.Apps))] {
			warn := "  "
			if _, ok := limits[a.Name]; ok {
				warn = amberStyle.Render("⚠ ")
			}
			nameW := max(10, width-24)
			line := mutedStyle.Render(fmt.Sprintf("%2d. ", i+1)) + warn +
				padRight(truncate(a.Name, nameW), nameW) +
				padLeft(formatDuration(a.Duration), 8) + "  " +
				cyanStyle.Render(bar(a.Duration, maxDuration, 3, "█"))
			if i == m.dashSelected && m.panelFocus == 1 {
				line = selectedRowStyle.Render(padRight(line, width-2))
			}
			lines = append(lines, line)
		}
	}
	refresh := "↻"
	if m.refreshing {
		refresh = cyanStyle.Render("↻")
	}
	return panel("TOP APPS", refresh, width, "\n"+strings.Join(lines, "\n")+"\n", m.panelFocus == 1)
}

func (m Model) renderHourlyPanel(width int) string {
	now := time.Now()
	start := 0
	count := 24
	if width < 90 {
		count = 12
		start = max(0, now.Hour()-8)
		if start+count > 24 {
			start = 24 - count
		}
	}
	var labels, blocks []string
	for h := start; h < start+count; h++ {
		labels = append(labels, fmt.Sprintf("%02d", h))
		block := " "
		if h > now.Hour() {
			block = mutedStyle.Render("·")
		} else if m.dashboard.Hourly[h] > 1800 {
			block = "█"
		} else if m.dashboard.Hourly[h] > 600 {
			block = "▓"
		} else if m.dashboard.Hourly[h] > 0 {
			block = "░"
		}
		if h == now.Hour() {
			block = cyanStyle.Render(block)
		}
		blocks = append(blocks, " "+block)
	}
	prefix := ""
	suffix := ""
	if count < 24 {
		if start > 0 {
			prefix = mutedStyle.Render("< ")
		}
		if start+count < 24 {
			suffix = mutedStyle.Render(" >")
		}
	}
	body := "\n" + prefix + strings.Join(labels, " ") + suffix + "\n" + prefix + strings.Join(blocks, " ") + suffix + "\n"
	return panel("HOURLY ACTIVITY", now.Format("Mon, 02 Jan"), width, body, m.panelFocus == 2)
}

func (m Model) renderStats(width, height int) string {
	inner := width - 2
	rangePanel := panel("TIME RANGE", "", inner, "\n"+m.renderRangeSelector(inner-4)+"\n", m.panelFocus == 0)
	tableRows := max(3, min(10, height-15))
	if inner >= 120 {
		leftW := (inner - 1) / 2
		tables := lipgloss.JoinHorizontal(lipgloss.Top,
			m.renderUsageBreakdown(leftW, tableRows),
			" ",
			m.renderBrowserUsage(inner-leftW-1, tableRows),
		)
		return lipgloss.JoinVertical(lipgloss.Left, rangePanel, tables, m.renderDailyHistory(inner))
	}
	tableRows = max(3, min(6, (height-15)/2))
	if height < 30 {
		switch m.panelFocus {
		case 2:
			return lipgloss.JoinVertical(lipgloss.Left, rangePanel, m.renderBrowserUsage(inner, max(4, height-12)))
		case 3:
			return lipgloss.JoinVertical(lipgloss.Left, rangePanel, m.renderDailyHistory(inner))
		default:
			return lipgloss.JoinVertical(lipgloss.Left, rangePanel, m.renderUsageBreakdown(inner, max(4, height-12)))
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left, rangePanel, m.renderUsageBreakdown(inner, tableRows), m.renderBrowserUsage(inner, tableRows), m.renderDailyHistory(inner))
}

func (m Model) renderRangeSelector(width int) string {
	ranges := []string{"TODAY", "THIS WEEK", "THIS MONTH", "CUSTOM RANGE"}
	var parts []string
	for i, r := range ranges {
		if i == m.statsRange {
			parts = append(parts, cyanStyle.Bold(true).Render("[ "+r+" ]"))
		} else {
			parts = append(parts, mutedStyle.Render("[ "+r+" ]"))
		}
	}
	line := truncate(strings.Join(parts, "  "), width)
	if m.statsRange == 3 {
		line += "\n" + truncate("From: [ "+m.statsCustomFrom+" ]  To: [ "+m.statsCustomTo+" ]  Enter Edit", width)
	}
	return line
}

func (m Model) renderUsageBreakdown(width, visibleRows int) string {
	apps := m.sortedStatsApps()
	total := 0
	for _, a := range apps {
		total += a.Duration
	}
	sortNames := []string{"Time", "Name", "%"}
	sortLabel := sortNames[m.statsSort]
	sortArrow := "↓"
	if m.statsAsc {
		sortArrow = "↑"
	}
	headerPlain := padRight("App", max(8, width-28)) + padLeft(sortLabel+sortArrow, 9) + "   %"
	header := boldStyle.Render(fitLine(headerPlain, width-4))
	var lines []string
	lines = append(lines, header, mutedStyle.Render(fill(max(1, width-4), "─")))
	start := clampScrollOffset(m.statsUsageOffset, visibleRows, len(apps))
	end := min(len(apps), start+visibleRows)
	for i := start; i < end; i++ {
		a := apps[i]
		pct := 0
		if total > 0 {
			pct = a.Duration * 100 / total
		}
		color := cyanStyle
		if i > 0 {
			color = lipgloss.NewStyle().Foreground(cMuted)
		}
		nameWidth := max(8, width-28)
		line := padRight(truncate(a.Name, nameWidth), nameWidth) +
			padLeft(formatDuration(a.Duration), 8) + "  " +
			color.Render(bar(pct, 100, min(8, max(3, width/8)), "█")) + fmt.Sprintf(" %2d%%", pct)
		line = fitLine(line, width-4)
		if i == m.statsSelected && m.panelFocus == 1 {
			line = selectedRowStyle.Render(padRight(line, width-4))
		}
		lines = append(lines, line)
	}
	if len(apps) == 0 {
		lines = append(lines, mutedStyle.Render("No usage recorded for this range."))
	}
	right := fmt.Sprintf("(%d-%d/%d)", min(len(apps), start+1), end, len(apps))
	if m.refreshing {
		right = "REFRESHING..."
	}
	return panel("USAGE BREAKDOWN", right, width, "\n"+strings.Join(lines, "\n")+"\n", m.panelFocus == 1)
}

func (m Model) renderBrowserUsage(width, visibleRows int) string {
	var lines []string
	lines = append(lines, boldStyle.Render(fitLine(padRight("Tab Title", max(8, width-16))+padLeft("Time", 8), width-4)), mutedStyle.Render(fill(max(1, width-4), "─")))
	start := clampScrollOffset(m.statsBrowserOffset, visibleRows, len(m.stats.Browsers))
	end := min(len(m.stats.Browsers), start+visibleRows)
	for i := start; i < end; i++ {
		b := m.stats.Browsers[i]
		prefix := mutedStyle.Render("▶ ")
		line := prefix + padRight(truncate(b.Name, max(8, width-18)), max(8, width-18)) + padLeft(formatDuration(b.Duration), 8)
		line = fitLine(line, width-4)
		if i == m.browserSelected && m.panelFocus == 2 {
			line = selectedRowStyle.Render(padRight(line, width-4))
		}
		lines = append(lines, line)
	}
	if len(m.stats.Browsers) == 0 {
		lines = append(lines, mutedStyle.Render("No browser activity recorded."))
	}
	return panel("BROWSER USAGE", fmt.Sprintf("(%d-%d/%d)", min(len(m.stats.Browsers), start+1), end, len(m.stats.Browsers)), width, "\n"+strings.Join(lines, "\n")+"\n", m.panelFocus == 2)
}

func (m Model) renderDailyHistory(width int) string {
	maxDay := 0
	for _, d := range m.stats.Days {
		maxDay = max(maxDay, d.Duration)
	}
	var lines []string
	if m.statsRange == 2 || len(m.stats.Days) > 14 {
		var cols []string
		for _, d := range m.stats.Days {
			ch := "▁"
			if d.Duration > 0 && maxDay > 0 {
				level := d.Duration * 8 / maxDay
				ch = []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}[min(7, max(0, level-1))]
			}
			if d.Today {
				ch = cyanStyle.Render(ch)
			} else if d.Weekend {
				ch = violetStyle.Render(ch)
			}
			cols = append(cols, ch)
		}
		lines = append(lines, strings.Join(cols, " "))
	} else {
		for _, d := range m.stats.Days {
			style := lipgloss.NewStyle().Foreground(cWhite)
			if d.Today {
				style = cyanStyle
			} else if d.Weekend {
				style = violetStyle
			}
			line := padRight(d.Label, 4) + style.Render(bar(d.Duration, maxDay, min(30, width-20), "█")) + "  " + padRight(formatDuration(d.Duration), 8)
			if d.Today {
				line += mutedStyle.Render(" ← today")
			}
			lines = append(lines, line)
		}
	}
	title := "DAILY HISTORY (7 DAYS)"
	switch m.statsRange {
	case 0:
		title = "DAILY HISTORY (TODAY)"
	case 2:
		title = "DAILY HISTORY (30 DAYS)"
	case 3:
		title = "DAILY HISTORY (CUSTOM RANGE)"
	}
	return panel(title, "", width, "\n"+strings.Join(lines, "\n")+"\n", m.panelFocus == 3)
}

func (m Model) renderFocus(width, height int) string {
	active, remaining, total := core.GetPomodoroStatus()
	if total == 0 {
		total = m.focusDuration
		remaining = time.Duration(total) * time.Minute
	}
	elapsed := total*60 - int(remaining.Seconds())
	if elapsed < 0 {
		elapsed = 0
	}
	status := "IDLE"
	borderFocus := m.panelFocus == 0
	if active && !m.focusPaused {
		status = "RUNNING"
	} else if m.focusPaused {
		status = "PAUSED"
	}
	timeText := fmt.Sprintf("%02d:%02d", int(remaining.Minutes()), int(remaining.Seconds())%60)
	if active && remaining <= 0 {
		timeText = "✓ DONE"
		status = "DONE"
	}
	timerBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(cCyan).
		Padding(1, 4).
		Render(boldStyle.Render(timeText) + "\n" + cyanStyle.Render(bar(elapsed, total*60, 10, "▓")))
	buttons := []string{"▶ START", "⏸ PAUSE", "■ STOP", "⟳ RESET"}
	var rendered []string
	for i, b := range buttons {
		label := "[ " + b + " ]"
		if i == m.focusButton && m.panelFocus == 0 {
			rendered = append(rendered, activeButtonStyle.Render(label))
		} else {
			rendered = append(rendered, buttonStyle.Render(label))
		}
	}
	body := lipgloss.JoinVertical(lipgloss.Center,
		"",
		lipgloss.JoinHorizontal(lipgloss.Center, timerBox, "   "+strings.Join([]string{
			"Session 1 of 4",
			"Goal: 4 sessions",
			"Streak: 🔥 3 days",
			status,
		}, "\n   ")),
		"",
		strings.Join(rendered, "   "),
		"",
		fmt.Sprintf("Duration: [ %02d ] min      Break: [ %02d ] min", m.focusDuration, m.focusBreak),
	)
	history := strings.Join([]string{
		boldStyle.Render("#   Started     Duration   Status"),
		mutedStyle.Render("────────────────────────────────────────────"),
		"1   09:15 AM    25:00      " + greenStyle.Render("✓ COMPLETED"),
		"2   09:45 AM    25:00      " + greenStyle.Render("✓ COMPLETED"),
		"3   10:15 AM    18:32      " + redStyle.Render("✗ INTERRUPTED"),
		"4   (current)   --:--      " + cyanStyle.Render("○ IN PROGRESS"),
	}, "\n")
	if height < 24 {
		if m.panelFocus == 1 {
			return panel("SESSION HISTORY (TODAY)", "", width-2, "\n"+history+"\n", true)
		}
		return panel("FOCUS TIMER", status, width-2, body, borderFocus)
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		panel("FOCUS TIMER", status, width-2, body, borderFocus),
		panel("SESSION HISTORY (TODAY)", "", width-2, "\n"+history+"\n", m.panelFocus == 1),
	)
}

func (m Model) renderLimits(width, height int) string {
	rows := m.limitRows()
	warnAt := storage.GetWarningThresholdPercent()
	maxW := width - 2
	var lines []string
	lines = append(lines, boldStyle.Render(padRight("App", maxW-48)+padLeft("Limit", 8)+padLeft("Used", 8)+padLeft("Remaining", 11)+"   Status"))
	lines = append(lines, mutedStyle.Render(fill(maxW-4, "─")))
	visibleRows := max(3, height-10)
	if m.limitForm.Visible {
		visibleRows = max(3, height-18)
	}
	start := scrollStart(m.limitsSelected, visibleRows, len(rows))
	end := min(len(rows), start+visibleRows)
	for i := start; i < end; i++ {
		r := rows[i]
		limitSecs := r.Opens * 60
		used := r.Duration
		remaining := max(0, limitSecs-used)
		pct := 0
		if limitSecs > 0 {
			pct = used * 100 / limitSecs
		}
		style := greenStyle
		status := "✓ OK"
		if pct >= 95 {
			style = redStyle
			status = "✗ EXCEEDED"
		} else if pct >= warnAt {
			style = amberStyle
			status = "⚠ WARNING"
		}
		line := padRight(truncate(r.Name, maxW-52), maxW-52) +
			padLeft(formatDuration(limitSecs), 8) +
			padLeft(formatDuration(used), 8) +
			padLeft(formatDuration(remaining), 11) + "  " +
			style.Render(bar(pct, 100, 8, "█")) + "  " + style.Render(status)
		if pct >= 95 {
			line = dangerRowStyle.Render(padRight(line, maxW-2))
		} else if i == m.limitsSelected && !m.limitForm.Visible {
			line = selectedRowStyle.Render(padRight(line, maxW-2))
		}
		lines = append(lines, line)
	}
	if len(rows) == 0 {
		lines = append(lines, mutedStyle.Render("No limits set. Press n to add one."))
	}
	table := panel("APP LIMITS", fmt.Sprintf("(%d limits set)", len(rows)), maxW, "\n"+strings.Join(lines, "\n")+"\n", m.panelFocus == 0)
	if !m.limitForm.Visible {
		return table
	}
	formTitle := "ADD LIMIT"
	if m.limitForm.Editing {
		formTitle = "EDIT LIMIT"
	}
	fields := []string{
		"App name:    [ " + padRight(m.limitForm.App, 18) + " ]",
		fmt.Sprintf("Daily limit: [ %02d ] h  [ %02d ] m", m.limitForm.Hours, m.limitForm.Minutes),
		"[ SAVE ]   [ CANCEL ]",
	}
	for i := range fields {
		if i == m.limitForm.Field || (i == 2 && m.limitForm.Field == 3) {
			fields[i] = cyanStyle.Bold(true).Render(fields[i])
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left, table, panel(formTitle, "", maxW, "\n"+strings.Join(fields, "\n")+"\n", m.panelFocus == 1))
}

func (m Model) renderSettings(width, height int) string {
	auto, _, _ := system.GetAutoStartEnabled()
	paused := storage.IsPaused()
	trackingInterval := storage.GetTrackingIntervalSeconds()
	idleThreshold := storage.GetIdleThresholdSeconds()
	focusAlerts := system.GetBreakReminderEnabled()
	warningThreshold := storage.GetWarningThresholdPercent()
	browsers := append([]string{"chrome.exe", "firefox.exe", "edge.exe"}, storage.GetCustomBrowsersList()...)
	dbPath := m.dashboard.DBPath
	if dbPath == "" {
		dbPath, _ = storage.GetDBPath()
	}
	inner := max(20, width-6)
	browserValue := truncate(strings.Join(browsers, "   "), max(12, inner-42)) + "   " + buttonText("+ Add Browser")
	if m.settingsAddingBrowser {
		browserValue = "New browser: [ " + cyanStyle.Render(padRight(m.settingsBrowserInput, 18)) + " ]"
	}
	lines := []string{
		"DAEMON",
		settingRow(0, m.settingsSelected, "Background service", statusPill(m.daemonActive)+"  "+buttonText(ifThen(m.daemonActive, "Stop", "Start")), inner),
		settingRow(1, m.settingsSelected, "Auto-start on login", toggleText(auto), inner),
		"",
		"TRACKING",
		settingRow(2, m.settingsSelected, "Browser tracking", toggleText(!paused), inner),
		settingRow(3, m.settingsSelected, "Smart app grouping", toggleText(true)+"  "+mutedStyle.Render("(experimental)"), inner),
		settingRow(4, m.settingsSelected, "Tracking interval", fmt.Sprintf("[ %d ] seconds", trackingInterval), inner),
		settingRow(5, m.settingsSelected, "Idle threshold", fmt.Sprintf("[ %d ] seconds", idleThreshold), inner),
		"",
		"NOTIFICATIONS",
		settingRow(6, m.settingsSelected, "Focus complete alert", toggleText(focusAlerts), inner),
		settingRow(7, m.settingsSelected, "Limit warning at", fmt.Sprintf("[ %d ] %% usage", warningThreshold), inner),
		"",
		"BROWSERS",
		settingRow(8, m.settingsSelected, "Tracked browsers", browserValue, inner),
		"",
		"DATA",
		settingRow(9, m.settingsSelected, "Database path", mutedStyle.Render(truncate(dbPath, max(10, inner-45)))+"   "+buttonText("Open Folder"), inner),
		settingRow(10, m.settingsSelected, "Export data", buttonText("Export CSV")+"   "+buttonText("Export JSON"), inner),
		settingRow(11, m.settingsSelected, "Update app", buttonText("Update from GitHub"), inner),
		settingRow(12, m.settingsSelected, "Danger zone", redStyle.Bold(true).Render("[ Uninstall / Wipe All Data ]"), inner),
	}
	for i, line := range lines {
		switch line {
		case "DAEMON", "TRACKING", "NOTIFICATIONS", "BROWSERS", "DATA":
			lines[i] = boldStyle.Render(line) + "\n" + mutedStyle.Render("────────────────────────────────────────")
		}
	}
	bodyLines := strings.Split(strings.Join(lines, "\n"), "\n")
	maxBodyLines := max(6, height-6)
	if len(bodyLines) > maxBodyLines {
		selectedLine := 0
		for i, line := range bodyLines {
			if strings.Contains(line, ">") {
				selectedLine = i
				break
			}
		}
		start := scrollStart(selectedLine, maxBodyLines-2, len(bodyLines))
		end := min(len(bodyLines), start+maxBodyLines-2)
		window := bodyLines[start:end]
		if start > 0 {
			window = append([]string{mutedStyle.Render("↑ more")}, window...)
		}
		if end < len(bodyLines) {
			window = append(window, mutedStyle.Render("↓ more"))
		}
		bodyLines = window
	}
	return panel("SETTINGS", "", width-2, "\n"+strings.Join(bodyLines, "\n")+"\n", true)
}

func settingRow(index, selected int, label, value string, width int) string {
	cursor := "  "
	if index == selected {
		cursor = cyanStyle.Render("> ")
	}
	labelWidth := min(24, max(12, width/3))
	line := cursor + padRight(label, labelWidth) + value
	line = fitLine(line, width)
	if index == selected {
		return selectedRowStyle.Render(padRight(line, width))
	}
	return line
}

func statusPill(running bool) string {
	if running {
		return "[ " + greenStyle.Render("● RUNNING") + " ]"
	}
	return "[ " + redStyle.Render("○ STOPPED") + " ]"
}

func toggleText(on bool) string {
	if on {
		return "[" + greenStyle.Render("✓") + "] Enabled"
	}
	return "[" + mutedStyle.Render("·") + "] Disabled"
}

func buttonText(label string) string {
	return cyanStyle.Render("[ " + label + " ]")
}

func (m Model) renderHelp() string {
	left := []string{
		boldStyle.Render("GLOBAL"),
		cyanStyle.Render("1-5") + "     Switch tab",
		cyanStyle.Render("q") + "       Quit",
		cyanStyle.Render("?") + "       Toggle help",
		cyanStyle.Render("Tab") + "     Next panel",
		cyanStyle.Render("Esc") + "     Back / cancel",
		"",
		boldStyle.Render("STATS TAB"),
		cyanStyle.Render("s") + "       Sort column",
		cyanStyle.Render("r") + "       Refresh data",
		cyanStyle.Render("e") + "       Export",
		"",
		boldStyle.Render("LIMITS TAB"),
		cyanStyle.Render("n") + "       New limit",
		cyanStyle.Render("e") + "       Edit limit",
		cyanStyle.Render("d") + "       Delete limit",
	}
	right := []string{
		boldStyle.Render("NAVIGATION"),
		cyanStyle.Render("j / ↓") + "   Move down",
		cyanStyle.Render("k / ↑") + "   Move up",
		cyanStyle.Render("h / ←") + "   Left / previous",
		cyanStyle.Render("l / →") + "   Right / next",
		cyanStyle.Render("Enter") + "   Select / confirm",
		"",
		boldStyle.Render("FOCUS TAB"),
		cyanStyle.Render("s") + "       Start timer",
		cyanStyle.Render("p") + "       Pause timer",
		cyanStyle.Render("x") + "       Stop timer",
		cyanStyle.Render("r") + "       Reset timer",
		"",
		boldStyle.Render("SETTINGS TAB"),
		cyanStyle.Render("Space") + "   Toggle setting",
		cyanStyle.Render("Enter") + "   Edit / activate",
		cyanStyle.Render("d") + "       Delete in lists",
	}
	rows := make([]string, max(len(left), len(right)))
	for i := range rows {
		l, r := "", ""
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		rows[i] = padRight(l, 36) + "  " + r
	}
	rows = append(rows, "", center(mutedStyle.Render("Press ? or Esc to close"), 72))
	return panel("KEYBOARD SHORTCUTS", "", 78, "\n"+strings.Join(rows, "\n")+"\n", true)
}

func (m Model) renderModal() string {
	if m.modal.Title == "CUSTOM RANGE" {
		return m.renderCustomRangeModal()
	}

	msg := []string{
		"",
		amberStyle.Render("⚠  ") + m.modal.Message,
		"",
		`Type "` + m.modal.Required + `" to confirm:`,
		"[ " + padRight(m.modal.Input, 16) + " ]",
		"",
		center(buttonStyle.Render("[ CANCEL ]")+"   "+redStyle.Bold(true).Render("[ CONFIRM ]"), 40),
	}
	return panel("CONFIRM", "", 44, strings.Join(msg, "\n"), true)
}

func (m Model) renderCustomRangeModal() string {
	from := padRight(m.statsCustomDraftFrom, 10)
	to := padRight(m.statsCustomDraftTo, 10)
	if m.statsCustomField == 0 {
		from = cyanStyle.Bold(true).Render(from)
	} else {
		to = cyanStyle.Bold(true).Render(to)
	}
	errorLine := mutedStyle.Render("Digits are auto-formatted as YYYY-MM-DD")
	if m.modal.Error != "" {
		errorLine = redStyle.Render(m.modal.Error)
	}
	msg := []string{
		"",
		mutedStyle.Render(m.modal.Message),
		"",
		"From: [ " + from + " ]",
		"To:   [ " + to + " ]",
		"",
		errorLine,
		"",
		center(cyanStyle.Render("[ ENTER APPLY ]")+"   "+buttonStyle.Render("[ ESC CANCEL ]"), 42),
		"",
		mutedStyle.Render("Tab / ← → switch fields   Backspace deletes"),
	}
	return panel("CUSTOM RANGE", "", 50, strings.Join(msg, "\n"), true)
}

func (m Model) overlay(base, modalView string) string {
	dimmed := lipgloss.NewStyle().Foreground(cOverlay).Render(base)
	return overlayAt(dimmed, modalView, (m.width-lipgloss.Width(modalView))/2, (m.height-lipgloss.Height(modalView))/2)
}

func (m Model) placeToasts(base string) string {
	out := base
	for i, t := range m.toasts {
		view := renderToast(t)
		x := max(0, m.width-lipgloss.Width(view)-2)
		y := max(0, m.height-4-lipgloss.Height(view)*(len(m.toasts)-i))
		out = overlayAt(out, view, x, y)
	}
	return out
}

func overlayAt(base, over string, x, y int) string {
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	baseLines := strings.Split(base, "\n")
	overLines := strings.Split(over, "\n")
	for len(baseLines) < y+len(overLines) {
		baseLines = append(baseLines, "")
	}
	for i, overLine := range overLines {
		target := y + i
		plainWidth := lipgloss.Width(baseLines[target])
		if plainWidth < x {
			baseLines[target] += strings.Repeat(" ", x-plainWidth)
		}
		line := baseLines[target]
		prefix := line
		for lipgloss.Width(prefix) > x && len([]rune(prefix)) > 0 {
			r := []rune(prefix)
			prefix = string(r[:len(r)-1])
		}
		if lipgloss.Width(prefix) < x {
			prefix += strings.Repeat(" ", x-lipgloss.Width(prefix))
		}
		baseLines[target] = prefix + overLine
	}
	return strings.Join(baseLines, "\n")
}

func renderToast(t toast) string {
	style := greenStyle
	icon := "✓"
	switch t.Kind {
	case toastError:
		style = redStyle
		icon = "✗"
	case toastWarning:
		style = amberStyle
		icon = "⚠"
	case toastInfo:
		style = cyanStyle
		icon = "ℹ"
	}
	age := time.Since(t.CreatedAt)
	progress := max(0, 20-int(age.Seconds()*7))
	body := "  " + style.Render(icon) + " " + truncate(t.Text, 34) + "  \n" + style.Render(fill(progress, "─"))
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(style.GetForeground()).Padding(0, 1).Render(body)
}

func (m Model) sortedStatsApps() []appUsage {
	apps := append([]appUsage(nil), m.stats.Apps...)
	sort.Slice(apps, func(i, j int) bool {
		less := false
		switch m.statsSort {
		case 0:
			less = apps[i].Duration < apps[j].Duration
		case 1:
			less = apps[i].Name < apps[j].Name
		case 2:
			less = apps[i].Duration < apps[j].Duration
		}
		if m.statsAsc {
			return less
		}
		return !less
	})
	return apps
}

func (m Model) limitRows() []appUsage {
	limits := system.GetAppTimeLimits()
	rows := make([]appUsage, 0, len(limits))
	for app, mins := range limits {
		rows = append(rows, appUsage{Name: app, Opens: mins, Duration: storage.GetAppUsageTodayMinutes(app) * 60})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return rows
}

func rowKV(label, value string, width int) string {
	return "  " + mutedStyle.Render(padRight(label, max(8, width-18))) + boldStyle.Render(padLeft(value, 14))
}

func formatDuration(secs int) string {
	if secs < 60 {
		return fmt.Sprintf("%ds", secs)
	}
	if secs < 3600 {
		return fmt.Sprintf("%dm %ds", secs/60, secs%60)
	}
	return fmt.Sprintf("%dh %dm", secs/3600, (secs%3600)/60)
}

func ifThen(ok bool, a, b string) string {
	if ok {
		return a
	}
	return b
}

func openDBFolder() {
	if dbPath, err := storage.GetDBPath(); err == nil {
		exec.Command("explorer.exe", filepath.Dir(dbPath)).Start()
	}
}

func launchGitHubUpdate() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command("cmd", "/c", "start", "", exePath, "update")
	return cmd.Start()
}

func exportData(jsonOut bool) (string, error) {
	userProfile := os.Getenv("USERPROFILE")
	exportDir := filepath.Join(userProfile, "Desktop")
	if userProfile == "" {
		exportDir = "."
	}
	if jsonOut {
		path := filepath.Join(exportDir, "focusd_export.json")
		apps, err := storage.GetAllAppStats()
		if err != nil {
			return "", err
		}
		b, err := json.MarshalIndent(apps, "", "  ")
		if err != nil {
			return "", err
		}
		return path, os.WriteFile(path, b, 0644)
	}
	path := filepath.Join(exportDir, "focusd_export.csv")
	apps, err := storage.GetAllAppStats()
	if err != nil {
		return "", err
	}
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	w.Write([]string{"Date", "App Name", "Executable", "Duration (seconds)", "Open Count"})
	for _, app := range apps {
		w.Write([]string{app.Date, app.AppName, app.ExeName, strconv.Itoa(app.TotalDurationSecs), strconv.Itoa(app.OpenCount)})
	}
	return path, nil
}
