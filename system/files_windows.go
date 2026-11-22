//go:build windows

package system

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// WritePrivateFileAtomic publishes a fully written file with a protected DACL.
// The current user, SYSTEM and administrators have access; inherited grants are
// excluded. Write and sync failures leave an existing destination unchanged.
func WritePrivateFileAtomic(path string, write func(io.Writer) error) error {
	if write == nil {
		return fmt.Errorf("private file writer is required")
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if err := ensurePrivateFileParent(target); err != nil {
		return err
	}
	if err := validatePrivateFilePath(target); err != nil {
		return err
	}
	targetGuard, err := lockPrivateFileTarget(target)
	if err != nil {
		return err
	}
	if targetGuard != 0 {
		defer func() {
			if targetGuard != 0 {
				windows.CloseHandle(targetGuard)
			}
		}()
	}
	descriptor, err := privateFileDescriptor()
	if err != nil {
		return err
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	stageDir := filepath.Join(filepath.Dir(target), ".focusd-export-"+hex.EncodeToString(random[:])+".tmp")
	dirPtr, err := windows.UTF16PtrFromString(stageDir)
	if err != nil {
		return err
	}
	attributes := windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: descriptor,
	}
	if err := windows.CreateDirectory(dirPtr, &attributes); err != nil {
		return fmt.Errorf("create private staging directory: %w", err)
	}
	defer os.Remove(stageDir)
	stage := filepath.Join(stageDir, "data")
	stagePtr, err := windows.UTF16PtrFromString(stage)
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(stagePtr, windows.GENERIC_WRITE, 0, &attributes, windows.CREATE_NEW, windows.FILE_ATTRIBUTE_NORMAL, 0)
	runtime.KeepAlive(descriptor)
	if err != nil {
		return fmt.Errorf("create private staged file: %w", err)
	}
	file := os.NewFile(uintptr(handle), stage)
	published := false
	defer func() {
		if !published {
			file.Close()
			os.Remove(stage)
		}
	}()
	if err := write(file); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := validatePrivateFilePath(target); err != nil {
		return err
	}
	targetPtr, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	// MoveFileEx opens the destination for replacement; our write-denying
	// verification handle must be closed before that open can succeed.
	if targetGuard != 0 {
		if err := windows.CloseHandle(targetGuard); err != nil {
			return fmt.Errorf("release existing export before publication: %w", err)
		}
		targetGuard = 0
	}
	if err := windows.MoveFileEx(stagePtr, targetPtr, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH); err != nil {
		return fmt.Errorf("publish private file: %w", err)
	}
	published = true
	return nil
}

func ensurePrivateFileParent(path string) error {
	volume := filepath.VolumeName(path)
	if volume == "" || strings.HasPrefix(volume, `\\`) || strings.Contains(path[len(volume):], ":") {
		return fmt.Errorf("private export requires a local regular-file path")
	}
	var missing []string
	dir := filepath.Dir(path)
	for {
		info, err := os.Lstat(dir)
		if err == nil {
			if !info.IsDir() {
				return fmt.Errorf("export parent is not a directory")
			}
			break
		}
		if !os.IsNotExist(err) {
			return err
		}
		missing = append(missing, dir)
		parent := filepath.Dir(dir)
		if parent == dir {
			return err
		}
		dir = parent
	}
	for current := dir; ; current = filepath.Dir(current) {
		if err := rejectReparsePath(current); err != nil {
			return err
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	for i := len(missing) - 1; i >= 0; i-- {
		if err := os.Mkdir(missing[i], 0700); err != nil && !os.IsExist(err) {
			return err
		}
		if err := rejectReparsePath(missing[i]); err != nil {
			return err
		}
	}
	return nil
}

func privateFileDescriptor() (*windows.SECURITY_DESCRIPTOR, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, fmt.Errorf("read current file owner: %w", err)
	}
	sid := user.User.Sid.String()
	return windows.SecurityDescriptorFromString("O:" + sid + "D:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;FA;;;" + sid + ")")
}

func validatePrivateFilePath(path string) error {
	volume := filepath.VolumeName(path)
	if volume == "" || strings.HasPrefix(volume, `\\`) {
		return fmt.Errorf("private export requires a local filesystem path")
	}
	if strings.Contains(path[len(volume):], ":") {
		return fmt.Errorf("alternate data streams are not export files")
	}
	info, err := os.Lstat(path)
	if err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("export destination must be a regular file")
		}
		if err := rejectReparsePath(path); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		if err := rejectReparsePath(dir); err != nil {
			return err
		}
		info, err := os.Stat(dir)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("export parent is not a directory")
		}
		if parent := filepath.Dir(dir); parent == dir {
			break
		}
	}
	return nil
}

func rejectReparsePath(path string) error {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attributes, err := windows.GetFileAttributes(ptr)
	if err != nil {
		return err
	}
	if attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("export path may not contain a reparse point")
	}
	return nil
}

func lockPrivateFileTarget(path string) (windows.Handle, error) {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	handle, err := windows.CreateFile(ptr, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err == windows.ERROR_FILE_NOT_FOUND {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("lock existing export: %w", err)
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		windows.CloseHandle(handle)
		return 0, err
	}
	if info.FileAttributes&(windows.FILE_ATTRIBUTE_DIRECTORY|windows.FILE_ATTRIBUTE_REPARSE_POINT) != 0 {
		windows.CloseHandle(handle)
		return 0, fmt.Errorf("existing export is not a regular file")
	}
	return handle, nil
}
