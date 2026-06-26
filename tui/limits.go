package tui

import (
	"fmt"
	"focusd/storage"
	"focusd/system"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

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

func (m Model) limitRows() []appUsage {
	limits := system.GetAppTimeLimits()
	rows := make([]appUsage, 0, len(limits))
	for app, mins := range limits {
		rows = append(rows, appUsage{Name: app, Opens: mins, Duration: storage.GetAppUsageTodayMinutes(app) * 60})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return rows
}
