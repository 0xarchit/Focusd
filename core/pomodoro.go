package core

import (
	"focusd/coreapi"
)

// pomodoroNotify is the toast channel for timer completion; tests swap it.
var pomodoroNotify = func(title, msg string) bool {
	return showNotification(notifyContext, title, msg)
}

// CheckPomodoroCompletion finishes an expired pomodoro and notifies through
// the toast channel. The daemon calls it on every tick. The CLI (status read)
// and the TUI (periodic refresh) call it only while the daemon is stopped, so
// a timer still completes without waiting forever and the alert goes out once.
func CheckPomodoroCompletion() {
	coreapi.CheckPomodoroAndNotify(pomodoroNotify)
}
