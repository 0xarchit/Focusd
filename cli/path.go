package cli

import (
	"fmt"
	"focusd/storage"
	"focusd/system"
	"focusd/ui"
	"os"
)

func RunPathEnable() {
	if err := storage.Init(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to initialize: %v", err))
		os.Exit(1)
	}
	defer storage.Close()

	if err := EnablePathLogic(); err != nil {
		if err.Error() == "not initialized" {
			ui.PrintError("focusd is not initialized. Run 'focusd init' first.")
		} else {
			ui.PrintError(err.Error())
		}
		os.Exit(1)
	}
}

func RunPathDisable() {
	if err := storage.Init(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to initialize: %v", err))
		os.Exit(1)
	}
	defer storage.Close()

	if err := DisablePathLogic(); err != nil {
		ui.PrintError(err.Error())
		os.Exit(1)
	}
}

func RunPathStatus() {
	enabled, err := system.GetPathEnabled()
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to check PATH status: %v", err))
		os.Exit(1)
	}

	exePath, _ := os.Executable()

	if enabled {
		ui.PrintInfo("PATH integration is ENABLED")
		fmt.Println("focusd directory is in your user PATH.")
	} else {
		ui.PrintInfo("PATH integration is DISABLED")
		fmt.Printf("To run focusd from anywhere, use 'focusd path enable'\n")
		fmt.Printf("Or run from: %s\n", exePath)
	}
}
