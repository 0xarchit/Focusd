package core

import (
	"focusd/coreapi"
	"log"
	"time"
)

func checkPomodoroAndNotify() {
	state := coreapi.LoadPomodoroStateFresh()
	if !state.Active || state.Notified {
		return
	}

	elapsed := time.Since(state.StartTime)
	totalDuration := time.Duration(state.Duration) * time.Minute

	if elapsed >= totalDuration {
		showNotification("Pomodoro Complete!", "Great work! Take a break.")
		state.Notified = true
		state.Active = false
		if err := coreapi.SavePomodoroState(state); err != nil {
			log.Printf("ERROR: failed to save pomodoro state: %v", err)
		}
	}
}
