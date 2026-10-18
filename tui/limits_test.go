package tui

import (
	"os"
	"testing"

	"focusd/storage"
	"focusd/system"
)

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

// TestHandleLimitsKeyClampsSelection pins the clamp: rows shrink under the async
// dashboard refresh while the cursor stays put, so edit and delete used to
// index past the slice and panic.
func TestHandleLimitsKeyClampsSelection(t *testing.T) {
	dir := t.TempDir()
	original, had := os.LookupEnv("APPDATA")
	if err := os.Setenv("APPDATA", dir); err != nil {
		t.Fatalf("setenv failed: %v", err)
	}
	t.Cleanup(func() {
		storage.Close()
		if had {
			os.Setenv("APPDATA", original)
		} else {
			os.Unsetenv("APPDATA")
		}
	})
	storage.Close()
	if err := storage.Init(); err != nil {
		t.Fatalf("storage init failed: %v", err)
	}

	if err := system.SetAppTimeLimit("chrome", 60); err != nil {
		t.Fatalf("SetAppTimeLimit failed: %v", err)
	}

	// Cursor at an index the current rows no longer cover.
	m := &Model{limitsSelected: 5}
	m.handleLimitsKey("d")
	if m.limitsSelected != 0 {
		t.Errorf("after delete, limitsSelected = %d; want 0", m.limitsSelected)
	}

	if err := system.SetAppTimeLimit("chrome", 60); err != nil {
		t.Fatalf("SetAppTimeLimit failed: %v", err)
	}
	m = &Model{limitsSelected: 5}
	m.handleLimitsKey("e")
	if m.limitsSelected != 0 {
		t.Errorf("after edit, limitsSelected = %d; want 0", m.limitsSelected)
	}
	if !m.limitForm.Visible {
		t.Error("edit should open the limit form")
	}
}
