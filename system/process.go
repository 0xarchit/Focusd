package system

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

const (
	th32cs_snapprocess = 0x00000002
	maxPath            = 260
	processTerminate   = 0x0001
)

type processentry32w struct {
	Size              uint32
	CntUsage          uint32
	ProcessID         uint32
	DefaultHeapID     uintptr
	ModuleID          uint32
	CntThreads        uint32
	ParentProcessID   uint32
	PriorityClassBase int32
	Flags             uint32
	ExeFile           [maxPath]uint16
}

var (
	procCreateToolhelp32    = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW     = kernel32.NewProc("Process32FirstW")
	procProcess32NextW      = kernel32.NewProc("Process32NextW")
	procTerminateProcess    = kernel32.NewProc("TerminateProcess")
	procQueryFullProcessImg = kernel32.NewProc("QueryFullProcessImageNameW")
)

const (
	// processQueryLimitedInformation needs fewer rights than
	// PROCESS_QUERY_INFORMATION | PROCESS_VM_READ, so it resolves for more
	// processes than reading the module base name does.
	processQueryLimitedInformation = 0x1000
)

// getProcessPath returns the full image path for a pid. It returns an empty
// string when the process cannot be opened, which happens for elevated or
// protected processes even at the reduced right level.
func getProcessPath(pid uint32) (string, error) {
	handle, _, err := procOpenProcess.Call(
		processQueryLimitedInformation,
		0,
		uintptr(pid),
	)
	if handle == 0 {
		return "", err
	}
	defer procCloseHandle.Call(handle)

	// Windows caps this at 32767 characters including the terminator.
	buf := make([]uint16, 32768)
	size := uint32(len(buf))
	ret, _, err := procQueryFullProcessImg.Call(
		handle,
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ret == 0 {
		return "", err
	}
	return syscall.UTF16ToString(buf[:size]), nil
}

// sameDir reports whether an executable path sits in dir. Kept separate from
// the termination loop so the matching rule can be tested without spawning or
// killing processes.
func sameDir(exePath, dir string) bool {
	if exePath == "" {
		return false
	}
	return normalizeDir(filepath.Dir(exePath)) == normalizeDir(dir)
}

func getProcessSnapshot() (syscall.Handle, error) {
	ret, _, err := procCreateToolhelp32.Call(uintptr(th32cs_snapprocess), 0)
	if ret == uintptr(syscall.InvalidHandle) {
		return syscall.InvalidHandle, err
	}
	return syscall.Handle(ret), nil
}

func iterateProcesses(callback func(pe *processentry32w) bool) error {
	snapshot, err := getProcessSnapshot()
	if err != nil {
		return err
	}
	defer procCloseHandle.Call(uintptr(snapshot))

	var pe processentry32w
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

func processName(pe *processentry32w) string {
	return syscall.UTF16ToString(pe.ExeFile[:])
}

func GetProcessCount(name string) int {
	count := 0
	iterateProcesses(func(pe *processentry32w) bool {
		if strings.EqualFold(processName(pe), name) {
			count++
		}
		return true
	})
	return count
}

func terminateProcessByPID(pid uint32) error {
	handle, _, _ := procOpenProcess.Call(uintptr(processTerminate), 0, uintptr(pid))
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
	iterateProcesses(func(pe *processentry32w) bool {
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
	iterateProcesses(func(pe *processentry32w) bool {
		if strings.EqualFold(processName(pe), name) && pe.ProcessID != myPID {
			if err := terminateProcessByPID(pe.ProcessID); err != nil {
				lastErr = err
			}
		}
		return true
	})
	return lastErr
}

// KillInstancesInDir terminates other instances of name whose executable lives
// in dir. Matching on the name alone also matches an unrelated focusd.exe run
// from somewhere else on disk, so callers that act destructively should scope by
// directory. dir is compared case-insensitively and trailing separators are
// ignored.
func KillInstancesInDir(name, dir string) error {
	target := normalizeDir(dir)
	if target == "" {
		return fmt.Errorf("target directory not available")
	}

	myPID := uint32(os.Getpid())
	var lastErr error
	iterateProcesses(func(pe *processentry32w) bool {
		if pe.ProcessID == myPID || !strings.EqualFold(processName(pe), name) {
			return true
		}

		exePath, err := getProcessPath(pe.ProcessID)
		if err != nil || exePath == "" {
			// Cannot confirm the location, so leave it alone rather than
			// terminate a process that may not be ours.
			return true
		}
		if !sameDir(exePath, target) {
			return true
		}

		if err := terminateProcessByPID(pe.ProcessID); err != nil {
			lastErr = err
		}
		return true
	})
	return lastErr
}

// normalizeDir puts a directory into a comparable form: separators unified,
// trailing separators dropped, case lowered.
func normalizeDir(dir string) string {
	trimmed := strings.TrimSpace(dir)
	trimmed = strings.ReplaceAll(trimmed, "/", `\`)
	trimmed = strings.TrimRight(trimmed, `\`)
	return strings.ToLower(trimmed)
}

// CountInstancesInDir reports how many processes with this name run from dir.
func CountInstancesInDir(name, dir string) int {
	target := normalizeDir(dir)
	if target == "" {
		return 0
	}

	count := 0
	iterateProcesses(func(pe *processentry32w) bool {
		if !strings.EqualFold(processName(pe), name) {
			return true
		}
		exePath, err := getProcessPath(pe.ProcessID)
		if err != nil || exePath == "" {
			return true
		}
		if sameDir(exePath, target) {
			count++
		}
		return true
	})
	return count
}
