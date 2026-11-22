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
		{"Chrome.exe|AAAA1111", 1200},
		{" Chrome.exe | AAAA1111 ", 1200},
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
	if got, want := GetAppUsageTodaySecondsForConfig("Chrome.exe|AAAA1111"), GetAppUsageTodaySecondsForConfig("chrome|aaaa1111"); got != want {
		t.Errorf("legacy uppercase full key usage = %d; want stored and live total %d", got, want)
	}
	// ...while another install does not pick it up.
	if got := GetAppUsageTodaySecondsForConfig("chrome|bbbb2222"); got != 0 {
		t.Errorf("usage for an unrelated install = %d; want 0", got)
	}
}

// TestLiveElapsedTodayClipsAtMidnight pins the "today" portion of a session
// that crossed local midnight: only the time after midnight counts, not the
// prior day's stretch.
func TestLiveElapsedTodayClipsAtMidnight(t *testing.T) {
	midnight := time.Date(2026, 6, 26, 0, 0, 0, 0, time.Local)
	now := midnight.Add(2 * time.Minute)

	// Session started yesterday 23:00: only the 2 minutes since midnight.
	if got := liveElapsedToday(midnight.Add(-time.Hour), now); got != 120 {
		t.Errorf("pre-midnight start = %d; want 120", got)
	}

	// Same-day session: the full elapsed time.
	now2 := midnight.Add(30 * time.Minute)
	if got := liveElapsedToday(now2.Add(-10*time.Minute), now2); got != 600 {
		t.Errorf("same-day start = %d; want 600", got)
	}
}

// TestGetAppUsageTodayMinutesMapFloorsOnce pins that stored and live seconds
// are summed before the minute conversion: flooring them separately can drop a
// minute the combined usage has crossed.
func TestGetAppUsageTodayMinutesMapFloorsOnce(t *testing.T) {
	setupStorageEnv(t)

	// Near local midnight the live 30 seconds can be clipped or land in the
	// previous day, which would flake the assertion for reasons unrelated to
	// flooring.
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	if now.Sub(midnight) < 2*time.Minute || nextMidnight.Sub(now) < 2*time.Minute {
		t.Skip("too close to local midnight; live elapsed time is clipped")
	}

	if _, err := db.Exec(`
		INSERT INTO apps_daily (date, app_name, exe_name, app_id, total_duration_secs, open_count)
		VALUES (?, 'Code', 'code.exe', 'code|aaaa1111', 90, 1)
	`, Today()); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	if err := SaveActiveSession(&ActiveSessionRecord{
		AppName:   "Code",
		ExeName:   "code.exe",
		AppID:     "code|aaaa1111",
		StartTime: time.Now().Add(-30 * time.Second),
		LastSeen:  time.Now(),
		Date:      Today(),
	}); err != nil {
		t.Fatalf("SaveActiveSession failed: %v", err)
	}

	got, err := GetAppUsageTodayMinutesMap()
	if err != nil {
		t.Fatalf("GetAppUsageTodayMinutesMap failed: %v", err)
	}
	// 90 stored + 30 live = 120s: two whole minutes, where 90/60 + 30/60
	// floors to one.
	if got["code|aaaa1111"] != 2 {
		t.Errorf("minutes = %d; want 2 (floor once on the combined seconds)", got["code|aaaa1111"])
	}
}

// TestGetAppUsageTodaySecondsForConfigMultibyteBase pins that the bare-name
// prefix match counts characters, not bytes: a CJK base name is longer in
// bytes than in characters, and a byte-counted substr misses every install.
func TestGetAppUsageTodaySecondsForConfigMultibyteBase(t *testing.T) {
	setupStorageEnv(t)

	if _, err := db.Exec(`
		INSERT INTO apps_daily (date, app_name, exe_name, app_id, total_duration_secs, open_count)
		VALUES (?, '记事本', '记事本.exe', '记事本|aaaa1111', 600, 1)
	`, Today()); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	if got := GetAppUsageTodaySecondsForConfig("记事本.exe"); got != 600 {
		t.Errorf("multibyte base usage = %d; want 600", got)
	}
}

// Site rows retain browser install identities even when their site names match.
func TestGetBrowserStatsInRangePreservesInstallIdentity(t *testing.T) {
	setupStorageEnv(t)

	if _, err := db.Exec(`
		INSERT INTO browsing_daily (date, app_id, site_name, total_duration_secs, open_count)
		VALUES (?, 'firefox|aaaa1111', 'GitHub', 600, 2), (?, 'chrome|bbbb2222', 'GitHub', 300, 1)
	`, Today(), Today()); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	stats, err := GetBrowserStatsInRange(Today(), Today())
	if err != nil {
		t.Fatalf("GetBrowserStatsInRange failed: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("rows = %d; want one per browser install and site", len(stats))
	}
	want := map[string]AppDailyStat{
		"firefox|aaaa1111": {Date: Today(), AppName: "GitHub", AppID: "firefox|aaaa1111", ExeName: "firefox.exe", TotalDurationSecs: 600, OpenCount: 2},
		"chrome|bbbb2222":  {Date: Today(), AppName: "GitHub", AppID: "chrome|bbbb2222", ExeName: "chrome.exe", TotalDurationSecs: 300, OpenCount: 1},
	}
	var seconds, opens int
	for _, stat := range stats {
		if stat != want[stat.AppID] {
			t.Errorf("stat = %+v; want %+v", stat, want[stat.AppID])
		}
		seconds += stat.TotalDurationSecs
		opens += stat.OpenCount
	}
	if seconds != 900 || opens != 3 {
		t.Errorf("combined total = %ds/%d opens; want 900s/3", seconds, opens)
	}
}

func TestGetBrowserStatsInRangeLiveMatchesInstallAndSite(t *testing.T) {
	setupStorageEnv(t)
	date := "2026-06-26"
	if _, err := db.Exec(`
		INSERT INTO browsing_daily (date, app_id, site_name, total_duration_secs, open_count)
		VALUES (?, 'chrome|first', 'GitHub', 600, 2), (?, 'chrome|second', 'GitHub', 300, 1),
		       (?, 'chrome|first', 'YouTube', 120, 1)
	`, date, date, date); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO apps_daily (date, app_name, exe_name, app_id)
		VALUES (?, 'Chrome', 'Chrome.exe', 'chrome|first')`, date); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 6, 26, 12, 0, 0, 0, time.Local)
	if err := SaveActiveSession(&ActiveSessionRecord{
		AppName: "Chrome", ExeName: "Chrome.exe", AppID: "chrome|first", SiteKnown: true, SiteName: "GitHub",
		StartTime: start, LastSeen: start.Add(time.Minute), Date: date,
	}); err != nil {
		t.Fatal(err)
	}
	stats, err := GetBrowserStatsInRange(date, date)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 3 {
		t.Fatalf("rows = %d; want three distinct install/site pairs: %+v", len(stats), stats)
	}
	want := map[string]AppDailyStat{
		"chrome|first/GitHub":  {Date: date, AppName: "GitHub", ExeName: "Chrome.exe", AppID: "chrome|first", TotalDurationSecs: 660, OpenCount: 3},
		"chrome|second/GitHub": {Date: date, AppName: "GitHub", ExeName: "chrome.exe", AppID: "chrome|second", TotalDurationSecs: 300, OpenCount: 1},
		"chrome|first/YouTube": {Date: date, AppName: "YouTube", ExeName: "Chrome.exe", AppID: "chrome|first", TotalDurationSecs: 120, OpenCount: 1},
	}
	for _, stat := range stats {
		if expected := want[stat.AppID+"/"+stat.AppName]; stat != expected {
			t.Errorf("stat = %+v; want %+v", stat, expected)
		}
	}
}

// TestSessionExistsMatchesIdentityAndStart pins the recovery de-dup lookup:
// only the same app identity at the same start second counts as a duplicate.
func TestSessionExistsMatchesIdentityAndStart(t *testing.T) {
	setupStorageEnv(t)

	start := time.Now().Add(-time.Hour).Truncate(time.Second)
	session := &Session{
		AppName:      "Code",
		ExeName:      "code",
		ExePath:      `C:\code.exe`,
		AppID:        "code|aaaa1111",
		WindowTitle:  "main.go",
		StartTime:    start,
		EndTime:      start.Add(10 * time.Minute),
		DurationSecs: 600,
		Date:         Today(),
	}
	if err := InsertSessionWithDaily(session, ""); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	exists, err := SessionExists(&Session{ExeName: "code", AppID: "code|aaaa1111", StartTime: start})
	if err != nil {
		t.Fatalf("SessionExists failed: %v", err)
	}
	if !exists {
		t.Error("the same identity and start time should match")
	}

	if exists, _ := SessionExists(&Session{ExeName: "code", AppID: "code|aaaa1111", StartTime: start.Add(time.Minute)}); exists {
		t.Error("a different start time must not match")
	}
	if exists, _ := SessionExists(&Session{ExeName: "code", AppID: "code|bbbb2222", StartTime: start}); exists {
		t.Error("a different identity must not match")
	}
	// A row without an app id resolves through the legacy key, which must not
	// accidentally match an install that has a real key.
	if exists, _ := SessionExists(&Session{ExeName: "code", StartTime: start}); exists {
		t.Error("the legacy identity must not match a hashed install")
	}
}
