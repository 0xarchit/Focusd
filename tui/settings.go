package tui

import (
	"encoding/csv"
	"encoding/json"
	"focusd/core"
	"focusd/storage"
	"focusd/system"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) handleSettingsKey(key string) (Model, tea.Cmd) {
	if m.settingsAddingBrowser {
		switch key {
		case "esc":
			m.settingsAddingBrowser = false
			m.settingsBrowserInput = ""
		case "backspace":
			if len(m.settingsBrowserInput) > 0 {
				m.settingsBrowserInput = m.settingsBrowserInput[:len(m.settingsBrowserInput)-1]
			}
		case "enter":
			if strings.TrimSpace(m.settingsBrowserInput) == "" {
				m.addToast("Browser name is required", toastError)
				return *m, nil
			}
			if err := system.AddCustomBrowser(m.settingsBrowserInput); err != nil {
				m.addToast(err.Error(), toastError)
			} else {
				m.addToast("Added "+m.settingsBrowserInput, toastSuccess)
			}
			m.settingsAddingBrowser = false
			m.settingsBrowserInput = ""
		default:
			if len(key) == 1 && len(m.settingsBrowserInput) < 32 {
				m.settingsBrowserInput += key
			}
		}
		return *m, nil
	}

	switch key {
	case "j", "down":
		m.settingsSelected = min(m.settingsSelected+1, 8)
	case "k", "up":
		m.settingsSelected = max(0, m.settingsSelected-1)
	case "h", "left":
		switch m.settingsSelected {
		case 4:
			m.settingsBrowserSelected = max(0, m.settingsBrowserSelected-1)
		case 6:
			m.settingsExportSelected = 0
		}
	case "l", "right":
		switch m.settingsSelected {
		case 4:
			browsers := system.GetCustomBrowsersList()
			m.settingsBrowserSelected = min(m.settingsBrowserSelected+1, max(0, len(browsers)-1))
		case 6:
			m.settingsExportSelected = 1
		}
	case "space", "enter":
		switch m.settingsSelected {
		case 0:
			if m.daemonActive {
				if core.SendIPCCmd("stop") {
					m.addToast("Daemon stopped gracefully", toastSuccess)
				} else {
					m.addToast("Failed to stop daemon gracefully", toastError)
				}
			} else {
				m.addToast("Run focusd start to launch daemon", toastInfo)
			}
		case 1:
			enabled, _, _ := system.GetAutoStartEnabled()
			if enabled {
				if err := system.DisableAutoStart(); err != nil {
					m.addToast("Failed to disable auto-start", toastError)
				} else {
					m.addToast("Auto-start disabled", toastSuccess)
				}
			} else {
				if err := system.EnableAutoStart(); err != nil {
					m.addToast("Failed to enable auto-start", toastError)
				} else {
					m.addToast("Auto-start enabled", toastSuccess)
				}
			}
		case 2:
			paused := storage.IsPaused()
			if err := storage.SetPaused(!paused); err != nil {
				m.addToast("Failed to toggle browser tracking", toastError)
			} else {
				m.addToast("Browser tracking toggled", toastSuccess)
			}
		case 3:
			enabled := system.GetBreakReminderEnabled()
			minutes := system.GetBreakReminderMinutes()
			if err := system.SetBreakReminder(!enabled, minutes); err != nil {
				m.addToast("Failed to toggle focus alerts", toastError)
			} else {
				m.addToast("Focus alerts toggled", toastSuccess)
			}
		case 4:
			m.settingsAddingBrowser = true
			m.settingsBrowserInput = ""
		case 5:
			openDBFolder()
		case 6:
			jsonOut := m.settingsExportSelected == 1
			path, err := exportData(jsonOut)
			if err != nil {
				m.addToast("Export failed", toastError)
			} else {
				m.addToast("Exported to "+filepath.Base(path), toastSuccess)
			}
		case 7:
			if err := launchGitHubUpdate(); err != nil {
				m.addToast("Failed to launch updater", toastError)
			} else {
				m.addToast("Updater opened in new window", toastSuccess)
			}
		case 8:
			m.modal = modal{
				Active: true, Title: "WIPE DATA", Required: "DELETE", Danger: true,
				Message: "This will delete all tracking data and cannot be undone.",
			}
		}
	case "d":
		if m.settingsSelected == 4 {
			browsers := system.GetCustomBrowsersList()
			if len(browsers) > 0 {
				i := min(m.settingsBrowserSelected, len(browsers)-1)
				system.RemoveCustomBrowser(browsers[i])
				m.addToast("Removed "+browsers[i], toastWarning)
			}
		}
	}
	return *m, nil
}

func (m *Model) renderSettings(width, height int) string {
	auto, _, _ := system.GetAutoStartEnabled()
	paused := storage.IsPaused()
	focusAlerts := system.GetBreakReminderEnabled()
	browsers := append([]string{"chrome.exe", "firefox.exe", "edge.exe"}, system.GetCustomBrowsersList()...)
	dbPath := m.dashboard.DBPath
	if dbPath == "" {
		dbPath, _ = storage.GetDBPath()
	}
	inner := max(20, width-6)
	browserValue := truncate(strings.Join(browsers, "   "), max(12, inner-42)) + "   " + buttonText("+ Add Browser")
	if m.settingsAddingBrowser {
		browserValue = "New browser: [ " + cyanStyle.Render(padRight(m.settingsBrowserInput, 18)) + " ]"
	}

	lines := []string{
		"DAEMON",
		m.settingRow(0, m.settingsSelected, "Background service", statusPill(m.daemonActive)+"  "+buttonText(func() string {
			if m.daemonActive {
				return "Stop"
			}
			return "Start"
		}()), inner),
		m.settingRow(1, m.settingsSelected, "Auto-start on login", toggleText(auto), inner),
		"",
		"TRACKING",
		m.settingRow(2, m.settingsSelected, "Browser tracking", toggleText(!paused), inner),
		"",
		"NOTIFICATIONS",
		m.settingRow(3, m.settingsSelected, "Focus complete alert", toggleText(focusAlerts), inner),
		"",
		"BROWSERS",
		m.settingRow(4, m.settingsSelected, "Tracked browsers", browserValue, inner),
		"",
		"DATA",
		m.settingRow(5, m.settingsSelected, "Database path", mutedStyle.Render(truncate(dbPath, max(10, inner-45)))+"   "+buttonText("Open Folder"), inner),
		m.settingRow(6, m.settingsSelected, "Export data", renderExportButtons(m.settingsSelected == 6, m.settingsExportSelected), inner),
		m.settingRow(7, m.settingsSelected, "Update app", buttonText("Update from GitHub"), inner),
		"",
		"DANGER ZONE",
		m.settingRow(8, m.settingsSelected, "Danger zone", redStyle.Bold(true).Render("[ Uninstall / Wipe All Data ]"), inner),
	}
	for i, line := range lines {
		switch line {
		case "DAEMON", "TRACKING", "NOTIFICATIONS", "BROWSERS", "DATA":
			lines[i] = boldStyle.Render(line) + "\n" + mutedStyle.Render("────────────────────────────────────────")
		case "DANGER ZONE":
			lines[i] = redStyle.Bold(true).Render(line) + "\n" + redStyle.Render("────────────────────────────────────────")
		}
	}
	bodyLines := strings.Split(strings.Join(lines, "\n"), "\n")
	maxBodyLines := max(6, height-6)
	if len(bodyLines) > maxBodyLines {
		selectedLine := 0
		for i, line := range bodyLines {
			if strings.Contains(line, ">") {
				selectedLine = i
				break
			}
		}
		start := scrollStart(selectedLine, maxBodyLines-2, len(bodyLines))
		end := min(len(bodyLines), start+maxBodyLines-2)
		window := bodyLines[start:end]
		if start > 0 {
			window = append([]string{mutedStyle.Render("↑ more")}, window...)
		}
		if end < len(bodyLines) {
			window = append(window, mutedStyle.Render("↓ more"))
		}
		bodyLines = window
	}

	m.clickableRegions = append(m.clickableRegions, ClickableRegion{
		X1: 1, Y1: 5, X2: width - 1, Y2: height - 1,
		ID: "panel-0", Kind: "panel",
	})

	return panelWithHover("SETTINGS", "", width-2, "\n"+strings.Join(bodyLines, "\n")+"\n", true, m.hoveredPanel == 0)
}

func (m *Model) settingRow(index, selected int, label, value string, width int) string {
	cursor := "  "
	if index == selected {
		cursor = cyanStyle.Render("> ")
	}
	labelWidth := min(24, max(12, width/3))
	line := cursor + padRight(label, labelWidth) + value
	line = fitLine(line, width)
	if index == selected {
		return selectedRowStyle.Render(padRight(line, width))
	}
	return line
}

func statusPill(running bool) string {
	if running {
		return "[ " + greenStyle.Render("● RUNNING") + " ]"
	}
	return "[ " + redStyle.Render("○ STOPPED") + " ]"
}

func toggleText(on bool) string {
	if on {
		return "[" + greenStyle.Render("✓") + "] Enabled"
	}
	return "[" + mutedStyle.Render("·") + "] Disabled"
}

func buttonText(label string) string {
	return cyanStyle.Render("[ " + label + " ]")
}

func openDBFolder() {
	if dbPath, err := storage.GetDBPath(); err == nil {
		exec.Command("explorer.exe", filepath.Dir(dbPath)).Start()
	}
}

func launchGitHubUpdate() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command("cmd", "/c", "start", "", exePath, "update")
	return cmd.Start()
}

func exportData(jsonOut bool) (string, error) {
	exportDir := "."
	if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
		exportDir = filepath.Join(userProfile, "Desktop")
	}
	if jsonOut {
		path := filepath.Join(exportDir, "focusd_export.json")
		apps, err := storage.GetAllAppStats()
		if err != nil {
			return "", err
		}
		b, err := json.MarshalIndent(apps, "", "  ")
		if err != nil {
			return "", err
		}
		return path, os.WriteFile(path, b, 0644)
	}
	path := filepath.Join(exportDir, "focusd_export.csv")
	apps, err := storage.GetAllAppStats()
	if err != nil {
		return "", err
	}
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{"Date", "App Name", "Executable", "Duration (seconds)", "Open Count"}); err != nil {
		return "", err
	}
	for _, app := range apps {
		if err := w.Write([]string{app.Date, app.AppName, app.ExeName, strconv.Itoa(app.TotalDurationSecs), strconv.Itoa(app.OpenCount)}); err != nil {
			return "", err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	return path, nil
}

func renderExportButtons(active bool, selectedButton int) string {
	c := [2]string{"Export CSV", "Export JSON"}
	s := [2]string{cyanStyle.Render("[ " + c[0] + " ]"), cyanStyle.Render("[ " + c[1] + " ]")}
	if active {
		s[selectedButton] = greenStyle.Render("[ " + c[selectedButton] + " ]")
	}
	return s[0] + "   " + s[1]
}
