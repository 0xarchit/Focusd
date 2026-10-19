package core

import (
	"testing"
	"time"

	"focusd/storage"
)

func newPersistTestTracker() *Tracker {
	return &Tracker{
		pollInterval: time.Second,
		currentSession: &activeSession{
			AppName:     "TestApp",
			ExeName:     "test.exe",
			WindowTitle: "Window",
			StartTime:   time.Now(),
			Date:        "2026-06-26",
		},
	}
}

// markPersisted simulates a completed write so the change detection can be
// exercised without touching the database.
func markPersisted(t *Tracker) {
	t.persistedAppName = t.currentSession.AppName
	t.persistedExeName = t.currentSession.ExeName
	t.persistedWindowTitle = t.currentSession.WindowTitle
	t.persistedStartTime = t.currentSession.StartTime
	t.persistedDate = t.currentSession.Date
}

// TestNeedsPersistDetectsChange covers the comparison that decides whether a
// write is needed at all.
func TestNeedsPersistDetectsChange(t *testing.T) {
	tracker := newPersistTestTracker()

	if !tracker.needsPersist() {
		t.Error("a fresh session should need a first persist")
	}

	markPersisted(tracker)

	if tracker.needsPersist() {
		t.Error("an unchanged session should not need a persist")
	}

	tracker.currentSession.WindowTitle = "Different Window"
	if !tracker.needsPersist() {
		t.Error("a title change should need a persist")
	}
}

func TestNeedsPersistIgnoresLastSeenOnly(t *testing.T) {
	tracker := newPersistTestTracker()
	markPersisted(tracker)

	// lastSeen is not part of the comparison, so an unchanged session does not
	// trigger a rewrite on every poll.
	if tracker.needsPersist() {
		t.Error("lastSeen alone must not mark the session as changed")
	}
}

func TestNeedsPersistDetectsMidnightRebase(t *testing.T) {
	tracker := newPersistTestTracker()
	markPersisted(tracker)

	// A midnight rebase changes start time and date, which must force a write
	// or the stored session keeps pointing at yesterday.
	tracker.currentSession.StartTime = tracker.currentSession.StartTime.Add(2 * time.Hour)
	tracker.currentSession.Date = "2026-06-27"

	if !tracker.needsPersist() {
		t.Error("a midnight rebase should force a persist")
	}
}

func TestNeedsPersistNilSession(t *testing.T) {
	tracker := &Tracker{}
	if tracker.needsPersist() {
		t.Error("a nil session should not need a persist")
	}
}

// The tests below assert on write counts rather than timestamps: two writes can
// land inside a single Windows clock tick, so two timestamps taken moments
// apart are not reliably distinguishable.

func TestWriteActiveSessionThrottle(t *testing.T) {
	setupTestDB(t)

	tracker := newPersistTestTracker()

	// First write always happens.
	tracker.writeActiveSession(false)
	if tracker.persistWrites != 1 {
		t.Fatalf("expected 1 write, got %d", tracker.persistWrites)
	}

	// Immediate repeat with nothing changed must be skipped.
	tracker.writeActiveSession(false)
	if tracker.persistWrites != 1 {
		t.Errorf("unchanged session was rewritten inside the throttle window, writes = %d", tracker.persistWrites)
	}

	// Forcing bypasses the throttle.
	tracker.forcePersistActiveSession()
	if tracker.persistWrites != 2 {
		t.Errorf("a forced persist should have written, writes = %d", tracker.persistWrites)
	}
}

func TestWriteActiveSessionPersistsChangeImmediately(t *testing.T) {
	setupTestDB(t)

	tracker := newPersistTestTracker()
	tracker.writeActiveSession(false)

	tracker.currentSession.WindowTitle = "New Window"
	tracker.writeActiveSession(false)

	if tracker.persistWrites != 2 {
		t.Errorf("a title change should have written straight away, writes = %d", tracker.persistWrites)
	}
	if tracker.persistedWindowTitle != "New Window" {
		t.Errorf("persisted title = %q; want the new title", tracker.persistedWindowTitle)
	}
}

func TestWriteActiveSessionThrottleExpires(t *testing.T) {
	setupTestDB(t)

	tracker := newPersistTestTracker()
	tracker.writeActiveSession(false)

	// Pretend the last write happened longer ago than the throttle window so
	// lastSeen gets refreshed, which orphan recovery depends on.
	tracker.lastPersistAt = time.Now().Add(-2 * persistInterval)

	tracker.writeActiveSession(false)

	if tracker.persistWrites != 2 {
		t.Errorf("the throttle should have expired and refreshed lastSeen, writes = %d", tracker.persistWrites)
	}
}

func TestWriteActiveSessionNilClearsRow(t *testing.T) {
	setupTestDB(t)

	tracker := newPersistTestTracker()
	tracker.writeActiveSession(false)

	tracker.currentSession = nil
	tracker.writeActiveSession(false)

	active, err := storage.GetActiveSession()
	if err != nil {
		t.Fatalf("GetActiveSession failed: %v", err)
	}
	if active != nil {
		t.Error("expected the stored session to be cleared when nothing is active")
	}
}
