//go:build windows

package system

import (
	"os"
	"syscall"
)

var (
	modkernel32       = syscall.NewLazyDLL("kernel32.dll")
	procAttachConsole = modkernel32.NewProc("AttachConsole")
)

// AttachParentConsole attaches the process to the parent console (CMD/PowerShell)
// and re-routes standard file descriptors.
func AttachParentConsole() {
	ret, _, _ := procAttachConsole.Call(uintptr(0xFFFFFFFF)) // ATTACH_PARENT_PROCESS
	if ret != 0 {
		if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
			os.Stdout = f
		}
		if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
			os.Stderr = f
		}
		if f, err := os.OpenFile("CONIN$", os.O_RDWR, 0); err == nil {
			os.Stdin = f
		}
	}
}
