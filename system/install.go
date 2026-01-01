package system

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const DaemonProcessName = "focusd.exe"

func GetInstallDir() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return ""
	}
	return filepath.Join(appData, "focusd")
}

func GetInstalledExePath() string {
	installDir := GetInstallDir()
	if installDir == "" {
		return ""
	}
	return filepath.Join(installDir, "focusd.exe")
}

func GetLauncherPath() string {
	installDir := GetInstallDir()
	if installDir == "" {
		return ""
	}
	return filepath.Join(installDir, "FocusDaemon.vbs")
}

func InstallExes() error {

	src, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable: %w", err)
	}
	src, _ = filepath.Abs(src)

	installDir := GetInstallDir()
	if installDir == "" {
		return fmt.Errorf("APPDATA environment variable not set")
	}
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("failed to create install dir: %w", err)
	}
	dest := filepath.Join(installDir, "focusd.exe")
	dest, _ = filepath.Abs(dest)

	if strings.EqualFold(src, dest) {

		return nil
	}

	if err := installFile(src, dest); err != nil {
		return fmt.Errorf("failed to install focusd.exe: %w", err)
	}

	vbsContent := fmt.Sprintf(`Set WshShell = CreateObject("WScript.Shell")
WshShell.Run """%s"" --daemon", 0, False
`, dest)

	vbsPath := filepath.Join(installDir, "FocusDaemon.vbs")
	if err := os.WriteFile(vbsPath, []byte(vbsContent), 0644); err != nil {
		return fmt.Errorf("failed to create VBS launcher: %w", err)
	}

	return nil
}

func installFile(src, dst string) error {

	if _, err := os.Stat(dst); err == nil {
		oldPath := dst + ".old"

		os.Remove(oldPath)

		if err := os.Rename(dst, oldPath); err != nil {

			return fmt.Errorf("failed to move existing file %s to %s (is it locked?): %w", dst, oldPath, err)
		}
	}
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}
	return nil
}

func CleanupOldBinary() {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	oldPath := exePath + ".old"
	if _, err := os.Stat(oldPath); err == nil {

		_ = os.Remove(oldPath)
	}
}

func IsInstalled() bool {
	exePath := GetInstalledExePath()
	if exePath == "" {
		return false
	}
	_, err := os.Stat(exePath)
	if err != nil {
		return false
	}
	launcherPath := GetLauncherPath()
	if launcherPath == "" {
		return false
	}
	_, err = os.Stat(launcherPath)
	return err == nil
}

func UninstallExe() error {
	exePath := GetInstalledExePath()
	if exePath != "" {
		os.Remove(exePath)
	}
	launcherPath := GetLauncherPath()
	if launcherPath != "" {
		os.Remove(launcherPath)
	}
	return nil
}
