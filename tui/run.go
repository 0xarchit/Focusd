package tui

import (
	"focusd/system"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func StartTUI() error {
	w, h := system.DisableConsoleScrollback()
	defer system.RestoreConsoleBufferSize(w, h)

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
