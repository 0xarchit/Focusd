package coreapi

import (
	"encoding/json"
	"fmt"
	"focusd/storage"
	"focusd/system"
	"log"
	"os"
	"path/filepath"
	"time"
)

const defaultPomodoroMinutes = 25

// MaxPomodoroMinutes bounds the timer. Larger values can overflow
// time.Duration(minutes)*time.Minute into the negative, which made the timer
// read as already complete the moment it started.
const MaxPomodoroMinutes = 1440

type PomodoroState struct {
	Active         bool      `json:"active"`
	StartTime      time.Time `json:"start_time"`
	Duration       int       `json:"duration_minutes"`
	Notified       bool      `json:"notified"`
	Frozen         bool      `json:"frozen,omitempty"`
	RemainingNS    *int64    `json:"remaining_ns,omitempty"`
	BootID         string    `json:"boot_id,omitempty"`
	InterruptTicks uint64    `json:"interrupt_ticks,omitempty"`
}

var readPomodoroClock = system.ReadClockSample

func LoadPomodoroStateFresh() *PomodoroState {
	state, _ := LoadPomodoroStateRaw()
	return state
}

func LoadPomodoroStateRaw() (*PomodoroState, string) {
	state := &PomodoroState{Duration: defaultPomodoroMinutes}
	dataStr, err := storage.GetConfig("pomodoro_state")
	if err == nil && dataStr != "" {
		if err := json.Unmarshal([]byte(dataStr), state); err != nil {
			log.Printf("WARN: failed to parse pomodoro state: %v", err)
		}
		return state, dataStr
	}

	// Migrate from legacy pomodoro.json if present
	appData := os.Getenv("APPDATA")
	if appData != "" {
		legacyPath := filepath.Join(appData, "focusd", "pomodoro.json")
		if bytes, readErr := os.ReadFile(legacyPath); readErr == nil {
			var legacyState PomodoroState
			if unmarshalErr := json.Unmarshal(bytes, &legacyState); unmarshalErr == nil {
				if setErr := storage.SetConfig("pomodoro_state", string(bytes)); setErr == nil {
					os.Remove(legacyPath)
				}
				return &legacyState, string(bytes)
			}
		}
	}

	return state, ""
}

func SavePomodoroState(state *PomodoroState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return storage.SetConfig("pomodoro_state", string(data))
}

func SavePomodoroStateCAS(expectedRaw string, state *PomodoroState) (bool, error) {
	data, err := json.Marshal(state)
	if err != nil {
		return false, err
	}
	return storage.CompareAndSwapConfig("pomodoro_state", expectedRaw, string(data))
}

func StartPomodoro(minutes int) error {
	if storage.TimeGuarded() {
		return fmt.Errorf("resolve the clock warning before starting a timer")
	}
	for i := 0; i < 5; i++ {
		existing, raw := LoadPomodoroStateRaw()
		if existing.Active {
			return fmt.Errorf("pomodoro already active")
		}

		if minutes <= 0 {
			minutes = defaultPomodoroMinutes
		}
		if minutes > MaxPomodoroMinutes {
			minutes = MaxPomodoroMinutes
		}

		state := &PomodoroState{
			Active:    true,
			StartTime: time.Now(),
			Duration:  minutes,
			Notified:  false,
		}
		anchorPomodoroClock(state, state.StartTime)

		ok, err := SavePomodoroStateCAS(raw, state)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("failed to start pomodoro due to lock contention")
}

func StopPomodoro() error {
	for i := 0; i < 5; i++ {
		_, raw := LoadPomodoroStateRaw()
		state := &PomodoroState{
			Active:    false,
			StartTime: time.Time{},
			Duration:  defaultPomodoroMinutes,
			Notified:  false,
		}
		ok, err := SavePomodoroStateCAS(raw, state)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("failed to stop pomodoro due to lock contention")
}

func GetPomodoroStatus() (active bool, remaining time.Duration, total int) {
	state := LoadPomodoroStateFresh()
	if !state.Active {
		return false, 0, 0
	}

	remaining = currentPomodoroRemaining(state)

	if remaining <= 0 {
		return true, 0, state.Duration
	}

	return true, remaining, state.Duration
}

// CheckPomodoroAndNotify completes an expired timer. The completion is
// announced before the state is committed: a throttled toast must not mark the
// timer notified, or the single completion alert would be lost. A lost CAS can
// at worst re-announce, and any writer that beats it either marks the timer
// notified itself or starts a new one.
func CheckPomodoroAndNotify(notifyFn func(title, msg string) bool) {
	if storage.TimeGuarded() {
		return
	}
	for i := 0; i < 5; i++ {
		state, raw := LoadPomodoroStateRaw()
		if !state.Active || state.Notified || state.Frozen {
			return
		}

		if currentPomodoroRemaining(state) > 0 {
			return
		}

		if notifyFn != nil && !notifyFn("Pomodoro Complete!", "Great work! Take a break.") {
			// The toast was throttled; leave the timer un-notified so the next
			// check retries instead of dropping the only completion alert.
			return
		}

		state.Notified = true
		state.Active = false
		ok, err := SavePomodoroStateCAS(raw, state)
		if err != nil {
			log.Printf("ERROR: failed to save pomodoro state: %v", err)
			return
		}
		if !ok {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		return
	}
}

func pomodoroRemainingAt(state *PomodoroState, at time.Time) time.Duration {
	if !state.Active {
		return 0
	}
	duration := time.Duration(min(max(state.Duration, 1), MaxPomodoroMinutes)) * time.Minute
	if state.RemainingNS != nil {
		duration = time.Duration(min(max(*state.RemainingNS, 0), int64(MaxPomodoroMinutes)*int64(time.Minute)))
	}
	if state.Frozen {
		return duration
	}
	elapsed := at.Sub(state.StartTime)
	if elapsed < 0 {
		elapsed = 0
	}
	return max(0, duration-elapsed)
}

func currentPomodoroRemaining(state *PomodoroState) time.Duration {
	if sample, err := readPomodoroClock(); err == nil {
		return pomodoroRemainingAtSample(state, sample)
	}
	return pomodoroRemainingAt(state, time.Now())
}

func pomodoroRemainingAtSample(state *PomodoroState, sample system.ClockSample) time.Duration {
	if !state.Active {
		return 0
	}
	if state.Frozen {
		return pomodoroRemainingAt(state, sample.WallUTC)
	}
	if state.BootID != "" && sample.BootID == state.BootID && sample.InterruptTicks >= state.InterruptTicks {
		duration := time.Duration(min(max(state.Duration, 1), MaxPomodoroMinutes)) * time.Minute
		if state.RemainingNS != nil {
			duration = time.Duration(min(max(*state.RemainingNS, 0), int64(MaxPomodoroMinutes)*int64(time.Minute)))
		}
		ticks := sample.InterruptTicks - state.InterruptTicks
		if ticks > uint64(duration)/100 {
			return 0
		}
		return max(0, duration-time.Duration(ticks)*100)
	}
	return pomodoroRemainingAt(state, sample.WallUTC)
}

func anchorPomodoroClock(state *PomodoroState, at time.Time) {
	state.BootID = ""
	state.InterruptTicks = 0
	if sample, err := readPomodoroClock(); err == nil && sample.WallUTC.Sub(at).Abs() <= 2*time.Second {
		state.BootID = sample.BootID
		state.InterruptTicks = sample.InterruptTicks
	}
}

// FreezePomodoroAt preserves only the duration left at the last accepted sample.
func FreezePomodoroAt(lastSafe time.Time) error {
	return freezePomodoro(func(state *PomodoroState) time.Duration { return pomodoroRemainingAt(state, lastSafe) })
}

func FreezePomodoroAtSample(lastSafe system.ClockSample) error {
	return freezePomodoro(func(state *PomodoroState) time.Duration { return pomodoroRemainingAtSample(state, lastSafe) })
}

func freezePomodoro(remainingAt func(*PomodoroState) time.Duration) error {
	for attempt := 0; attempt < 5; attempt++ {
		state, raw := LoadPomodoroStateRaw()
		if !state.Active || state.Frozen {
			return nil
		}
		remaining := int64(remainingAt(state))
		state.RemainingNS = &remaining
		state.Frozen = true
		ok, err := SavePomodoroStateCAS(raw, state)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	return fmt.Errorf("failed to freeze timer due to lock contention")
}

// ResumeFrozenPomodoroAt reanchors the exact remaining duration to accepted wall time.
func ResumeFrozenPomodoroAt(now time.Time) error {
	for attempt := 0; attempt < 5; attempt++ {
		state, raw := LoadPomodoroStateRaw()
		if !state.Active || !state.Frozen {
			return nil
		}
		state.StartTime = now
		state.Frozen = false
		anchorPomodoroClock(state, now)
		ok, err := SavePomodoroStateCAS(raw, state)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	return fmt.Errorf("failed to resume timer due to lock contention")
}
