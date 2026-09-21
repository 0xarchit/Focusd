package core

import (
	"testing"
	"time"
)

func activeSessionBetween(start time.Time) *activeSession {
	return &activeSession{
		AppName:     "TestApp",
		ExeName:     "test.exe",
		WindowTitle: "Window",
		StartTime:   start,
		Date:        dateStr(start),
	}
}

// TestSplitSessionWithinOneDay is the ordinary case: one slice, dated by start.
func TestSplitSessionWithinOneDay(t *testing.T) {
	start := time.Date(2026, 6, 26, 10, 0, 0, 0, time.Local)
	end := start.Add(30 * time.Minute)

	slices := splitSessionAtMidnight(activeSessionBetween(start), end)

	if len(slices) != 1 {
		t.Fatalf("expected 1 slice, got %d", len(slices))
	}
	s := slices[0]
	if s.DurationSecs != 1800 {
		t.Errorf("duration = %d; want 1800", s.DurationSecs)
	}
	if s.Date != "2026-06-26" {
		t.Errorf("date = %q; want 2026-06-26", s.Date)
	}
	if !s.StartTime.Equal(start) || !s.EndTime.Equal(end) {
		t.Errorf("times = %v..%v; want %v..%v", s.StartTime, s.EndTime, start, end)
	}
}

// TestSplitSessionAcrossMidnight is the regression guard for the mis-attribution
// bug: a session running over midnight used to be stamped entirely with the
// previous day, so post-midnight time was counted as yesterday.
func TestSplitSessionAcrossMidnight(t *testing.T) {
	start := time.Date(2026, 6, 26, 22, 0, 0, 0, time.Local)
	midnight := time.Date(2026, 6, 27, 0, 0, 0, 0, time.Local)
	end := midnight.Add(2 * time.Hour)

	slices := splitSessionAtMidnight(activeSessionBetween(start), end)

	if len(slices) != 2 {
		t.Fatalf("expected 2 slices, got %d", len(slices))
	}

	yesterday := slices[0]
	if yesterday.Date != "2026-06-26" {
		t.Errorf("first slice date = %q; want 2026-06-26", yesterday.Date)
	}
	if yesterday.DurationSecs != 2*3600 {
		t.Errorf("first slice duration = %d; want %d", yesterday.DurationSecs, 2*3600)
	}
	if !yesterday.EndTime.Equal(midnight) {
		t.Errorf("first slice should end at midnight, got %v", yesterday.EndTime)
	}

	todaySlice := slices[1]
	if todaySlice.Date != "2026-06-27" {
		t.Errorf("second slice date = %q; want 2026-06-27", todaySlice.Date)
	}
	if todaySlice.DurationSecs != 2*3600 {
		t.Errorf("second slice duration = %d; want %d", todaySlice.DurationSecs, 2*3600)
	}
	if !todaySlice.StartTime.Equal(midnight) {
		t.Errorf("second slice should start at midnight, got %v", todaySlice.StartTime)
	}

	// No time may be lost or invented across the boundary.
	total := 0
	for _, s := range slices {
		total += s.DurationSecs
	}
	if total != int(end.Sub(start).Seconds()) {
		t.Errorf("total duration = %d; want %d", total, int(end.Sub(start).Seconds()))
	}
}

// TestSplitSessionMultiDaySession ensures each day is accounted exactly once
// when a session runs across more than one boundary.
func TestSplitSessionMultiDaySession(t *testing.T) {
	start := time.Date(2026, 6, 25, 20, 0, 0, 0, time.Local)
	end := time.Date(2026, 6, 27, 4, 0, 0, 0, time.Local)

	slices := splitSessionAtMidnight(activeSessionBetween(start), end)

	if len(slices) != 3 {
		t.Fatalf("expected 3 slices, got %d", len(slices))
	}

	wantDates := []string{"2026-06-25", "2026-06-26", "2026-06-27"}
	for i, want := range wantDates {
		if slices[i].Date != want {
			t.Errorf("slice %d date = %q; want %q", i, slices[i].Date, want)
		}
	}

	total := 0
	for _, s := range slices {
		total += s.DurationSecs
	}
	if total != int(end.Sub(start).Seconds()) {
		t.Errorf("total duration = %d; want %d", total, int(end.Sub(start).Seconds()))
	}
}

func TestSplitSessionTooShortIsDropped(t *testing.T) {
	start := time.Date(2026, 6, 26, 10, 0, 0, 0, time.Local)
	end := start.Add(400 * time.Millisecond)

	if slices := splitSessionAtMidnight(activeSessionBetween(start), end); len(slices) != 0 {
		t.Errorf("expected a sub-second session to be dropped, got %d slices", len(slices))
	}
}

func TestSplitSessionNil(t *testing.T) {
	if slices := splitSessionAtMidnight(nil, time.Now()); slices != nil {
		t.Errorf("expected nil for a nil session, got %d slices", len(slices))
	}
}

// TestSplitSessionSubSecondAcrossMidnight drops only the too-short half, so the
// usable remainder is still recorded.
func TestSplitSessionSubSecondAcrossMidnight(t *testing.T) {
	midnight := time.Date(2026, 6, 27, 0, 0, 0, 0, time.Local)
	start := midnight.Add(-500 * time.Millisecond)
	end := midnight.Add(30 * time.Minute)

	slices := splitSessionAtMidnight(activeSessionBetween(start), end)

	if len(slices) != 1 {
		t.Fatalf("expected 1 slice, got %d", len(slices))
	}
	if slices[0].Date != "2026-06-27" {
		t.Errorf("date = %q; want 2026-06-27", slices[0].Date)
	}
	if slices[0].DurationSecs != 30*60 {
		t.Errorf("duration = %d; want %d", slices[0].DurationSecs, 30*60)
	}
}
