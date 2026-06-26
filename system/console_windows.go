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
		// Re-initialize Go's internal standard handles using the attached console's handles
		const (
			stdInputHandle  = 0xFFFFFFF6 // -10
			stdOutputHandle = 0xFFFFFFF5 // -11
			stdErrorHandle  = 0xFFFFFFF4 // -12
		)
		modkernel32 := syscall.NewLazyDLL("kernel32.dll")
		procGetStdHandle := modkernel32.NewProc("GetStdHandle")

		hIn, _, _ := procGetStdHandle.Call(uintptr(stdInputHandle))
		hOut, _, _ := procGetStdHandle.Call(uintptr(stdOutputHandle))
		hErr, _, _ := procGetStdHandle.Call(uintptr(stdErrorHandle))

		if hIn != 0 && hIn != ^uintptr(0) {
			os.Stdin = os.NewFile(hIn, "/dev/stdin")
		}
		if hOut != 0 && hOut != ^uintptr(0) {
			os.Stdout = os.NewFile(hOut, "/dev/stdout")
		}
		if hErr != 0 && hErr != ^uintptr(0) {
			os.Stderr = os.NewFile(hErr, "/dev/stderr")
		}
	}
}
