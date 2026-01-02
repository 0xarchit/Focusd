package core

import (
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	lastNotificationTime time.Time
	notificationMutex    sync.Mutex
	notificationCooldown = 30 * time.Second
)

var (
	user32          = syscall.NewLazyDLL("user32.dll")
	procMessageBoxW = user32.NewProc("MessageBoxW")
)

const (
	MB_OK              = 0x00000000
	MB_ICONINFORMATION = 0x00000040
	MB_SYSTEMMODAL     = 0x00001000
	MB_SETFOREGROUND   = 0x00010000
)

func ShowNotification(title, message string) {
	notificationMutex.Lock()
	if time.Since(lastNotificationTime) < notificationCooldown {
		notificationMutex.Unlock()
		return
	}
	lastNotificationTime = time.Now()
	notificationMutex.Unlock()

	go func() {
		titlePtr, _ := syscall.UTF16PtrFromString(title)
		messagePtr, _ := syscall.UTF16PtrFromString(message)
		procMessageBoxW.Call(
			0,
			uintptr(unsafe.Pointer(messagePtr)),
			uintptr(unsafe.Pointer(titlePtr)),
			uintptr(MB_OK|MB_ICONINFORMATION|MB_SETFOREGROUND),
		)
	}()
}
