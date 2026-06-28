package cli

import (
	"crypto/sha256"
	"encoding/hex"
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

func RunUpdate() {
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

	var response string
	fmt.Scanln(&response)
	if strings.ToLower(strings.TrimSpace(response)) != "y" {
		fmt.Println("Update cancelled.")
		return
	}

	daemonWasRunning := system.GetProcessCount(system.DaemonProcessName) >= 1
	if daemonWasRunning {
		ui.PrintStatus("Stopping focusd daemon...", "", false)
		system.KillOtherInstances(system.DaemonProcessName)
		time.Sleep(500 * time.Millisecond)
	}

	if err := performUpdate(latestVer); err != nil {
		ui.PrintError(fmt.Sprintf("Update failed: %v", err))
		if daemonWasRunning {
			ui.PrintInfo("Attempting to restart daemon...")
			restartDaemon()
		}
		return
	}

	ui.PrintOK(fmt.Sprintf("Successfully updated to v%s!", latestVer))

	if daemonWasRunning {
		ui.PrintStatus("Restarting focusd daemon...", "", false)
		time.Sleep(500 * time.Millisecond)
		restartDaemon()
		ui.PrintOK("Daemon restarted with new version.")
	}

	os.Exit(0)
}

func restartDaemon() {
	if _, err := system.StartDaemon(); err != nil {
		log.Printf("WARN: failed to restart daemon after update: %v", err)
	}
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

func fetchChecksum(version string) (string, error) {
	checksumURL := fmt.Sprintf("https://github.com/%s/%s/releases/download/v%s/checksums.txt",
		system.RepoOwner, system.RepoName, version)

	resp, err := httpClient.Get(checksumURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checksums not available (status %d)", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	for _, line := range lines {
		parts := strings.Fields(strings.TrimSpace(line))
		if len(parts) >= 2 {
			filename := strings.TrimPrefix(strings.TrimPrefix(parts[1], "*"), "./")
			if strings.EqualFold(filename, "focusd_setup.exe") {
				return strings.ToLower(parts[0]), nil
			}
		}
	}
	return "", fmt.Errorf("focusd_setup.exe checksum not found in checksums.txt")
}

func calculateFileHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func performUpdate(version string) error {
	ui.PrintStatus("Downloading update...", "0%", false)

	downloadURL := fmt.Sprintf("https://github.com/%s/%s/releases/download/v%s/focusd_setup.exe",
		system.RepoOwner, system.RepoName, version)

	tmpFile, err := os.CreateTemp("", "focusd-setup-*.exe")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	resp, err := httpClient.Get(downloadURL)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write failed: %w", err)
	}
	tmpFile.Close()

	ui.PrintStatus("Verifying checksum...", "", false)
	expectedHash, err := fetchChecksum(version)
	if err != nil {
		ui.PrintWarn(fmt.Sprintf("Checksum verification skipped: %v", err))
	} else {
		actualHash, err := calculateFileHash(tmpPath)
		if err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("failed to calculate hash: %w", err)
		}
		if actualHash != expectedHash {
			os.Remove(tmpPath)
			return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
		}
		ui.PrintOK("Checksum verified")
	}

	ui.PrintStatus("Installing...", "", false)

	verbPtr, err := syscall.UTF16PtrFromString("runas")
	if err != nil {
		os.Remove(tmpPath)
		return err
	}
	pathPtr, err := syscall.UTF16PtrFromString(tmpPath)
	if err != nil {
		os.Remove(tmpPath)
		return err
	}
	argsPtr, err := syscall.UTF16PtrFromString("/S")
	if err != nil {
		os.Remove(tmpPath)
		return err
	}

	err = windows.ShellExecute(0, verbPtr, pathPtr, argsPtr, nil, windows.SW_HIDE)
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to start installer with elevation: %w", err)
	}

	ui.PrintOK("Installer started silently. focusd will now close to complete the update.")
	return nil
}
