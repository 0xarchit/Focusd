//go:build windows

package system

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

const createNoWindow = 0x08000000

func StartDaemon() (uint32, error) {
	exePath, err := os.Executable()
	if err != nil {
		return 0, err
	}
	dir := filepath.Dir(exePath)
	daemonPath := filepath.Join(dir, "focusd_daemon.exe")

	cmd := exec.Command(daemonPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | createNoWindow,
	}

	if err := cmd.Start(); err != nil {
		return 0, err
	}

	return uint32(cmd.Process.Pid), nil
}
