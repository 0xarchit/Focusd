package system

import (
	"path/filepath"
	"strings"
	"sync"
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
	procEnumChildWindows         = user32.NewProc("EnumChildWindows")
	procGetClassNameW            = user32.NewProc("GetClassNameW")
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
	state, err := ReadSessionState()
	if err != nil || !state.Active() {
		return nil, err
	}
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
	if isSessionScreenExecutable(exeName) {
		return nil, nil
	}
	if strings.EqualFold(exeName, "ApplicationFrameHost.exe") {
		appPID := resolveFrameApplicationPID(hwnd, pid)
		if appPID != pid {
			if path, err := getProcessPath(appPID); err == nil && path != "" {
				exePath, exeName = path, filepath.Base(path)
			}
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

func isSessionScreenExecutable(exeName string) bool {
	return strings.EqualFold(exeName, "LockApp.exe") || strings.EqualFold(exeName, "LogonUI.exe")
}

type frameChild struct {
	PID   uint32
	Class string
}

var (
	frameEnumerationMu       sync.Mutex
	frameEnumerationChildren []frameChild
	frameEnumerationCallback = syscall.NewCallback(collectFrameChild)
)

func collectFrameChild(child, _ uintptr) uintptr {
	var pid uint32
	procGetWindowThreadProcessId.Call(child, uintptr(unsafe.Pointer(&pid)))
	var name [256]uint16
	length, _, _ := procGetClassNameW.Call(child, uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)))
	if length > 0 && length < uintptr(len(name)) {
		frameEnumerationChildren = append(frameEnumerationChildren, frameChild{PID: pid, Class: syscall.UTF16ToString(name[:length])})
	}
	return 1
}

func frameApplicationPID(hostPID uint32, children []frameChild, readable func(uint32) bool) uint32 {
	var appPID uint32
	for _, child := range children {
		if child.PID == 0 || child.PID == hostPID || child.Class != "Windows.UI.Core.CoreWindow" || !readable(child.PID) {
			continue
		}
		if appPID != 0 && appPID != child.PID {
			return hostPID
		}
		appPID = child.PID
	}
	if appPID == 0 {
		return hostPID
	}
	return appPID
}

func resolveFrameApplicationPID(hwnd uintptr, hostPID uint32) uint32 {
	// NewCallback allocations are permanent; reuse one synchronous callback.
	frameEnumerationMu.Lock()
	frameEnumerationChildren = nil
	procEnumChildWindows.Call(hwnd, frameEnumerationCallback, 0)
	children := frameEnumerationChildren
	frameEnumerationChildren = nil
	frameEnumerationMu.Unlock()
	return frameApplicationPID(hostPID, children, func(pid uint32) bool {
		path, err := getProcessPath(pid)
		return err == nil && path != "" && !strings.EqualFold(filepath.Base(path), "ApplicationFrameHost.exe")
	})
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
