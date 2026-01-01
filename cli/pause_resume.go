package cli

import (
	"fmt"
	"focusd/storage"
	"focusd/ui"
	"os"
)

func RunPause() {
	if err := storage.Init(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to initialize: %v", err))
		os.Exit(1)
	}
	defer storage.Close()

	if !storage.IsConsentGranted() {
		ui.PrintError("focusd is not initialized. Run 'focusd init' first.")
		os.Exit(1)
	}

	if storage.IsPaused() {
		ui.PrintInfo("Tracking is already paused.")
		return
	}

	if err := storage.SetPaused(true); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to pause tracking: %v", err))
		os.Exit(1)
	}

	ui.PrintOK("Tracking paused.")
	fmt.Println("Run 'focusd resume' to resume tracking.")
}

func RunResume() {
	if err := storage.Init(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to initialize: %v", err))
		os.Exit(1)
	}
	defer storage.Close()

	if !storage.IsConsentGranted() {
		ui.PrintError("focusd is not initialized. Run 'focusd init' first.")
		os.Exit(1)
	}

	if !storage.IsPaused() {
		ui.PrintInfo("Tracking is already active.")
		return
	}

	if err := storage.SetPaused(false); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to resume tracking: %v", err))
		os.Exit(1)
	}

	ui.PrintOK("Tracking resumed.")
}
