package coreapi

import (
	"focusd/storage"
	"focusd/system"
	"testing"
	"time"
)

func TestClockWarningFreezesAndReanchorsExactTimerDuration(t *testing.T) {
	setupPomodoroEnv(t)
	start := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
	if err := SavePomodoroState(&PomodoroState{Active: true, StartTime: start, Duration: 25}); err != nil {
		t.Fatal(err)
	}
	if err := FreezePomodoroAt(start.Add(5*time.Minute + 17*time.Second)); err != nil {
		t.Fatal(err)
	}
	state, _ := LoadPomodoroStateRaw()
	want := 19*time.Minute + 43*time.Second
	if !state.Frozen || pomodoroRemainingAt(state, start.Add(2*time.Hour)) != want {
		t.Fatalf("frozen=%+v remaining=%s", state, pomodoroRemainingAt(state, start.Add(2*time.Hour)))
	}
	accepted := start.Add(-24 * time.Hour)
	if err := ResumeFrozenPomodoroAt(accepted); err != nil {
		t.Fatal(err)
	}
	state, _ = LoadPomodoroStateRaw()
	if state.Frozen || pomodoroRemainingAt(state, accepted.Add(time.Minute)) != want-time.Minute {
		t.Fatalf("resumed=%+v", state)
	}
}

func TestPomodoroElapsedAnchorIgnoresSmallWallClockCorrections(t *testing.T) {
	start := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
	state := &PomodoroState{Active: true, StartTime: start, Duration: 25, BootID: "boot", InterruptTicks: 10000000000}
	sample := system.ClockSample{WallUTC: start.Add(10*time.Minute + time.Minute), BootID: "boot", InterruptTicks: state.InterruptTicks + uint64(10*time.Minute)/100}
	if got := pomodoroRemainingAtSample(state, sample); got != 15*time.Minute {
		t.Fatalf("remaining=%s want 15m despite one-minute wall correction", got)
	}
	sample.InterruptTicks = ^uint64(0)
	if got := pomodoroRemainingAtSample(state, sample); got != 0 {
		t.Fatalf("huge elapsed reading overflowed remaining=%s", got)
	}
}
func TestClockGuardSuppressesLazyCompletionAndNewTimers(t *testing.T) {
	setupPomodoroEnv(t)
	expiredPomodoro(t)
	now := time.Now()
	if err := storage.SaveClockState(storage.ClockState{Incident: &storage.ClockIncident{ID: "pending", Revision: 1, Current: storage.ClockCheckpoint{WallUTC: now}}}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	CheckPomodoroAndNotify(func(string, string) bool { calls++; return true })
	if calls != 0 {
		t.Fatal("guarded timer announced completion")
	}
	if err := StartPomodoro(10); err == nil {
		t.Fatal("guarded profile started timer")
	}
	state, _ := LoadPomodoroStateRaw()
	if !state.Active || state.Notified {
		t.Fatal("guarded completion changed state")
	}
}
func TestStopFrozenTimerClearsRemainingState(t *testing.T) {
	setupPomodoroEnv(t)
	if err := StartPomodoro(5); err != nil {
		t.Fatal(err)
	}
	if err := FreezePomodoroAt(time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := StopPomodoro(); err != nil {
		t.Fatal(err)
	}
	state, _ := LoadPomodoroStateRaw()
	if state.Active || state.Frozen || state.RemainingNS != nil {
		t.Fatalf("stop state=%+v", state)
	}
}
