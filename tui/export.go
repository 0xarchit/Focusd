package tui

import (
	"focusd/coreapi"
	tea "github.com/charmbracelet/bubbletea"
	"time"
)

type exportDoneMsg struct {
	path string
	err  error
}

func exportData(jsonOut bool) (string, error) {
	format := coreapi.ExportCSV
	if jsonOut {
		format = coreapi.ExportJSON
	}
	return coreapi.ExportUsage(coreapi.ExportOptions{Format: format})
}
func (m Model) exportCmd(allHistory bool) tea.Cmd {
	options := coreapi.ExportOptions{Format: coreapi.ExportCSV}
	if m.settingsExportSelected == 1 {
		options.Format = coreapi.ExportJSON
	}
	if !allHistory {
		options.StartDate, options.EndDate, _ = statsRangeDates(m.statsRange, m.statsCustomFrom, m.statsCustomTo, time.Now())
	}
	export := m.exportUsage
	if export == nil {
		export = coreapi.ExportUsage
	}
	return func() tea.Msg { path, err := export(options); return exportDoneMsg{path, err} }
}
func (m Model) handleExportKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.modal = modal{}
	case "left", "right", "tab":
		m.settingsExportSelected = (m.settingsExportSelected + 1) % 2
	case "1":
		m.settingsExportSelected = 0
	case "2":
		m.settingsExportSelected = 1
	case "enter":
		m.modal = modal{}
		return m, m.exportCmd(false)
	}
	if m.modal.Active {
		m.modal.Message = "Export selected Stats range: " + renderExportButtons(true, m.settingsExportSelected) + ". Enter exports; Esc cancels."
	}
	return m, nil
}
