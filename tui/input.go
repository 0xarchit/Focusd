package tui

import (
	"unicode"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
)

func appendInputRunes(value string, runes []rune, limit int) string {
	count := utf8.RuneCountInString(value)
	for _, r := range runes {
		if count >= limit {
			break
		}
		if unicode.IsControl(r) {
			continue
		}
		value += string(r)
		count++
	}
	return value
}

func deleteInputRune(value string) string {
	if value == "" {
		return value
	}
	_, size := utf8.DecodeLastRuneInString(value)
	return value[:len(value)-size]
}

func inputRunesForKey(key string) []rune {
	if key == "space" {
		return []rune{' '}
	}
	if utf8.RuneCountInString(key) == 1 {
		return []rune(key)
	}
	return nil
}

func (m *Model) insertFormRunes(msg tea.KeyMsg) bool {
	if msg.Type != tea.KeyRunes || msg.Alt {
		return false
	}
	if m.activeTab == tabLimits && m.limitForm.Visible {
		if m.limitForm.Field == 0 {
			m.limitForm.App = appendInputRunes(m.limitForm.App, msg.Runes, 32)
		} else if m.limitForm.Field == 1 || m.limitForm.Field == 2 {
			for _, r := range msg.Runes {
				if r >= '0' && r <= '9' {
					m.handleLimitsKey(string(r))
				}
			}
		}
		return true
	}
	if m.activeTab == tabSettings && m.settingsAddingBrowser {
		m.settingsBrowserInput = appendInputRunes(m.settingsBrowserInput, msg.Runes, 32)
		return true
	}
	if m.activeTab == tabSettings && m.settingsAddingWhitelist {
		m.settingsWhitelistInput = appendInputRunes(m.settingsWhitelistInput, msg.Runes, 32)
		return true
	}
	return false
}
