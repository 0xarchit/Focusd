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

func (m *Model) handleFocusKey(key string) (Model, tea.Cmd) {
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
	return *m, nil
}

func (m *Model) startFocus() (Model, tea.Cmd) {
	if err := core.StartPomodoro(m.focusDuration); err != nil {
		m.addToast("Could not start focus timer", toastError)
	} else {
		system.SetPomodoroMinutes(m.focusDuration)
		m.addToast("Focus timer started", toastSuccess)
	}
	return *m, nil
}

func (m *Model) renderFocus(width, height int) string {
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
	if active {
		status = "RUNNING"
	}
	if active && remaining <= 0 {
		status = "DONE"
	}

	inner := width - 2

	mins := int(remaining.Minutes())
	secs := int(remaining.Seconds()) % 60
	timerDigits := RenderBlockTimer(mins, secs)

	buttons := []string{"▶ START", "■ STOP", "⟳ RESET"}
	var renderedBtns []string
	btnRowY := 5 + 3 + 5 + 2 // Panel title and margins offset
	btnStartX := 4
	for i, b := range buttons {
		label := "[ " + b + " ]"
		btnW := lipgloss.Width(label)
		m.clickableRegions = append(m.clickableRegions, ClickableRegion{
			X1: btnStartX, Y1: btnRowY, X2: btnStartX + btnW, Y2: btnRowY + 1,
			ID: fmt.Sprintf("focus-btn-%d", i), Kind: "button",
		})
		btnStartX += btnW + 2

		if i == m.focusButton {
			renderedBtns = append(renderedBtns, activeButtonStyle.Render(label))
		} else {
			renderedBtns = append(renderedBtns, buttonStyle.Render(label))
		}
	}
	buttonsLine := strings.Join(renderedBtns, "  ")
	filled := 0
	barWidth := max(10, inner-8)
	if total > 0 {
		filled = elapsed * barWidth / (total * 60)
		if filled == 0 && elapsed > 0 {
			filled = 1
		}
		if filled > barWidth {
			filled = barWidth
		}
	}
	barStr := strings.Repeat("▓", filled) + strings.Repeat("░", barWidth-filled)
	progressBar := cyanStyle.Render(barStr)
	timerBody := lipgloss.JoinVertical(lipgloss.Left,
		timerDigits,
		"",
		progressBar,
		"",
		buttonsLine,
		"",
		fmt.Sprintf("Duration: [ %02d ] min      Break: [ %02d ] min", m.focusDuration, m.focusBreak),
	)
	m.clickableRegions = append(m.clickableRegions, ClickableRegion{
		X1: 1, Y1: 5, X2: width - 1, Y2: height - 1,
		ID: "panel-0", Kind: "panel",
	})

	return panelWithHover("FOCUS TIMER", status, inner, "\n"+timerBody+"\n", true, m.hoveredPanel == 0)
}
