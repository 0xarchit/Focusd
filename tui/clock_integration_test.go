package tui

import (
	"encoding/json"
	"errors"
	"focusd/coreapi"
	"focusd/storage"
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	"testing"
	"time"
)

func incidentState(revision int64) storage.ClockState {
	return storage.ClockState{Incident: &storage.ClockIncident{ID: "incident", Revision: revision, Reason: "clock moved backward", LastSafe: time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC), Current: storage.ClockCheckpoint{WallUTC: time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)}}}
}
func TestClockPromptPreservesDraftAcrossSplashHelpAndSmallTerminal(t *testing.T) {
	m := Model{width: 60, height: 12, splash: true, showHelp: true, activeTab: tabLimits, limitForm: limitForm{Visible: true, App: "编辑器.exe"}}
	m.acceptClockStatus(incidentState(1))
	view := m.View()
	if !strings.Contains(view, "CLOCK WARNING") || !strings.Contains(view, "Continue") || !strings.Contains(view, "Reset") || !strings.Contains(view, "Off") {
		t.Fatalf("choices missing: %s", view)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(*Model)
	if got.clockPrompt || got.limitForm.App != "编辑器.exe" || !got.showHelp || !got.splash {
		t.Fatalf("dismiss changed existing state: %+v", got.limitForm)
	}
	if !strings.Contains(got.View(), "tracking is paused") {
		t.Fatal("dismiss removed paused banner")
	}
	reopened, _ := got.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if !reopened.clockPrompt {
		t.Fatal("warning could not reopen over help")
	}
}
func TestClockResetRequiresSecondConfirmationAndReusesUncertainRequest(t *testing.T) {
	m := Model{daemonActive: true, daemonGeneration: "generation", clientID: strings.Repeat("a", 32)}
	m.acceptClockStatus(incidentState(2))
	m.callIPC = func(request coreapi.Request) (coreapi.Response, error) {
		if request.Confirm {
			return coreapi.Response{}, errors.New("reply lost")
		}
		return coreapi.Response{OK: true, State: "confirmation-required", Data: json.RawMessage(`{"confirm_token":"token"}`)}, nil
	}
	m, cmd := m.handleClockKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	if cmd == nil || m.clockRequest.Confirm {
		t.Fatal("reset skipped initial confirmation")
	}
	message := cmd().(daemonRPCMsg)
	m.handleRPCResult(message)
	if m.clockRequest.ConfirmToken != "token" || m.clockRequest.Confirm {
		t.Fatal("reset confirmation state missing")
	}
	m, cmd = m.handleClockKey(tea.KeyMsg{Type: tea.KeyEnter})
	requestID := m.clockRequest.RequestID
	if !m.clockRequest.Confirm || m.clockRequest.IncidentID != "incident" || m.clockRequest.Revision != 2 {
		t.Fatal("confirmed reset lost incident binding")
	}
	m.handleRPCResult(cmd().(daemonRPCMsg))
	m, cmd = m.handleClockKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || m.clockRequest.RequestID != requestID {
		t.Fatal("uncertain reset retry changed request ID")
	}
}
func TestNewClockRevisionInvalidatesUnsentConfirmation(t *testing.T) {
	m := Model{clockRequest: coreapi.Request{Command: "clock-resolve", RequestID: "old", ConfirmToken: "old"}}
	m.clockState = incidentState(1)
	m.acceptClockStatus(incidentState(2))
	if !m.clockPrompt || m.clockRequest.RequestID != "" || m.clockState.Incident.Revision != 2 {
		t.Fatal("stale clock decision retained")
	}
}

func TestLateResetConfirmationCannotReplaceNewClockRevision(t *testing.T) {
	m := Model{clockState: incidentState(3)}
	m.handleRPCResult(daemonRPCMsg{request: coreapi.Request{Command: "clock-resolve", Action: "reset", IncidentID: "incident", Revision: 2}, response: coreapi.Response{State: "confirmation-required", Data: json.RawMessage(`{"confirm_token":"stale"}`)}})
	if m.clockRequest.ConfirmToken != "" {
		t.Fatal("stale reset confirmation was accepted")
	}
}
func TestHealthyStatusPreservesOrdinaryResetConfirmation(t *testing.T) {
	m := Model{clockRequest: coreapi.Request{Command: "reset", RequestID: "reset-id", ConfirmToken: "token"}, modal: modal{Active: true, Title: "WIPE DATA"}}
	m.acceptClockStatus(storage.ClockState{})
	if m.clockRequest.RequestID != "reset-id" || m.clockRequest.ConfirmToken != "token" {
		t.Fatal("healthy polling erased wipe confirmation")
	}
}
func TestOperationFailureStopsPolling(t *testing.T) {
	m := Model{operationID: "operation", operationPending: true}
	m.handleRPCResult(daemonRPCMsg{request: coreapi.Request{Command: "operation-status"}, response: coreapi.Response{State: "failed", Error: "backup failed"}, err: errors.New("backup failed")})
	if m.operationID != "" || m.operationPending {
		t.Fatal("failed operation remains pending")
	}
}
func TestExportUsesSelectedRangeAndSettingsExportsAllHistory(t *testing.T) {
	m := Model{statsRange: 3, statsCustomFrom: "2024-01-01", statsCustomTo: "2024-12-31", settingsExportSelected: 1}
	var captured coreapi.ExportOptions
	m.exportUsage = func(options coreapi.ExportOptions) (string, error) { captured = options; return "test.json", nil }
	m, cmd := m.handleStatsKey("e")
	if cmd != nil || !m.modal.Active {
		t.Fatal("Stats export skipped format choice")
	}
	m, cmd = m.handleExportKey(tea.KeyMsg{Type: tea.KeyEnter})
	cmd()
	if captured.StartDate != "2024-01-01" || captured.EndDate != "2024-12-31" || captured.Format != coreapi.ExportJSON {
		t.Fatalf("wrong export options: %+v", captured)
	}
	m.exportCmd(true)()
	if captured.StartDate != "" || captured.EndDate != "" {
		t.Fatal("Settings export was range limited")
	}
	_, _, days := statsRangeDates(3, "2024-01-01", "2024-12-31", time.Now())
	if days != 366 {
		t.Fatalf("leap-year range clipped to %d days", days)
	}
}
func TestCompactDashboardShowsFocusedLowerPanel(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	for index, label := range map[int]string{3: "WEEKLY TREND", 4: "QUICK ACTIONS"} {
		m := Model{width: 80, height: 24, panelFocus: index, hoveredPanel: -1}
		view := m.View()
		if !strings.Contains(view, label) {
			t.Fatalf("focused %s panel hidden at 80x24", label)
		}
		region := regionByID(&m, "panel-"+string(rune('0'+index)))
		if region == nil || region.Y1 >= region.Y2 || region.Y2 > m.height-2 {
			t.Fatalf("invalid focused panel region: %+v", region)
		}
	}
}
func TestSettingsRowsScrollAndUpdateExitsBeforeLaunch(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	m := Model{activeTab: tabSettings, width: 80, height: 24, hoveredPanel: -1}
	for i := 0; i < 30; i++ {
		m, _ = m.handleSettingsKey("down")
	}
	if m.settingsSelected != len(settingsRows)-1 || !strings.Contains(m.View(), "Review held cleanup") {
		t.Fatal("last setting is unreachable")
	}
	m.settingsSelected = 8
	m, cmd := m.handleSettingsKey("enter")
	if !m.updateRequested || cmd == nil {
		t.Fatal("update did not schedule TUI exit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("update executed before terminal restoration")
	}
}

func TestDuplicateUsageLabelsPreserveInstallIdentityAndTieOrder(t *testing.T) {
	m := Model{stats: tuiData{Apps: []appUsage{{Name: "Editor", AppID: "editor.exe|bbb", Duration: 60}, {Name: "Editor", AppID: "editor.exe|aaa", Duration: 60}}}}
	apps := m.sortedStatsApps()
	if apps[0].AppID != "editor.exe|aaa" {
		t.Fatal("equal app labels have unstable identity order")
	}
	if usageLabel(apps[0], apps) == usageLabel(apps[1], apps) {
		t.Fatal("separate installs render with identical labels")
	}
}

func TestFocusTimerMutationRunsAsCommand(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	called := false
	m := Model{focusDuration: 25, changeTimer: func(start bool, minutes int) error {
		called = true
		if !start || minutes != 25 {
			t.Fatal("wrong timer intent")
		}
		return nil
	}}
	_, cmd := m.handleFocusKey("s")
	if called || cmd == nil {
		t.Fatal("timer mutation ran inside key handler")
	}
	message := cmd().(timerDoneMsg)
	if !called || message.err != nil || !message.start {
		t.Fatal("timer command did not report completion")
	}
}
