package tui

import (
	"fmt"
	"focusd/core"
	"focusd/storage"
	"focusd/system"
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
	settingsExportSelected  int
	focusButton             int
	focusDuration           int
	focusBreak              int
	focusPaused             bool
	settingsAddingBrowser   bool
	settingsBrowserInput    string

	limitForm limitForm
	modal     modal
	toasts    []toast

	// Clickable regions coordinate map & mouse hover state
	hoveredElement   string
	hoveredPanel     int // panel index hovered (e.g. 0, 1, 2)
	clickableRegions []ClickableRegion
}

type ClickableRegion struct {
	X1, Y1, X2, Y2 int
	ID             string
	Kind           string
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
		settingsSelected:       0,
		settingsExportSelected: 0,
		hoveredPanel:           -1,
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
	return statsRangeDatesTime(rangeIndex, customFrom, customTo, time.Now())
}

func statsRangeDatesTime(rangeIndex int, customFrom, customTo string, now time.Time) (string, string, int) {
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
		return data
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

	browsers, err := aggregateBrowserStats(startDate, endDate)
	if err != nil {
		data.Err = err
		return data
	}
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
		sessions, _, err := storage.GetSessionsPaginated(500, 0, today, today)
		if err != nil {
			data.Err = err
			return data
		}
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
		dayApps, err := storage.GetAppStatsForDate(date)
		if err != nil {
			data.Err = err
			return data
		}
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

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			cmds = append(cmds, loadDashboard(), loadStats(m.statsRange))
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
	contentHeight := m.height - lipgloss.Height(m.renderHeader()) - lipgloss.Height(m.renderTabs()) - 2
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
		m.refreshing = true
		// Request daemon flush to DB first so statistics are 100% current
		core.SendIPCCmd("flush")
		return m, tea.Batch(m.loadStatsCmd(), loadDashboard())
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
					// Immediately clear data cache structures
					m.dashboard = tuiData{}
					m.stats = tuiData{}
					m.dashSelected = 0
					m.limitsSelected = 0
					m.statsSelected = 0
					m.browserSelected = 0
					m.statsUsageOffset = 0
					m.statsBrowserOffset = 0
				}
				m.modal = modal{}
				return m, tea.Batch(loadDashboard(), loadStats(m.statsRange))
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

	m.clickableRegions = nil // Reset clickable regions map every render frame

	header := m.renderHeader()
	nav := m.renderTabs()
	footer := m.renderFooter()
	
	// Total chrome height: header + nav + footer
	chromeHeight := lipgloss.Height(header) + lipgloss.Height(nav) + lipgloss.Height(footer)
	contentHeight := m.height - chromeHeight
	if contentHeight < 1 {
		contentHeight = 1
	}

	body := m.renderActiveTab(m.width, contentHeight)
	screen := lipgloss.JoinVertical(lipgloss.Left, header, nav, body, footer)
	// Ensure the screen is strictly clipped to the terminal height to prevent terminal scroll overflow
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
	return lipgloss.JoinVertical(lipgloss.Left, padRight(line, m.width), mutedStyle.Render(fill(m.width, "─")))
}

func (m *Model) renderTabs() string {
	var labels []string
	var underlines []string
	
	// Collapse tab labels to numbers-only if window width is below 80 columns
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
	contentWidth := min(max(lipgloss.Width(labelLine), lipgloss.Width(underlineLine)), max(1, m.width-8))
	bar := center(labelLine, contentWidth) + "\n" + center(underlineLine, contentWidth)

	// Register clickable coordinates for tabs (tab bar sits at row Y=2, Y=3)
	// We offset by left padding: (m.width - 2 - contentWidth)/2 + 2
	leftMargin := (m.width - 2 - contentWidth) / 2
	if leftMargin < 0 {
		leftMargin = 0
	}
	startX := leftMargin + 2
	for i := range tabNames {
		w := tabWidths[i]
		m.clickableRegions = append(m.clickableRegions, ClickableRegion{
			X1: startX,
			Y1: 2,
			X2: startX + w,
			Y2: 4,
			ID: fmt.Sprintf("tab-%d", i),
			Kind: "tab",
		})
		startX += w + sepWidth
	}

	return lipgloss.NewStyle().
		Width(m.width-2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cMuted).
		Padding(0, 1).
		Render(bar)
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
			pairs = append([]string{"s Start", "p Pause", "x Stop", "r Reset"}, pairs...)
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
	return mutedStyle.Render(fill(m.width, "─")) + "\n  " + mutedStyle.Render(truncate(hint, m.width-2))
}

func (m *Model) renderActiveTab(width, height int) string {
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
	body = clipLines(body, height, width)
	return body
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

func (m Model) handleMouse(msg tea.MouseMsg) (Model, tea.Cmd) {
	if m.splash || m.showHelp || m.modal.Active {
		return m, nil
	}

	var cmd tea.Cmd

	// Update transient hover state based on coordinate hit-testing
	m.hoveredElement = ""
	m.hoveredPanel = -1

	// Determine if mouse is inside any registered region
	var hoveredRegion *ClickableRegion
	for _, region := range m.clickableRegions {
		if msg.X >= region.X1 && msg.X < region.X2 && msg.Y >= region.Y1 && msg.Y < region.Y2 {
			hoveredRegion = &region
			break
		}
	}

	if hoveredRegion != nil {
		if hoveredRegion.Kind == "tab" {
			m.hoveredElement = fmt.Sprintf("Click to switch to %s", strings.ToUpper(strings.TrimPrefix(hoveredRegion.ID, "tab-")))
		} else if hoveredRegion.Kind == "panel" {
			// Extract numerical index of panel from ID (e.g. "panel-0" -> 0)
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

	// Process Click events
	if msg.Type == tea.MouseRelease || msg.Action == tea.MouseActionRelease {
		if hoveredRegion != nil {
			if hoveredRegion.Kind == "tab" {
				var idx int
				if _, err := fmt.Sscanf(hoveredRegion.ID, "tab-%d", &idx); err == nil {
					m.activeTab = idx
					m.panelFocus = 0
					// Reset hover immediately after switching
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
				// Translate click on range button or focus button to key action simulations
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
						// Simulate Enter key press on the focus button
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
			// Clicked empty space outside any panels -> clear panel focus
			m.panelFocus = 0
		}
	}

	// Handle Scroll wheel delta dispatching
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
			m.statsUsageOffset = clampScrollOffset(m.statsUsageOffset+delta, m.statsVisibleUsageRows(), appsLen)
			m.statsSelected = min(m.statsUsageOffset, max(0, appsLen-1))
			m.panelFocus = 1
		} else if panelIdx == 2 {
			browserLen := len(m.stats.Browsers)
			m.statsBrowserOffset = clampScrollOffset(m.statsBrowserOffset+delta, m.statsVisibleBrowserRows(), browserLen)
			m.browserSelected = min(m.statsBrowserOffset, max(0, browserLen-1))
			m.panelFocus = 2
		}
	}
	return m
}
