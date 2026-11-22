package system

import (
	"errors"
	"testing"
)

func TestFrameHostResolvesOnlyApplicationChild(t *testing.T) {
	children := []frameChild{{PID: 11, Class: "ApplicationFrameInputSinkWindow"}, {PID: 22, Class: "Windows.UI.Core.CoreWindow"}}
	pid := frameApplicationPID(11, children, func(pid uint32) bool { return pid == 22 })
	if pid != 22 {
		t.Fatalf("resolved PID = %d; want application PID 22", pid)
	}
}

func TestFrameHostLeavesUnresolvedHostAlone(t *testing.T) {
	for _, children := range [][]frameChild{nil, {{PID: 11, Class: "Windows.UI.Core.CoreWindow"}}, {{PID: 22, Class: "OtherWindow"}}, {{PID: 22, Class: "Windows.UI.Core.CoreWindow"}, {PID: 23, Class: "Windows.UI.Core.CoreWindow"}}} {
		if got := frameApplicationPID(11, children, func(uint32) bool { return true }); got != 11 {
			t.Errorf("resolved ambiguous children to %d; want host 11", got)
		}
	}
	if got := frameApplicationPID(11, []frameChild{{PID: 22, Class: "Windows.UI.Core.CoreWindow"}}, func(uint32) bool { return false }); got != 11 {
		t.Errorf("unreadable child resolved to %d", got)
	}
}

func TestSessionStateRejectsLockAndDisconnect(t *testing.T) {
	tests := []struct {
		name       string
		connected  bool
		input, own string
		err        error
		active     bool
	}{
		{"normal desktop", true, "Default", "Default", nil, true},
		{"same custom desktop", true, "Work", "Work", nil, true},
		{"secure desktop", true, "Winlogon", "Default", nil, false},
		{"disconnected", false, "Default", "Default", nil, false},
		{"desktop inaccessible", true, "", "Default", errors.New("access denied"), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state, err := sessionStateFor(tc.connected, tc.input, tc.own, tc.err)
			if state.Active() != tc.active {
				t.Errorf("active = %v; want %v", state.Active(), tc.active)
			}
			if tc.err != nil && err == nil {
				t.Error("desktop error must remain visible")
			}
		})
	}
}

func TestSessionStateReadOnlyObservation(t *testing.T) {
	state, err := ReadSessionState()
	if err != nil && state.Active() {
		t.Fatal("failed desktop observation must not be active")
	}
	if state.Active() && state.Desktop == "" {
		t.Fatal("active desktop has no name")
	}
}

func TestSessionScreenFallbackDoesNotIgnoreShellApps(t *testing.T) {
	for _, name := range []string{"LockApp.exe", "LOGONUI.EXE"} {
		if !isSessionScreenExecutable(name) {
			t.Errorf("session screen %s is recordable", name)
		}
	}
	for _, name := range []string{"explorer.exe", "SearchHost.exe", "ApplicationFrameHost.exe", "code.exe"} {
		if isSessionScreenExecutable(name) {
			t.Errorf("ordinary shell/app %s was excluded", name)
		}
	}
}
