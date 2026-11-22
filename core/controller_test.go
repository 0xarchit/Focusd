package core

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"focusd/coreapi"
	"focusd/storage"
	"net"
	"testing"
	"time"
)

func testController(t *testing.T) *daemonController {
	t.Helper()
	setupTestDB(t)
	guard, err := NewClockGuard()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	tracker := &Tracker{ctx: ctx, cancel: cancel, ipcToken: "test-token"}
	c := &daemonController{tracker: tracker, guard: guard, generation: "test-generation", requests: make(chan controllerRequest, controllerQueueSize), clients: make(map[string]clientLease), confirmations: make(map[string]resetConfirmation), operations: make(map[string]coreapi.Response)}
	tracker.controller = c
	return c
}
func controllerCommand(command string) coreapi.Request {
	return coreapi.Request{Version: coreapi.ProtocolVersion, Command: command, Token: "test-token", Generation: "test-generation", RequestID: "request-1"}
}
func TestControllerStatusIncludesGuardAndPauseWithoutMutation(t *testing.T) {
	c := testController(t)
	incident := &storage.ClockIncident{ID: "clock-1", Revision: 3, Reason: "rollback", Current: storage.ClockCheckpoint{WallUTC: time.Now()}}
	state := storage.ClockState{Version: 1, Incident: incident, RetentionHold: true}
	if err := storage.SaveClockState(state); err != nil {
		t.Fatal(err)
	}
	c.guard, _ = NewClockGuard()
	before, _ := storage.GetClockState()
	response := c.handle(controllerCommand("clock-status"))
	if !response.OK {
		t.Fatal(response.Error)
	}
	var status daemonStatus
	if err := json.Unmarshal(response.Data, &status); err != nil {
		t.Fatal(err)
	}
	if status.Clock.Incident == nil || status.Clock.Incident.Revision != 3 {
		t.Fatalf("missing guard: %s", response.Data)
	}
	after, _ := storage.GetClockState()
	if after.Incident.Revision != before.Incident.Revision || after.Epoch != before.Epoch {
		t.Fatal("status changed clock policy")
	}
}
func TestControllerRejectsStaleDecisionAndResetConfirmation(t *testing.T) {
	c := testController(t)
	request := controllerCommand("clock-resolve")
	request.IncidentID = "missing"
	request.Revision = 1
	request.Action = "continue"
	if response := c.handle(request); response.OK {
		t.Fatal("stale clock choice accepted")
	}
	request = controllerCommand("reset")
	response := c.handle(request)
	if !response.OK || response.State != "confirmation-required" {
		t.Fatalf("reset confirmation missing: %+v", response)
	}
	request.Confirm = true
	request.ConfirmToken = "wrong"
	if response = c.handle(request); response.OK || c.busy {
		t.Fatal("incorrect confirmation started reset")
	}
}
func TestControllerLeaseExpiryAndShutdown(t *testing.T) {
	c := testController(t)
	request := controllerCommand("tui-register")
	request.ClientID = "client-1"
	request.PID = 123
	request.BootID = "boot-1"
	request.LeaseSeconds = 10
	if response := c.handle(request); !response.OK {
		t.Fatal(response.Error)
	}
	if response := c.handle(controllerCommand("shutdown-clients")); !response.OK {
		t.Fatal(response.Error)
	}
	request.Command = "tui-renew"
	response := c.handle(request)
	var status daemonStatus
	_ = json.Unmarshal(response.Data, &status)
	if !status.Shutdown || status.ActiveClients != 1 {
		t.Fatalf("shutdown ownership missing: %s", response.Data)
	}
	lease := c.clients[request.ClientID]
	lease.expires = time.Now().Add(-time.Second)
	c.clients[request.ClientID] = lease
	if response = c.handle(request); response.OK {
		t.Fatal("expired client renewed without registration")
	}
}
func TestControllerQueueIsBounded(t *testing.T) {
	c := testController(t)
	for i := 0; i < controllerQueueSize; i++ {
		c.requests <- controllerRequest{}
	}
	response := c.tracker.SubmitRequest(controllerCommand("status"))
	if response.OK || response.Error != "daemon request queue is full" {
		t.Fatalf("unbounded queue response: %+v", response)
	}
}

func TestControllerTransientClockFailureKeepsIncident(t *testing.T) {
	c := testController(t)
	c.enterGuard(ClockEvent{}, fmt.Errorf("sample unavailable"))
	if !c.guarded || c.guard.State().Incident == nil {
		t.Fatal("clock failure did not create recoverable decision")
	}
	c.observe(false)
	if !c.guarded || c.guard.State().Incident == nil {
		t.Fatal("next sample silently cleared clock failure")
	}
}

func TestControllerTimerWritesRespectGuardAndBusyOperation(t *testing.T) {
	c := testController(t)
	c.enterGuard(ClockEvent{}, fmt.Errorf("invalid clock"))
	request := controllerCommand("timer-start")
	request.DurationMinutes = 25
	if response := c.handle(request); response.OK {
		t.Fatal("guarded timer started")
	}
	c.busy = true
	request = controllerCommand("timer-stop")
	if response := c.handle(request); response.OK || response.State != "busy" {
		t.Fatal("timer mutation bypassed reset worker")
	}
	c.busy = false
	if response := c.handle(request); !response.OK {
		t.Fatal("guarded timer stop rejected")
	}
}

func TestControllerCommittedResetReceiptIsReadBeforeNewIncident(t *testing.T) {
	c := testController(t)
	result := storage.ResetResult{Generation: 1, BackupPath: "verified-backup"}
	encoded, _ := json.Marshal(result)
	if err := storage.SetConfig("reset_result:old-request", string(encoded)); err != nil {
		t.Fatal(err)
	}
	c.enterGuard(ClockEvent{}, fmt.Errorf("new clock warning"))
	request := controllerCommand("clock-resolve")
	request.RequestID = "old-request"
	request.Action = "reset"
	request.Confirm = true
	request.IncidentID = "old-incident"
	request.Revision = 1
	response := c.handle(request)
	if !response.OK || response.State != "completed" || c.busy {
		t.Fatalf("committed retry was not reconciled: %+v", response)
	}
	if c.guard.State().Incident == nil {
		t.Fatal("receipt lookup cleared a new incident")
	}
	request = controllerCommand("operation-status")
	request.OperationID = "old-request"
	if response = c.handle(request); !response.OK || response.State != "completed" {
		t.Fatalf("restart receipt missing: %+v", response)
	}
}

func TestControllerNormalStopPreservesTimer(t *testing.T) {
	c := testController(t)
	state := &coreapi.PomodoroState{Active: true, StartTime: time.Now(), Duration: 25}
	if err := coreapi.SavePomodoroState(state); err != nil {
		t.Fatal(err)
	}
	response := c.handle(controllerCommand("stop"))
	if !response.OK {
		t.Fatal(response.Error)
	}
	if !coreapi.LoadPomodoroStateFresh().Active {
		t.Fatal("normal daemon stop cancelled timer")
	}
}

func TestSubmittedStopRepliesBeforeCancellation(t *testing.T) {
	for _, action := range []string{"stop", "off"} {
		t.Run(action, func(t *testing.T) {
			c := testController(t)
			request := controllerCommand("stop")
			if action == "off" {
				c.enterGuard(ClockEvent{}, fmt.Errorf("clock warning"))
				incident := c.guard.State().Incident
				request.Command = "clock-resolve"
				request.Action = "off"
				request.IncidentID = incident.ID
				request.Revision = incident.Revision
			}
			go func() { queued := <-c.requests; c.processRequest(queued) }()
			response := c.tracker.SubmitRequest(request)
			if !response.OK || response.Error != "" || response.RequestID != request.RequestID {
				t.Fatalf("stop acknowledgement lost: %+v", response)
			}
			select {
			case <-c.tracker.ctx.Done():
			case <-time.After(time.Second):
				t.Fatal("acknowledged stop did not stop controller")
			}
		})
	}
}

func TestAuthenticatedStopWritesSocketAcknowledgement(t *testing.T) {
	c := testController(t)
	server, client := net.Pipe()
	defer client.Close()
	done := make(chan struct{})
	go func() { c.tracker.handleIPCConnection(server); close(done) }()
	go func() { queued := <-c.requests; c.processRequest(queued) }()
	request := controllerCommand("stop")
	encoded, _ := json.Marshal(request)
	if _, err := client.Write(append(encoded, '\n')); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(client).ReadBytes('\n')
	if err != nil {
		t.Fatalf("stop socket closed before ACK: %v", err)
	}
	var response coreapi.Response
	if err = json.Unmarshal(line, &response); err != nil || !response.OK || response.RequestID != request.RequestID {
		t.Fatalf("stop socket acknowledgement invalid: %s", line)
	}
	<-done
}
