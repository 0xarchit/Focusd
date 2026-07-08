package core

import (
	"time"

	"github.com/go-toast/toast"
)

var lastNotification time.Time

func showNotification(title, message string) bool {
	if time.Since(lastNotification) < 10*time.Second {
		return false
	}
	lastNotification = time.Now()

	go func() {
		notification := toast.Notification{
			AppID:   "Focusd",
			Title:   title,
			Message: message,
		}
		_ = notification.Push()
	}()
	return true
}

func showNotificationWithAction(title, message string, callback func(disable bool)) bool {
	if time.Since(lastNotification) < 10*time.Second {
		return false
	}
	lastNotification = time.Now()

	go func() {
		notification := toast.Notification{
			AppID:   "Focusd",
			Title:   title,
			Message: message,
		}
		_ = notification.Push()
		// ponytail: go-toast is push-and-forget; callback defaults to non-snooze
		if callback != nil {
			callback(false)
		}
	}()
	return true
}
