package system

import (
	"errors"
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
	enumerateProcesses      = iterateProcesses
	lookupProcessPath       = getProcessPath
	openKillProcess         = openProcessForKill
	queryKillProcessPath    = queryProcessPath
	terminateKillProcess    = terminateHandle
	closeKillProcess        = func(handle uintptr) { procCloseHandle.Call(handle) }
)

const (
	// Fewer rights than PROCESS_QUERY_INFORMATION | PROCESS_VM_READ, so it resolves
	// for more processes.
	processQueryLimitedInformation = 0x1000
)

// getProcessPath returns the full image path for a pid, or an empty string for
// elevated and protected processes that cannot be opened.
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

	return queryProcessPath(handle)
}

// queryProcessPath resolves the image path of an already-open process handle.
func queryProcessPath(handle uintptr) (string, error) {
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

// openProcessForKill opens one handle with both query and terminate rights, so
// a path can be verified and the process terminated through the same handle.
// Reopening by pid between the two steps races pid reuse and can terminate an
// unrelated process.
func openProcessForKill(pid uint32) (uintptr, error) {
	handle, _, err := procOpenProcess.Call(
		uintptr(processQueryLimitedInformation|processTerminate),
		0,
		uintptr(pid),
	)
	if handle == 0 {
		return 0, err
	}
	return handle, nil
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

	ret, _, callErr := procProcess32FirstW.Call(uintptr(snapshot), uintptr(unsafe.Pointer(&pe)))
	if ret == 0 {
		if errors.Is(callErr, syscall.ERROR_NO_MORE_FILES) {
			return nil
		}
		return fmt.Errorf("enumerate first process: %w", callErr)
	}

	for {
		if !callback(&pe) {
			break
		}
		ret, _, callErr = procProcess32NextW.Call(uintptr(snapshot), uintptr(unsafe.Pointer(&pe)))
		if ret == 0 {
			if errors.Is(callErr, syscall.ERROR_NO_MORE_FILES) {
				return nil
			}
			return fmt.Errorf("enumerate next process: %w", callErr)
		}
	}
	return nil
}

func processName(pe *processentry32w) string {
	return syscall.UTF16ToString(pe.ExeFile[:])
}

func GetProcessCount(name string) int {
	count, _ := GetProcessCountChecked(name)
	return count
}

// GetProcessCountChecked distinguishes a failed snapshot from an empty one.
func GetProcessCountChecked(name string) (int, error) {
	count := 0
	err := enumerateProcesses(func(pe *processentry32w) bool {
		if strings.EqualFold(processName(pe), name) {
			count++
		}
		return true
	})
	return count, err
}

func terminateProcessByPID(pid uint32) error {
	handle, _, _ := procOpenProcess.Call(uintptr(processTerminate), 0, uintptr(pid))
	if handle == 0 {
		return fmt.Errorf("failed to open process %d", pid)
	}
	defer procCloseHandle.Call(handle)

	return terminateHandle(handle, pid)
}

// terminateHandle terminates an already-open process handle. The caller owns
// the handle.
func terminateHandle(handle uintptr, pid uint32) error {
	ret, _, _ := procTerminateProcess.Call(handle, 1)
	if ret == 0 {
		return fmt.Errorf("failed to terminate process %d", pid)
	}
	return nil
}

func KillProcess(name string) error {
	var failures []error
	err := enumerateProcesses(func(pe *processentry32w) bool {
		if strings.EqualFold(processName(pe), name) {
			if err := terminateProcessByPID(pe.ProcessID); err != nil {
				failures = append(failures, err)
			}
		}
		return true
	})
	return errors.Join(append(failures, err)...)
}

func KillOtherInstances(name string) error {
	myPID := uint32(os.Getpid())
	var failures []error
	err := enumerateProcesses(func(pe *processentry32w) bool {
		if strings.EqualFold(processName(pe), name) && pe.ProcessID != myPID {
			if err := terminateProcessByPID(pe.ProcessID); err != nil {
				failures = append(failures, err)
			}
		}
		return true
	})
	return errors.Join(append(failures, err)...)
}

// KillInstancesInDir terminates instances of name running from dir. Matching on
// the name alone would also terminate an unrelated copy elsewhere on disk.
// Processes whose path cannot be read are left alone.
func KillInstancesInDir(name, dir string) error {
	target := normalizeDir(dir)
	if target == "" {
		return fmt.Errorf("target directory not available")
	}

	myPID := uint32(os.Getpid())
	var failures []error
	err := enumerateProcesses(func(pe *processentry32w) bool {
		if pe.ProcessID == myPID || !strings.EqualFold(processName(pe), name) {
			return true
		}

		// Verify and terminate through a single handle: reopening the pid
		// between the two calls races pid reuse, and the kill could then land
		// on an unrelated process.
		handle, oerr := openKillProcess(pe.ProcessID)
		if oerr != nil {
			failures = append(failures, fmt.Errorf("open process %d for verified termination: %w", pe.ProcessID, oerr))
			return true
		}
		defer closeKillProcess(handle)

		exePath, qerr := queryKillProcessPath(handle)
		if qerr != nil || exePath == "" {
			// Cannot confirm the location, so leave it alone rather than
			// terminate a process that may not be ours.
			if qerr == nil {
				qerr = fmt.Errorf("empty executable path")
			}
			failures = append(failures, fmt.Errorf("verify process %d before termination: %w", pe.ProcessID, qerr))
			return true
		}
		if !sameDir(exePath, target) {
			return true
		}

		if err := terminateKillProcess(handle, pe.ProcessID); err != nil {
			failures = append(failures, err)
		}
		return true
	})
	return errors.Join(append(failures, err)...)
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
	count, _ := CountInstancesInDirChecked(name, dir)
	return count
}

// CountInstancesInDirChecked returns verified matches and errors for unresolved candidates.
func CountInstancesInDirChecked(name, dir string) (int, error) {
	target := normalizeDir(dir)
	if target == "" {
		return 0, fmt.Errorf("target directory not available")
	}

	count := 0
	var lookupErrors []error
	err := enumerateProcesses(func(pe *processentry32w) bool {
		if !strings.EqualFold(processName(pe), name) {
			return true
		}
		exePath, err := lookupProcessPath(pe.ProcessID)
		if err != nil || exePath == "" {
			if err == nil {
				err = fmt.Errorf("empty executable path")
			}
			lookupErrors = append(lookupErrors, fmt.Errorf("resolve process %d: %w", pe.ProcessID, err))
			return true
		}
		if sameDir(exePath, target) {
			count++
		}
		return true
	})
	return count, errors.Join(append(lookupErrors, err)...)
}
