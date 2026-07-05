package cli

import (
	"fmt"
	"focusd/system"
	"focusd/ui"
	"os"
)

func handleBrowsersCommand(args []string) {
	if len(args) < 3 {
		runBrowserList()
		return
	}

	switch args[2] {
	case "list":
		runBrowserList()
	case "add":
		if len(args) < 4 {
			fmt.Println("Usage: focusd browser add <exe_name>")
			os.Exit(1)
		}
		runBrowserAdd(args[3])
	case "remove":
		if len(args) < 4 {
			fmt.Println("Usage: focusd browser remove <exe_name>")
			os.Exit(1)
		}
		runBrowserRemove(args[3])
	default:
		fmt.Printf("Unknown browser command: %s\n", args[2])
		fmt.Println("Available: list, add, remove")
		os.Exit(1)
	}
}

func runBrowserList() {
	ui.PrintSectionHeader("Browser Configuration")

	customs := system.GetCustomBrowsersList()
	if len(customs) > 0 {
		fmt.Println("  User-Defined Browsers:")
		for _, b := range customs {
			fmt.Printf("   • %s\n", b)
		}
		fmt.Println()
	} else {
		fmt.Println("  No user-defined browsers.")
		fmt.Println()
	}

	fmt.Println("  (Default browsers like Chrome, Edge, and Firefox are automatically supported)")
	fmt.Println()
}

func runBrowserAdd(exeName string) {
	if err := system.AddCustomBrowser(exeName); err != nil {
		ui.PrintError(err.Error())
		os.Exit(1)
	}
	ui.PrintOK(fmt.Sprintf("Added '%s' to browser list.", exeName))
}

func runBrowserRemove(exeName string) {
	if err := system.RemoveCustomBrowser(exeName); err != nil {
		ui.PrintError(err.Error())
		os.Exit(1)
	}
	ui.PrintOK(fmt.Sprintf("Removed '%s' from browser list.", exeName))
}
