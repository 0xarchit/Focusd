package tui

import tea "github.com/charmbracelet/bubbletea"

func StartTUI() error {
	p := tea.NewProgram(NewModel(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
