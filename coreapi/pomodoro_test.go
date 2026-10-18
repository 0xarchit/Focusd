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

// TestCheckPomodoroNotifiesOnce checks the happy path: the completion is
// announced exactly once and the completed state is committed.
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

// TestCheckPomodoroRetriesThrottledToast pins that a throttled toast does not
// burn the one-shot completion: the timer stays un-notified and the next check
// announces it.
func TestCheckPomodoroRetriesThrottledToast(t *testing.T) {
	setupPomodoroEnv(t)
	expiredPomodoro(t)

	throttled := 0
	CheckPomodoroAndNotify(func(title, msg string) bool {
		throttled++
		return false
	})
	if throttled != 1 {
		t.Fatalf("notify called %d times; want 1", throttled)
	}

	state, _ := LoadPomodoroStateRaw()
	if !state.Active || state.Notified {
		t.Fatalf("a throttled toast must leave the timer un-notified (active=%v notified=%v)", state.Active, state.Notified)
	}

	announced := 0
	CheckPomodoroAndNotify(func(title, msg string) bool {
		announced++
		return true
	})
	if announced != 1 {
		t.Errorf("retry notify called %d times; want 1", announced)
	}

	state, _ = LoadPomodoroStateRaw()
	if state.Active || !state.Notified {
		t.Errorf("timer should be completed after the retry (active=%v notified=%v)", state.Active, state.Notified)
	}
}

// TestStartPomodoroClampsMinutes pins that a huge minute count cannot overflow
// time.Duration and make the timer complete instantly.
func TestStartPomodoroClampsMinutes(t *testing.T) {
	setupPomodoroEnv(t)

	if err := StartPomodoro(1 << 40); err != nil {
		t.Fatalf("StartPomodoro failed: %v", err)
	}

	state, _ := LoadPomodoroStateRaw()
	if state.Duration != MaxPomodoroMinutes {
		t.Errorf("duration = %d; want clamp to %d", state.Duration, MaxPomodoroMinutes)
	}

	active, remaining, _ := GetPomodoroStatus()
	if !active {
		t.Fatal("timer should be active right after starting")
	}
	if remaining <= 0 {
		t.Errorf("remaining = %v; want a positive remaining time", remaining)
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
