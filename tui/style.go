package tui

import "github.com/charmbracelet/lipgloss"

var (
	accentColor = lipgloss.Color("#C5B358")
	slateColor  = lipgloss.Color("#708090")
	steelColor  = lipgloss.Color("#A9A9A9")
	whiteColor  = lipgloss.Color("#FFFFFF")
	navyColor   = lipgloss.Color("#000080")
	bgNavy      = lipgloss.Color("#001F3F")

	headerStyle = lipgloss.NewStyle().
			Foreground(whiteColor).
			Background(navyColor).
			Padding(0, 1).
			Bold(true)

	tabStyle = lipgloss.NewStyle().
			Foreground(steelColor).
			Padding(0, 2)

	activeTabStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(accentColor).
			Padding(0, 2).
			Bold(true)

	footerStyle = lipgloss.NewStyle().
			Foreground(slateColor).
			Italic(true)

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(slateColor).
			Padding(1, 2).
			MarginRight(1)

	titleStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true).
			MarginBottom(1)

	tableHeaderStyle = lipgloss.NewStyle().
				Foreground(whiteColor).
				Background(slateColor).
				Bold(true)

	tableSelectedStyle = lipgloss.NewStyle().
				Foreground(whiteColor).
				Background(navyColor)

	statusRunning = lipgloss.NewStyle().Foreground(lipgloss.Color("#4CAF50"))
	statusStopped = lipgloss.NewStyle().Foreground(lipgloss.Color("#F44336"))
	statusPaused  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF9800"))
)
