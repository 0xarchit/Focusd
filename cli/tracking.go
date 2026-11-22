package cli

import (
	"fmt"
	"focusd/coreapi"
	"focusd/system"
	"focusd/ui"
	"os"
	"path/filepath"
	"time"
)

func runStart() {
	if err := startDaemonProcess(); err != nil {
		ui.PrintError(err.Error())
		os.Exit(1)
	}
}

func daemonDirectory() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve current executable: %w", err)
	}
	return filepath.Dir(exePath), nil
}

func startDaemonProcess() error {
	if response, err := coreapi.Call(coreapi.Request{Command: "status"}); err == nil && response.OK {
		ui.PrintInfo("focusd is already running.")
		return nil
	}
	dir, err := daemonDirectory()
	if err != nil {
		return err
	}
	count, err := system.CountInstancesInDirChecked(system.DaemonProcessName, dir)
	if err != nil {
		return fmt.Errorf("could not determine daemon state: %w", err)
	}
	if count >= 1 {
		return fmt.Errorf("focusd process exists but authenticated control is unavailable; check its startup status")
	}

	pid, err := system.StartDaemon()
	if err != nil {
		return fmt.Errorf("failed to start background process: %w", err)
	}
	ready := false
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		response, callErr := coreapi.Call(coreapi.Request{Command: "status"})
		if callErr == nil && response.OK {
			ready = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		return fmt.Errorf("background process %d did not publish an authenticated ready endpoint", pid)
	}

	ui.PrintOK("focusd started in background")
	fmt.Printf("Process ID: %d\n", pid)
	fmt.Println()
	fmt.Println("Tracking is now active. You can close this terminal.")
	fmt.Println("Use 'focusd status' to check tracking status.")
	fmt.Println("Use 'focusd stop' to stop tracking.")
	return nil
}

func runStop() {
	if coreapi.SendIPCCmd("stop") {
		ui.PrintOK("focusd stopped gracefully")
		return
	}
	dir, err := daemonDirectory()
	if err != nil {
		ui.PrintError(err.Error())
		os.Exit(1)
	}
	count, err := system.CountInstancesInDirChecked(system.DaemonProcessName, dir)
	if err != nil {
		ui.PrintError(fmt.Sprintf("Could not determine daemon state: %v", err))
		os.Exit(1)
	}
	if count < 1 {
		ui.PrintInfo("focusd is not running.")
		return
	}

	if err := system.KillInstancesInDir(system.DaemonProcessName, dir); err != nil {
		ui.PrintError(fmt.Sprintf("Could not stop focusd completely: %v", err))
		os.Exit(1)
	} else {
		ui.PrintOK("focusd stopped (forced)")
	}
}
