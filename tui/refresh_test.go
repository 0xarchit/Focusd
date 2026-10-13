package tui

import (
	"testing"
	"time"
)

// TestDueForRefreshHonoursInterval checks the gate fires on elapsed time.
func TestDueForRefreshHonoursInterval(t *testing.T) {
	m := Model{}
	start := time.Date(2026, 6, 26, 10, 0, 0, 0, time.UTC)

	// The first tick establishes a baseline and does not reload.
	if m.dueForRefreshForTest(start) {
		t.Error("the first tick should not trigger a reload")
	}

	if m.dueForRefreshForTest(start.Add(refreshInterval - time.Second)) {
		t.Error("a reload fired before the interval elapsed")
	}

	if !m.dueForRefreshForTest(start.Add(refreshInterval)) {
		t.Error("a reload should fire once the interval has elapsed")
	}

	if m.dueForRefreshForTest(start.Add(refreshInterval + time.Second)) {
		t.Error("a second reload should not fire immediately after the first")
	}
}

// TestDueForRefreshNotModuloDriven is the regression guard for the old
// Second()%5 condition, which fired on every multiple of five within a minute
// rather than every five seconds.
func TestDueForRefreshNotModuloDriven(t *testing.T) {
	m := Model{}
	start := time.Date(2026, 6, 26, 10, 0, 0, 0, time.UTC)
	m.dueForRefreshForTest(start)

	fired := 0
	for i := 1; i <= 60; i++ {
		if m.dueForRefreshForTest(start.Add(time.Duration(i) * time.Second)) {
			fired++
		}
	}

	// Over one minute the gate may fire about four times, not twelve.
	if fired > 5 {
		t.Errorf("reload fired %d times in a minute; the interval is not being honoured", fired)
	}
	if fired == 0 {
		t.Error("the gate never fired over a minute")
	}
}

func TestNoteRefreshSuppressesNextAutomatic(t *testing.T) {
	m := Model{}
	start := time.Date(2026, 6, 26, 10, 0, 0, 0, time.UTC)
	m.dueForRefreshForTest(start)

	// A manual reload resets the clock.
	m.noteRefresh()

	if m.dueForRefreshForTest(start.Add(refreshInterval)) {
		t.Error("an automatic reload fired right after a manual one")
	}
}
