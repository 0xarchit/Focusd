package core

import (
	"golang.org/x/time/rate"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

var (
	limiter         = rate.NewLimiter(rate.Every(10*time.Second), 1)
	activeInstances int32
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

func show(title, message string, flags uintptr, callback func(ret uintptr)) {
	if !limiter.Allow() {
		return
	}
	if !atomic.CompareAndSwapInt32(&activeInstances, 0, 1) {
		return
	}

	go func() {
		defer atomic.StoreInt32(&activeInstances, 0)

		titlePtr, _ := syscall.UTF16PtrFromString(title)
		messagePtr, _ := syscall.UTF16PtrFromString(message)
		ret, _, _ := procMessageBoxW.Call(
			0,
			uintptr(unsafe.Pointer(messagePtr)),
			uintptr(unsafe.Pointer(titlePtr)),
			flags|MB_SETFOREGROUND,
		)
		if callback != nil {
			callback(ret)
		}
	}()
}

func ShowNotification(title, message string) {
	show(title, message, MB_OK|MB_ICONINFORMATION, nil)
}

func ShowNotificationWithAction(title, message string, callback func(disable bool)) {
	fullMessage := message + "\n\n[OK] Disable this reminder\n[Cancel] Just close"
	show(title, fullMessage, MB_OKCANCEL|MB_ICONWARNING, func(ret uintptr) {
		if callback != nil {
			callback(ret == IDOK)
		}
	})
}
