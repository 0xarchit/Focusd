package core

import (
	"focusd/coreapi"
)

func checkPomodoroAndNotify() {
	coreapi.CheckPomodoroAndNotify(func(title, msg string) bool {
		return showNotification(notifyContext, title, msg)
	})
}
