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

type limitRow struct {
	Name         string
	LimitMinutes int
	UsedSeconds  int
}

func (m *Model) handleLimitsKey(key string) (Model, tea.Cmd) {
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
			} else if m.limitForm.Field == 1 {
				m.limitForm.Hours /= 10
			} else if m.limitForm.Field == 2 {
				m.limitForm.Minutes /= 10
			}
		case "enter":
			if m.limitForm.Field == 3 {
				mins := m.limitForm.Hours*60 + m.limitForm.Minutes
				if m.limitForm.App != "" && mins > 0 {
					var originalMins int
					var hasOriginal bool
					if m.limitForm.Editing && m.limitForm.OriginalApp != "" && !strings.EqualFold(m.limitForm.App, m.limitForm.OriginalApp) {
						limits := system.GetAppTimeLimits()
						if val, ok := limits[m.limitForm.OriginalApp]; ok {
							originalMins = val
							hasOriginal = true
						}
						if err := system.SetAppTimeLimit(m.limitForm.OriginalApp, 0); err != nil {
							m.addToast("Failed to rename limit: "+err.Error(), toastError)
							return *m, nil
						}
					}
					if err := system.SetAppTimeLimit(m.limitForm.App, mins); err != nil {
						if hasOriginal {
							_ = system.SetAppTimeLimit(m.limitForm.OriginalApp, originalMins)
						}
						m.addToast("Failed to save limit: "+err.Error(), toastError)
						return *m, nil
					}
					m.addToast("Limit saved for "+m.limitForm.App, toastSuccess)
					m.limitForm = limitForm{}
					return *m, loadDashboard()
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
							if m.limitForm.Hours*10+n > 24 {
								m.limitForm.Hours = 24
							} else {
								m.limitForm.Hours = m.limitForm.Hours*10 + n
							}
						} else {
							if m.limitForm.Minutes*10+n > 59 {
								m.limitForm.Minutes = 59
							} else {
								m.limitForm.Minutes = m.limitForm.Minutes*10 + n
							}
						}
					}
				}
			} else if key == "space" || key == " " {
				if m.limitForm.Field == 0 && len(m.limitForm.App) < 32 {
					m.limitForm.App += " "
				}
			}
		}
		return *m, nil
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
			m.limitForm = limitForm{Visible: true, Editing: true, App: row.Name, OriginalApp: row.Name, Hours: row.LimitMinutes / 60, Minutes: row.LimitMinutes % 60}
		}
	case "d":
		rows := m.limitRows()
		if len(rows) > 0 {
			appName := rows[m.limitsSelected].Name
			if err := system.SetAppTimeLimit(appName, 0); err != nil {
				m.addToast("Failed to delete limit: "+err.Error(), toastError)
				return *m, nil
			}
			m.addToast("Limit deleted for "+appName, toastWarning)
			return *m, loadDashboard()
		}
	}
	return *m, nil
}

func (m *Model) renderLimits(width, height int) string {
	rows := m.limitRows()
	warnAt := 80
	maxW := width - 2
	var lines []string

	col1W := (maxW * 30) / 100
	col2W := (maxW * 15) / 100
	col3W := (maxW * 15) / 100
	col4W := (maxW * 15) / 100
	col5W := maxW - col1W - col2W - col3W - col4W - 4

	headerPlain := padRight("App", col1W) + padLeft("Limit", col2W) + padLeft("Used", col3W) + padLeft("Remaining", col4W) + "   Status"
	lines = append(lines, boldStyle.Render(headerPlain))
	lines = append(lines, mutedStyle.Render(strings.Repeat("─", max(0, maxW-4))))

	visibleRows := max(3, height-10)
	if m.limitForm.Visible {
		visibleRows = max(3, height-18)
	}

	start := scrollStart(m.limitsSelected, visibleRows, len(rows))
	end := min(len(rows), start+visibleRows)
	for i := start; i < end; i++ {
		r := rows[i]
		limitSecs := r.LimitMinutes * 60
		used := r.UsedSeconds
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

		barWidth := max(4, col5W-12)
		filled := 0
		if pct > 0 {
			filled = pct * barWidth / 100
			if filled == 0 {
				filled = 1
			}
			if filled > barWidth {
				filled = barWidth
			}
		}
		barStr := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
		line := padRight(truncate(r.Name, col1W), col1W) +
			padLeft(formatDuration(limitSecs), col2W) +
			padLeft(formatDuration(used), col3W) +
			padLeft(formatDuration(remaining), col4W) + "  " +
			style.Render(barStr) + "  " + style.Render(status)
		if pct >= 95 {
			line = dangerRowStyle.Render(padRight(line, maxW-2))
		} else if i == m.limitsSelected && !m.limitForm.Visible {
			line = selectedRowStyle.Render(padRight(line, maxW-2))
		}
		lines = append(lines, line)
	}

	if len(rows) == 0 {
		lines = append(lines, "", mutedStyle.Render("No limits set. Press n to add one."))
	}

	tableY2 := 5 + height - 2
	if m.limitForm.Visible {
		tableY2 = 5 + height - 9
	}
	m.clickableRegions = append(m.clickableRegions, clickableRegion{
		X1: 1, Y1: 5, X2: width - 1, Y2: tableY2,
		ID: "panel-0", Kind: "panel",
	})

	table := panelWithHover("APP LIMITS", fmt.Sprintf("(%d limits set)", len(rows)), maxW, "\n"+strings.Join(lines, "\n")+"\n", m.panelFocus == 0, m.hoveredPanel == 0)
	if !m.limitForm.Visible {
		return table
	}

	formTitle := "ADD LIMIT"
	if m.limitForm.Editing {
		formTitle = "EDIT LIMIT"
	}
	appNameVal := padRight(m.limitForm.App, 18)
	if m.limitForm.Field == 0 {
		appNameVal = cyanStyle.Bold(true).Render(appNameVal)
	}

	hVal := fmt.Sprintf("%02d", m.limitForm.Hours)
	mVal := fmt.Sprintf("%02d", m.limitForm.Minutes)
	if m.limitForm.Field == 1 {
		hVal = cyanStyle.Bold(true).Render(hVal)
	} else if m.limitForm.Field == 2 {
		mVal = cyanStyle.Bold(true).Render(mVal)
	}

	saveBtn := "[ SAVE (Enter) ]"
	cancelBtn := "[ CANCEL (Esc) ]"
	if m.limitForm.Field == 3 {
		saveBtn = cyanStyle.Bold(true).Render(saveBtn)
	}

	fields := []string{
		"App name:    [ " + appNameVal + " ]",
		fmt.Sprintf("Daily limit: [ %s ] h  [ %s ] m", hVal, mVal),
		saveBtn + "   " + cancelBtn,
		"",
		mutedStyle.Render("  Hint: Enter the process/exe name (e.g. 'chrome' or 'code')"),
	}

	formY := 5 + height - 8
	m.clickableRegions = append(m.clickableRegions, clickableRegion{
		X1: 1, Y1: formY, X2: width - 1, Y2: height - 1,
		ID: "panel-1", Kind: "panel",
	})

	return lipgloss.JoinVertical(lipgloss.Left, table, panelWithHover(formTitle, "", maxW, "\n"+strings.Join(fields, "\n")+"\n", m.panelFocus == 1, m.hoveredPanel == 1))
}

func (m *Model) limitRows() []limitRow {
	limits := system.GetAppTimeLimits()
	rows := make([]limitRow, 0, len(limits))
	for app, mins := range limits {
		rows = append(rows, limitRow{Name: app, LimitMinutes: mins, UsedSeconds: storage.GetAppUsageTodaySeconds(app)})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return rows
}
