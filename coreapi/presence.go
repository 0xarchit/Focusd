package coreapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"focusd/system"
	"golang.org/x/sys/windows"
)

const (
	maxTUIPresences = 32
	tuiLeaseTicks   = uint64(10 * 10_000_000)
)

var (
	clientIDPattern   = regexp.MustCompile(`^[0-9a-f]{32}$`)
	presenceTrustees  = regexp.MustCompile(`\([^()]*;;;([^()]*)\)`)
	readPresenceOwner = presenceProcessIdentity
)

// TUIPresence allows an already-open TUI to claim an incident before daemon startup.
type TUIPresence struct {
	ClientID       string `json:"client_id"`
	PID            int    `json:"pid"`
	BootID         string `json:"boot_id"`
	InterruptTicks uint64 `json:"interrupt_ticks"`
	ExePath        string `json:"exe_path"`
	ProcessCreated uint64 `json:"process_created"`
}

func presenceDirectory() (string, error) {
	path, err := endpointPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "tui-clients"), nil
}

func presencePath(clientID string) (string, error) {
	if !clientIDPattern.MatchString(clientID) {
		return "", fmt.Errorf("TUI client ID must contain 32 lowercase hexadecimal characters")
	}
	dir, err := presenceDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, clientID+".json"), nil
}

// PublishTUIPresence renews one process-bound ten-second lease without daemon IPC.
func PublishTUIPresence(clientID string, sample system.ClockSample) error {
	path, err := presencePath(clientID)
	if err != nil {
		return err
	}
	if sample.BootID == "" {
		return fmt.Errorf("TUI presence requires a verified boot identity")
	}
	exePath, created, err := readPresenceOwner(os.Getpid())
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Base(exePath), "focusd.exe") || created == 0 {
		return fmt.Errorf("TUI presence publisher is not a verified Focusd process")
	}
	entries, err := readPresenceEntries(filepath.Dir(path))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(entries) >= maxTUIPresences {
		if _, err := os.Lstat(path); err != nil {
			return fmt.Errorf("too many TUI presence records")
		}
	}
	presence := TUIPresence{ClientID: clientID, PID: os.Getpid(), BootID: sample.BootID, InterruptTicks: sample.InterruptTicks, ExePath: exePath, ProcessCreated: created}
	return system.WritePrivateFileAtomic(path, func(writer io.Writer) error { return json.NewEncoder(writer).Encode(presence) })
}

// RemoveTUIPresence removes only the caller's process-bound lease.
func RemoveTUIPresence(clientID string) error {
	path, err := presencePath(clientID)
	if err != nil {
		return err
	}
	presence, err := readPresenceFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	exePath, created, err := readPresenceOwner(os.Getpid())
	if err != nil {
		return err
	}
	if presence.ClientID != clientID || presence.PID != os.Getpid() || presence.ProcessCreated != created || !strings.EqualFold(presence.ExePath, exePath) {
		return fmt.Errorf("TUI presence belongs to another process")
	}
	return os.Remove(path)
}

// ReadTUIPresences ignores expired or invalid leases without mutating profile files.
func ReadTUIPresences(sample system.ClockSample) ([]TUIPresence, error) {
	if sample.BootID == "" {
		return nil, nil
	}
	dir, err := presenceDirectory()
	if err != nil {
		return nil, err
	}
	entries, err := readPresenceEntries(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var active []TUIPresence
	for _, entry := range entries {
		name := entry.Name()
		clientID := strings.TrimSuffix(name, ".json")
		if name != clientID+".json" || !clientIDPattern.MatchString(clientID) || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		presence, err := readPresenceFile(filepath.Join(dir, name))
		if err != nil || presence.ClientID != clientID || presence.PID <= 0 || presence.BootID != sample.BootID || presence.InterruptTicks > sample.InterruptTicks || sample.InterruptTicks-presence.InterruptTicks > tuiLeaseTicks {
			continue
		}
		exePath, created, err := readPresenceOwner(presence.PID)
		if err != nil || created == 0 || created != presence.ProcessCreated || !strings.EqualFold(filepath.Base(exePath), "focusd.exe") || !strings.EqualFold(exePath, presence.ExePath) {
			continue
		}
		active = append(active, presence)
	}
	return active, nil
}

func readPresenceEntries(dir string) ([]os.DirEntry, error) {
	if err := rejectPresenceReparsePath(dir); err != nil {
		return nil, err
	}
	file, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	entries, err := file.ReadDir(maxTUIPresences + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(entries) > maxTUIPresences {
		return nil, fmt.Errorf("TUI presence inventory exceeds %d records", maxTUIPresences)
	}
	return entries, nil
}

func rejectPresenceReparsePath(path string) error {
	for current := path; ; current = filepath.Dir(current) {
		ptr, err := windows.UTF16PtrFromString(current)
		if err != nil {
			return err
		}
		attributes, err := windows.GetFileAttributes(ptr)
		if err != nil {
			return err
		}
		if attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return fmt.Errorf("TUI presence path contains a reparse point")
		}
		if filepath.Dir(current) == current {
			return nil
		}
	}
}

func readPresenceFile(path string) (TUIPresence, error) {
	var presence TUIPresence
	if err := rejectPresenceReparsePath(path); err != nil {
		return presence, err
	}
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return presence, err
	}
	handle, err := windows.CreateFile(ptr, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return presence, err
	}
	file := os.NewFile(uintptr(handle), path)
	defer file.Close()
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return presence, err
	}
	if info.FileAttributes&(windows.FILE_ATTRIBUTE_DIRECTORY|windows.FILE_ATTRIBUTE_REPARSE_POINT) != 0 {
		return presence, fmt.Errorf("TUI presence must be a regular file")
	}
	if uint64(info.FileSizeHigh)<<32|uint64(info.FileSizeLow) > MaxIPCBytes {
		return presence, fmt.Errorf("TUI presence exceeds size limit")
	}
	descriptor, err := windows.GetSecurityInfo(handle, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return presence, err
	}
	owner, _, err := descriptor.Owner()
	if err != nil {
		return presence, err
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || owner == nil || !owner.Equals(user.User.Sid) {
		return presence, fmt.Errorf("TUI presence has another owner")
	}
	control, _, err := descriptor.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		return presence, fmt.Errorf("TUI presence requires a protected ACL")
	}
	dacl, _, err := descriptor.DACL()
	if err != nil || dacl == nil {
		return presence, fmt.Errorf("TUI presence has no restrictive ACL")
	}
	for _, ace := range presenceTrustees.FindAllStringSubmatch(descriptor.String(), -1) {
		if ace[1] != "SY" && ace[1] != "BA" && ace[1] != user.User.Sid.String() {
			return presence, fmt.Errorf("TUI presence grants access outside its owner")
		}
	}
	data, err := io.ReadAll(io.LimitReader(file, MaxIPCBytes+1))
	if err != nil {
		return presence, err
	}
	if len(data) > MaxIPCBytes {
		return presence, fmt.Errorf("TUI presence exceeds size limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&presence); err != nil {
		return presence, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return presence, fmt.Errorf("TUI presence contains trailing data")
	}
	return presence, nil
}

func presenceProcessIdentity(pid int) (string, uint64, error) {
	if pid < 1 || uint64(pid) > uint64(^uint32(0)) {
		return "", 0, fmt.Errorf("invalid TUI process ID")
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return "", 0, err
	}
	defer windows.CloseHandle(handle)
	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil {
		return "", 0, err
	}
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &created, &exited, &kernel, &user); err != nil {
		return "", 0, err
	}
	if exited.LowDateTime != 0 || exited.HighDateTime != 0 {
		return "", 0, fmt.Errorf("TUI process has exited")
	}
	return windows.UTF16ToString(buffer[:size]), uint64(created.HighDateTime)<<32 | uint64(created.LowDateTime), nil
}
