package system

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const DaemonProcessName = "focusd_daemon.exe"

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

func GetInstalledDaemonPath() string {
	installDir := GetInstallDir()
	if installDir == "" {
		return ""
	}
	return filepath.Join(installDir, "focusd_daemon.exe")
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

	if !strings.EqualFold(src, dest) {
		if err := installFile(src, dest); err != nil {
			return fmt.Errorf("failed to install focusd.exe: %w", err)
		}
	}

	// Copy focusd_daemon.exe next to it
	srcDir := filepath.Dir(src)
	daemonSrc := filepath.Join(srcDir, "focusd_daemon.exe")
	daemonDest := filepath.Join(installDir, "focusd_daemon.exe")
	if _, err := os.Stat(daemonSrc); err == nil {
		if err := installFile(daemonSrc, daemonDest); err != nil {
			return fmt.Errorf("failed to install focusd_daemon.exe: %w", err)
		}
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

	_, err = io.Copy(dstFile, srcFile)
	return err
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
	daemonPath := GetInstalledDaemonPath()
	if exePath == "" || daemonPath == "" {
		return false
	}
	_, err1 := os.Stat(exePath)
	_, err2 := os.Stat(daemonPath)
	return err1 == nil && err2 == nil
}
