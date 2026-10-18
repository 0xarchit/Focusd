package cli

import (
	"testing"

	"focusd/coreapi"
	"focusd/storage"
)

// TestRunFocusStartsTimer pins that the focus command opens the database
// itself: it runs in a throwaway CLI process, and without storage.Init the
// timer never started ("Failed to start timer: database not initialized"),
// so the awaited completion toast could never fire.
func TestRunFocusStartsTimer(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())

	storage.Close()
	t.Cleanup(func() { storage.Close() })

	runFocus([]string{"focusd", "focus", "1"})

	if err := storage.Init(); err != nil {
		t.Fatalf("reopen storage: %v", err)
	}
	state, _ := coreapi.LoadPomodoroStateRaw()
	if !state.Active {
		t.Fatal("runFocus did not leave an active timer")
	}
	if state.Duration != 1 {
		t.Fatalf("timer duration = %d, want 1", state.Duration)
	}
}

func TestRunStopTimerStopsTimer(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())

	storage.Close()
	t.Cleanup(func() { storage.Close() })

	runFocus([]string{"focusd", "focus", "1"})
	runStopTimer()

	if err := storage.Init(); err != nil {
		t.Fatalf("reopen storage: %v", err)
	}
	state, _ := coreapi.LoadPomodoroStateRaw()
	if state.Active {
		t.Fatal("runStopTimer did not stop the timer")
	}
}
