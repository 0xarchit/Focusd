package cli

import (
	"fmt"
	"focusd/core"
	"focusd/storage"
	"focusd/system"
	"focusd/ui"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func RunBackgroundTracking() {
	if err := storage.Init(); err != nil {
		os.Exit(1)
	}
	defer storage.Close()

	if !storage.IsConsentGranted() {
		ui.PrintError("focusd is not initialized. Run 'focusd init' first.")
		os.Exit(1)
	}

	if err := storage.EnforceRetention(); err != nil {
	}

	tracker := core.NewTracker()
	tracker.Start()
}

func RunStart() {
	if err := storage.Init(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to initialize: %v", err))
		os.Exit(1)
	}
	defer storage.Close()

	StartDaemonProcess()
}

func StartDaemonProcess() {
	if !storage.IsConsentGranted() {
		ui.PrintError("focusd is not initialized. Run 'focusd init' first.")
		return
	}

	if system.GetProcessCount(system.DaemonProcessName) > 1 {
		ui.PrintInfo("focusd is already running.")
		return
	}

	exePath, err := os.Executable()
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to get executable path: %v", err))
		return
	}

	cmd := exec.Command(exePath, "--daemon")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x00000008,
	}

	if err := cmd.Start(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to start background process: %v", err))
		return
	}

	ui.PrintOK("focusd started in background")
	fmt.Printf("Process ID: %d\n", cmd.Process.Pid)
	fmt.Println()
	fmt.Println("Tracking is now active. You can close this terminal.")
	fmt.Println("Use 'focusd status' to check tracking status.")
	fmt.Println("Use 'focusd stop' to stop tracking.")
}

func RunStop() {
	if system.GetProcessCount(system.DaemonProcessName) <= 1 {
		ui.PrintInfo("focusd is not running.")
		return
	}

	if err := system.KillProcess(system.DaemonProcessName); err != nil {
		ui.PrintWarn("Could not stop focusd. It may still be running.")
	} else {
		ui.PrintOK("focusd stopped")
	}
}

func RunDaemon() {
	time.Sleep(3 * time.Second)

	var err error
	for i := 0; i < 5; i++ {
		err = storage.Init()
		if err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		os.Exit(1)
	}
	defer storage.Close()

	for i := 0; i < 10; i++ {
		if storage.IsConsentGranted() {
			break
		}
		time.Sleep(1 * time.Second)
		if i == 9 {
			os.Exit(1)
		}
	}

	storage.EnforceRetention()

	tracker := core.NewTracker()
	tracker.Start()
}
