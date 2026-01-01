package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type PomodoroState struct {
	Active    bool      `json:"active"`
	StartTime time.Time `json:"start_time"`
	Duration  int       `json:"duration_minutes"`
	Notified  bool      `json:"notified"`
}

func getPomodoroPath() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return ""
	}
	return filepath.Join(appData, "focusd", "pomodoro.json")
}

func loadPomodoroStateFresh() *PomodoroState {
	state := &PomodoroState{
		Active:    false,
		StartTime: time.Time{},
		Duration:  25,
		Notified:  false,
	}

	path := getPomodoroPath()
	if path == "" {
		return state
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return state
	}

	json.Unmarshal(data, state)
	return state
}

func savePomodoroState(state *PomodoroState) error {
	path := getPomodoroPath()
	if path == "" {
		return nil
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0600); err != nil {
		return err
	}

	return os.Rename(tempPath, path)
}

func StartPomodoro(minutes int) error {
	if minutes <= 0 {
		minutes = 25
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
	state := &PomodoroState{
		Active:    false,
		StartTime: time.Time{},
		Duration:  25,
		Notified:  false,
	}
	return savePomodoroState(state)
}

func GetPomodoroStatus() (active bool, remaining time.Duration, total int) {
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

func IsPomodoroComplete() bool {
	state := loadPomodoroStateFresh()
	if !state.Active {
		return false
	}

	elapsed := time.Since(state.StartTime)
	totalDuration := time.Duration(state.Duration) * time.Minute

	return elapsed >= totalDuration
}

func CheckPomodoroAndNotify() {
	state := loadPomodoroStateFresh()
	if !state.Active {
		return
	}

	if state.Notified {
		return
	}

	elapsed := time.Since(state.StartTime)
	totalDuration := time.Duration(state.Duration) * time.Minute

	if elapsed >= totalDuration {
		ShowNotification("Pomodoro Complete!", "Great work! Take a break.")
		state.Notified = true
		state.Active = false
		savePomodoroState(state)
	}
}
