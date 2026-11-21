package tui

import (
	"focusd/coreapi"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func StartTUI() error {
	m := NewModel()
	defer func() {
		_ = coreapi.RemoveTUIPresence(m.clientID)
		_, _ = coreapi.Call(coreapi.Request{Command: "tui-release", ClientID: m.clientID})
	}()
	p := tea.NewProgram(
		&m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithInput(os.Stdin),
		tea.WithOutput(os.Stdout),
	)
	_, err := p.Run()
	if err == nil && m.updateRequested {
		_ = coreapi.RemoveTUIPresence(m.clientID)
		_, _ = coreapi.Call(coreapi.Request{Command: "tui-release", ClientID: m.clientID})
		return launchGitHubUpdate()
	}
	return err
}
