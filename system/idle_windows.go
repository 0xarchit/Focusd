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
// keyboard and mouse.
func GetIdleSeconds() (int, error) {
	var info lastInputInfo
	info.CBSize = uint32(unsafe.Sizeof(info))

	ret, _, err := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		return 0, err
	}

	// uint32 subtraction so a GetTickCount wraparound every 49 days cannot go
	// negative. The result is only compared against a threshold, never summed.
	idleMillis := uint32(GetTickCount()) - info.DwTime
	return int(idleMillis / 1000), nil
}

// GetTickCount returns the number of milliseconds since the system started.
func GetTickCount() uint32 {
	ret, _, _ := procGetTickCount.Call()
	return uint32(ret)
}
