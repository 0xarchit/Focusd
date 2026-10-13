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

// GetInstallDir returns the per-user install directory. Callers that terminate
// processes scope by it so they never touch an unrelated binary elsewhere.
func GetInstallDir() string {
	return getInstallDir()
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
	if err := os.MkdirAll(installDir, 0700); err != nil {
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

// installFile replaces dst with src. The copy is staged in the destination
// directory so the final rename is atomic and dst is never left truncated.
func installFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	info, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dir := filepath.Dir(dst)
	// The temporary file must share a directory with the destination so the
	// rename stays on one volume and is therefore atomic.
	tmp, err := os.CreateTemp(dir, filepath.Base(dst)+".new-*")
	if err != nil {
		return fmt.Errorf("failed to stage %s: %w", dst, err)
	}
	tmpPath := tmp.Name()

	cleanup := func() {
		tmp.Close()
		os.Remove(tmpPath)
	}

	if _, err := io.Copy(tmp, srcFile); err != nil {
		cleanup()
		return fmt.Errorf("failed to copy %s: %w", src, err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("failed to flush %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close %s: %w", tmpPath, err)
	}
	if err := os.Chmod(tmpPath, info.Mode().Perm()); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to set permissions on %s: %w", tmpPath, err)
	}

	// Windows refuses to rename onto a running executable. If the destination is
	// in use, move the old file aside first and report clearly rather than
	// leaving a half-installed state.
	if err := os.Rename(tmpPath, dst); err == nil {
		return nil
	} else if _, statErr := os.Stat(dst); statErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to install %s: %w", dst, err)
	}

	oldPath := dst + ".old"
	os.Remove(oldPath)
	if err := os.Rename(dst, oldPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to replace %s, it may be running: %w", dst, err)
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		// Put the previous binary back rather than leaving nothing installed.
		os.Rename(oldPath, dst)
		os.Remove(tmpPath)
		return fmt.Errorf("failed to install %s: %w", dst, err)
	}
	os.Remove(oldPath)
	return nil
}
