package tui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"focusd/system"

	tea "github.com/charmbracelet/bubbletea"
)

func TestFormsAcceptUnicodeRunesAndPaste(t *testing.T) {
	for _, kind := range []string{"limit", "browser", "whitelist", "modal"} {
		t.Run(kind, func(t *testing.T) {
			m := Model{}
			switch kind {
			case "limit":
				m.activeTab = tabLimits
				m.limitForm = limitForm{Visible: true}
			case "browser":
				m.activeTab = tabSettings
				m.settingsAddingBrowser = true
			case "whitelist":
				m.activeTab = tabSettings
				m.settingsAddingWhitelist = true
			case "modal":
				m.modal = modal{Active: true, Title: "TEST", Required: "CONFIRM"}
			}
			text := "über 记事本"
			m, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text), Paste: true})
			switch kind {
			case "limit":
				if m.limitForm.App != text {
					t.Fatalf("input = %q; want %q", m.limitForm.App, text)
				}
			case "browser":
				if m.settingsBrowserInput != text {
					t.Fatalf("input = %q; want %q", m.settingsBrowserInput, text)
				}
			case "whitelist":
				if m.settingsWhitelistInput != text {
					t.Fatalf("input = %q; want %q", m.settingsWhitelistInput, text)
				}
			case "modal":
				if m.modal.Input != text {
					t.Fatalf("input = %q; want %q", m.modal.Input, text)
				}
			}
		})
	}
}

func TestFormBackspaceKeepsValidUnicode(t *testing.T) {
	cases := []struct {
		name  string
		model Model
		read  func(Model) string
	}{
		{"limit", Model{activeTab: tabLimits, limitForm: limitForm{Visible: true, App: "code记"}}, func(m Model) string { return m.limitForm.App }},
		{"browser", Model{activeTab: tabSettings, settingsAddingBrowser: true, settingsBrowserInput: "code记"}, func(m Model) string { return m.settingsBrowserInput }},
		{"whitelist", Model{activeTab: tabSettings, settingsAddingWhitelist: true, settingsWhitelistInput: "code记"}, func(m Model) string { return m.settingsWhitelistInput }},
		{"modal", Model{modal: modal{Active: true, Input: "code记"}}, func(m Model) string { return m.modal.Input }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := tc.model.handleKey(tea.KeyMsg{Type: tea.KeyBackspace})
			got := tc.read(m)
			if got != "code" || !utf8.ValidString(got) {
				t.Fatalf("backspace = %q; want valid UTF-8 %q", got, "code")
			}
		})
	}
}

func TestFormPasteDoesNotTriggerGlobalShortcuts(t *testing.T) {
	m := Model{activeTab: tabLimits, limitForm: limitForm{Visible: true}}
	m, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q?5"), Paste: true})
	if m.limitForm.App != "q?5" || m.showHelp || m.activeTab != tabLimits || cmd != nil {
		t.Fatalf("paste invoked a command or lost text: %+v", m.limitForm)
	}
}

func TestFormUnicodeLimitCountsRunes(t *testing.T) {
	m := Model{activeTab: tabLimits, limitForm: limitForm{Visible: true}}
	m, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(strings.Repeat("记", 40)), Paste: true})
	if got := utf8.RuneCountInString(m.limitForm.App); got != 32 || !utf8.ValidString(m.limitForm.App) {
		t.Fatalf("input rune count = %d; want 32 valid runes", got)
	}
}

func TestLimitFormEnterSubmitsFromEveryField(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Cleanup(func() { _ = system.SetAppTimeLimit("code", 0) })
	for field := range 4 {
		m := &Model{activeTab: tabLimits, limitForm: limitForm{Visible: true, Field: field, App: "code", Minutes: field + 1}}
		_, cmd := m.handleLimitsKey("enter")
		if m.limitForm.Visible || cmd == nil {
			t.Fatalf("field %d did not submit valid limit", field)
		}
		if got := system.GetAppTimeLimits()["code"]; got != field+1 {
			t.Fatalf("field %d saved %d minutes; want %d", field, got, field+1)
		}
	}
}

func TestFormPasteFiltersControlCharacters(t *testing.T) {
	m := Model{activeTab: tabLimits, limitForm: limitForm{Visible: true}}
	m, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("code\n\x1b[31m\t记"), Paste: true})
	if m.limitForm.App != "code[31m记" {
		t.Fatalf("control characters reached input: %q", m.limitForm.App)
	}
}

func TestLimitNumericPasteIgnoresNonDigits(t *testing.T) {
	m := Model{activeTab: tabLimits, limitForm: limitForm{Visible: true, Field: 2}}
	m, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1x2"), Paste: true})
	if m.limitForm.Minutes != 12 {
		t.Fatalf("numeric paste = %d; want 12", m.limitForm.Minutes)
	}
}

func TestDirectFormHandlerAcceptsUnicodeCharacter(t *testing.T) {
	m := &Model{limitForm: limitForm{Visible: true}}
	m.handleLimitsKey("记")
	if m.limitForm.App != "记" {
		t.Fatalf("limit character = %q", m.limitForm.App)
	}
	m = &Model{settingsAddingBrowser: true}
	m.handleSettingsKey("记")
	if m.settingsBrowserInput != "记" {
		t.Fatalf("browser character = %q", m.settingsBrowserInput)
	}
}

func TestInvalidLimitFormEnterPreservesDraft(t *testing.T) {
	m := &Model{activeTab: tabLimits, limitForm: limitForm{Visible: true, Field: 1, App: "code"}}
	_, cmd := m.handleLimitsKey("enter")
	if !m.limitForm.Visible || cmd != nil || len(m.toasts) == 0 {
		t.Fatal("invalid limit should retain draft and report validation")
	}
}

func TestLimitFormRenameUpdatesOneLimit(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	if err := system.SetAppTimeLimit("Firefox.exe", 15); err != nil {
		t.Fatal(err)
	}
	m := &Model{limitForm: limitForm{Visible: true, Editing: true, Field: 1, App: "firefox|abcd1234", OriginalApp: "Firefox.exe", Minutes: 20}}
	_, cmd := m.handleLimitsKey("enter")
	limits := system.GetAppTimeLimits()
	if m.limitForm.Visible || cmd == nil || len(limits) != 1 || limits["firefox|abcd1234"] != 20 {
		t.Fatalf("rename left unexpected limits: %#v", limits)
	}
}

func TestDashboardTabVisitsEveryPanel(t *testing.T) {
	for _, key := range []tea.KeyType{tea.KeyTab, tea.KeyShiftTab} {
		m := Model{activeTab: tabDashboard}
		seen := map[int]bool{}
		for range 5 {
			seen[m.panelFocus] = true
			m, _ = m.handleKey(tea.KeyMsg{Type: key})
		}
		if len(seen) != 5 || m.panelFocus != 0 {
			t.Fatalf("key %v visited %v; want all five panels and wrap to zero", key, seen)
		}
	}
}
