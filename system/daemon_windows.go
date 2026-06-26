//go:build windows

package system

import (
	"os"
	"os/exec"
	"syscall"
)

func StartDaemon() (uint32, error) {
	exePath, err := os.Executable()
	if err != nil {
		return 0, err
	}

	cmd := exec.Command(exePath, "--daemon")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x00000008,
	}

	if err := cmd.Start(); err != nil {
		return 0, err
	}

	return uint32(cmd.Process.Pid), nil
}
