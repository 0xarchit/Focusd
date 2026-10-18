package tui

import "testing"

// TestSettingsStopReturnsCommand pins that stopping the daemon from the TUI
// must hand back a command for the runtime to run, not perform the multi-second
// IPC round trip inline, which froze the UI. The command is deliberately not
// executed here - it would stop a real daemon if one is running.
func TestSettingsStopReturnsCommand(t *testing.T) {
	m := &Model{daemonActive: true}
	_, cmd := m.handleSettingsKey("space")
	if cmd == nil {
		t.Fatal("stopping the daemon should return a command")
	}
	if m.daemonActive != true {
		t.Error("the status should only change when the command reports back")
	}
}
