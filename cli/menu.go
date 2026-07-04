package cli

import (
	"fmt"
	"focusd/storage"
	"focusd/tui"
	"os"
)

func RunInteractiveMenu() {
	if err := storage.Init(); err != nil {
		fmt.Printf("Failed to initialize: %v\n", err)
		os.Exit(1)
	}
	defer storage.Close()

	if err := tui.StartTUI(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
