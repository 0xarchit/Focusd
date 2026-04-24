package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorBg      = lipgloss.Color("#1E1E1E")
	colorPanel   = lipgloss.Color("#252525")
	colorSlate   = lipgloss.Color("#5C5C5C")
	colorWhite   = lipgloss.Color("#E0E0E0")
	colorGold    = lipgloss.Color("#CBA153")
	colorNavy    = lipgloss.Color("#1F2D3D")
	colorGreen   = lipgloss.Color("#4A7056")
	colorDim     = lipgloss.Color("#707070")

	appStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Background(colorBg).
			Foreground(colorWhite)

	headerStyle = lipgloss.NewStyle().
			Height(3).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(colorSlate).
			Padding(0, 1)

	logoStyle = lipgloss.NewStyle().
			Foreground(colorGold).
			Bold(true).
			MarginRight(4)

	tabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(colorDim)

	activeTabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(colorWhite).
			Bold(true)

	statusRunningStyle = lipgloss.NewStyle().
				Foreground(colorGreen)

	statusStoppedStyle = lipgloss.NewStyle().
				Foreground(colorDim)

	mainViewportStyle = lipgloss.NewStyle().
				Padding(1, 2)

	footerStyle = lipgloss.NewStyle().
			Height(1).
			Foreground(colorDim).
			Padding(0, 1)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorSlate).
			Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(colorGold).
			Bold(true).
			MarginBottom(1)

	selectedRowStyle = lipgloss.NewStyle().
				Background(colorSlate).
				Foreground(colorWhite)
)
