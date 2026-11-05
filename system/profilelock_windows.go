//go:build windows

package system

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/windows"
)

var ErrProfileInUse = errors.New("another process owns the Focusd profile")

// ProfileLock is released by Close or by Windows when the owning process exits.
type ProfileLock struct {
	file       *os.File
	overlapped windows.Overlapped
	once       sync.Once
	err        error
}

// AcquireProfileLock immediately rejects a second writer to the user's data root.
func AcquireProfileLock() (*ProfileLock, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return nil, fmt.Errorf("APPDATA environment variable not set")
	}
	dir := filepath.Join(appData, "focusd")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(dir, "daemon.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	lock := &ProfileLock{file: file}
	if err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &lock.overlapped); err != nil {
		file.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, ErrProfileInUse
		}
		return nil, fmt.Errorf("lock Focusd profile: %w", err)
	}
	return lock, nil
}

func (lock *ProfileLock) Close() error {
	if lock == nil {
		return nil
	}
	lock.once.Do(func() {
		lock.err = errors.Join(windows.UnlockFileEx(windows.Handle(lock.file.Fd()), 0, 1, 0, &lock.overlapped), lock.file.Close())
	})
	return lock.err
}
