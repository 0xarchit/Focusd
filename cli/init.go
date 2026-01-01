package cli

import (
	"bufio"
	"fmt"
	"focusd/storage"
	"focusd/ui"
	"os"
	"strings"
)

func RunInit() {
	if err := storage.Init(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to initialize storage: %v", err))
		os.Exit(1)
	}

	if storage.IsConsentGranted() {
		ui.PrintInfo("focusd is already initialized.")
		fmt.Println()
		fmt.Println("To view your data: focusd stats")
		fmt.Println("To reconfigure:    focusd uninstall && focusd init")
		return
	}

	ui.PrintHeader()

	fmt.Println("Welcome to focusd - your privacy-first digital wellbeing tracker.")
	fmt.Println()
	fmt.Println("This tool tracks:")
	fmt.Println("  • Active application usage time")
	fmt.Println()
	fmt.Println("Privacy guarantees:")
	fmt.Println("  • No keystrokes are recorded")
	fmt.Println("  • No network traffic is monitored")
	fmt.Println("  • No data leaves your computer")
	fmt.Println("  • All data stored locally in %APPDATA%\\focusd")
	fmt.Println("  • Data auto-deleted after 7 days (configurable)")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Do you consent to this tracking? [y/N]: ")
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	if response != "y" && response != "yes" {
		ui.PrintInfo("Consent not granted. focusd will not run without consent.")
		fmt.Println("You can run 'focusd init' again to reconsider.")
		return
	}

	fmt.Println()
	fmt.Print("Enable auto-start on Windows boot? [y/N]: ")
	autoStartResp, _ := reader.ReadString('\n')
	autoStart := strings.TrimSpace(strings.ToLower(autoStartResp)) == "y" || strings.TrimSpace(strings.ToLower(autoStartResp)) == "yes"

	fmt.Println()
	fmt.Print("Add focusd to your PATH? [y/N]: ")
	pathResp, _ := reader.ReadString('\n')
	path := strings.TrimSpace(strings.ToLower(pathResp)) == "y" || strings.TrimSpace(strings.ToLower(pathResp)) == "yes"

	if err := InitLogic(true, autoStart, path); err != nil {
		ui.PrintError(err.Error())
		os.Exit(1)
	}

	fmt.Println()
	ui.PrintInfo("Starting focusd background service...")
	RunStart()
}
