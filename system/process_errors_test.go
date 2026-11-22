package system

import (
	"errors"
	"strings"
	"syscall"
	"testing"
)

func fakeProcess(name string, pid uint32) processentry32w {
	pe := processentry32w{ProcessID: pid}
	copy(pe.ExeFile[:], syscall.StringToUTF16(name))
	return pe
}

func stubProcesses(t *testing.T, entries []processentry32w, enumerationError error, paths map[uint32]string) {
	t.Helper()
	originalEnumeration, originalPath := enumerateProcesses, lookupProcessPath
	t.Cleanup(func() {
		enumerateProcesses, lookupProcessPath = originalEnumeration, originalPath
	})
	enumerateProcesses = func(callback func(*processentry32w) bool) error {
		for i := range entries {
			if !callback(&entries[i]) {
				break
			}
		}
		return enumerationError
	}
	lookupProcessPath = func(pid uint32) (string, error) {
		if path, ok := paths[pid]; ok {
			return path, nil
		}
		return "", errors.New("path unavailable")
	}
}

func TestCheckedProcessCountReturnsEnumerationFailure(t *testing.T) {
	want := errors.New("snapshot unavailable")
	stubProcesses(t, nil, want, nil)
	if count, err := CountInstancesInDirChecked("focusd_daemon.exe", `C:\focusd`); count != 0 || !errors.Is(err, want) {
		t.Fatalf("count=%d err=%v; want count 0 and snapshot failure", count, err)
	}
	if count, err := GetProcessCountChecked("focusd_daemon.exe"); count != 0 || !errors.Is(err, want) {
		t.Fatalf("name count=%d err=%v; want snapshot failure", count, err)
	}
}

func TestCheckedProcessCountScopesDirectoriesAndReportsUnknownPaths(t *testing.T) {
	entries := []processentry32w{fakeProcess("focusd_daemon.exe", 1), fakeProcess("FOCUSD_DAEMON.EXE", 2), fakeProcess("focusd_daemon.exe", 3), fakeProcess("unrelated.exe", 4)}
	stubProcesses(t, entries, nil, map[uint32]string{1: `C:\focusd\focusd_daemon.exe`, 2: `C:\other\focusd_daemon.exe`})
	count, err := CountInstancesInDirChecked("focusd_daemon.exe", `C:\focusd`)
	if count != 1 || err == nil || !strings.Contains(err.Error(), "3") {
		t.Fatalf("count=%d err=%v; want one local process plus unresolved process 3", count, err)
	}
}

func TestCheckedProcessCountDistinguishesSuccessfulEmptySnapshot(t *testing.T) {
	stubProcesses(t, nil, nil, nil)
	if count, err := CountInstancesInDirChecked("focusd_daemon.exe", `C:\focusd`); count != 0 || err != nil {
		t.Fatalf("count=%d err=%v; want a successful empty snapshot", count, err)
	}
	if _, err := CountInstancesInDirChecked("focusd_daemon.exe", ""); err == nil {
		t.Fatal("an absent target directory must not be reported as a successful empty snapshot")
	}
}

func TestScopedKillReportsEnumerationFailureWithoutOpeningProcess(t *testing.T) {
	want := errors.New("snapshot unavailable")
	stubProcesses(t, nil, want, nil)
	if err := KillInstancesInDir("focusd_daemon.exe", `C:\focusd`); !errors.Is(err, want) {
		t.Fatalf("err=%v; want snapshot failure", err)
	}
}

func TestScopedKillVerifiesAndTerminatesSameHandle(t *testing.T) {
	stubProcesses(t, []processentry32w{fakeProcess("focusd_daemon.exe", 123), fakeProcess("focusd_daemon.exe", 456)}, nil, nil)
	originalOpen, originalQuery, originalClose, originalTerminate := openKillProcess, queryKillProcessPath, closeKillProcess, terminateKillProcess
	t.Cleanup(func() {
		openKillProcess, queryKillProcessPath, closeKillProcess, terminateKillProcess = originalOpen, originalQuery, originalClose, originalTerminate
	})
	opened, closed, terminated := []uintptr{}, []uintptr{}, []uintptr{}
	openKillProcess = func(pid uint32) (uintptr, error) {
		handle := uintptr(pid + 1000)
		opened = append(opened, handle)
		return handle, nil
	}
	queryKillProcessPath = func(handle uintptr) (string, error) {
		if handle == 1123 {
			return `C:\focusd\focusd_daemon.exe`, nil
		}
		return `C:\other\focusd_daemon.exe`, nil
	}
	closeKillProcess = func(handle uintptr) { closed = append(closed, handle) }
	terminateKillProcess = func(handle uintptr, pid uint32) error {
		terminated = append(terminated, handle)
		if handle != uintptr(pid+1000) {
			t.Fatal("verified process handle was replaced")
		}
		return nil
	}
	if err := KillInstancesInDir("focusd_daemon.exe", `C:\focusd`); err != nil {
		t.Fatal(err)
	}
	if len(opened) != 2 || len(closed) != 2 || len(terminated) != 1 || terminated[0] != 1123 {
		t.Fatalf("opened=%v closed=%v terminated=%v", opened, closed, terminated)
	}
}

func TestScopedKillDoesNotTerminateUnverifiableProcess(t *testing.T) {
	stubProcesses(t, []processentry32w{fakeProcess("focusd_daemon.exe", 123)}, nil, nil)
	originalOpen, originalQuery, originalClose, originalTerminate := openKillProcess, queryKillProcessPath, closeKillProcess, terminateKillProcess
	t.Cleanup(func() {
		openKillProcess, queryKillProcessPath, closeKillProcess, terminateKillProcess = originalOpen, originalQuery, originalClose, originalTerminate
	})
	openKillProcess = func(uint32) (uintptr, error) { return 789, nil }
	queryKillProcessPath = func(uintptr) (string, error) { return "", errors.New("access denied") }
	closed := false
	closeKillProcess = func(handle uintptr) { closed = handle == 789 }
	terminateKillProcess = func(uintptr, uint32) error { t.Fatal("unverified process was terminated"); return nil }
	if err := KillInstancesInDir("focusd_daemon.exe", `C:\focusd`); err == nil {
		t.Fatal("unverified process must report an incomplete stop")
	}
	if !closed {
		t.Fatal("verification handle was not closed")
	}
}

func TestScopedKillReturnsOpenAndTerminateFailures(t *testing.T) {
	stubProcesses(t, []processentry32w{fakeProcess("focusd_daemon.exe", 123), fakeProcess("focusd_daemon.exe", 456)}, nil, nil)
	originalOpen, originalQuery, originalClose, originalTerminate := openKillProcess, queryKillProcessPath, closeKillProcess, terminateKillProcess
	t.Cleanup(func() {
		openKillProcess, queryKillProcessPath, closeKillProcess, terminateKillProcess = originalOpen, originalQuery, originalClose, originalTerminate
	})
	wantOpen := errors.New("cannot open candidate")
	wantTerminate := errors.New("cannot terminate candidate")
	openKillProcess = func(pid uint32) (uintptr, error) {
		if pid == 123 {
			return 0, wantOpen
		}
		return 789, nil
	}
	queryKillProcessPath = func(uintptr) (string, error) { return `C:\focusd\focusd_daemon.exe`, nil }
	closed := false
	closeKillProcess = func(handle uintptr) { closed = handle == 789 }
	terminateKillProcess = func(handle uintptr, pid uint32) error {
		if handle != 789 || pid != 456 {
			t.Fatalf("unexpected termination target %d/%d", handle, pid)
		}
		return wantTerminate
	}
	err := KillInstancesInDir("focusd_daemon.exe", `C:\focusd`)
	if !errors.Is(err, wantOpen) || !errors.Is(err, wantTerminate) {
		t.Fatalf("err=%v; want both incomplete operations", err)
	}
	if !closed {
		t.Fatal("failed termination did not close its verification handle")
	}
}
