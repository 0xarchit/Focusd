package storage

import (
	"testing"
	"time"
)

// TestGetAppUsageTodaySecondsForConfig pins the wildcard semantics the TUI
// limits view relies on: a bare exe name sums every install of it, a full app
// key matches exactly one install.
func TestGetAppUsageTodaySecondsForConfig(t *testing.T) {
	setupStorageEnv(t)

	insert := func(appID string, secs int) {
		t.Helper()
		if _, err := db.Exec(`
			INSERT INTO apps_daily (date, app_name, exe_name, app_id, total_duration_secs, open_count)
			VALUES (?, 'Chrome', 'chrome.exe', ?, ?, 1)
		`, Today(), appID, secs); err != nil {
			t.Fatalf("insert failed: %v", err)
		}
	}
	insert("chrome|aaaa1111", 1200)
	insert("chrome|bbbb2222", 300)
	insert("firefox|cccc3333", 900)

	tests := []struct {
		key  string
		want int
	}{
		// A bare key covers every install of the executable.
		{"chrome", 1500},
		{"chrome.exe", 1500},
		{"Chrome", 1500},
		// A full key matches one install only.
		{"chrome|aaaa1111", 1200},
		{"chrome|bbbb2222", 300},
		{"firefox", 900},
		{"notepad", 0},
	}
	for _, tc := range tests {
		if got := GetAppUsageTodaySecondsForConfig(tc.key); got != tc.want {
			t.Errorf("GetAppUsageTodaySecondsForConfig(%q) = %d; want %d", tc.key, got, tc.want)
		}
	}
}

func TestGetAppUsageTodaySecondsForConfigIncludesLiveSession(t *testing.T) {
	setupStorageEnv(t)

	if _, err := db.Exec(`
		INSERT INTO apps_daily (date, app_name, exe_name, app_id, total_duration_secs, open_count)
		VALUES (?, 'Chrome', 'chrome.exe', 'chrome|aaaa1111', 600, 1)
	`, Today()); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	if err := SaveActiveSession(&ActiveSessionRecord{
		AppName:   "Chrome",
		ExeName:   "chrome.exe",
		AppID:     "chrome|aaaa1111",
		StartTime: time.Now().Add(-90 * time.Second),
		LastSeen:  time.Now(),
		Date:      Today(),
	}); err != nil {
		t.Fatalf("SaveActiveSession failed: %v", err)
	}

	// The bare key includes the live session's elapsed time...
	if got := GetAppUsageTodaySecondsForConfig("chrome"); got <= 600 || got > 900 {
		t.Errorf("bare key usage = %d; want the live elapsed time added on top of 600", got)
	}
	// ...a full key of the live install too...
	if got := GetAppUsageTodaySecondsForConfig("chrome|aaaa1111"); got <= 600 || got > 900 {
		t.Errorf("full key usage = %d; want the live elapsed time added on top of 600", got)
	}
	// ...while another install does not pick it up.
	if got := GetAppUsageTodaySecondsForConfig("chrome|bbbb2222"); got != 0 {
		t.Errorf("usage for an unrelated install = %d; want 0", got)
	}
}
