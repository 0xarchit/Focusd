package tui

import (
	"testing"
	"time"
)

func TestStatsRangeDates(t *testing.T) {
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	today := now.Format("2006-01-02")

	tests := []struct {
		name          string
		rangeIndex    int
		customFrom    string
		customTo      string
		expectedStart string
		expectedEnd   string
		expectedDays  int
	}{
		{
			name:          "Today range",
			rangeIndex:    0,
			expectedStart: today,
			expectedEnd:   today,
			expectedDays:  1,
		},
		{
			name:          "Last 7 Days range",
			rangeIndex:    1,
			expectedStart: now.AddDate(0, 0, -6).Format("2006-01-02"),
			expectedEnd:   today,
			expectedDays:  7,
		},
		{
			name:          "Last 30 Days range",
			rangeIndex:    2,
			expectedStart: now.AddDate(0, 0, -29).Format("2006-01-02"),
			expectedEnd:   today,
			expectedDays:  30,
		},
		{
			name:          "Custom range - valid normal",
			rangeIndex:    3,
			customFrom:    "2026-06-01",
			customTo:      "2026-06-05",
			expectedStart: "2026-06-01",
			expectedEnd:   "2026-06-05",
			expectedDays:  5,
		},
		{
			name:          "Custom range - swapped dates",
			rangeIndex:    3,
			customFrom:    "2026-06-05",
			customTo:      "2026-06-01",
			expectedStart: "2026-06-01",
			expectedEnd:   "2026-06-05",
			expectedDays:  5,
		},
		{
			name:          "Custom range - empty custom dates",
			rangeIndex:    3,
			customFrom:    "",
			customTo:      "",
			expectedStart: today,
			expectedEnd:   today,
			expectedDays:  1,
		},
		{
			name:          "Custom range - invalid custom dates format",
			rangeIndex:    3,
			customFrom:    "invalid-date",
			customTo:      "2026-06-05",
			expectedStart: today,
			expectedEnd:   today,
			expectedDays:  1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start, end, days := statsRangeDates(tc.rangeIndex, tc.customFrom, tc.customTo, now)
			if start != tc.expectedStart {
				t.Errorf("start = %q; want %q", start, tc.expectedStart)
			}
			if end != tc.expectedEnd {
				t.Errorf("end = %q; want %q", end, tc.expectedEnd)
			}
			if days != tc.expectedDays {
				t.Errorf("days = %d; want %d", days, tc.expectedDays)
			}
		})
	}
}

// The range limit is inclusive: a full leap year spans 366 dates and must be
// accepted, while one date more crosses the documented bound.
func TestValidateCustomRangeLeapBoundary(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	if _, _, err := validateCustomRange("2024-01-01", "2024-12-31", now); err != nil {
		t.Errorf("a full leap year (366 dates) was rejected: %v", err)
	}
	if _, _, err := validateCustomRange("2024-01-01", "2025-01-01", now); err == nil {
		t.Error("367 dates were accepted; want rejection")
	}
}

// "Today" must be selectable at any local time. The dates parse as UTC
// midnight, so the old wall-clock comparison rejected the local "today" in
// every timezone east of UTC until its offset had elapsed.
func TestValidateCustomRangeAcceptsTodayAtLocalMidnight(t *testing.T) {
	ist := time.FixedZone("IST", 5*3600+1800) // UTC+5:30
	now := time.Date(2026, 10, 18, 2, 0, 0, 0, ist)
	from, to, err := validateCustomRange("2026-10-18", "2026-10-18", now)
	if err != nil {
		t.Fatalf("today was rejected at 02:00 local in a UTC+5:30 zone: %v", err)
	}
	if from != "2026-10-18" || to != "2026-10-18" {
		t.Errorf("validateCustomRange returned %q, %q; want both unchanged", from, to)
	}
}

func TestValidateCustomRangeRejectsTomorrow(t *testing.T) {
	ist := time.FixedZone("IST", 5*3600+1800) // UTC+5:30
	now := time.Date(2026, 10, 18, 2, 0, 0, 0, ist)
	if _, _, err := validateCustomRange("2026-10-18", "2026-10-19", now); err == nil {
		t.Error("a future end date was accepted; want rejection")
	}
}
