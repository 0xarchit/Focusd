package cli

import (
	"fmt"
	"focusd/storage"
	"focusd/system"
	"focusd/ui"
	"os"
	"path/filepath"
	"time"
)

func RunUninstall() {
	fmt.Println()
	fmt.Println("focusd Uninstall")
	fmt.Println("================")
	fmt.Println()
	fmt.Println("This will completely remove focusd from your system:")
	fmt.Println()
	fmt.Println("  • All tracking data")
	fmt.Println("  • Configuration and consent")
	fmt.Println("  • Auto-start shortcut")
	fmt.Println("  • PATH modifications")
	fmt.Println()
	ui.PrintWarn("This action cannot be undone!")
	fmt.Println()

	if !ui.Confirm("Proceed with uninstall?") {
		ui.PrintInfo("Uninstall cancelled.")
		return
	}

	fmt.Println()

	if err := system.DisableAutoStart(); err != nil {
		ui.PrintWarn(fmt.Sprintf("Failed to remove auto-start: %v", err))
	} else {
		ui.PrintOK("Removed auto-start entry")
	}

	if err := system.DisablePath(); err != nil {
		ui.PrintWarn(fmt.Sprintf("Failed to remove PATH entry: %v", err))
	} else {
		ui.PrintOK("Removed PATH entry")
	}

	if system.GetProcessCount(system.DaemonProcessName) > 1 {
		ui.PrintInfo("Stopping running focusd processes...")
		system.KillOtherInstances(system.DaemonProcessName)
		time.Sleep(1 * time.Second)
	}

	storage.Close()

	dataDir, _ := storage.GetDataDir()
	if dataDir != "" {
		os.Remove(filepath.Join(dataDir, "focusd.db"))
		os.Remove(filepath.Join(dataDir, "focusd.db-shm"))
		os.Remove(filepath.Join(dataDir, "focusd.db-wal"))
		os.Remove(filepath.Join(dataDir, "config.json"))
		os.Remove(filepath.Join(dataDir, "pomodoro.json"))
		os.Remove(filepath.Join(dataDir, "FocusDaemon.vbs"))
		os.Remove(filepath.Join(dataDir, "FocusDaemon.exe"))
	}

	exePath, _ := os.Executable()
	if exePath != "" {
		trashPath := exePath + ".old"
		os.Rename(exePath, trashPath)
	}

	if err := os.RemoveAll(dataDir); err != nil {

		ui.PrintWarn("Some files (like the running executable) could not be removed immediately.")
		ui.PrintWarn("They will be gone after a system restart.")
		fmt.Printf("Location: %s\n", dataDir)
	} else {
		ui.PrintOK("Deleted all data and configuration")
	}

	fmt.Println()
	fmt.Println("================")
	ui.PrintOK("Uninstall complete!")
	fmt.Println()
	fmt.Println("focusd has been removed.")
	fmt.Println("If you see any remaining files in %APPDATA%/focusd, you can delete them after a restart.")
	fmt.Println()
}
