package storage

import (
	"errors"
	"testing"
	"time"
)

func TestClockStateMissingIsHealthyLegacyBaseline(t *testing.T) {
	setupStorageEnv(t)
	state, err := GetClockState()
	if err != nil || state.Accepted != nil || TimeGuarded() || RetentionHeld() {
		t.Fatalf("legacy state = %+v, %v", state, err)
	}
}
func TestClockStateRoundTripAndPendingGuard(t *testing.T) {
	setupStorageEnv(t)
	now := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
	state := ClockState{Accepted: &ClockCheckpoint{WallUTC: now, BootID: "boot"}, RetentionHold: true, Incident: &ClockIncident{ID: "incident", Revision: 2, Current: ClockCheckpoint{WallUTC: now.Add(-time.Hour)}}}
	if err := SaveClockState(state); err != nil {
		t.Fatal(err)
	}
	got, err := GetClockState()
	if err != nil || got.Incident == nil || got.Incident.Revision != 2 || !TimeGuarded() || !RetentionHeld() {
		t.Fatalf("state = %+v, %v", got, err)
	}
}
func TestClockStateCorruptFailsClosed(t *testing.T) {
	setupStorageEnv(t)
	if err := SetConfig(clockStateKey, "{"); err != nil {
		t.Fatal(err)
	}
	_, err := GetClockState()
	if !errors.Is(err, ErrMalformedClockState) || !TimeGuarded() || !RetentionHeld() {
		t.Fatalf("corrupt state guard err=%v", err)
	}
}
