package tui

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/mattn/go-runewidth"
	"os"
	"strings"

	"focusd/coreapi"
	"focusd/storage"
	"focusd/system"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type daemonStatus struct {
	Clock         storage.ClockState `json:"clock"`
	ActiveClients int                `json:"active_clients"`
	ManualPaused  bool               `json:"manual_paused"`
	Shutdown      bool               `json:"shutdown"`
	Error         string             `json:"error"`
}
type daemonStatusMsg struct {
	response     coreapi.Response
	status       daemonStatus
	err          error
	offlineState *storage.ClockState
}
type daemonRPCMsg struct {
	request  coreapi.Request
	response coreapi.Response
	err      error
}
type daemonStartMsg struct{ err error }

func newClientID() string {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(id[:])
}
func (m Model) ipcCaller() func(coreapi.Request) (coreapi.Response, error) {
	if m.callIPC != nil {
		return m.callIPC
	}
	return coreapi.Call
}
func (m Model) daemonStatusCmd() tea.Cmd {
	call := m.ipcCaller()
	return func() tea.Msg {
		request := coreapi.Request{Command: "status", ClientID: m.clientID}
		if sample, err := system.ReadClockSample(); err == nil && sample.BootID != "" && m.clientID != "" {
			_ = coreapi.PublishTUIPresence(m.clientID, sample)
			request.Command = "tui-register"
			if m.leaseRegistered {
				request.Command = "tui-renew"
			}
			request.PID, request.BootID, request.InterruptTicks, request.LeaseSeconds = os.Getpid(), sample.BootID, sample.InterruptTicks, 10
		}
		response, err := call(request)
		if err != nil && request.Command == "tui-renew" {
			request.Command = "tui-register"
			response, err = call(request)
		}
		message := daemonStatusMsg{response: response, err: err}
		if err == nil {
			message.err = json.Unmarshal(response.Data, &message.status)
		}
		if message.err != nil {
			if state, stateErr := storage.GetClockState(); stateErr == nil {
				message.offlineState = &state
			}
		}
		return message
	}
}
func (m Model) rpcCmd(request coreapi.Request) tea.Cmd {
	if request.RequestID == "" {
		request.RequestID = newClientID()
	}
	request.ClientID = m.clientID
	call := m.ipcCaller()
	return func() tea.Msg {
		response, err := call(request)
		return daemonRPCMsg{request, response, err}
	}
}
func offlineResetCmd(request coreapi.Request) tea.Cmd {
	return func() tea.Msg {
		response := coreapi.Response{RequestID: request.RequestID}
		lock, err := system.AcquireProfileLock()
		if err == nil {
			defer lock.Close()
			var result storage.ResetResult
			result, err = storage.ResetTrackingData(request.RequestID, false)
			if err == nil {
				response.OK = true
				response.State = "completed"
				response.Data, _ = json.Marshal(result)
			}
		}
		return daemonRPCMsg{request, response, err}
	}
}
func (m *Model) acceptClockStatus(state storage.ClockState) {
	previous := m.clockState.Incident
	next := state.Incident
	changed := next != nil && (previous == nil || previous.ID != next.ID || previous.Revision != next.Revision)
	m.clockState = state
	if changed {
		m.clockPrompt = true
		m.clockSelection = 0
		m.clockError = ""
		if !m.operationPending && m.operationID == "" {
			m.clockRequest = coreapi.Request{}
		}
	}
	if next == nil && m.operationID == "" && !m.operationPending {
		m.clockPrompt = false
		if m.clockRequest.Command == "clock-resolve" {
			m.clockRequest = coreapi.Request{}
		}
	}
}
func (m Model) handleClockKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		m.clockPrompt = false
		return m, nil
	case "up", "left", "shift+tab":
		m.clockSelection = (m.clockSelection + 2) % 3
		return m, nil
	case "down", "right", "tab":
		m.clockSelection = (m.clockSelection + 1) % 3
		return m, nil
	case "1", "2", "3":
		m.clockSelection = int(key[0] - '1')
	case "enter", "r":
	default:
		return m, nil
	}
	if m.operationPending || m.operationID != "" {
		return m, nil
	}
	if !m.daemonActive {
		m.clockError = "Start the background service in Settings to resolve this warning."
		m.clockPrompt = false
		m.addToast(m.clockError, toastWarning)
		return m, nil
	}
	incident := m.clockState.Incident
	if incident == nil {
		m.clockPrompt = false
		return m, nil
	}
	action := []string{"continue", "reset", "off"}[m.clockSelection]
	if m.clockRequest.RequestID == "" || m.clockRequest.Action != action {
		m.clockRequest = coreapi.Request{Command: "clock-resolve", Action: action, IncidentID: incident.ID, Revision: incident.Revision, Generation: m.daemonGeneration, RequestID: newClientID()}
	}
	if action == "reset" && m.clockRequest.ConfirmToken != "" && !m.clockRequest.Confirm {
		m.clockRequest.Confirm = true
	}
	m.operationPending = true
	m.clockError = ""
	return m, m.rpcCmd(m.clockRequest)
}
func (m *Model) handleRPCResult(msg daemonRPCMsg) tea.Cmd {
	m.operationPending = false
	response := msg.response
	if response.State == "failed" {
		m.operationID = ""
		m.clockError = response.Error
		m.addToast("Reset failed: "+response.Error, toastError)
		return nil
	}
	if msg.err != nil {
		m.clockError = msg.err.Error()
		if msg.request.Command == "clock-resolve" || msg.request.Command == "reset" {
			m.clockRequest = msg.request
			if response.Error != "" {
				m.clockRequest = coreapi.Request{}
			}
			m.addToast("Action was not confirmed: "+msg.err.Error(), toastError)
		} else if msg.request.Command == "operation-status" && response.Error != "" {
			m.operationID = ""
			m.addToast("Operation status rejected: "+msg.err.Error(), toastError)
		} else if msg.request.Command != "operation-status" {
			m.addToast(msg.err.Error(), toastError)
		}
		return m.daemonStatusCmd()
	}
	switch response.State {
	case "confirmation-required":
		if msg.request.Command == "clock-resolve" {
			incident := m.clockState.Incident
			if incident == nil || incident.ID != msg.request.IncidentID || incident.Revision != msg.request.Revision {
				m.clockRequest = coreapi.Request{}
				m.clockError = "The clock warning changed; review the current choices."
				return nil
			}
		}
		var confirmation struct {
			Token string `json:"confirm_token"`
		}
		if json.Unmarshal(response.Data, &confirmation) != nil || confirmation.Token == "" {
			m.clockError = "Missing reset confirmation"
			return nil
		}
		m.clockRequest = msg.request
		m.clockRequest.ConfirmToken = confirmation.Token
		if msg.request.Command == "clock-resolve" {
			m.clockError = "Reset will back up history, clear usage and timer, and preserve settings. Press 2 or Enter again to confirm."
		} else {
			m.modal = modal{Active: true, Title: "WIPE DATA", Required: "DELETE", Message: "Type DELETE to clear usage and timer. Settings are preserved."}
		}
		return nil
	case "running":
		m.operationID = response.OperationID
		m.clockError = "Creating verified backup and resetting tracking data…"
		return nil
	case "completed":
		m.operationID = ""
		m.clockRequest = coreapi.Request{}
		m.clockPrompt = false
		m.dashboard = tuiData{}
		m.stats = tuiData{}
		m.sortedAppsCache = nil
		m.dashSelected, m.statsSelected, m.browserSelected, m.statsUsageOffset, m.statsBrowserOffset = 0, 0, 0, 0, 0
		var result storage.ResetResult
		_ = json.Unmarshal(response.Data, &result)
		message := "Tracking data reset; settings preserved"
		if result.BackupPath != "" {
			message += ". Backup: " + result.BackupPath
		}
		m.addToast(message, toastSuccess)
		if result.Warning != "" {
			m.addToast(result.Warning, toastWarning)
		}
		return tea.Batch(m.daemonStatusCmd(), loadDashboard(), m.loadStatsCmd())
	case "failed":
		m.operationID = ""
		m.clockError = response.Error
		m.addToast("Reset failed: "+response.Error, toastError)
		return nil
	}
	if msg.request.Command == "retention-preview" {
		if err := json.Unmarshal(response.Data, &m.retentionPreview); err != nil {
			m.addToast(err.Error(), toastError)
			return nil
		}
		m.modal = modal{Active: true, Title: "REVIEW CLEANUP", Message: fmt.Sprintf("Delete %d rows before %s (%s to %s)? Enter confirms; Esc keeps cleanup held.", m.retentionPreview.Rows, m.retentionPreview.Cutoff, m.retentionPreview.Oldest, m.retentionPreview.Newest)}
		return nil
	}
	if msg.request.Command == "retention-confirm" {
		m.addToast("Cleanup completed; retention resumed", toastSuccess)
	}
	if msg.request.Command == "clock-resolve" {
		m.clockPrompt = false
		m.clockRequest = coreapi.Request{}
		if msg.request.Action == "off" {
			m.daemonActive = false
			m.addToast("Background service and timer stopped", toastInfo)
		} else {
			m.addToast("Current system time accepted; history preserved", toastSuccess)
		}
	}
	return m.daemonStatusCmd()
}
func (m Model) renderClockPrompt() string {
	incident := m.clockState.Incident
	if incident == nil {
		return ""
	}
	width := max(12, min(72, m.width-2))
	if m.height < 18 {
		lines := []string{"CLOCK WARNING — tracking paused", "1 Continue: accept time, keep history", "2 Reset: back up, clear usage/timer", "3 Off: stop service/timer", "Esc dismisses; c reopens"}
		if m.clockRequest.ConfirmToken != "" && !m.clockRequest.Confirm {
			lines[0] = "CONFIRM RESET: press 2 again"
		}
		if m.operationPending || m.operationID != "" {
			lines[0] = "Clock recovery operation pending"
		}
		if m.clockError != "" && m.height > 5 {
			lines = append(lines, m.clockError)
		}
		return clipLines(strings.Join(lines, "\n"), m.height, m.width)
	}
	lines := []string{"SYSTEM DATE/TIME WARNING", "Tracking is paused until you choose.", fmt.Sprintf("Reason: %s", incident.Reason), "Last safe: " + incident.LastSafe.Format("2006-01-02 15:04:05 MST"), "System now: " + incident.Current.WallUTC.Format("2006-01-02 15:04:05 MST"), ""}
	for i, label := range []string{"1  Continue — accept system time, keep history", "2  Reset — verified backup, clear usage/timer, continue", "3  Turn off — stop service/timer, keep auto-start"} {
		if i == m.clockSelection {
			label = "> " + label
		} else {
			label = "  " + label
		}
		lines = append(lines, label)
	}
	if m.clockError != "" {
		lines = append(lines, "", wrapModalText(m.clockError, width-4))
	}
	lines = append(lines, "", "Esc dismisses; tracking stays paused. [c] reopens.")
	for i := range lines {
		if !strings.Contains(lines[i], "\n") {
			lines[i] = fitLine(lines[i], width-4)
		}
	}
	view := panelWithHover("CLOCK WARNING", "", width, strings.Join(lines, "\n"), true, false)
	return clipLines(view, max(1, m.height), max(1, m.width))
}
func (m Model) handleClockMouse(msg tea.MouseMsg) (Model, tea.Cmd) {
	if msg.Action != tea.MouseActionRelease && msg.Type != tea.MouseRelease {
		return m, nil
	}
	view := m.renderClockPrompt()
	x, y := (m.width-lipgloss.Width(view))/2, (m.height-lipgloss.Height(view))/2
	if m.width < 80 || m.height < 24 || m.splash {
		x, y = 0, 0
	}
	if msg.X < x || msg.X >= x+lipgloss.Width(view) {
		return m, nil
	}
	row := msg.Y - y - 7
	if m.height < 18 {
		row = msg.Y - 1
	}
	if row < 0 || row > 2 {
		return m, nil
	}
	return m.handleClockKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune('1' + row)}})
}

func wrapModalText(text string, width int) string {
	width = max(1, width)
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		if line != "" && runewidth.StringWidth(line)+1+runewidth.StringWidth(word) > width {
			lines = append(lines, line)
			line = ""
		}
		if line != "" {
			line += " "
		}
		line += word
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
