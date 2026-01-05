package core

import (
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	lastNotificationTime  time.Time
	notificationMutex     sync.Mutex
	notificationCooldown  = 10 * time.Second
	isNotificationVisible bool
)

var (
	user32          = syscall.NewLazyDLL("user32.dll")
	procMessageBoxW = user32.NewProc("MessageBoxW")
)

const (
	MB_OK              = 0x00000000
	MB_OKCANCEL        = 0x00000001
	MB_YESNO           = 0x00000004
	MB_ICONINFORMATION = 0x00000040
	MB_ICONWARNING     = 0x00000030
	MB_SYSTEMMODAL     = 0x00001000
	MB_SETFOREGROUND   = 0x00010000
	IDOK               = 1
	IDYES              = 6
	IDNO               = 7
)

func ShowNotification(title, message string) {
	notificationMutex.Lock()
	if time.Since(lastNotificationTime) < notificationCooldown || isNotificationVisible {
		notificationMutex.Unlock()
		return
	}
	lastNotificationTime = time.Now()
	isNotificationVisible = true
	notificationMutex.Unlock()

	go func() {
		defer func() {
			notificationMutex.Lock()
			isNotificationVisible = false
			notificationMutex.Unlock()
		}()

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

func ShowNotificationWithAction(title, message string, callback func(disable bool)) {
	notificationMutex.Lock()
	if time.Since(lastNotificationTime) < notificationCooldown || isNotificationVisible {
		notificationMutex.Unlock()
		return
	}
	lastNotificationTime = time.Now()
	isNotificationVisible = true
	notificationMutex.Unlock()

	go func() {
		defer func() {
			notificationMutex.Lock()
			isNotificationVisible = false
			notificationMutex.Unlock()
		}()

		titlePtr, _ := syscall.UTF16PtrFromString(title)
		fullMessage := message + "\n\n[OK] Disable this reminder\n[Cancel] Just close"
		messagePtr, _ := syscall.UTF16PtrFromString(fullMessage)
		ret, _, _ := procMessageBoxW.Call(
			0,
			uintptr(unsafe.Pointer(messagePtr)),
			uintptr(unsafe.Pointer(titlePtr)),
			uintptr(MB_OKCANCEL|MB_ICONWARNING|MB_SETFOREGROUND),
		)
		if callback != nil {
			callback(ret == IDOK)
		}
	}()
}
