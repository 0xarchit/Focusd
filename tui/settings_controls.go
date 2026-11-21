package tui

import (
	"fmt"
	"focusd/coreapi"
	"focusd/system"
	tea "github.com/charmbracelet/bubbletea"
	"sort"
	"strings"
)

var settingsRows = []string{"Background service", "Auto-start on login", "Activity tracking", "Whitelisted apps", "Tracked browsers", "Data retention", "Database path", "Export data", "Update app", "Tracking history", "Privacy mode", "Idle threshold", "Track while idle", "App display names", "Site display names", "Review held cleanup"}

func (m Model) additionalSettingValue(index int) string {
	switch index {
	case 10:
		return "[ ← " + system.GetPrivacyMode() + " → ]"
	case 11:
		return fmt.Sprintf("[ ← %d minutes → ]", system.GetIdleThresholdMinutes())
	case 12:
		return toggleText(system.GetTrackWhileIdle())
	case 13, 14:
		entries := m.overrideEntries(index)
		if len(entries) == 0 {
			return "[ Add label ]"
		}
		i := min(m.settingsOverrideSelected, len(entries)-1)
		return truncate(entries[i], 34) + "  [Enter edit] [d remove] [+ add]"
	case 15:
		return "[ Preview and confirm cleanup ]"
	}
	return ""
}
func (m Model) overrideEntries(index int) []string {
	entries := system.GetDisplayOverrides()
	if index == 14 {
		entries = system.GetSiteOverrides()
	}
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
func (m *Model) handleAdditionalSetting(key string) (Model, tea.Cmd) {
	index := m.settingsSelected
	if key == "d" && index != 13 && index != 14 {
		return *m, nil
	}
	delta := 1
	if key == "left" || key == "h" {
		delta = -1
	}
	var err error
	switch index {
	case 10:
		modes := []string{system.PrivacyModeTitles, system.PrivacyModeCategories, system.PrivacyModeStrict}
		current := 0
		for i, value := range modes {
			if value == system.GetPrivacyMode() {
				current = i
			}
		}
		err = system.SetPrivacyMode(modes[(current+delta+len(modes))%len(modes)])
	case 11:
		err = system.SetIdleThreshold(system.GetIdleThresholdMinutes() + delta)
	case 12:
		err = system.SetTrackWhileIdle(!system.GetTrackWhileIdle())
	case 13, 14:
		entries := m.overrideEntries(index)
		if key == "left" || key == "right" || key == "h" || key == "l" {
			m.settingsOverrideSelected = min(max(0, m.settingsOverrideSelected+delta), max(0, len(entries)-1))
			return *m, nil
		}
		selected := ""
		if len(entries) > 0 {
			selected = entries[min(m.settingsOverrideSelected, len(entries)-1)]
		}
		if key == "d" {
			if selected != "" {
				if index == 13 {
					err = system.SetDisplayOverride(selected, "")
				} else {
					err = system.SetSiteOverride(selected, "")
				}
			}
		} else {
			m.settingsEditor = "APP DISPLAY NAME"
			values := system.GetDisplayOverrides()
			if index == 14 {
				m.settingsEditor = "SITE DISPLAY NAME"
				values = system.GetSiteOverrides()
			}
			m.settingsEditorKey = selected
			m.settingsEditorValue = values[selected]
			m.settingsEditorField = 0
			m.settingsEditorOriginalKey = selected
			return *m, nil
		}
	case 15:
		if !m.daemonActive {
			m.addToast("Start the background service to review cleanup", toastWarning)
			return *m, nil
		}
		return *m, m.rpcCmd(coreapi.Request{Command: "retention-preview", Generation: m.daemonGeneration})
	}
	if err != nil {
		m.addToast(err.Error(), toastError)
	} else {
		m.addToast("Setting saved", toastSuccess)
	}
	return *m, nil
}
func (m Model) handleSettingsEditor(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.settingsEditor = ""
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "tab", "shift+tab":
		m.settingsEditorField = (m.settingsEditorField + 1) % 2
	case "backspace":
		if m.settingsEditorField == 0 {
			if m.settingsEditorOriginalKey != "" {
				return m, nil
			}
			m.settingsEditorKey = deleteInputRune(m.settingsEditorKey)
		} else {
			m.settingsEditorValue = deleteInputRune(m.settingsEditorValue)
		}
	case "enter":
		key := strings.TrimSpace(m.settingsEditorKey)
		if key == "" {
			m.addToast("App identity or site name is required", toastError)
			return m, nil
		}
		var err error
		if m.settingsEditor == "APP DISPLAY NAME" {
			err = system.SetDisplayOverride(key, m.settingsEditorValue)
		} else {
			err = system.SetSiteOverride(key, m.settingsEditorValue)
		}
		if err != nil {
			m.addToast(err.Error(), toastError)
			return m, nil
		}
		m.settingsEditor = ""
		m.addToast("Display label saved", toastSuccess)
		return m, tea.Batch(loadDashboard(), m.loadStatsCmd())
	default:
		if msg.Type == tea.KeyRunes && !msg.Alt {
			if m.settingsEditorField == 0 {
				if m.settingsEditorOriginalKey != "" {
					return m, nil
				}
				m.settingsEditorKey = appendInputRunes(m.settingsEditorKey, msg.Runes, 128)
			} else {
				m.settingsEditorValue = appendInputRunes(m.settingsEditorValue, msg.Runes, 128)
			}
		}
	}
	return m, nil
}
func (m Model) renderSettingsEditor() string {
	keyLabel := "App ID (exe|install hash)"
	if m.settingsEditor == "SITE DISPLAY NAME" {
		keyLabel = "Recorded site name or domain"
	}
	lines := []string{keyLabel + ": " + m.settingsEditorKey, "Display label: " + m.settingsEditorValue, "", "Tab switches field; Enter saves; Esc cancels.", "An empty label removes the override."}
	lines[m.settingsEditorField] = "> " + lines[m.settingsEditorField]
	return panelWithHover(m.settingsEditor, "", min(72, m.width-2), strings.Join(lines, "\n"), true, false)
}
