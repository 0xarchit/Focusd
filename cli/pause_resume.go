package cli

import (
	"fmt"
	"focusd/storage"
	"focusd/ui"
)

func RunPause() {
	if !storage.IsConsentGranted() {
		ui.PrintError("focusd is not initialized. Run 'focusd init' first.")
		return
	}

	if storage.IsPaused() {
		ui.PrintInfo("Tracking is already paused.")
		return
	}

	if err := storage.SetPaused(true); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to pause tracking: %v", err))
		return
	}

	ui.PrintOK("Tracking paused.")
	fmt.Println("Run 'focusd resume' to resume tracking.")
}

func RunResume() {
	if !storage.IsConsentGranted() {
		ui.PrintError("focusd is not initialized. Run 'focusd init' first.")
		return
	}

	if !storage.IsPaused() {
		ui.PrintInfo("Tracking is already active.")
		return
	}

	if err := storage.SetPaused(false); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to resume tracking: %v", err))
		return
	}

	ui.PrintOK("Tracking resumed.")
}
