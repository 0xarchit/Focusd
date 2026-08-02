package tui

import (
	"fmt"
	"focusd/coreapi"
	"focusd/storage"
	"focusd/system"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
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
	Apps             []appUsage
	Browsers         []appUsage
	Days             []dailyUsage
	Hourly           [24]int
	Total            int
	YesterdayTotal   int
	Last3DaysTotal   int
	Last7DaysTotal   int
	DailyAvg         int
	PeakDayLabel     string
	PeakDayDuration  int
	TotalAppLaunches int
	ActiveApps       int
	LimitsHit        int
}

type dashboardLoadedMsg tuiData
type statsLoadedMsg tuiData
type secondTickMsg time.Time
type splashDoneMsg struct{}
type flushDoneMsg struct{}
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
	Visible     bool
	Editing     bool
	App         string
	OriginalApp string
	Hours       int
	Minutes     int
	Field       int
}

type modal struct {
	Active   bool
	Title    string
	Message  string
	Required string
	Input    string
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
	settingsExportSelected  int
	focusButton             int
	focusDuration           int
	settingsAddingBrowser    bool
	settingsBrowserInput     string
	settingsAddingWhitelist  bool
	settingsWhitelistInput   string
	settingsWhitelistSelected int

	limitForm limitForm
	modal     modal
	toasts    []toast

	hoveredElement   string
	hoveredPanel     int
	clickableRegions []clickableRegion

	sortedAppsCache    []appUsage
	sortedAppsCacheKey string
}

type clickableRegion struct {
	X1, Y1, X2, Y2 int
	ID             string
	Kind           string
}

func NewModel() Model {
	return Model{
		activeTab:              tabDashboard,
		splash:                 true,
		clock:                  time.Now(),
		focusDuration:          system.GetPomodoroMinutes(),
		statsRange:             0,
		statsSort:              0,
		statsCustomFrom:        time.Now().Format("2006-01-02"),
		statsCustomTo:          time.Now().Format("2006-01-02"),
		settingsSelected:       0,
		settingsExportSelected: 0,
		hoveredPanel:           -1,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		secondTick(),
		tea.Tick(1500*time.Millisecond, func(time.Time) tea.Msg { return splashDoneMsg{} }),
		checkDaemon(),
		loadDashboard(),
		loadStats(m.statsRange, m.statsCustomFrom, m.statsCustomTo),
	)
}

func secondTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return secondTickMsg(t) })
}

func flushCmd() tea.Cmd {
	return func() tea.Msg {
		coreapi.SendIPCCmd("flush")
		return flushDoneMsg{}
	}
}

func checkDaemon() tea.Cmd {
	return func() tea.Msg {
		return statusMsg(system.GetProcessCount(system.DaemonProcessName) >= 1)
	}
}

func loadDashboard() tea.Cmd {
	return func() tea.Msg {
		today := storage.Today()
		return dashboardLoadedMsg(readTUIData(today, today, 7, true))
	}
}

func loadStats(rangeIndex int, from, to string) tea.Cmd {
	return func() tea.Msg {
		start, end, historyDays := statsRangeDates(rangeIndex, from, to, time.Now())
		return statsLoadedMsg(readTUIData(start, end, historyDays, false))
	}
}

func statsRangeDates(rangeIndex int, customFrom, customTo string, now time.Time) (string, string, int) {
	today := now.Format("2006-01-02")
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
		return customFrom, customTo, min(max(days, 1), storage.MaxRetentionDays)
	default:
		return today, today, 1
	}
}

func readTUIData(startDate, endDate string, historyDays int, includeHourly bool) tuiData {
	today := storage.Today()
	var data tuiData

	apps, err := aggregateAppStats(startDate, endDate)
	if err != nil {
		log.Printf("WARN: failed to load app stats: %v", err)
	}
	for _, s := range apps {
		name := s.ExeName
		if name == "" {
			name = s.AppName
		}
		data.Apps = append(data.Apps, appUsage{Name: name, Duration: s.TotalDurationSecs, Opens: s.OpenCount})
		data.Total += s.TotalDurationSecs
	}
	data.ActiveApps = len(data.Apps)

	browsers, err := aggregateBrowserStats(startDate, endDate)
	if err != nil {
		log.Printf("WARN: failed to load browser stats: %v", err)
	}
	for _, s := range browsers {
		data.Browsers = append(data.Browsers, appUsage{Name: s.AppName, Duration: s.TotalDurationSecs, Opens: s.OpenCount})
	}

	limits := system.GetAppTimeLimits()
	if len(limits) > 0 {
		usageMap, err := storage.GetAppUsageTodayMinutesMap()
		if err == nil {
			for app, mins := range limits {
				if usageMap[app] >= mins {
					data.LimitsHit++
				}
			}
		}
	}

	if includeHourly {
		sessions, err := storage.GetSessionsPaginated(500, 0, today, today)
		if err == nil {
			for _, s := range sessions {
				h := s.StartTime.Hour()
				if h < 24 {
					data.Hourly[h] += s.DurationSecs
				}
			}
		}
	}

	historyEnd, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		historyEnd = time.Now()
	}

	if includeHourly {
		yesterday := historyEnd.AddDate(0, 0, -1).Format("2006-01-02")
		yesterdayApps, err := aggregateAppStats(yesterday, yesterday)
		if err == nil {
			for _, s := range yesterdayApps {
				data.YesterdayTotal += s.TotalDurationSecs
			}
		}
	}
	earliestDate := historyEnd.AddDate(0, 0, -(historyDays - 1)).Format("2006-01-02")
	allStats, err := storage.GetAppStatsInRange(earliestDate, endDate)
	dateTotals := make(map[string]int)
	if err == nil {
		for _, s := range allStats {
			dateTotals[s.Date] += s.TotalDurationSecs
		}
	}

	for i := historyDays - 1; i >= 0; i-- {
		d := historyEnd.AddDate(0, 0, -i)
		date := d.Format("2006-01-02")
		dur := dateTotals[date]
		data.Days = append(data.Days, dailyUsage{
			Date:     date,
			Label:    d.Format("Mon"),
			Duration: dur,
			Today:    date == today,
			Weekend:  d.Weekday() == time.Saturday || d.Weekday() == time.Sunday,
		})
	}

	numDays := len(data.Days)
	sumDays := 0
	for i, d := range data.Days {
		sumDays += d.Duration
		if numDays-i <= 3 {
			data.Last3DaysTotal += d.Duration
		}
		if numDays-i <= 7 {
			data.Last7DaysTotal += d.Duration
		}
		if d.Duration > data.PeakDayDuration {
			data.PeakDayDuration = d.Duration
			if t, err := time.Parse("2006-01-02", d.Date); err == nil {
				data.PeakDayLabel = t.Format("Mon 01/02")
			} else {
				data.PeakDayLabel = d.Date
			}
		}
	}
	if numDays > 0 {
		data.DailyAvg = sumDays / numDays
	}

	for _, a := range data.Apps {
		data.TotalAppLaunches += a.Opens
	}

	return data
}

func aggregateAppStats(startDate, endDate string) ([]storage.AppDailyStat, error) {
	stats, err := storage.GetAppStatsInRange(startDate, endDate)
	if err != nil {
		return nil, err
	}
	combined := map[string]storage.AppDailyStat{}
	for _, s := range stats {
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
	stats, err := storage.GetBrowserStatsInRange(startDate, endDate)
	if err != nil {
		return nil, err
	}
	combined := map[string]storage.AppDailyStat{}
	for _, s := range stats {
		key := s.AppName
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

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case splashDoneMsg:
		m.splash = false
	case flushDoneMsg:
		cmds = append(cmds, m.loadStatsCmd(), loadDashboard())
	case secondTickMsg:
		m.clock = time.Time(msg)
		m.splashFrame++
		m.pruneToasts()
		cmds = append(cmds, secondTick(), checkDaemon())
		if m.clock.Second()%5 == 0 {
			isEditing := m.limitForm.Visible || m.settingsAddingBrowser || m.settingsAddingWhitelist
			if !isEditing {
				m.refreshing = true
				cmds = append(cmds, loadDashboard(), loadStats(m.statsRange, m.statsCustomFrom, m.statsCustomTo))
			}
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
		m.sortedAppsCache = nil
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
		*m = next
		cmds = append(cmds, cmd)
	case tea.MouseMsg:
		next, cmd := m.handleMouse(msg)
		*m = next
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m Model) contentHeight() int {
	contentHeight := m.height - lipgloss.Height(m.renderHeader()) - lipgloss.Height(m.renderTabs()) - lipgloss.Height(m.renderFooter())
	if contentHeight < 1 {
		return 1
	}
	return contentHeight
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

	isInputActive := (m.activeTab == tabLimits && m.limitForm.Visible) ||
		(m.activeTab == tabSettings && (m.settingsAddingBrowser || m.settingsAddingWhitelist))

	if isInputActive {
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.activeTab {
		case tabLimits:
			return m.handleLimitsKey(key)
		case tabSettings:
			return m.handleSettingsKey(key)
		}
		return m, nil
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
		if m.activeTab == tabFocus {
			return m.handleFocusKey(key)
		}
		m.panelFocus = (m.panelFocus + 1) % m.panelCount()
		return m, nil
	case "shift+tab":
		if m.activeTab == tabFocus {
			return m.handleFocusKey(key)
		}
		m.panelFocus = (m.panelFocus + m.panelCount() - 1) % m.panelCount()
		return m, nil
	case "r":
		// 'r' is reserved for resetting the pomodoro timer on the Focus tab,
		// so we only trigger a database flush/refresh on other tabs.
		if m.activeTab != tabFocus {
			m.refreshing = true
			return m, flushCmd()
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
					m.dashboard = tuiData{}
					m.stats = tuiData{}
					m.sortedAppsCache = nil
					m.dashSelected = 0
					m.limitsSelected = 0
					m.statsSelected = 0
					m.browserSelected = 0
					m.statsUsageOffset = 0
					m.statsBrowserOffset = 0
				}
				m.modal = modal{}
				return m, tea.Batch(loadDashboard(), loadStats(m.statsRange, m.statsCustomFrom, m.statsCustomTo))
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

func (m *Model) View() string {
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

	originalWidth := m.width
	m.width--
	defer func() { m.width = originalWidth }()

	m.clickableRegions = nil

	header := m.renderHeader()
	nav := m.renderTabs()
	footer := m.renderFooter()

	contentHeight := m.contentHeight()

	var body string
	switch m.activeTab {
	case tabDashboard:
		body = m.renderDashboard(m.width, contentHeight)
	case tabStats:
		body = m.renderStats(m.width, contentHeight)
	case tabFocus:
		body = m.renderFocus(m.width, contentHeight)
	case tabLimits:
		body = m.renderLimits(m.width, contentHeight)
	case tabSettings:
		body = m.renderSettings(m.width, contentHeight)
	}
	body = clipLines(body, contentHeight, m.width)
	screen := lipgloss.JoinVertical(lipgloss.Left, header, nav, body, footer)
	screen = clipLines(screen, m.height, m.width)
	if m.showHelp {
		screen = m.overlay(screen, m.renderHelp())
	}
	if m.modal.Active {
		screen = m.overlay(screen, m.renderModal())
	}
	if len(m.toasts) > 0 {
		screen = m.placeToasts(screen)
	}
	return appStyle.Width(originalWidth).Height(m.height).Render(screen)
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

func (m *Model) renderHeader() string {
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
	return lipgloss.JoinVertical(lipgloss.Left, padRight(line, m.width), mutedStyle.Render(strings.Repeat("─", max(0, m.width))))
}

func (m *Model) renderTabs() string {
	var labels []string
	var underlines []string

	collapseNames := m.width < 80

	var barWidth int
	var tabWidths []int
	for i, name := range tabNames {
		var raw string
		if collapseNames {
			raw = fmt.Sprintf("[%d]", i+1)
		} else {
			raw = fmt.Sprintf("[%d] %s", i+1, name)
		}
		cellWidth := lipgloss.Width(raw)
		tabWidths = append(tabWidths, cellWidth)
		barWidth += cellWidth
		if i == m.activeTab {
			labels = append(labels, cyanStyle.Bold(true).Render(raw))
			underlines = append(underlines, cyanStyle.Render(center(strings.Repeat("▔", min(10, cellWidth)), cellWidth)))
		} else {
			labels = append(labels, mutedStyle.Render(raw))
			underlines = append(underlines, strings.Repeat(" ", cellWidth))
		}
	}
	separator := " │ "
	sepWidth := lipgloss.Width(separator)
	barWidth += sepWidth * (len(tabNames) - 1)

	labelLine := strings.Join(labels, mutedStyle.Render(separator))
	underlineLine := strings.Join(underlines, "   ")
	tabWidth := lipgloss.Width(labelLine)
	
	inner := m.width - 4
	leftMargin := (inner - tabWidth) / 2
	if leftMargin < 0 {
		leftMargin = 0
	}

	centeredLabel := strings.Repeat(" ", leftMargin) + labelLine
	centeredUnderline := strings.Repeat(" ", leftMargin) + underlineLine

	centeredLabel += strings.Repeat(" ", max(0, inner-lipgloss.Width(centeredLabel)))
	centeredUnderline += strings.Repeat(" ", max(0, inner-lipgloss.Width(centeredUnderline)))

	edge := lipgloss.NewStyle().Foreground(cMuted)
	top := edge.Render("╭" + strings.Repeat("─", m.width-2) + "╮")
	mid1 := edge.Render("│ ") + centeredLabel + edge.Render(" │")
	mid2 := edge.Render("│ ") + centeredUnderline + edge.Render(" │")
	bottom := edge.Render("╰" + strings.Repeat("─", m.width-2) + "╯")

	bar := lipgloss.JoinVertical(lipgloss.Left, top, mid1, mid2, bottom)

	startX := leftMargin + 2
	for i := range tabNames {
		w := tabWidths[i]
		m.clickableRegions = append(m.clickableRegions, clickableRegion{
			X1:   startX,
			Y1:   2,
			X2:   startX + w,
			Y2:   4,
			ID:   fmt.Sprintf("tab-%d", i),
			Kind: "tab",
		})
		startX += w + sepWidth
	}

	return bar
}

func (m *Model) renderFooter() string {
	var hint string
	if m.hoveredElement != "" {
		hint = m.hoveredElement
	} else {
		pairs := []string{"q Quit", "? Help"}
		switch m.activeTab {
		case tabDashboard:
			pairs = append([]string{"↑↓/jk Navigate", "Tab Switch Panel", "r Refresh"}, pairs...)
		case tabStats:
			pairs = append([]string{"←→ Range", "Enter Edit Custom Range", "s Sort", "r Refresh"}, pairs...)
		case tabFocus:
			pairs = append([]string{"s Start", "x Stop", "r Reset"}, pairs...)
		case tabLimits:
			pairs = append([]string{"n New", "e Edit", "d Delete", "Enter Save"}, pairs...)
		case tabSettings:
			pairs = append([]string{"↑↓ Select Row", "Enter Activate / Add Browser", "Esc Cancel Input", "d Delete"}, pairs...)
		}
		if !m.daemonActive {
			pairs = append(pairs, amberStyle.Render("⚠ Daemon not running - run focusd start"))
		}
		hint = strings.Join(pairs, "  ·  ")
	}
	return mutedStyle.Render(strings.Repeat("─", max(0, m.width))) + "\n  " + mutedStyle.Render(truncate(hint, m.width-2))
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
	return panelWithHover("CONFIRM", "", 44, strings.Join(msg, "\n"), true, false)
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

func truncateAnsi(s string, limit int) string {
	var sb strings.Builder
	printed := 0
	inEsc := false
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\x1b' {
			inEsc = true
			sb.WriteRune(r)
			continue
		}
		if inEsc {
			sb.WriteRune(r)
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		w := runewidth.RuneWidth(r)
		if printed+w > limit {
			break
		}
		sb.WriteRune(r)
		printed += w
	}
	return sb.String()
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
		prefix := truncateAnsi(line, x)
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
	body := "  " + style.Render(icon) + " " + truncate(t.Text, 34) + "  \n" + style.Render(strings.Repeat("─", max(0, progress)))
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(style.GetForeground()).Padding(0, 1).Render(body)
}

func rowKV(label, value string, width int) string {
	usable := max(6, width-2)
	valWidth := 8
	if usable > 20 {
		valWidth = 12
	}
	lblWidth := max(4, usable-valWidth)
	return " " + mutedStyle.Render(padRight(label, lblWidth)) + boldStyle.Render(padLeft(value, valWidth)) + " "
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
	return panelWithHover("KEYBOARD SHORTCUTS", "", 78, "\n"+strings.Join(rows, "\n")+"\n", true, false)
}

func (m Model) handleMouse(msg tea.MouseMsg) (Model, tea.Cmd) {
	if m.splash || m.showHelp || m.modal.Active {
		return m, nil
	}

	var cmd tea.Cmd

	m.hoveredElement = ""
	m.hoveredPanel = -1

	var hoveredRegion *clickableRegion
	for _, region := range m.clickableRegions {
		if msg.X >= region.X1 && msg.X < region.X2 && msg.Y >= region.Y1 && msg.Y < region.Y2 {
			hoveredRegion = &region
			break
		}
	}

	if hoveredRegion != nil {
		if hoveredRegion.Kind == "tab" {
			var idx int
			if _, err := fmt.Sscanf(hoveredRegion.ID, "tab-%d", &idx); err == nil && idx >= 0 && idx < len(tabNames) {
				m.hoveredElement = fmt.Sprintf("Click to switch to %s", tabNames[idx])
			} else {
				m.hoveredElement = fmt.Sprintf("Click to switch to %s", strings.ToUpper(strings.TrimPrefix(hoveredRegion.ID, "tab-")))
			}
		} else if hoveredRegion.Kind == "panel" {
			var idx int
			if _, err := fmt.Sscanf(hoveredRegion.ID, "panel-%d", &idx); err == nil {
				m.hoveredPanel = idx
			}
		} else if hoveredRegion.Kind == "row" {
			m.hoveredElement = "Use arrow keys or click to select this item"
		} else if hoveredRegion.Kind == "button" {
			m.hoveredElement = fmt.Sprintf("Click to activate %s", hoveredRegion.ID)
		}
	}

	if msg.Type == tea.MouseRelease || msg.Action == tea.MouseActionRelease {
		if hoveredRegion != nil {
			if hoveredRegion.Kind == "tab" {
				var idx int
				if _, err := fmt.Sscanf(hoveredRegion.ID, "tab-%d", &idx); err == nil && idx >= 0 && idx < len(tabNames) {
					m.activeTab = idx
					m.panelFocus = 0
					m.hoveredElement = ""
					m.hoveredPanel = -1
					if idx == tabStats {
						m.refreshing = true
						cmd = m.loadStatsCmd()
					}
				}
			} else if hoveredRegion.Kind == "panel" {
				var idx int
				if _, err := fmt.Sscanf(hoveredRegion.ID, "panel-%d", &idx); err == nil {
					m.panelFocus = idx
				}
			} else if hoveredRegion.Kind == "button" {
				if strings.HasPrefix(hoveredRegion.ID, "range-") {
					var idx int
					if _, err := fmt.Sscanf(hoveredRegion.ID, "range-%d", &idx); err == nil {
						m.statsRange = idx
						m.refreshing = true
						cmd = m.loadStatsCmd()
					}
				} else if strings.HasPrefix(hoveredRegion.ID, "focus-btn-") {
					var idx int
					if _, err := fmt.Sscanf(hoveredRegion.ID, "focus-btn-%d", &idx); err == nil {
						m.focusButton = idx
						var focusCmd tea.Cmd
						if idx == 0 {
							m, focusCmd = m.handleFocusKey("s")
						} else if idx == 1 {
							m, focusCmd = m.handleFocusKey("x")
						} else if idx == 2 {
							m, focusCmd = m.handleFocusKey("r")
						}
						cmd = focusCmd
					}
				}
			}
		} else {
			m.panelFocus = 0
		}
	}

	delta := 0
	if msg.Button == tea.MouseButtonWheelUp {
		delta = -1
	} else if msg.Button == tea.MouseButtonWheelDown {
		delta = 1
	}

	if delta != 0 && hoveredRegion != nil && hoveredRegion.Kind == "panel" {
		var idx int
		if _, err := fmt.Sscanf(hoveredRegion.ID, "panel-%d", &idx); err == nil {
			m = m.scrollPanel(idx, delta)
		}
	}

	return m, cmd
}

func (m Model) scrollPanel(panelIdx int, delta int) Model {
	switch m.activeTab {
	case tabStats:
		if panelIdx == 1 {
			appsLen := len(m.sortedStatsApps())
			m.statsUsageOffset = clampScrollOffset(m.statsUsageOffset+delta, m.statsVisibleRows(false), appsLen)
			m.statsSelected = min(m.statsUsageOffset, max(0, appsLen-1))
			m.panelFocus = 1
		} else if panelIdx == 2 {
			browserLen := len(m.stats.Browsers)
			m.statsBrowserOffset = clampScrollOffset(m.statsBrowserOffset+delta, m.statsVisibleRows(true), browserLen)
			m.browserSelected = min(m.statsBrowserOffset, max(0, browserLen-1))
			m.panelFocus = 2
		}
	}
	return m
}
