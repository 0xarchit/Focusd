package core

import (
	"testing"
	"time"
)

func TestReminderLimitOncePerDayAndExplicitSnooze(t *testing.T) {
	r := &reminderState{}
	now := time.Now()
	if !r.shouldLimit("2026-11-22", "app", now) {
		t.Fatal("first limit blocked")
	}
	r.fired["app"] = true
	if r.shouldLimit("2026-11-22", "app", now) {
		t.Fatal("same-day app switch rearmed limit")
	}
	r.snoozed["app"] = now.Add(time.Minute)
	if r.shouldLimit("2026-11-22", "app", now) {
		t.Fatal("snooze ignored")
	}
	if !r.shouldLimit("2026-11-22", "app", now.Add(2*time.Minute)) {
		t.Fatal("explicit snooze did not allow repeat")
	}
	r.fired["app"] = true
	if !r.shouldLimit("2026-11-23", "app", now) {
		t.Fatal("new day failed to rearm limit")
	}
}
func TestReminderPresenceIndependentOfForegroundSession(t *testing.T) {
	r := &reminderState{}
	now := time.Now()
	r.setPresent(now, true)
	r.setPresent(now.Add(time.Minute), true)
	if !r.presentSince.Equal(now) {
		t.Fatal("present foreground transition restarted break interval")
	}
	r.setPresent(now.Add(2*time.Minute), false)
	if !r.presentSince.IsZero() {
		t.Fatal("absence retained break interval")
	}
}
