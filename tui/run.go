package tui

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func StartTUI() error {
	m := NewModel()
	p := tea.NewProgram(
		&m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithInput(os.Stdin),
		tea.WithOutput(os.Stdout),
	)
	_, err := p.Run()
	return err
}
