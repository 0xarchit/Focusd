package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorBg        = lipgloss.Color("#0F141A")
	colorPanel     = lipgloss.Color("#18212B")
	colorPanelSoft = lipgloss.Color("#202B37")
	colorSlate     = lipgloss.Color("#6E7B8B")
	colorSteel     = lipgloss.Color("#B0BBC9")
	colorWhite     = lipgloss.Color("#F5F7FA")
	colorNavy      = lipgloss.Color("#1F3042")
	colorGold      = lipgloss.Color("#BDA57B")
	colorGreen     = lipgloss.Color("#6FA08C")
	colorRed       = lipgloss.Color("#A06A6A")

	appFrameStyle = lipgloss.NewStyle().
			Background(colorBg)

	headerBoxStyle = lipgloss.NewStyle().
			Background(colorPanel).
			Foreground(colorWhite).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorSlate).
			BorderBottom(true).
			Padding(0, 1)

	footerBoxStyle = lipgloss.NewStyle().
			Background(colorPanel).
			Foreground(colorSteel).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorSlate).
			BorderTop(true).
			Padding(0, 1)

	sectionTitleStyle = lipgloss.NewStyle().
				Foreground(colorGold).
				Bold(true)

	cardStyle = lipgloss.NewStyle().
			Background(colorPanel).
			Foreground(colorSteel).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorSlate).
			Padding(1, 2)

	cardSoftStyle = lipgloss.NewStyle().
			Background(colorPanelSoft).
			Foreground(colorWhite).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorSlate).
			Padding(1, 2)

	activeTabStyle = lipgloss.NewStyle().
			Foreground(colorWhite).
			Background(colorNavy).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorGold).
			Bold(true).
			Padding(0, 2)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(colorSteel).
				Padding(0, 2)

	statusOnStyle = lipgloss.NewStyle().
			Foreground(colorWhite).
			Background(colorGreen).
			Bold(true).
			Padding(0, 1)

	statusOffStyle = lipgloss.NewStyle().
			Foreground(colorWhite).
			Background(colorRed).
			Bold(true).
			Padding(0, 1)
)
