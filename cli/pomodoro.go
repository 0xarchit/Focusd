package cli

import (
	"fmt"
	"focusd/core"
	"focusd/ui"
	"strconv"
)

func RunFocus(args []string) {
	minutes := 25
	if len(args) > 2 {
		if m, err := strconv.Atoi(args[2]); err == nil && m > 0 {
			minutes = m
		}
	}

	if err := core.StartPomodoro(minutes); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to start timer: %v", err))
		return
	}

	ui.PrintHeader()
	ui.PrintOK(fmt.Sprintf("Focus timer started for %d minutes.", minutes))
	fmt.Println("You will be notified when it completes.")
}

func RunStopTimer() {
	if err := core.StopPomodoro(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to stop timer: %v", err))
		return
	}
	ui.PrintOK("Timer stopped.")
}
