//go:build windows

package system

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procAttachConsole              = kernel32.NewProc("AttachConsole")
	procSetConsoleScreenBufferSize = kernel32.NewProc("SetConsoleScreenBufferSize")
)

func AttachParentConsole() {
	ret, _, _ := procAttachConsole.Call(uintptr(0xFFFFFFFF))
	if ret != 0 {
		hIn, _ := windows.GetStdHandle(windows.STD_INPUT_HANDLE)
		hOut, _ := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
		hErr, _ := windows.GetStdHandle(windows.STD_ERROR_HANDLE)

		if hIn != 0 && hIn != windows.InvalidHandle {
			os.Stdin = os.NewFile(uintptr(hIn), "/dev/stdin")
		}
		if hOut != 0 && hOut != windows.InvalidHandle {
			os.Stdout = os.NewFile(uintptr(hOut), "/dev/stdout")
		}
		if hErr != 0 && hErr != windows.InvalidHandle {
			os.Stderr = os.NewFile(uintptr(hErr), "/dev/stderr")
		}
	}
}

func DisableConsoleScrollback() (int16, int16) {
	hOut, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err != nil || hOut == 0 || hOut == windows.InvalidHandle {
		return 0, 0
	}

	var info windows.ConsoleScreenBufferInfo
	err = windows.GetConsoleScreenBufferInfo(hOut, &info)
	if err != nil {
		return 0, 0
	}

	windowHeight := info.Window.Bottom - info.Window.Top + 1
	if info.Size.Y > windowHeight {
		newSize := windows.Coord{X: info.Size.X, Y: windowHeight}
		_, _, _ = procSetConsoleScreenBufferSize.Call(uintptr(hOut), uintptr(*(*int32)(unsafe.Pointer(&newSize))))
	}

	return info.Size.X, info.Size.Y
}

func RestoreConsoleBufferSize(width, height int16) {
	if width <= 0 || height <= 0 {
		return
	}
	hOut, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err != nil || hOut == 0 || hOut == windows.InvalidHandle {
		return
	}
	newSize := windows.Coord{X: width, Y: height}
	_, _, _ = procSetConsoleScreenBufferSize.Call(uintptr(hOut), uintptr(*(*int32)(unsafe.Pointer(&newSize))))
}
