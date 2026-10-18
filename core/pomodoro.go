package core

import (
	"focusd/coreapi"
)

// pomodoroNotify is the toast channel for timer completion; tests swap it.
var pomodoroNotify = func(title, msg string) bool {
	return showNotification(notifyContext, title, msg)
}

// CheckPomodoroCompletion finishes an expired pomodoro and notifies through
// the toast channel. The daemon calls it on every tick; the CLI (status read)
// and the TUI (periodic refresh) also call it, so a timer started while the
// daemon is stopped still completes instead of waiting forever.
func CheckPomodoroCompletion() {
	coreapi.CheckPomodoroAndNotify(pomodoroNotify)
}
