package core

import (
	"fmt"
	"focusd/storage"
	"focusd/system"
	"strings"
	"sync"
	"time"
)

// Notification callbacks share reminder state with the controller, but never
// mutate tracking or clock state.
type reminderState struct {
	mu           sync.Mutex
	presentSince time.Time
	breakSnoozed time.Time
	date         string
	fired        map[string]bool
	snoozed      map[string]time.Time
}

func (r *reminderState) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.presentSince = time.Time{}
	r.breakSnoozed = time.Time{}
	r.date = ""
	r.fired = make(map[string]bool)
	r.snoozed = make(map[string]time.Time)
}

func (r *reminderState) setPresent(now time.Time, present bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !present {
		r.presentSince = time.Time{}
	} else if r.presentSince.IsZero() {
		r.presentSince = now
	}
}
func (r *reminderState) shouldLimit(date, key string, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.date != date {
		r.date = date
		r.fired = make(map[string]bool)
		r.snoozed = make(map[string]time.Time)
	}
	if until := r.snoozed[key]; !until.IsZero() {
		if now.Before(until) {
			return false
		}
		delete(r.snoozed, key)
		delete(r.fired, key)
	}
	return !r.fired[key]
}
func (c *daemonController) checkReminders() {
	now := time.Now()
	r := &c.reminders
	snooze := time.Duration(system.GetSnoozeDurationMinutes()) * time.Minute
	r.mu.Lock()
	breakDue := !r.presentSince.IsZero() && !now.Before(r.breakSnoozed) && now.Sub(r.presentSince) >= time.Duration(system.GetBreakReminderMinutes())*time.Minute
	r.mu.Unlock()
	if system.GetBreakReminderEnabled() && breakDue {
		if showNotificationWithAction(notifyContext, "Break Reminder", fmt.Sprintf("You've been working for %d min. Take a break!", system.GetBreakReminderMinutes()), func(disable bool) {
			if disable {
				r.mu.Lock()
				r.breakSnoozed = time.Now().Add(snooze)
				r.mu.Unlock()
			}
		}) {
			r.mu.Lock()
			r.presentSince = now
			r.mu.Unlock()
		}
	}
	c.tracker.mu.Lock()
	var exe, appID, name string
	if session := c.tracker.currentSession; session != nil {
		exe = strings.ToLower(session.ExeName)
		appID = session.AppID
		name = session.AppName
	}
	c.tracker.mu.Unlock()
	limits := system.GetAppTimeLimits()
	key, ok := system.MatchLimitKey(limits, appID, exe)
	if !ok || exe == "" || !r.shouldLimit(storage.Today(), key, now) {
		return
	}
	if storage.GetAppUsageTodaySecondsForConfig(key)/60 < limits[key] {
		return
	}
	if showNotificationWithAction(notifyContext, "App Time Limit", name+" has exceeded daily limit!", func(disable bool) {
		if disable {
			r.mu.Lock()
			r.snoozed[key] = time.Now().Add(snooze)
			r.mu.Unlock()
		}
	}) {
		r.mu.Lock()
		r.fired[key] = true
		r.mu.Unlock()
	}
}
