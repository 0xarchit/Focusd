package system

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	desktopReadObjects = 0x0001
	userObjectName     = 2
	wtsConnectState    = 8
)

var (
	procOpenInputDesktop           = user32.NewProc("OpenInputDesktop")
	procCloseDesktop               = user32.NewProc("CloseDesktop")
	procGetThreadDesktop           = user32.NewProc("GetThreadDesktop")
	procGetUserObjectInformationW  = user32.NewProc("GetUserObjectInformationW")
	procWTSQuerySessionInformation = syscall.NewLazyDLL("wtsapi32.dll").NewProc("WTSQuerySessionInformationW")
)

// SessionState describes whether this process's interactive desktop is receiving input.
type SessionState struct {
	Locked    bool
	Connected bool
	Desktop   string
}

func (s SessionState) Active() bool { return s.Connected && !s.Locked }

func sessionStateFor(connected bool, input, own string, err error) (SessionState, error) {
	s := SessionState{Connected: connected, Desktop: input, Locked: true}
	if err == nil {
		s.Locked = input == "" || own == "" || !strings.EqualFold(input, own) || strings.EqualFold(input, "Winlogon")
	}
	return s, err
}

// ReadSessionState reads connection/desktop state without switching or locking the workstation.
func ReadSessionState() (SessionState, error) {
	connected, err := sessionConnected()
	if err != nil || !connected {
		return sessionStateFor(connected, "", "", err)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ownHandle, _, callErr := procGetThreadDesktop.Call(uintptr(windows.GetCurrentThreadId()))
	if ownHandle == 0 {
		return sessionStateFor(connected, "", "", fmt.Errorf("read process desktop: %w", callErr))
	}
	own, err := desktopName(ownHandle)
	if err != nil {
		return sessionStateFor(connected, "", "", err)
	}
	inputHandle, _, callErr := procOpenInputDesktop.Call(0, 0, desktopReadObjects)
	if inputHandle == 0 {
		return sessionStateFor(connected, "", own, fmt.Errorf("open input desktop: %w", callErr))
	}
	defer procCloseDesktop.Call(inputHandle)
	input, err := desktopName(inputHandle)
	return sessionStateFor(connected, input, own, err)
}

func desktopName(handle uintptr) (string, error) {
	var name [256]uint16
	var needed uint32
	result, _, err := procGetUserObjectInformationW.Call(handle, userObjectName, uintptr(unsafe.Pointer(&name[0])), uintptr(unsafe.Sizeof(name)), uintptr(unsafe.Pointer(&needed)))
	if result == 0 {
		return "", fmt.Errorf("read desktop name: %w", err)
	}
	if needed == 0 || needed > uint32(unsafe.Sizeof(name)) || needed%2 != 0 {
		return "", fmt.Errorf("invalid desktop name length %d", needed)
	}
	return syscall.UTF16ToString(name[:needed/2]), nil
}

func sessionConnected() (bool, error) {
	var sessionID uint32
	if err := windows.ProcessIdToSessionId(uint32(os.Getpid()), &sessionID); err != nil {
		return false, fmt.Errorf("read session id: %w", err)
	}
	var buffer unsafe.Pointer
	var size uint32
	result, _, err := procWTSQuerySessionInformation.Call(0, uintptr(sessionID), wtsConnectState, uintptr(unsafe.Pointer(&buffer)), uintptr(unsafe.Pointer(&size)))
	if result == 0 {
		return false, fmt.Errorf("read session connection: %w", err)
	}
	if buffer == nil || size < 4 {
		if buffer != nil {
			windows.WTSFreeMemory(uintptr(buffer))
		}
		return false, fmt.Errorf("invalid session state length %d", size)
	}
	defer windows.WTSFreeMemory(uintptr(buffer))
	return *(*uint32)(buffer) == 0, nil
}
