package tui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
)

func (m mainModel) renderTools() string {
	pStatus := "No active session"
	if m.pomodoroActive {
		pStatus = fmt.Sprintf("RUNNING: %d min left", m.pomodoroRem)
	}

	var limits []string
	for app, mins := range m.appLimits {
		limits = append(limits, fmt.Sprintf("%s: %dm", app, mins))
	}
	if len(limits) == 0 {
		limits = append(limits, "No limits configured")
	}

	content := lipgloss.JoinVertical(lipgloss.Center,
		titleStyle.Render("Focus Tools"),
		pStatus,
		"",
		"Pomodoro Duration:",
		m.pomodoroIn.View(),
		"",
		lipgloss.NewStyle().
			Foreground(whiteColor).
			Background(navyColor).
			Padding(0, 3).
			Render("START POMODORO"),
		"",
		lipgloss.NewStyle().Width(m.width/2).Border(lipgloss.NormalBorder(), true, false, false, false).BorderForeground(slateColor).Render(""),
		"",
		titleStyle.Render("Active App Limits"),
		lipgloss.JoinVertical(lipgloss.Center, limits...),
	)

	return lipgloss.Place(m.width, m.height-10, lipgloss.Center, lipgloss.Center, content)
}
