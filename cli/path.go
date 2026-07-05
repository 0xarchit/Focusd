package cli

import (
	"fmt"
	"focusd/system"
	"focusd/ui"
	"os"
)

func runPathEnable() {
	if err := system.EnablePath(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to add to PATH: %v", err))
		os.Exit(1)
	}
	ui.PrintOK("Added to user PATH.")
	ui.PrintWarn("Restart your terminal for changes to take effect.")
}

func runPathDisable() {
	if err := system.DisablePath(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to remove from PATH: %v", err))
		os.Exit(1)
	}
	ui.PrintOK("Removed from user PATH.")
	ui.PrintWarn("Restart your terminal for changes to take effect.")
}

func runPathStatus() {
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
