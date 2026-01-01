package cli

import (
	"fmt"
	"focusd/storage"
	"focusd/ui"
	"os"
	"strconv"
)

func RunRetentionStatus() {
	if err := storage.Init(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to initialize: %v", err))
		os.Exit(1)
	}
	defer storage.Close()

	days := storage.GetRetentionDays()
	ui.PrintInfo(fmt.Sprintf("Data retention: %d days", days))
	fmt.Printf("Data older than %d days is automatically deleted.\n", days)
	fmt.Printf("Allowed range: %d-%d days. Default: %d days.\n",
		storage.MinRetentionDays, storage.MaxRetentionDays, storage.DefaultRetentionDays)
}

func RunRetentionSet(daysStr string) {
	if err := storage.Init(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to initialize: %v", err))
		os.Exit(1)
	}
	defer storage.Close()

	days, err := strconv.Atoi(daysStr)
	if err != nil {
		ui.PrintError("Invalid number of days. Please provide a number between 1 and 30.")
		os.Exit(1)
	}

	if err := SetRetentionLogic(days); err != nil {
		ui.PrintError(err.Error())
		os.Exit(1)
	}
}

func RunRetentionReset() {
	if err := storage.Init(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to initialize: %v", err))
		os.Exit(1)
	}
	defer storage.Close()

	if err := storage.SetRetentionDays(storage.DefaultRetentionDays); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to reset retention: %v", err))
		os.Exit(1)
	}

	ui.PrintOK(fmt.Sprintf("Retention reset to %d days (default).", storage.DefaultRetentionDays))
}
