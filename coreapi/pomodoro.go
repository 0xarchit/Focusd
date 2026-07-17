package coreapi

import (
	"encoding/json"
	"focusd/storage"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var pomodoroMu sync.Mutex

const defaultPomodoroMinutes = 25

type PomodoroState struct {
	Active    bool      `json:"active"`
	StartTime time.Time `json:"start_time"`
	Duration  int       `json:"duration_minutes"`
	Notified  bool      `json:"notified"`
}

func LoadPomodoroStateFresh() *PomodoroState {
	state := &PomodoroState{Duration: defaultPomodoroMinutes}
	dataStr, err := storage.GetConfig("pomodoro_state")
	if err == nil && dataStr != "" {
		if err := json.Unmarshal([]byte(dataStr), state); err != nil {
			log.Printf("WARN: failed to parse pomodoro state: %v", err)
		}
		return state
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
				return &legacyState
			}
		}
	}

	return state
}

func SavePomodoroState(state *PomodoroState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return storage.SetConfig("pomodoro_state", string(data))
}

func StartPomodoro(minutes int) error {
	pomodoroMu.Lock()
	defer pomodoroMu.Unlock()

	existing := LoadPomodoroStateFresh()
	if existing.Active {
		return fmt.Errorf("pomodoro already active")
	}

	if minutes <= 0 {
		minutes = defaultPomodoroMinutes
	}

	state := &PomodoroState{
		Active:    true,
		StartTime: time.Now(),
		Duration:  minutes,
		Notified:  false,
	}

	return SavePomodoroState(state)
}

func StopPomodoro() error {
	pomodoroMu.Lock()
	defer pomodoroMu.Unlock()
	state := &PomodoroState{
		Active:    false,
		StartTime: time.Time{},
		Duration:  defaultPomodoroMinutes,
		Notified:  false,
	}
	return SavePomodoroState(state)
}

func GetPomodoroStatus() (active bool, remaining time.Duration, total int) {
	pomodoroMu.Lock()
	defer pomodoroMu.Unlock()
	state := LoadPomodoroStateFresh()
	if !state.Active {
		return false, 0, 0
	}

	elapsed := time.Since(state.StartTime)
	totalDuration := time.Duration(state.Duration) * time.Minute
	remaining = totalDuration - elapsed

	if remaining <= 0 {
		return true, 0, state.Duration
	}

	return true, remaining, state.Duration
}
