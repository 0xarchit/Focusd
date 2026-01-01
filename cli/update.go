package cli

import (
	"fmt"
	"focusd/system"
	"focusd/ui"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	versionURL = "https://raw.githubusercontent.com/0xarchit/Focusd/main/version"
)

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

	if err := performUpdate(latestVer); err != nil {
		ui.PrintError(fmt.Sprintf("Update failed: %v", err))
		return
	}

	ui.PrintOK(fmt.Sprintf("Successfully updated to v%s!", latestVer))
	ui.PrintWarn("Please restart focusd to use the new version.")
	os.Exit(0)
}

func fetchLatestVersion() (string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		fmt.Sprintf("(Invoke-WebRequest -Uri '%s' -UseBasicParsing).Content", versionURL))

	out, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	cmd = exec.Command("curl", "-sL", versionURL)
	out, err = cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}

	return "", fmt.Errorf("failed to fetch version: %v", err)
}

func performUpdate(version string) error {
	ui.PrintStatus("Downloading update...", "0%", false)

	downloadURL := fmt.Sprintf("https://github.com/%s/%s/releases/download/v%s/focusd.exe",
		system.RepoOwner, system.RepoName, version)

	tmpFile, err := os.CreateTemp("", "focusd-update-*.exe")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpFile.Close()
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	downloadCmd := fmt.Sprintf("Invoke-WebRequest -Uri '%s' -OutFile '%s'", downloadURL, tmpPath)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", downloadCmd)

	if err := cmd.Run(); err != nil {
		cmd = exec.Command("curl", "-L", "-o", tmpPath, downloadURL)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("download failed: %w", err)
		}
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
