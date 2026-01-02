package system

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

const (
	TH32CS_SNAPPROCESS = 0x00000002
	MAX_PATH           = 260
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

func KillProcess(name string) error {
	cmd := exec.Command("taskkill", "/F", "/IM", name)
	return cmd.Run()
}

func KillOtherInstances(name string) error {
	myPID := uint32(os.Getpid())
	iterateProcesses(func(pe *PROCESSENTRY32W) bool {
		if strings.EqualFold(processName(pe), name) && pe.ProcessID != myPID {
			exec.Command("taskkill", "/F", "/PID", strconv.Itoa(int(pe.ProcessID))).Run()
		}
		return true
	})
	return nil
}

func getMyPID() int {
	return os.Getpid()
}
