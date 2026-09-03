package system

import (
	"syscall"
	"unsafe"
)

var (
	user32                       = syscall.NewLazyDLL("user32.dll")
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	psapi                        = syscall.NewLazyDLL("psapi.dll")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procGetWindowTextW           = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW     = user32.NewProc("GetWindowTextLengthW")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procOpenProcess              = kernel32.NewProc("OpenProcess")
	procCloseHandle              = kernel32.NewProc("CloseHandle")
	procGetModuleBaseNameW       = psapi.NewProc("GetModuleBaseNameW")
)

const (
	processQueryInformation = 0x0400
	processVMRead           = 0x0010
)

type WindowInfo struct {
	Title   string
	ExeName string
}

func GetForegroundWindowInfo() (*WindowInfo, error) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return nil, nil
	}

	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))

	// A blank title is common: modal dialogs, toolbars, context menus,
	// splash screens, and some games all surface with an empty caption.
	// Resolving the process first lets the tracker keep the session alive
	// instead of treating the dip as a switch away.
	title := getWindowText(hwnd)

	exeName := ""
	if pid != 0 {
		exeName = getProcessName(pid)
	}

	if title == "" && exeName == "" {
		return nil, nil
	}

	return &WindowInfo{
		Title:   title,
		ExeName: exeName,
	}, nil
}

func getWindowText(hwnd uintptr) string {
	length, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if length == 0 {
		return ""
	}

	buf := make([]uint16, length+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), length+1)
	return syscall.UTF16ToString(buf)
}

func getProcessName(pid uint32) string {
	handle, _, _ := procOpenProcess.Call(
		processQueryInformation|processVMRead,
		0,
		uintptr(pid),
	)
	if handle == 0 {
		return ""
	}
	defer procCloseHandle.Call(handle)

	buf := make([]uint16, 260)
	ret, _, _ := procGetModuleBaseNameW.Call(
		handle,
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		260,
	)
	if ret == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}
