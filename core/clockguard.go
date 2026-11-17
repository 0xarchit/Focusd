package core

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"focusd/storage"
	"focusd/system"
)

const ClockChangeTolerance = 120 * time.Second
const ClockUnknownBootGap = 30 * 24 * time.Hour
const clockTicksPerSecond = uint64(10000000)

type ClockEvent struct {
	Incident    *storage.ClockIncident
	Created     bool
	Changed     bool
	Slept       bool
	ZoneChanged bool
	LastSafe    time.Time
}
type ClockGuard struct {
	mu             sync.Mutex
	state          storage.ClockState
	last           *storage.ClockCheckpoint
	malformed      bool
	lastSavedTicks uint64
	saveState      func(storage.ClockState) error
	incidentDirty  bool
}

func NewClockGuard() (*ClockGuard, error) {
	state, err := storage.GetClockState()
	if errors.Is(err, storage.ErrMalformedClockState) {
		return &ClockGuard{state: storage.ClockState{Version: 1}, malformed: true}, nil
	}
	if err != nil {
		return nil, err
	}
	return &ClockGuard{state: state}, nil
}
func checkpoint(sample system.ClockSample) storage.ClockCheckpoint {
	return storage.ClockCheckpoint{WallUTC: sample.WallUTC.UTC(), InterruptTicks: sample.InterruptTicks,
		UnbiasedTicks: sample.UnbiasedTicks, BootID: sample.BootID, ZoneKey: sample.ZoneKey, OffsetSeconds: sample.OffsetSeconds}
}
func cloneClockState(state storage.ClockState) storage.ClockState {
	if state.Accepted != nil {
		copy := *state.Accepted
		state.Accepted = &copy
	}
	if state.Anchor != nil {
		copy := *state.Anchor
		state.Anchor = &copy
	}
	if state.Incident != nil {
		copy := *state.Incident
		state.Incident = &copy
	}
	return state
}
func (g *ClockGuard) State() storage.ClockState {
	g.mu.Lock()
	defer g.mu.Unlock()
	return cloneClockState(g.state)
}
func sameBoot(a, b storage.ClockCheckpoint) bool { return a.BootID != "" && a.BootID == b.BootID }
func elapsedSeconds(a, b uint64) (float64, bool) {
	if b < a {
		return 0, false
	}
	return float64(b-a) / float64(clockTicksPerSecond), true
}
func wallSeconds(a, b time.Time) float64 {
	return float64(b.Unix()-a.Unix()) + float64(b.Nanosecond()-a.Nanosecond())/1e9
}
func discrepancy(a, b storage.ClockCheckpoint) bool {
	elapsed, ok := elapsedSeconds(a.InterruptTicks, b.InterruptTicks)
	return !ok || math.Abs(wallSeconds(a.WallUTC, b.WallUTC)-elapsed) > ClockChangeTolerance.Seconds()
}
func randomClockID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(id[:]), nil
}
func (g *ClockGuard) save(state storage.ClockState) error {
	// A failed incident write must still stop this process from accepting time.
	if state.Incident != nil {
		g.state = state
	}
	write := g.saveState
	if write == nil {
		write = storage.SaveClockState
	}
	if err := write(state); err != nil {
		if state.Incident != nil {
			g.incidentDirty = true
		}
		return err
	}
	g.state = state
	g.incidentDirty = false
	return nil
}

// ReportFault keeps a sampling or persistence failure guarded until an explicit choice.
func (g *ClockGuard) ReportFault(lastSafe time.Time, reason string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	state := cloneClockState(g.state)
	if state.Incident == nil {
		current := storage.ClockCheckpoint{WallUTC: lastSafe.UTC()}
		if g.last != nil {
			current = *g.last
		} else if state.Accepted != nil {
			current = *state.Accepted
		}
		if current.WallUTC.IsZero() {
			current.WallUTC = time.Now().UTC()
		}
		if lastSafe.IsZero() {
			lastSafe = current.WallUTC
		}
		id, err := randomClockID()
		if err != nil {
			return err
		}
		if reason == "" {
			reason = "system time could not be verified"
		}
		if len(reason) > 512 {
			reason = reason[:512]
		}
		state.Incident = &storage.ClockIncident{ID: id, Revision: 1, Reason: reason, LastSafe: lastSafe.UTC(), Current: current, DetectedAt: current.WallUTC}
	}
	state.RetentionHold = true
	g.malformed = false
	return g.save(state)
}
func (g *ClockGuard) Observe(sample system.ClockSample, startup bool) (ClockEvent, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	current := checkpoint(sample)
	if current.WallUTC.IsZero() {
		return ClockEvent{}, fmt.Errorf("empty system clock sample")
	}
	state := cloneClockState(g.state)
	event := ClockEvent{}
	previous := g.last
	if previous == nil {
		if state.Incident != nil {
			previous = &state.Incident.Current
		} else {
			previous = state.Accepted
		}
	}
	if previous != nil {
		event.LastSafe = previous.WallUTC
		event.ZoneChanged = previous.ZoneKey != current.ZoneKey
		if elapsed, ok := elapsedSeconds(previous.InterruptTicks, current.InterruptTicks); ok {
			if active, ok := elapsedSeconds(previous.UnbiasedTicks, current.UnbiasedTicks); ok {
				event.Slept = (!startup || sameBoot(*previous, current)) && elapsed-active > 2
			}
		}
	}
	// An unresolved choice survives restarts. Only another discontinuity invalidates its revision.
	if state.Incident != nil {
		baseline := previous
		if baseline == nil {
			baseline = &state.Incident.Current
		}
		changed := false
		if !startup || sameBoot(*baseline, current) {
			changed = discrepancy(*baseline, current)
		} else {
			changed = wallSeconds(baseline.WallUTC, current.WallUTC) < -ClockChangeTolerance.Seconds()
		}
		if changed {
			state.Incident.Revision++
			state.Incident.Current = current
			event.Changed = true
			g.last = &current
			if err := g.save(state); err != nil {
				return event, err
			}
		} else if g.incidentDirty {
			if err := g.save(state); err != nil {
				return event, err
			}
		}
		g.last = &current
		incident := *state.Incident
		event.Incident = &incident
		return event, nil
	}
	reason := ""
	if g.malformed {
		reason = "persisted clock checkpoint is invalid"
	} else if (current.WallUTC.Year() < 2020 || current.WallUTC.Year() > 2100) && !state.AcceptedExplicit {
		reason = "system date is outside the expected range"
	} else if previous != nil {
		if !startup || sameBoot(*previous, current) {
			if discrepancy(*previous, current) {
				reason = "system clock changed relative to elapsed time"
			}
			if reason == "" && state.Anchor != nil && sameBoot(*state.Anchor, current) && discrepancy(*state.Anchor, current) {
				reason = "system clock drifted from the accepted time"
			}
		} else {
			delta := wallSeconds(previous.WallUTC, current.WallUTC)
			if delta < -ClockChangeTolerance.Seconds() {
				reason = "system time moved backward since the previous start"
			} else if delta > ClockUnknownBootGap.Seconds() {
				reason = "system date advanced substantially while Focusd was stopped"
			}
		}
	}
	if reason != "" {
		id, err := randomClockID()
		if err != nil {
			return event, err
		}
		lastSafe := event.LastSafe
		if lastSafe.IsZero() {
			lastSafe = current.WallUTC
		}
		state.Incident = &storage.ClockIncident{ID: id, Revision: 1, Reason: reason, LastSafe: lastSafe, Current: current, DetectedAt: current.WallUTC}
		state.RetentionHold = true
		g.last = &current
		if err := g.save(state); err != nil {
			return event, err
		}
		g.malformed = false
		g.last = &current
		incident := *state.Incident
		event.Incident = &incident
		event.Created = true
		return event, nil
	}
	state.Accepted = &current
	if state.Anchor == nil || startup && !sameBoot(*state.Anchor, current) {
		anchor := current
		state.Anchor = &anchor
	}
	if startup || g.state.Accepted == nil || current.InterruptTicks < g.lastSavedTicks || current.InterruptTicks-g.lastSavedTicks >= 30*clockTicksPerSecond {
		if err := g.save(state); err != nil {
			return event, err
		}
		g.lastSavedTicks = current.InterruptTicks
	} else {
		g.state = state
	}
	g.last = &current
	return event, nil
}

// Accept rechecks the current sample against the displayed incident before adopting it.
func (g *ClockGuard) Accept(id string, revision int64, sample system.ClockSample) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	state := cloneClockState(g.state)
	if state.Incident == nil || state.Incident.ID != id || state.Incident.Revision != revision {
		return fmt.Errorf("clock decision is no longer current")
	}
	current := checkpoint(sample)
	if current.WallUTC.IsZero() {
		return fmt.Errorf("empty system clock sample")
	}
	baseline := state.Incident.Current
	if g.last != nil {
		baseline = *g.last
	}
	changed := false
	if sameBoot(baseline, current) || g.last != nil {
		changed = discrepancy(baseline, current)
	} else {
		changed = wallSeconds(baseline.WallUTC, current.WallUTC) < -ClockChangeTolerance.Seconds()
	}
	if changed {
		state.Incident.Revision++
		state.Incident.Current = current
		g.last = &current
		if err := g.save(state); err != nil {
			return err
		}
		g.last = &current
		return fmt.Errorf("system clock changed again; review the new warning")
	}
	state.Incident = nil
	state.Accepted = &current
	anchor := current
	state.Anchor = &anchor
	state.AcceptedExplicit = true
	state.RetentionHold = true
	state.Epoch++
	if err := g.save(state); err != nil {
		return err
	}
	g.last = &current
	g.lastSavedTicks = current.InterruptTicks
	return nil
}
func (g *ClockGuard) SetRetentionHold(hold bool) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	state := cloneClockState(g.state)
	if !hold && state.Incident != nil {
		return fmt.Errorf("resolve the clock warning before releasing cleanup")
	}
	state.RetentionHold = hold
	return g.save(state)
}

// Persist checkpoints the latest accepted observation on clean daemon shutdown.
func (g *ClockGuard) Persist() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.save(cloneClockState(g.state))
}
