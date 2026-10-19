package core

import (
	"encoding/json"
	"testing"
	"time"

	"focusd/coreapi"
	"focusd/storage"
)

// TestCheckPomodoroCompletionCompletesExpiredTimer pins that a lazy status
// read finishes an expired timer even though the daemon's ticker is not
// running.
func TestCheckPomodoroCompletionCompletesExpiredTimer(t *testing.T) {
	setupTestDB(t)

	state := &coreapi.PomodoroState{
		Active:    true,
		StartTime: time.Now().Add(-31 * time.Minute),
		Duration:  25,
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if err := storage.SetConfig("pomodoro_state", string(data)); err != nil {
		t.Fatalf("SetConfig failed: %v", err)
	}

	original := pomodoroNotify
	notified := 0
	pomodoroNotify = func(title, msg string) bool {
		notified++
		return true
	}
	t.Cleanup(func() { pomodoroNotify = original })

	CheckPomodoroCompletion()

	if notified != 1 {
		t.Errorf("notify called %d times; want 1", notified)
	}
	after, _ := coreapi.LoadPomodoroStateRaw()
	if after.Active || !after.Notified {
		t.Errorf("timer should be completed (active=%v notified=%v)", after.Active, after.Notified)
	}

	// Idempotent: a second read must not re-announce.
	CheckPomodoroCompletion()
	if notified != 1 {
		t.Errorf("notify called %d times after a second read; want 1", notified)
	}
}
