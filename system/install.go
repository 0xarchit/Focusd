package system

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const DaemonProcessName = "focusd_daemon.exe"

func getInstallDir() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return ""
	}
	return filepath.Join(appData, "focusd")
}

func getInstalledDaemonPath() string {
	installDir := getInstallDir()
	if installDir == "" {
		return ""
	}
	return filepath.Join(installDir, "focusd_daemon.exe")
}

func installExes() error {
	src, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable: %w", err)
	}
	src, _ = filepath.Abs(src)

	installDir := getInstallDir()
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
	srcDir := filepath.Dir(src)
	daemonSrc := filepath.Join(srcDir, "focusd_daemon.exe")
	daemonDest := filepath.Join(installDir, "focusd_daemon.exe")
	if _, err := os.Stat(daemonSrc); err == nil {
		if !strings.EqualFold(daemonSrc, daemonDest) {
			if err := installFile(daemonSrc, daemonDest); err != nil {
				return fmt.Errorf("failed to install focusd_daemon.exe: %w", err)
			}
		}
	}

	return nil
}

func installFile(src, dst string) error {
	hasBackup := false
	if _, err := os.Stat(dst); err == nil {
		oldPath := dst + ".old"
		os.Remove(oldPath)
		if err := os.Rename(dst, oldPath); err != nil {
			return fmt.Errorf("failed to move existing file %s to %s (is it locked?): %w", dst, oldPath, err)
		}
		hasBackup = true
	}

	srcFile, err := os.Open(src)
	if err != nil {
		if hasBackup {
			os.Rename(dst+".old", dst)
		}
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		if hasBackup {
			os.Rename(dst+".old", dst)
		}
		return err
	}

	_, err = io.Copy(dstFile, srcFile)
	if closeErr := dstFile.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(dst)
		if hasBackup {
			os.Rename(dst+".old", dst)
		}
		return err
	}
	if hasBackup {
		os.Remove(dst + ".old")
	}
	return nil
}
