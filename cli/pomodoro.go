package cli

import (
	"fmt"
	"focusd/core"
	"focusd/coreapi"
	"focusd/storage"
	"focusd/system"
	"focusd/ui"
	"os"
	"strconv"
)

func runFocus(args []string) {
	minutes := system.GetPomodoroMinutes()
	if len(args) > 2 {
		m, err := strconv.Atoi(args[2])
		if err != nil || m <= 0 {
			ui.PrintError("Invalid focus duration. Please provide a positive number of minutes.")
			os.Exit(1)
		}
		minutes = m
	}
	minutes = min(minutes, coreapi.MaxPomodoroMinutes)

	if err := storage.Init(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to initialize: %v", err))
		os.Exit(1)
	}
	defer storage.Close()

	if err := core.ChangePomodoro(true, minutes); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to start timer: %v", err))
		os.Exit(1)
	}

	ui.PrintHeader()
	ui.PrintOK(fmt.Sprintf("Focus timer started for %d minutes.", minutes))
	fmt.Println("You will be notified when it completes; run 'focusd status' if the daemon is stopped.")
}

func runStopTimer() {
	if err := storage.Init(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to initialize: %v", err))
		os.Exit(1)
	}
	defer storage.Close()

	if err := core.ChangePomodoro(false, 0); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to stop timer: %v", err))
		os.Exit(1)
	}
	ui.PrintOK("Timer stopped.")
}
