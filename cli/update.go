package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"focusd/system"
	"focusd/ui"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

func getVersionURL() string {
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/version",
		system.RepoOwner, system.RepoName)
}

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

	daemonWasRunning := system.GetProcessCount(system.DaemonProcessName) > 1
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
	installedExe := system.GetInstalledExePath()
	if installedExe == "" {
		return
	}

	cmd := exec.Command(installedExe, "--daemon")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x00000008,
	}
	cmd.Start()
}

func fetchLatestVersion() (string, error) {
	resp, err := httpClient.Get(getVersionURL())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(body)), nil
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

	line := strings.TrimSpace(string(body))
	parts := strings.Fields(line)
	if len(parts) >= 1 {
		return strings.ToLower(parts[0]), nil
	}
	return "", fmt.Errorf("invalid checksum format")
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

	downloadURL := fmt.Sprintf("https://github.com/%s/%s/releases/download/v%s/focusd.exe",
		system.RepoOwner, system.RepoName, version)

	tmpFile, err := os.CreateTemp("", "focusd-update-*.exe")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	resp, err := httpClient.Get(downloadURL)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
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
			return fmt.Errorf("failed to calculate hash: %w", err)
		}
		if actualHash != expectedHash {
			return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
		}
		ui.PrintOK("Checksum verified")
	}

	ui.PrintStatus("Installing...", "   ", false)

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to locate current executable: %w", err)
	}

	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return err
	}

	oldPath := exePath + ".old"
	os.Remove(oldPath)

	if err := os.Rename(exePath, oldPath); err != nil {
		return fmt.Errorf("failed to move current executable to .old: %w", err)
	}

	if err := os.Rename(tmpPath, exePath); err != nil {
		if err := copyFile(tmpPath, exePath); err != nil {
			os.Rename(oldPath, exePath)
			return fmt.Errorf("failed to install new binary: %w", err)
		}
	}

	if system.IsInstalled() {
		if err := system.InstallExes(); err != nil {
			ui.PrintWarn(fmt.Sprintf("Failed to sync installed binary: %v", err))
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
