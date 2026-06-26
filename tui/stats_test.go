package tui

import (
	"focusd/storage"
	"testing"
	"time"
)

func TestStatsRangeDates(t *testing.T) {
	today := storage.Today()
	now := time.Now()

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
			start, end, days := statsRangeDates(tc.rangeIndex, tc.customFrom, tc.customTo)
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
