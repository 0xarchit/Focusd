package tui

import "testing"

// TestLimitUsageTodayMinutes pins the dashboard "Limits Hit" matching: a bare
// limit key sums every install of the executable, a full key matches one.
func TestLimitUsageTodayMinutes(t *testing.T) {
	usage := map[string]int{
		"chrome|aaaa1111":  30,
		"chrome|bbbb2222":  20,
		"firefox|cccc3333": 80,
	}

	tests := []struct {
		key  string
		want int
	}{
		{"chrome", 50},
		{"chrome.exe", 50},
		{"chrome|aaaa1111", 30},
		{"chrome|dddd4444", 0},
		{"firefox", 80},
		{"edge", 0},
	}
	for _, tc := range tests {
		if got := limitUsageTodayMinutes(usage, tc.key); got != tc.want {
			t.Errorf("limitUsageTodayMinutes(%q) = %d; want %d", tc.key, got, tc.want)
		}
	}
}
