package system

import (
	"unsafe"
)

// lastInputInfo mirrors the Win32 LASTINPUTINFO structure.
type lastInputInfo struct {
	CBSize uint32
	DwTime uint32
}

var (
	procGetLastInputInfo = user32.NewProc("GetLastInputInfo")
	procGetTickCount     = kernel32.NewProc("GetTickCount")
)

// GetIdleSeconds returns how many whole seconds the user has been away from the
// keyboard and mouse. A focused window is otherwise counted as active forever,
// so a paused video or an abandoned tab inflates tracked time.
func GetIdleSeconds() (int, error) {
	var info lastInputInfo
	info.CBSize = uint32(unsafe.Sizeof(info))

	ret, _, err := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		return 0, err
	}

	// Both values are millisecond tick counts from the same counter, so the
	// difference is a duration. The subtraction stays in uint32 so a
	// GetTickCount wraparound around every 49 days does not go negative; the
	// result is only ever compared against a threshold, never accumulated.
	idleMillis := uint32(GetTickCount()) - info.DwTime
	return int(idleMillis / 1000), nil
}

// GetTickCount returns the number of milliseconds since the system started.
func GetTickCount() uint32 {
	ret, _, _ := procGetTickCount.Call()
	return uint32(ret)
}