package core

import (
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

var (
	lastNotificationTime int64
	activeInstances      int32
)

var (
	user32          = syscall.NewLazyDLL("user32.dll")
	procMessageBoxW = user32.NewProc("MessageBoxW")
)

const (
	mbOKCancel        = 0x00000001
	mbIconInformation = 0x00000040
	mbIconWarning     = 0x00000030
	mbSetForeground   = 0x00010000
	idOK              = 1
)

func show(title, message string, flags uintptr, callback func(ret uintptr)) bool {
	now := time.Now().Unix()
	last := atomic.LoadInt64(&lastNotificationTime)
	if now-last < 10 {
		return false
	}
	if !atomic.CompareAndSwapInt32(&activeInstances, 0, 1) {
		return false
	}
	atomic.StoreInt64(&lastNotificationTime, now)

	go func() {
		defer atomic.StoreInt32(&activeInstances, 0)

		titlePtr, err := syscall.UTF16PtrFromString(title)
		if err != nil {
			return
		}
		messagePtr, err := syscall.UTF16PtrFromString(message)
		if err != nil {
			return
		}
		ret, _, _ := procMessageBoxW.Call(
			0,
			uintptr(unsafe.Pointer(messagePtr)),
			uintptr(unsafe.Pointer(titlePtr)),
			flags|mbSetForeground,
		)
		if callback != nil {
			callback(ret)
		}
	}()
	return true
}

func showNotification(title, message string) bool {
	return show(title, message, mbIconInformation, nil)
}

func showNotificationWithAction(title, message string, callback func(disable bool)) bool {
	fullMessage := message + "\n\n[OK] Disable this reminder\n[Cancel] Just close"
	return show(title, fullMessage, mbOKCancel|mbIconWarning, func(ret uintptr) {
		if callback != nil {
			callback(ret == idOK)
		}
	})
}
