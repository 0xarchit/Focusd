package system

import (
	"path/filepath"
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
	ExePath string
	AppID   string
}

func GetForegroundWindowInfo() (*WindowInfo, error) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return nil, nil
	}

	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))

	// Modal dialogs, toolbars, and splash screens surface with an empty caption, so
	// resolve the process before treating a blank title as a switch away.
	title := getWindowText(hwnd)

	exeName := ""
	exePath := ""
	if pid != 0 {
		// The full image path is what distinguishes one install from another.
		exePath, _ = getProcessPath(pid)
		if exePath != "" {
			exeName = filepath.Base(exePath)
		} else {
			exeName = getProcessName(pid)
		}
	}

	if title == "" && exeName == "" {
		return nil, nil
	}

	id := ResolveIdentity(exeName, exePath)

	return &WindowInfo{
		Title:   title,
		ExeName: exeName,
		ExePath: exePath,
		AppID:   id.AppID,
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
	// Limited query rights are granted where full query plus VM read are not
	// (elevated and packaged processes), so try the limited handle first and
	// widen only as a fallback.
	handle, _, _ := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if handle == 0 {
		handle, _, _ = procOpenProcess.Call(
			processQueryInformation|processVMRead,
			0,
			uintptr(pid),
		)
	}
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
		// Module enumeration needs VM read; the image path still resolves
		// with only limited query rights.
		if path, err := getProcessPath(pid); err == nil && path != "" {
			return filepath.Base(path)
		}
		return ""
	}
	return syscall.UTF16ToString(buf)
}
