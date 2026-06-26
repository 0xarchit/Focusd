package tui

import (
	"fmt"
	"focusd/core"
	"focusd/system"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) handleFocusKey(key string) (Model, tea.Cmd) {
	switch key {
	case "h", "left":
		m.focusButton = max(0, m.focusButton-1)
	case "l", "right":
		m.focusButton = min(2, m.focusButton+1)
	case "s":
		return m.startFocus()
	case "x":
		if err := core.StopPomodoro(); err != nil {
			m.addToast(fmt.Sprintf("Failed to stop timer: %v", err), toastError)
		} else {
			m.focusPaused = false
			m.addToast("Focus timer stopped", toastWarning)
		}
	case "r":
		if err := core.StopPomodoro(); err != nil {
			m.addToast(fmt.Sprintf("Failed to reset timer: %v", err), toastError)
		} else {
			m.focusPaused = false
			m.addToast("Focus timer reset", toastInfo)
		}
	case "enter":
		switch m.focusButton {
		case 0:
			return m.startFocus()
		case 1:
			if err := core.StopPomodoro(); err != nil {
				m.addToast(fmt.Sprintf("Failed to stop timer: %v", err), toastError)
			} else {
				m.focusPaused = false
				m.addToast("Focus timer stopped", toastWarning)
			}
		case 2:
			if err := core.StopPomodoro(); err != nil {
				m.addToast(fmt.Sprintf("Failed to reset timer: %v", err), toastError)
			} else {
				m.focusPaused = false
				m.addToast("Focus timer reset", toastInfo)
			}
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
		m.addToast("Focus timer started", toastSuccess)
	}
	return m, nil
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
	if active {
		status = "RUNNING"
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
	buttons := []string{"▶ START", "■ STOP", "⟳ RESET"}
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
			"Focus Status:",
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
		mutedStyle.Render("   No focus sessions recorded today."),
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
