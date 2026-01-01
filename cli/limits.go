package cli

import (
	"fmt"
	"focusd/system"
	"focusd/ui"
	"strconv"
)

func RunLimits(args []string) {
	if len(args) < 3 {
		showLimits()
		return
	}

	app := args[2]

	if len(args) < 4 {

		ui.PrintError("Usage: focusd limit <app_name> <minutes>")
		return
	}

	minutes, err := strconv.Atoi(args[3])
	if err != nil {
		ui.PrintError("Invalid minutes. usage: focusd limit <app> <minutes>")
		return
	}

	if err := system.SetAppTimeLimit(app, minutes); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to set limit: %v", err))
		return
	}

	if minutes > 0 {
		ui.PrintOK(fmt.Sprintf("Limit set: %s -> %d mins/day", app, minutes))
	} else {
		ui.PrintOK(fmt.Sprintf("Limit removed for %s", app))
	}
}

func showLimits() {
	ui.PrintHeader()
	fmt.Println("App Time Limits:")
	fmt.Println()

	limits := system.GetAppTimeLimits()
	if len(limits) == 0 {
		fmt.Println("  No limits set.")
		return
	}

	for app, min := range limits {
		fmt.Printf("  %-20s : %d mins\n", app, min)
	}
}
