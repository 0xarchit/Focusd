package storage

import (
	"testing"
	"time"
)

func setupLiveAccountingDB(t *testing.T) time.Time {
	t.Helper()
	setupStorageEnv(t)
	originalLocation := time.Local
	time.Local = time.FixedZone("AccountingTest", (12-time.Now().UTC().Hour())*3600)
	t.Cleanup(func() { time.Local = originalLocation })
	return time.Now().Truncate(time.Second)
}

func saveAccountingActiveSession(t *testing.T, start, lastSeen time.Time) {
	t.Helper()
	if err := SaveActiveSession(&ActiveSessionRecord{
		AppName: "Editor", ExeName: "editor.exe", AppID: "editor|abcd1234",
		WindowTitle: "Document", StartTime: start, LastSeen: lastSeen,
		Date: start.Format("2006-01-02"),
	}); err != nil {
		t.Fatalf("SaveActiveSession: %v", err)
	}
}

func TestStaleActiveSessionReadModelsStopAtHeartbeat(t *testing.T) {
	for _, view := range []string{"app stats", "exact app usage", "wildcard limit usage", "specific limit usage", "minute map"} {
		t.Run(view, func(t *testing.T) {
			now := setupLiveAccountingDB(t)
			if err := InsertSessionWithDaily(&Session{
				AppName: "Editor", ExeName: "editor.exe", AppID: "editor|abcd1234",
				StartTime: now.Add(-20 * time.Minute), EndTime: now.Add(-18 * time.Minute),
				DurationSecs: 120, Date: Today(),
			}, ""); err != nil {
				t.Fatalf("InsertSessionWithDaily: %v", err)
			}
			start := now.Add(-10 * time.Minute)
			saveAccountingActiveSession(t, start, start.Add(time.Minute))

			var got int
			want := 180
			switch view {
			case "app stats":
				stats, err := GetAppStatsInRange(Today(), Today())
				if err != nil {
					t.Fatalf("GetAppStatsInRange: %v", err)
				}
				for _, stat := range stats {
					got += stat.TotalDurationSecs
				}
			case "exact app usage":
				got = GetAppUsageTodaySeconds("editor|abcd1234")
			case "wildcard limit usage":
				got = GetAppUsageTodaySecondsForConfig("editor.exe")
			case "specific limit usage":
				got = GetAppUsageTodaySecondsForConfig("editor|abcd1234")
			case "minute map":
				usage, err := GetAppUsageTodayMinutesMap()
				if err != nil {
					t.Fatalf("GetAppUsageTodayMinutesMap: %v", err)
				}
				got, want = usage["editor|abcd1234"], 3
			}
			if got != want {
				t.Errorf("%s = %d; want %d from completed time plus the last observed active minute", view, got, want)
			}
		})
	}
}

func TestPreviousDayActiveSessionDoesNotAccrueToday(t *testing.T) {
	now := setupLiveAccountingDB(t)
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	start := midnight.Add(-10 * time.Minute)
	saveAccountingActiveSession(t, start, start.Add(time.Minute))

	stats, err := GetAppStatsInRange(Today(), Today())
	if err != nil {
		t.Fatalf("GetAppStatsInRange: %v", err)
	}
	if len(stats) != 0 {
		t.Errorf("today has %d app rows from a checkpoint last observed yesterday; want none", len(stats))
	}
	if got := GetAppUsageTodaySeconds("editor|abcd1234"); got != 0 {
		t.Errorf("exact usage = %d; want no usage after yesterday's heartbeat", got)
	}
	if got := GetAppUsageTodaySecondsForConfig("editor"); got != 0 {
		t.Errorf("wildcard usage = %d; want no usage after yesterday's heartbeat", got)
	}
	usage, err := GetAppUsageTodayMinutesMap()
	if err != nil {
		t.Fatalf("GetAppUsageTodayMinutesMap: %v", err)
	}
	if got := usage["editor|abcd1234"]; got != 0 {
		t.Errorf("minute map = %d; want no usage after yesterday's heartbeat", got)
	}
}

func TestHourlyUsageIncludesBoundedActiveSessionAcrossHours(t *testing.T) {
	now := setupLiveAccountingDB(t)
	start := time.Date(now.Year(), now.Month(), now.Day(), now.Hour()-2, 30, 0, 0, time.Local)
	lastSeen := start.Add(45 * time.Minute)
	saveAccountingActiveSession(t, start, lastSeen)

	buckets, err := GetHourlyUsageToday(Today())
	if err != nil {
		t.Fatalf("GetHourlyUsageToday: %v", err)
	}
	var want [24]int
	want[start.Hour()] = 1800
	want[lastSeen.Hour()] = 900
	if buckets != want {
		t.Errorf("hourly buckets = %v; want %v through the heartbeat only", buckets, want)
	}
}

func TestHourlyUsageDoesNotDoubleCountActiveToCompletedTransition(t *testing.T) {
	now := setupLiveAccountingDB(t)
	start := now.Add(-10 * time.Minute)
	end := start.Add(time.Minute)
	saveAccountingActiveSession(t, start, end)
	before, err := GetHourlyUsageToday(Today())
	if err != nil {
		t.Fatalf("GetHourlyUsageToday before transition: %v", err)
	}
	if err := InsertSessionWithDaily(&Session{
		AppName: "Editor", ExeName: "editor.exe", AppID: "editor|abcd1234",
		StartTime: start, EndTime: end, DurationSecs: 60, Date: Today(),
	}, ""); err != nil {
		t.Fatalf("InsertSessionWithDaily: %v", err)
	}
	if err := ClearActiveSession(); err != nil {
		t.Fatalf("ClearActiveSession: %v", err)
	}
	after, err := GetHourlyUsageToday(Today())
	if err != nil {
		t.Fatalf("GetHourlyUsageToday after transition: %v", err)
	}
	if before != after {
		t.Errorf("active-to-completed handoff changed hourly totals: before %v, after %v", before, after)
	}
	total := 0
	for _, seconds := range after {
		total += seconds
	}
	if total != 60 {
		t.Errorf("completed hourly total = %d; want exactly one observed minute", total)
	}
}
