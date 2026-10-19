package cli

import (
	"testing"

	"focusd/storage"
)

// TestRunPauseResumePersistState pins that pause and resume open the database
// themselves: without storage.Init in the throwaway CLI process the pause
// state could never be written.
func TestRunPauseResumePersistState(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())

	storage.Close()
	t.Cleanup(func() { storage.Close() })

	runPause()
	if err := storage.Init(); err != nil {
		t.Fatalf("reopen storage: %v", err)
	}
	if !storage.IsPaused() {
		t.Fatal("runPause did not persist the paused state")
	}

	runResume()
	if err := storage.Init(); err != nil {
		t.Fatalf("reopen storage: %v", err)
	}
	if storage.IsPaused() {
		t.Fatal("runResume did not persist the resumed state")
	}
}
