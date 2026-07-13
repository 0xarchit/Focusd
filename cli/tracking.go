package cli

import (
	"fmt"
	"focusd/core"
	"focusd/storage"
	"focusd/system"
	"focusd/ui"
	"log"
	"os"
	"time"
)

func runStart() {
	if err := storage.Init(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to initialize: %v", err))
		os.Exit(1)
	}
	defer storage.Close()

	startDaemonProcess()
}

func startDaemonProcess() {
	if system.GetProcessCount(system.DaemonProcessName) >= 1 {
		ui.PrintInfo("focusd is already running.")
		return
	}

	pid, err := system.StartDaemon()
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to start background process: %v", err))
		return
	}

	ui.PrintOK("focusd started in background")
	fmt.Printf("Process ID: %d\n", pid)
	fmt.Println()
	fmt.Println("Tracking is now active. You can close this terminal.")
	fmt.Println("Use 'focusd status' to check tracking status.")
	fmt.Println("Use 'focusd stop' to stop tracking.")
}

func runStop() {
	if system.GetProcessCount(system.DaemonProcessName) < 1 {
		ui.PrintInfo("focusd is not running.")
		return
	}

	if core.SendIPCCmd("stop") {
		ui.PrintOK("focusd stopped gracefully")
		return
	}

	if err := system.KillProcess(system.DaemonProcessName); err != nil {
		ui.PrintWarn("Could not stop focusd. It may still be running.")
	} else {
		ui.PrintOK("focusd stopped (forced)")
	}
}

func RunDaemon() {
	var dbErr error
	for i := 0; i < 5; i++ {
		dbErr = storage.Init()
		if dbErr == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if dbErr != nil {
		os.Exit(1)
	}
	defer storage.Close()

	storage.EnforceRetention()

	tracker := core.NewTracker()
	if err := tracker.Start(); err != nil {
		log.Printf("ERROR: daemon failed to start tracker: %v", err)
		os.Exit(1)
	}
}
