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
		if m.focusButton == -1 {
			// adjust break interval
			enabled := system.GetBreakReminderEnabled()
			mins := max(5, system.GetBreakReminderMinutes()-5)
			if err := system.SetBreakReminder(enabled, mins); err != nil {
				m.addToast("Failed to adjust break interval: "+err.Error(), toastError)
			}
		} else {
			m.focusButton = max(0, m.focusButton-1)
		}
	case "l", "right":
		if m.focusButton == -1 {
			enabled := system.GetBreakReminderEnabled()
			mins := min(300, system.GetBreakReminderMinutes()+5)
			if err := system.SetBreakReminder(enabled, mins); err != nil {
				m.addToast("Failed to adjust break interval: "+err.Error(), toastError)
			}
		} else {
			m.focusButton = min(2, m.focusButton+1)
		}
	case "tab":
		// toggle between button row and break row
		if m.focusButton == -1 {
			m.focusButton = 0
		} else {
			m.focusButton = -1
		}
	case "b":
		enabled := system.GetBreakReminderEnabled()
		if err := system.SetBreakReminder(!enabled, system.GetBreakReminderMinutes()); err != nil {
			m.addToast("Failed to toggle break reminder: "+err.Error(), toastError)
		}
	case "s":
		m.focusButton = 0
		return m.triggerFocusButton()
	case "x":
		m.focusButton = 1
		return m.triggerFocusButton()
	case "r":
		m.focusButton = 2
		return m.triggerFocusButton()
	case "enter":
		if m.focusButton >= 0 {
			return m.triggerFocusButton()
		}
	case "up":
		m.focusDuration = min(180, m.focusDuration+1)
	case "down":
		m.focusDuration = max(1, m.focusDuration-1)
	}
	return *m, nil
}

func (m *Model) triggerFocusButton() (Model, tea.Cmd) {
	switch m.focusButton {
	case 0:
		return m.startFocus()
	case 1, 2:
		action := "stop"
		toastType := toastWarning
		msg := "Focus timer stopped"
		if m.focusButton == 2 {
			action = "reset"
			toastType = toastInfo
			msg = "Focus timer reset"
		}
		if err := core.StopPomodoro(); err != nil {
			m.addToast(fmt.Sprintf("Failed to %s timer: %v", action, err), toastError)
		} else {
			m.addToast(msg, toastType)
		}
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
	timerDigits := renderBlockTimer(mins, secs)

	buttons := []string{"▶ START", "■ STOP", "⟳ RESET"}
	var renderedBtns []string
	btnRowY := 5 + 3 + 5 + 2 // Panel title and margins offset
	btnStartX := 4
	for i, b := range buttons {
		label := "[ " + b + " ]"
		btnW := lipgloss.Width(label)
		m.clickableRegions = append(m.clickableRegions, clickableRegion{
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
	breakEnabled := system.GetBreakReminderEnabled()
	breakMins := system.GetBreakReminderMinutes()

	breakToggle := mutedStyle.Render("[·] Off")
	if breakEnabled {
		breakToggle = greenStyle.Render("[✓] On ")
	}

	breakRow := fmt.Sprintf("Break Reminder: %s   Interval: ", breakToggle)
	if m.focusButton == -1 {
		breakRow += cyanStyle.Render(fmt.Sprintf("[ ← %d min → ]", breakMins))
		breakRow += mutedStyle.Render("  (b=toggle  ←→=interval  Tab=back)")
	} else {
		breakRow += fmt.Sprintf("%d min", breakMins)
		breakRow += mutedStyle.Render("  (b=toggle  Tab=adjust interval)")
	}

	timerBody := lipgloss.JoinVertical(lipgloss.Left,
		timerDigits,
		"",
		progressBar,
		"",
		fmt.Sprintf("Focus Duration: [ %02d ] min   (↑↓ to adjust)", m.focusDuration),
		"",
		buttonsLine,
		"",
		breakRow,
	)
	m.clickableRegions = append(m.clickableRegions, clickableRegion{
		X1: 1, Y1: 5, X2: width - 1, Y2: height - 1,
		ID: "panel-0", Kind: "panel",
	})

	return panelWithHover("FOCUS TIMER", status, inner, "\n"+timerBody+"\n", true, m.hoveredPanel == 0)
}
