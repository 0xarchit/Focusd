package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"focusd/system"
	"focusd/ui"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

func runUpdate() {
	ui.PrintHeader()
	fmt.Printf("Current Version: %s\n", system.Version)
	fmt.Println("Checking for updates...")

	latestVer, err := fetchLatestVersion()
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to check for updates: %v", err))
		return
	}

	if latestVer == system.Version {
		ui.PrintOK("You are on the latest version.")
		return
	}

	ui.PrintInfo(fmt.Sprintf("New version available: %s", latestVer))
	fmt.Print("Do you want to update? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to read input: %v", err))
		return
	}
	if strings.ToLower(strings.TrimSpace(response)) != "y" {
		fmt.Println("Update cancelled.")
		return
	}

	daemonWasRunning := system.GetProcessCount(system.DaemonProcessName) >= 1
	if daemonWasRunning {
		ui.PrintStatus("Stopping focusd daemon...", "", false)
		if err := system.KillOtherInstances(system.DaemonProcessName); err != nil {
			ui.PrintWarn(fmt.Sprintf("Could not stop daemon: %v", err))
		}
		time.Sleep(500 * time.Millisecond)
	}

	ui.PrintStatus("Closing other focusd instances...", "", false)
	if err := system.KillOtherInstances("focusd.exe"); err != nil {
		log.Printf("WARN: could not close other focusd instances: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	if err := performUpdate(latestVer); err != nil {
		ui.PrintError(fmt.Sprintf("Update failed: %v", err))
		if daemonWasRunning {
			ui.PrintInfo("Attempting to restart daemon...")
			if _, err := system.StartDaemon(); err != nil {
				log.Printf("WARN: failed to restart daemon after update: %v", err)
			}
		}
		os.Exit(1)
	}

	os.Exit(0)
}

func fetchLatestVersion() (string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", system.RepoOwner, system.RepoName), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "focusd-updater")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	return strings.TrimPrefix(release.TagName, "v"), nil
}

func performUpdate(version string) (err error) {
	ui.PrintStatus("Downloading update...", "0%", false)

	downloadURL := fmt.Sprintf("https://github.com/%s/%s/releases/download/v%s/focusd_setup.exe",
		system.RepoOwner, system.RepoName, version)

	tmpFile, err := os.CreateTemp("", "focusd-setup-*.exe")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	defer func() {
		tmpFile.Close()
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	resp, err := httpClient.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	if _, err = io.Copy(tmpFile, resp.Body); err != nil {
		return fmt.Errorf("write failed: %w", err)
	}
	tmpFile.Close()

	ui.PrintStatus("Installing...", "", false)

	verbPtr, err := syscall.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	pathPtr, err := syscall.UTF16PtrFromString(tmpPath)
	if err != nil {
		return err
	}
	argsPtr, err := syscall.UTF16PtrFromString("/S")
	if err != nil {
		return err
	}

	err = windows.ShellExecute(0, verbPtr, pathPtr, argsPtr, nil, windows.SW_HIDE)
	if err != nil {
		return fmt.Errorf("failed to start installer with elevation: %w", err)
	}

	ui.PrintOK("Installer started silently. focusd will now close to complete the update.")
	return nil
}
