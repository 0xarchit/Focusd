package system

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

const (
	TH32CS_SNAPPROCESS = 0x00000002
	MAX_PATH           = 260
	PROCESS_TERMINATE  = 0x0001
)

type PROCESSENTRY32W struct {
	Size              uint32
	CntUsage          uint32
	ProcessID         uint32
	DefaultHeapID     uintptr
	ModuleID          uint32
	CntThreads        uint32
	ParentProcessID   uint32
	PriorityClassBase int32
	Flags             uint32
	ExeFile           [MAX_PATH]uint16
}

var (
	procCreateToolhelp32 = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW  = kernel32.NewProc("Process32FirstW")
	procProcess32NextW   = kernel32.NewProc("Process32NextW")
	procTerminateProcess = kernel32.NewProc("TerminateProcess")
)

func getProcessSnapshot() (syscall.Handle, error) {
	ret, _, err := procCreateToolhelp32.Call(uintptr(TH32CS_SNAPPROCESS), 0)
	if ret == uintptr(syscall.InvalidHandle) {
		return syscall.InvalidHandle, err
	}
	return syscall.Handle(ret), nil
}

func iterateProcesses(callback func(pe *PROCESSENTRY32W) bool) error {
	snapshot, err := getProcessSnapshot()
	if err != nil {
		return err
	}
	defer procCloseHandle.Call(uintptr(snapshot))

	var pe PROCESSENTRY32W
	pe.Size = uint32(unsafe.Sizeof(pe))

	ret, _, _ := procProcess32FirstW.Call(uintptr(snapshot), uintptr(unsafe.Pointer(&pe)))
	if ret == 0 {
		return nil
	}

	for {
		if !callback(&pe) {
			break
		}
		ret, _, _ = procProcess32NextW.Call(uintptr(snapshot), uintptr(unsafe.Pointer(&pe)))
		if ret == 0 {
			break
		}
	}
	return nil
}

func processName(pe *PROCESSENTRY32W) string {
	return syscall.UTF16ToString(pe.ExeFile[:])
}

func IsProcessRunning(name string) bool {
	found := false
	iterateProcesses(func(pe *PROCESSENTRY32W) bool {
		if strings.EqualFold(processName(pe), name) {
			found = true
			return false
		}
		return true
	})
	return found
}

func GetProcessCount(name string) int {
	count := 0
	iterateProcesses(func(pe *PROCESSENTRY32W) bool {
		if strings.EqualFold(processName(pe), name) {
			count++
		}
		return true
	})
	return count
}

func GetPIDByName(name string) (uint32, error) {
	var pid uint32
	iterateProcesses(func(pe *PROCESSENTRY32W) bool {
		if strings.EqualFold(processName(pe), name) {
			pid = pe.ProcessID
			return false
		}
		return true
	})
	return pid, nil
}

func terminateProcessByPID(pid uint32) error {
	handle, _, _ := procOpenProcess.Call(uintptr(PROCESS_TERMINATE), 0, uintptr(pid))
	if handle == 0 {
		return fmt.Errorf("failed to open process %d", pid)
	}
	defer procCloseHandle.Call(handle)

	ret, _, _ := procTerminateProcess.Call(handle, 1)
	if ret == 0 {
		return fmt.Errorf("failed to terminate process %d", pid)
	}
	return nil
}

func KillProcess(name string) error {
	var lastErr error
	iterateProcesses(func(pe *PROCESSENTRY32W) bool {
		if strings.EqualFold(processName(pe), name) {
			if err := terminateProcessByPID(pe.ProcessID); err != nil {
				lastErr = err
			}
		}
		return true
	})
	return lastErr
}

func KillOtherInstances(name string) error {
	myPID := uint32(os.Getpid())
	var lastErr error
	iterateProcesses(func(pe *PROCESSENTRY32W) bool {
		if strings.EqualFold(processName(pe), name) && pe.ProcessID != myPID {
			if err := terminateProcessByPID(pe.ProcessID); err != nil {
				lastErr = err
			}
		}
		return true
	})
	return lastErr
}

func getMyPID() int {
	return os.Getpid()
}
