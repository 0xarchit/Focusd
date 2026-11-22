package core

import (
	"context"
	"encoding/json"
	"fmt"
	"focusd/coreapi"
	"focusd/storage"
	"focusd/system"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const controllerQueueSize = 32

type controllerRequest struct {
	request coreapi.Request
	reply   chan coreapi.Response
}
type clientLease struct {
	expires  time.Time
	pid      int
	bootID   string
	shutdown bool
}
type resetConfirmation struct {
	token, action, incident string
	revision                int64
	epoch                   uint64
	expires                 time.Time
}
type resetCompletion struct {
	operation string
	result    storage.ResetResult
	err       error
	request   coreapi.Request
}
type daemonStatus struct {
	Clock         storage.ClockState `json:"clock"`
	ActiveClients int                `json:"active_clients"`
	ManualPaused  bool               `json:"manual_paused"`
	Shutdown      bool               `json:"shutdown"`
	Error         string             `json:"error,omitempty"`
	ExePath       string             `json:"exe_path,omitempty"`
	PID           int                `json:"pid,omitempty"`
}
type daemonController struct {
	readClockSample  func() (system.ClockSample, error)
	tracker          *Tracker
	guard            *ClockGuard
	generation       string
	requests         chan controllerRequest
	completed        chan resetCompletion
	clients          map[string]clientLease
	confirmations    map[string]resetConfirmation
	operations       map[string]coreapi.Response
	resetRequests    map[string]string
	shutdownClients  map[string]bool
	retention        *storage.RetentionPreview
	retentionExpires time.Time
	busy             bool
	pendingStop      bool
	guarded          bool
	fault            string
	sample           system.ClockSample
	statusMu         sync.RWMutex
	status           daemonStatus
	notifyCancel     context.CancelFunc
	notifyKey        string
	reminders        reminderState
}

func (t *Tracker) startControlled() error {
	if t.ipcToken == "" {
		return fmt.Errorf("IPC token unavailable")
	}
	guard, err := NewClockGuard()
	if err != nil {
		return fmt.Errorf("load clock guard: %w", err)
	}
	generation, err := randomClockID()
	if err != nil {
		return err
	}
	c := &daemonController{tracker: t, guard: guard, generation: generation,
		requests: make(chan controllerRequest, controllerQueueSize), completed: make(chan resetCompletion, 1),
		clients: make(map[string]clientLease), confirmations: make(map[string]resetConfirmation), operations: make(map[string]coreapi.Response), resetRequests: make(map[string]string), shutdownClients: make(map[string]bool)}
	t.controller = c
	c.observe(true)
	if !c.guarded {
		if err := t.recoverOrphanedSession(); err != nil {
			if t.calendarErr != nil {
				c.enterGuard(ClockEvent{}, err)
			} else {
				return err
			}
		}
		if !c.guarded {
			storage.EnforceRetention()
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close()
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	c.status.ExePath, c.status.PID = exePath, os.Getpid()
	c.refreshStatus()
	if err := coreapi.PublishEndpoint(coreapi.Endpoint{Version: coreapi.ProtocolVersion, Address: listener.Addr().String(), Generation: generation, PID: os.Getpid(), ExePath: exePath}); err != nil {
		return err
	}
	defer coreapi.RemoveEndpoint(generation)
	t.ipcDone = make(chan struct{})
	go t.startIPCOnPort(listener)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	clockTicker := time.NewTicker(time.Second)
	pollTicker := time.NewTicker(t.pollInterval)
	persistTicker := time.NewTicker(5 * time.Minute)
	retentionTicker := time.NewTicker(time.Hour)
	focusTicker := time.NewTicker(5 * time.Second)
	defer clockTicker.Stop()
	defer pollTicker.Stop()
	defer persistTicker.Stop()
	defer retentionTicker.Stop()
	defer focusTicker.Stop()
	for {
		select {
		case <-signals:
			t.Stop()
		case <-t.ctx.Done():
			if c.busy {
				c.finishReset(<-c.completed)
			}
			c.cancelNotification()
			if !c.guarded {
				c.observe(false)
				if !c.guarded {
					t.flushCurrentSession()
				}
			}
			t.mu.Lock()
			hasWork := t.currentSession != nil || len(t.pendingSessions) > 0
			t.mu.Unlock()
			if hasWork {
				t.flushPendingSessions()
			}
			if err := c.guard.Persist(); err != nil {
				log.Printf("ERROR: save clock checkpoint: %v", err)
			}
			cancelNotify()
			// The accept loop stops adding handlers before waiting. In particular,
			// an authenticated stop's ACK must reach its socket before process exit.
			if t.ipcDone != nil {
				<-t.ipcDone
			}
			t.ipcHandlers.Wait()
			return nil
		case queued := <-c.requests:
			c.processRequest(queued)
		case result := <-c.completed:
			c.finishReset(result)
		case <-clockTicker.C:
			if !c.busy {
				c.observe(false)
			}
			c.pruneClients()
			c.refreshStatus()
			c.offerClockNotification()
		case <-pollTicker.C:
			if c.busy || c.guarded {
				continue
			}
			c.observe(false)
			if c.guarded {
				continue
			}
			if storage.IsPaused() {
				t.flushCurrentSession()
				t.flushPendingSessions()
				c.reminders.setPresent(time.Now(), false)
				continue
			}
			t.poll()
			t.mu.Lock()
			t.rotateActiveSessionAtMidnight()
			force := t.needsPersist() || len(t.pendingSessions) > 0
			t.mu.Unlock()
			if c.checkCalendarFault() {
				continue
			}
			t.writeActiveSession(force)
		case <-persistTicker.C:
			if !c.busy && !c.guarded {
				c.observe(false)
				if !c.guarded {
					t.flushForRead()
				}
			}
		case <-retentionTicker.C:
			if !c.busy && !c.guarded {
				c.observe(false)
				if !c.guarded {
					storage.EnforceRetention()
				}
			}
		case <-focusTicker.C:
			if !c.busy && !c.guarded {
				c.observe(false)
				if c.guarded {
					continue
				}
				coreapi.CheckPomodoroAndNotify(pomodoroNotify)
				c.checkReminders()
			}
		}
	}
}

func (c *daemonController) processRequest(queued controllerRequest) {
	response := c.handle(queued.request)
	queued.reply <- response
	if c.pendingStop {
		c.tracker.Stop()
	}
}

// SubmitRequest is also the notification action path. Every mutation is processed
// by the same loop as clock observations and tracking checkpoints.
func (t *Tracker) SubmitRequest(request coreapi.Request) coreapi.Response {
	c := t.controller
	if c == nil {
		return coreapi.Response{Error: "daemon controller is unavailable"}
	}
	base := coreapi.Response{Version: coreapi.ProtocolVersion, Generation: c.generation, RequestID: request.RequestID}
	queued := controllerRequest{request: request, reply: make(chan coreapi.Response, 1)}
	select {
	case c.requests <- queued:
	default:
		base.Error = "daemon request queue is full"
		return base
	}
	select {
	case response := <-queued.reply:
		return response
	case <-t.ctx.Done():
		select {
		case response := <-queued.reply:
			return response
		default:
		}
		base.Error = "daemon is stopping"
		return base
	}
}

func (c *daemonController) refreshStatus() {
	c.statusMu.Lock()
	defer c.statusMu.Unlock()
	c.status.Clock = c.guard.State()
	c.status.ActiveClients = len(c.clients)
	if !c.busy {
		c.status.ManualPaused = storage.IsPaused()
	}
	c.status.Error = c.fault
}
func (c *daemonController) data(clientID string) json.RawMessage {
	c.refreshStatus()
	c.statusMu.RLock()
	status := c.status
	c.statusMu.RUnlock()
	if lease, ok := c.clients[clientID]; ok {
		status.Shutdown = lease.shutdown
	}
	data, _ := json.Marshal(status)
	return data
}
func (c *daemonController) observe(startup bool) {
	reader := c.readClockSample
	if reader == nil {
		reader = system.ReadClockSample
	}
	sample, err := reader()
	if err != nil {
		c.enterGuard(ClockEvent{}, err)
		return
	}
	event, err := c.guard.Observe(sample, startup)
	if err != nil || event.Incident != nil {
		c.enterGuard(event, err)
		return
	}
	if event.Slept || event.ZoneChanged {
		c.tracker.mu.Lock()
		c.tracker.closeCurrentSessionAt(event.LastSafe)
		c.tracker.mu.Unlock()
		if c.checkCalendarFault() {
			return
		}
		c.tracker.flushPendingSessions()
		c.reminders.setPresent(time.Now(), false)
	}
	c.sample = sample
	c.fault = ""
	c.guarded = false
	c.checkCalendarFault()
}

func (c *daemonController) checkCalendarFault() bool {
	c.tracker.mu.Lock()
	err := c.tracker.calendarErr
	c.tracker.mu.Unlock()
	if err == nil {
		return false
	}
	c.enterGuard(ClockEvent{}, fmt.Errorf("local calendar unavailable: %w", err))
	return true
}
func (c *daemonController) enterGuard(event ClockEvent, err error) {
	if err != nil {
		c.fault = err.Error()
		if reportErr := c.guard.ReportFault(c.sample.WallUTC, err.Error()); reportErr != nil {
			c.fault = reportErr.Error()
		}
	}
	safe := event.LastSafe
	state := c.guard.State()
	if state.Incident != nil {
		safe = state.Incident.LastSafe
	}
	if safe.IsZero() {
		safe = c.sample.WallUTC
	}
	if safe.IsZero() {
		if state.Accepted != nil {
			safe = state.Accepted.WallUTC
		}
	}
	c.tracker.mu.Lock()
	if safe.IsZero() && c.tracker.currentSession != nil {
		safe = c.tracker.currentSession.StartTime
	}
	c.tracker.closeCurrentSessionAt(safe)
	if c.tracker.calendarErr != nil {
		// Keep the last committed heartbeat for recovery after calendar conversion
		// becomes available; never write a guessed date or a later unsafe endpoint.
		c.tracker.currentSession = nil
	}
	hasPending := len(c.tracker.pendingSessions) > 0
	c.tracker.mu.Unlock()
	if hasPending {
		c.tracker.flushPendingSessions()
	}
	if !c.sample.WallUTC.IsZero() {
		err = coreapi.FreezePomodoroAtSample(c.sample)
	} else if state.Accepted != nil {
		accepted := state.Accepted
		err = coreapi.FreezePomodoroAtSample(system.ClockSample{WallUTC: accepted.WallUTC, InterruptTicks: accepted.InterruptTicks, UnbiasedTicks: accepted.UnbiasedTicks, BootID: accepted.BootID})
	} else if !safe.IsZero() {
		err = coreapi.FreezePomodoroAt(safe)
	}
	if err != nil {
		c.fault = err.Error()
	}
	c.guarded = true
	c.reminders.setPresent(time.Now(), false)
	c.refreshStatus()
}
func (c *daemonController) pruneClients() {
	now := time.Now()
	for id, lease := range c.clients {
		if !now.Before(lease.expires) {
			delete(c.clients, id)
		}
	}
	for id, confirmation := range c.confirmations {
		if !now.Before(confirmation.expires) {
			delete(c.confirmations, id)
		}
	}
}
func (c *daemonController) cancelNotification() {
	if c.notifyCancel != nil {
		c.notifyCancel()
		c.notifyCancel = nil
	}
	c.notifyKey = ""
}
func (c *daemonController) offerClockNotification() {
	state := c.guard.State()
	if sample, err := system.ReadClockSample(); err == nil {
		presences, presenceErr := coreapi.ReadTUIPresences(sample)
		if presenceErr != nil || len(presences) > 0 {
			c.cancelNotification()
			return
		}
	}
	if state.Incident == nil || len(c.clients) > 0 || c.busy {
		c.cancelNotification()
		return
	}
	key := fmt.Sprintf("%s:%d", state.Incident.ID, state.Incident.Revision)
	if c.notifyKey == key {
		return
	}
	c.cancelNotification()
	ctx, cancel := context.WithCancel(c.tracker.ctx)
	c.notifyCancel = cancel
	incident := *state.Incident
	if ShowClockRecoveryNotification(ctx, state, func(action string, confirmed bool) {
		id, _ := randomClockID()
		request := coreapi.Request{Version: coreapi.ProtocolVersion, Command: "clock-resolve", Token: c.tracker.ipcToken, Generation: c.generation, RequestID: id, IncidentID: incident.ID, Revision: incident.Revision, Action: action}
		response := c.tracker.SubmitRequest(request)
		if confirmed && response.State == "confirmation-required" {
			var data struct {
				Token string `json:"confirm_token"`
			}
			_ = json.Unmarshal(response.Data, &data)
			request.Confirm = true
			request.ConfirmToken = data.Token
			request.RequestID, _ = randomClockID()
			c.tracker.SubmitRequest(request)
		}
	}) {
		c.notifyKey = key
	} else {
		cancel()
		c.notifyCancel = nil
	}
}

func (c *daemonController) handle(request coreapi.Request) coreapi.Response {
	response := coreapi.Response{Version: coreapi.ProtocolVersion, Generation: c.generation, RequestID: request.RequestID}
	if err := coreapi.ValidateRequest(request); err != nil {
		response.Error = err.Error()
		return response
	}
	if request.Generation != c.generation || !coreapi.TokensMatch(request.Token, c.tracker.ipcToken) {
		response.Error = "daemon identity changed"
		return response
	}
	c.pruneClients()
	if request.Confirm && (request.Command == "reset" || request.Command == "clock-resolve" && request.Action == "reset") {
		if result, found, err := storage.GetResetResult(request.RequestID); err != nil {
			response.Error = err.Error()
			return response
		} else if found {
			response.OK = true
			response.State = "completed"
			response.OperationID = request.RequestID
			response.Data, _ = json.Marshal(result)
			return response
		}
		if operation, ok := c.resetRequests[request.RequestID]; ok {
			receipt := c.operations[operation]
			receipt.RequestID = request.RequestID
			return receipt
		}
	}
	switch request.Command {
	case "status", "clock-status":
		response.OK = true
		response.Data = c.data(request.ClientID)
		return response
	case "operation-status":
		operation, ok := c.operations[request.OperationID]
		if !ok {
			result, found, err := storage.GetResetResult(request.OperationID)
			if err != nil {
				response.Error = err.Error()
				return response
			}
			if !found {
				response.Error = "unknown operation"
				return response
			}
			response.OK = true
			response.State = "completed"
			response.OperationID = request.OperationID
			response.Data, _ = json.Marshal(result)
			return response
		}
		operation.RequestID = request.RequestID
		return operation
	case "tui-register", "tui-renew":
		if request.ClientID == "" || request.PID <= 0 || request.BootID == "" {
			response.Error = "TUI identity is incomplete"
			return response
		}
		lease, exists := c.clients[request.ClientID]
		if request.Command == "tui-renew" && (!exists || lease.pid != request.PID || lease.bootID != request.BootID) {
			response.Error = "TUI lease expired"
			return response
		}
		if c.sample.BootID != "" && request.BootID != c.sample.BootID {
			response.Error = "TUI boot identity changed"
			return response
		}
		if len(c.clients) >= 32 && !exists {
			response.Error = "too many TUI clients"
			return response
		}
		lease.pid = request.PID
		lease.shutdown = lease.shutdown || c.shutdownClients[request.ClientID]
		lease.bootID = request.BootID
		lease.expires = time.Now().Add(time.Duration(max(5, request.LeaseSeconds)) * time.Second)
		c.clients[request.ClientID] = lease
		c.cancelNotification()
		response.OK = true
		response.Data = c.data(request.ClientID)
		return response
	case "tui-release":
		delete(c.clients, request.ClientID)
		response.OK = true
		return response
	case "shutdown-tui", "shutdown-clients":
		if c.shutdownClients == nil {
			c.shutdownClients = make(map[string]bool)
		}
		for id, lease := range c.clients {
			c.shutdownClients[id] = true
			lease.shutdown = true
			c.clients[id] = lease
		}
		response.OK = true
		return response
	}
	if c.busy {
		response.State = "busy"
		response.Error = "tracking database operation is running"
		return response
	}
	switch request.Command {
	case "timer-start":
		c.observe(false)
		if c.guarded {
			response.Error = "resolve the clock warning before starting a timer"
			break
		}
		if err := coreapi.StartPomodoro(request.DurationMinutes); err != nil {
			response.Error = err.Error()
		} else {
			response.OK = true
		}
	case "timer-stop":
		if err := coreapi.StopPomodoro(); err != nil {
			response.Error = err.Error()
		} else {
			response.OK = true
		}
	case "flush":
		c.observe(false)
		if !c.guarded {
			c.tracker.flushForRead()
		}
		response.OK = true
	case "stop":
		if !c.guarded {
			c.observe(false)
			if !c.guarded {
				c.tracker.flushCurrentSession()
			}
		}
		c.tracker.flushPendingSessions()
		response.OK = true
		c.pendingStop = true
	case "clock-resolve":
		incident := c.guard.State().Incident
		if incident == nil || incident.ID != request.IncidentID || incident.Revision != request.Revision {
			response.Error = "clock decision is no longer current"
			break
		}
		switch request.Action {
		case "continue":
			sample, err := system.ReadClockSample()
			if err == nil {
				err = c.tracker.recoverOrphanedSession()
			}
			if err == nil {
				err = coreapi.ResumeFrozenPomodoroAt(sample.WallUTC)
			}
			if err == nil {
				err = c.guard.Accept(request.IncidentID, request.Revision, sample)
			}
			if err != nil {
				if !sample.WallUTC.IsZero() {
					_ = coreapi.FreezePomodoroAtSample(sample)
				}
				c.enterGuard(ClockEvent{}, err)
				response.Error = err.Error()
				break
			}
			c.sample = sample
			c.tracker.mu.Lock()
			c.tracker.calendarErr = nil
			c.tracker.mu.Unlock()
			c.guarded = false
			c.fault = ""
			c.cancelNotification()
			response.OK = true
		case "reset":
			return c.requestReset(request, response)
		case "off":
			if err := coreapi.StopPomodoro(); err != nil {
				response.Error = err.Error()
				break
			}
			response.OK = true
			c.pendingStop = true
		default:
			response.Error = "unknown clock action"
		}
	case "reset":
		return c.requestReset(request, response)
	case "retention-preview":
		c.observe(false)
		if c.guarded {
			response.Error = "resolve the clock warning first"
			break
		}
		preview, err := storage.PreviewRetention(storage.Today())
		if err != nil {
			response.Error = err.Error()
			break
		}
		c.retention = &preview
		c.retentionExpires = time.Now().Add(2 * time.Minute)
		response.OK = true
		response.Data, _ = json.Marshal(preview)
		return response
	case "retention-confirm":
		c.observe(false)
		if c.guarded || c.retention == nil || !time.Now().Before(c.retentionExpires) || request.ConfirmToken != c.retention.Token {
			response.Error = "retention preview expired; review again"
			break
		}
		if err := storage.ConfirmRetention(*c.retention); err != nil {
			response.Error = err.Error()
			break
		}
		guard, err := NewClockGuard()
		if err != nil {
			response.Error = err.Error()
			break
		}
		c.guard = guard
		c.retention = nil
		response.OK = true
	}
	response.Data = c.data(request.ClientID)
	return response
}

func (c *daemonController) requestReset(request coreapi.Request, response coreapi.Response) coreapi.Response {
	if request.Confirm {
		if operation, ok := c.resetRequests[request.RequestID]; ok {
			receipt := c.operations[operation]
			receipt.RequestID = request.RequestID
			return receipt
		}
	}
	key := request.Command + ":" + request.IncidentID
	confirmation, exists := c.confirmations[key]
	if !request.Confirm {
		token, err := randomClockID()
		if err != nil {
			response.Error = err.Error()
			return response
		}
		c.confirmations[key] = resetConfirmation{token: token, action: request.Command, incident: request.IncidentID, revision: request.Revision, epoch: c.guard.State().Epoch, expires: time.Now().Add(2 * time.Minute)}
		response.OK = true
		response.State = "confirmation-required"
		response.Data, _ = json.Marshal(map[string]string{"confirm_token": token})
		return response
	}
	if !exists || !time.Now().Before(confirmation.expires) || request.ConfirmToken != confirmation.token || request.Revision != confirmation.revision {
		response.Error = "reset confirmation expired; review again"
		return response
	}
	delete(c.confirmations, key)
	c.observe(false)
	state := c.guard.State()
	if state.Epoch != confirmation.epoch || request.Command == "reset" && state.Incident != nil || request.Command == "clock-resolve" && (state.Incident == nil || state.Incident.ID != request.IncidentID || state.Incident.Revision != request.Revision) {
		response.Error = "clock changed after reset confirmation; review again"
		return response
	}
	if !c.guarded {
		c.tracker.flushCurrentSession()
		c.tracker.flushPendingSessions()
	}
	c.cancelNotification()
	c.busy = true
	operation := request.RequestID
	response.OK = true
	response.State = "running"
	response.OperationID = operation
	c.operations[operation] = response
	if c.resetRequests == nil {
		c.resetRequests = make(map[string]string)
	}
	c.resetRequests[request.RequestID] = operation
	go func() {
		result, err := storage.ResetTrackingData(request.RequestID, true)
		c.completed <- resetCompletion{operation: operation, result: result, err: err, request: request}
	}()
	return response
}
func (c *daemonController) finishReset(completion resetCompletion) {
	c.busy = false
	response := c.operations[completion.operation]
	if completion.err != nil {
		response.OK = false
		response.State = "failed"
		response.Error = completion.err.Error()
		c.operations[completion.operation] = response
		c.refreshStatus()
		return
	}
	c.tracker.mu.Lock()
	c.tracker.currentSession = nil
	c.tracker.pendingSessions = nil
	c.tracker.lastPersistAt = time.Time{}
	c.tracker.lastPollAt = time.Time{}
	c.tracker.calendarErr = nil
	c.tracker.mu.Unlock()
	_ = coreapi.StopPomodoro()
	if completion.request.Command == "clock-resolve" {
		sample, err := system.ReadClockSample()
		if err == nil {
			err = c.guard.Accept(completion.request.IncidentID, completion.request.Revision, sample)
		}
		if err != nil {
			c.enterGuard(ClockEvent{}, err)
			response.Error = err.Error()
		} else {
			c.sample = sample
			c.guarded = false
			c.fault = ""
		}
	}
	c.reminders.reset()
	response.State = "completed"
	response.Data, _ = json.Marshal(completion.result)
	c.operations[completion.operation] = response
	// Keep bounded completed receipts while the database retains durable reset IDs.
	if len(c.operations) > 64 {
		for id := range c.operations {
			if id != completion.operation {
				delete(c.operations, id)
				break
			}
		}
	}
	c.refreshStatus()
}
