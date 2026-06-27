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

func AttachParentConsole() {
	ret, _, _ := procAttachConsole.Call(uintptr(0xFFFFFFFF))
	if ret != 0 {
		const (
			stdInputHandle  = 0xFFFFFFF6
			stdOutputHandle = 0xFFFFFFF5
			stdErrorHandle  = 0xFFFFFFF4
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
