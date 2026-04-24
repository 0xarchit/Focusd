package tui

import (
	"fmt"
	"focusd/storage"
	"focusd/system"
	"github.com/charmbracelet/lipgloss"
)

func (m mainModel) renderSettings() string {
	leftPane := m.settingsList.View()
	
	retention := storage.GetRetentionDays()
	autoStart, _, _ := system.GetAutoStartEnabled()
	pathEnabled, _ := system.GetPathEnabled()

	rightPane := cardStyle.Width(m.width * 2 / 3).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("Configuration Details"),
			"Select a category on the left to modify settings.",
			"",
			"Current Status:",
			fmt.Sprintf("- Retention: %d days", retention),
			fmt.Sprintf("- Autostart: %v", autoStart),
			fmt.Sprintf("- PATH: %v", pathEnabled),
			fmt.Sprintf("- Whitelisted: %d apps", len(m.whitelist)),
			fmt.Sprintf("- Custom Browsers: %d", len(m.browsers)),
		),
	)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
}
