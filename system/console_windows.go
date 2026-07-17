//go:build windows

package system

import (
	"os"
	"unsafe"
)

var (
	procAttachConsole              = kernel32.NewProc("AttachConsole")
	procGetConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")
	procSetConsoleScreenBufferSize = kernel32.NewProc("SetConsoleScreenBufferSize")
)

type coord struct {
	X int16
	Y int16
}

type consoleScreenBufferInfo struct {
	dwSize              coord
	dwCursorPosition    coord
	wAttributes         uint16
	srWindow            struct{ Left, Top, Right, Bottom int16 }
	dwMaximumWindowSize coord
}

func AttachParentConsole() {
	ret, _, _ := procAttachConsole.Call(uintptr(0xFFFFFFFF))
	if ret != 0 {
		const (
			stdInputHandle  = 0xFFFFFFF6
			stdOutputHandle = 0xFFFFFFF5
			stdErrorHandle  = 0xFFFFFFF4
		)
		procGetStdHandle := kernel32.NewProc("GetStdHandle")

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

func DisableConsoleScrollback() (int16, int16) {
	const stdOutputHandle = 0xFFFFFFF5
	procGetStdHandle := kernel32.NewProc("GetStdHandle")
	hOut, _, _ := procGetStdHandle.Call(uintptr(stdOutputHandle))
	if hOut == 0 || hOut == ^uintptr(0) {
		return 0, 0
	}

	var info consoleScreenBufferInfo
	ret, _, _ := procGetConsoleScreenBufferInfo.Call(hOut, uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		return 0, 0
	}

	windowHeight := info.srWindow.Bottom - info.srWindow.Top + 1
	if info.dwSize.Y > windowHeight {
		newSize := coord{X: info.dwSize.X, Y: windowHeight}
		_, _, _ = procSetConsoleScreenBufferSize.Call(hOut, uintptr(*(*int32)(unsafe.Pointer(&newSize))))
	}

	return info.dwSize.X, info.dwSize.Y
}

func RestoreConsoleBufferSize(width, height int16) {
	if width <= 0 || height <= 0 {
		return
	}
	const stdOutputHandle = 0xFFFFFFF5
	procGetStdHandle := kernel32.NewProc("GetStdHandle")
	hOut, _, _ := procGetStdHandle.Call(uintptr(stdOutputHandle))
	if hOut == 0 || hOut == ^uintptr(0) {
		return
	}
	newSize := coord{X: width, Y: height}
	_, _, _ = procSetConsoleScreenBufferSize.Call(hOut, uintptr(*(*int32)(unsafe.Pointer(&newSize))))
}
