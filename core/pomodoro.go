package core

import (
	"encoding/json"
	"focusd/storage"
	"fmt"
	"log"
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

func loadPomodoroStateFresh() *PomodoroState {
	state := &PomodoroState{Duration: defaultPomodoroMinutes}
	dataStr, err := storage.GetConfig("pomodoro_state")
	if err == nil && dataStr != "" {
		if err := json.Unmarshal([]byte(dataStr), state); err != nil {
			log.Printf("WARN: failed to parse pomodoro state: %v", err)
		}
	}
	return state
}

func savePomodoroState(state *PomodoroState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return storage.SetConfig("pomodoro_state", string(data))
}

func StartPomodoro(minutes int) error {
	pomodoroMu.Lock()
	defer pomodoroMu.Unlock()

	existing := loadPomodoroStateFresh()
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

	return savePomodoroState(state)
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
	return savePomodoroState(state)
}

func GetPomodoroStatus() (active bool, remaining time.Duration, total int) {
	pomodoroMu.Lock()
	defer pomodoroMu.Unlock()
	state := loadPomodoroStateFresh()
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

func checkPomodoroAndNotify() {
	pomodoroMu.Lock()
	defer pomodoroMu.Unlock()

	state := loadPomodoroStateFresh()
	if !state.Active || state.Notified {
		return
	}

	elapsed := time.Since(state.StartTime)
	totalDuration := time.Duration(state.Duration) * time.Minute

	if elapsed >= totalDuration {
		showNotification("Pomodoro Complete!", "Great work! Take a break.")
		state.Notified = true
		state.Active = false
		if err := savePomodoroState(state); err != nil {
			log.Printf("ERROR: failed to save pomodoro state: %v", err)
		}
	}
}
