package coreapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"focusd/storage"
)

func setupPomodoroEnv(t *testing.T) {
	t.Helper()
	dir := t.TempDir()

	original, had := os.LookupEnv("APPDATA")
	if err := os.Setenv("APPDATA", dir); err != nil {
		t.Fatalf("failed to set APPDATA: %v", err)
	}
	t.Cleanup(func() {
		storage.Close()
		if had {
			os.Setenv("APPDATA", original)
		} else {
			os.Unsetenv("APPDATA")
		}
	})

	storage.Close()
	if err := storage.Init(); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
}

// expiredPomodoro writes a timer that ended a minute ago.
func expiredPomodoro(t *testing.T) {
	t.Helper()
	state := &PomodoroState{
		Active:    true,
		StartTime: time.Now().Add(-31 * time.Minute),
		Duration:  25,
		Notified:  false,
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if err := storage.SetConfig("pomodoro_state", string(data)); err != nil {
		t.Fatalf("SetConfig failed: %v", err)
	}
}

// TestCheckPomodoroNotifiesOnce checks that a lost CAS cannot produce a second
// notification on retry.
func TestCheckPomodoroNotifiesOnce(t *testing.T) {
	setupPomodoroEnv(t)
	expiredPomodoro(t)

	calls := 0
	CheckPomodoroAndNotify(func(title, msg string) bool {
		calls++
		return true
	})

	if calls != 1 {
		t.Errorf("notify called %d times; want exactly 1", calls)
	}

	state, _ := LoadPomodoroStateRaw()
	if state.Active {
		t.Error("the timer should no longer be active")
	}
	if !state.Notified {
		t.Error("the timer should be marked notified")
	}
}

func TestCheckPomodoroDoesNotRepeat(t *testing.T) {
	setupPomodoroEnv(t)
	expiredPomodoro(t)

	CheckPomodoroAndNotify(func(title, msg string) bool { return true })

	// A second tick must be silent because the state is already committed.
	calls := 0
	CheckPomodoroAndNotify(func(title, msg string) bool {
		calls++
		return true
	})
	if calls != 0 {
		t.Errorf("notify called %d times on an already-completed timer; want 0", calls)
	}
}

func TestCheckPomodoroIgnoresActiveTimer(t *testing.T) {
	setupPomodoroEnv(t)

	state := &PomodoroState{
		Active:    true,
		StartTime: time.Now().Add(-1 * time.Minute),
		Duration:  25,
		Notified:  false,
	}
	data, _ := json.Marshal(state)
	if err := storage.SetConfig("pomodoro_state", string(data)); err != nil {
		t.Fatalf("SetConfig failed: %v", err)
	}

	calls := 0
	CheckPomodoroAndNotify(func(title, msg string) bool {
		calls++
		return true
	})
	if calls != 0 {
		t.Errorf("notify called %d times for a running timer; want 0", calls)
	}

	after, _ := LoadPomodoroStateRaw()
	if !after.Active {
		t.Error("a running timer must stay active")
	}
}

func TestCheckPomodoroNilCallback(t *testing.T) {
	setupPomodoroEnv(t)
	expiredPomodoro(t)

	// A nil callback must not panic; the state still has to be committed.
	CheckPomodoroAndNotify(nil)

	state, _ := LoadPomodoroStateRaw()
	if state.Active {
		t.Error("the timer should have been completed")
	}
}

func TestCheckPomodoroMigratesLegacyFile(t *testing.T) {
	setupPomodoroEnv(t)

	installDir := filepath.Join(os.Getenv("APPDATA"), "focusd")
	if err := os.MkdirAll(installDir, 0700); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	legacy := &PomodoroState{
		Active:    true,
		StartTime: time.Now().Add(-40 * time.Minute),
		Duration:  25,
		Notified:  false,
	}
	data, _ := json.Marshal(legacy)
	if err := os.WriteFile(filepath.Join(installDir, "pomodoro.json"), data, 0600); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	calls := 0
	CheckPomodoroAndNotify(func(title, msg string) bool {
		calls++
		return true
	})

	if calls != 1 {
		t.Errorf("notify called %d times after migrating a legacy timer; want 1", calls)
	}
	if _, err := os.Stat(filepath.Join(installDir, "pomodoro.json")); !os.IsNotExist(err) {
		t.Error("the legacy pomodoro file should have been removed after migration")
	}
}
