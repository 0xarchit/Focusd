package cli

import (
	"fmt"
	"focusd/storage"
	"focusd/system"
	"focusd/ui"
	"os"
)

func RunAutostartEnable() {
	if err := storage.Init(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to initialize: %v", err))
		os.Exit(1)
	}
	defer storage.Close()

	if err := EnableAutostartLogic(); err != nil {
		if err.Error() == "not initialized" {
			ui.PrintError("focusd is not initialized. Run 'focusd init' first.")
		} else {
			ui.PrintError(err.Error())
		}
		os.Exit(1)
	}
}

func RunAutostartDisable() {
	if err := storage.Init(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to initialize: %v", err))
		os.Exit(1)
	}
	defer storage.Close()

	if err := DisableAutostartLogic(); err != nil {
		ui.PrintError(err.Error())
		os.Exit(1)
	}
}

func RunAutostartStatus() {
	enabled, path, err := system.GetAutoStartEnabled()
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to check auto-start status: %v", err))
		os.Exit(1)
	}

	if enabled {
		ui.PrintInfo("Auto-start is ENABLED")
		fmt.Printf("Executable: %s\n", path)
		fmt.Println("focusd will start automatically on Windows boot.")
	} else {
		ui.PrintInfo("Auto-start is DISABLED")
		fmt.Println("Run 'focusd autostart enable' to enable auto-start.")
	}
}
