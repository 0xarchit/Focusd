package core

import (
	"focusd/coreapi"
)

func checkPomodoroAndNotify() {
	coreapi.CheckPomodoroAndNotify(showNotification)
}
