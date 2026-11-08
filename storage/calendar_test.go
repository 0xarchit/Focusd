package storage

import (
	"errors"
	"testing"
	"time"
	_ "time/tzdata"
)

// Existing pure fixtures control time.Local without changing the host timezone.
func init() {
	calendarLocalTime = func(at time.Time) (time.Time, error) { return at.In(time.Local), nil }
	calendarDayBounds = func(at time.Time) (time.Time, time.Time, error) {
		day := startOfLocalDay(at.In(time.Local))
		return day, day.AddDate(0, 0, 1), nil
	}
}

func useTestCalendar(t *testing.T, at time.Time, location *time.Location) {
	t.Helper()
	previousNow, previousLocal, previousBounds := calendarNow, calendarLocalTime, calendarDayBounds
	calendarNow = func() time.Time { return at }
	calendarLocalTime = func(at time.Time) (time.Time, error) { return at.In(location), nil }
	calendarDayBounds = func(at time.Time) (time.Time, time.Time, error) {
		day := startOfLocalDay(at.In(location))
		return day, day.AddDate(0, 0, 1), nil
	}
	t.Cleanup(func() { calendarNow, calendarLocalTime, calendarDayBounds = previousNow, previousLocal, previousBounds })
}

func TestCalendarTodayAndActiveUsageFollowChangedZone(t *testing.T) {
	setupStorageEnv(t)
	zone := time.FixedZone("changed-zone", 14*3600)
	at := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	useTestCalendar(t, at, zone)
	if got := Today(); got != "2026-11-23" {
		t.Fatalf("changed-zone today = %s; want 2026-11-23", got)
	}
	start := time.Date(2026, 11, 22, 9, 30, 0, 0, time.UTC)
	if err := SaveActiveSession(&ActiveSessionRecord{
		AppName: "Editor", ExeName: "editor.exe", AppID: "editor|zone", SiteKnown: true, SiteName: "GitHub",
		StartTime: start, LastSeen: start.Add(time.Hour), Date: "2026-11-22",
	}); err != nil {
		t.Fatal(err)
	}
	for _, date := range []string{"2026-11-22", "2026-11-23"} {
		stats, err := GetAppStatsInRange(date, date)
		if err != nil || len(stats) != 1 || stats[0].Date != date || stats[0].TotalDurationSecs != 1800 {
			t.Errorf("app day %s = %+v, %v; want 1800s", date, stats, err)
		}
		sites, err := GetBrowserStatsInRange(date, date)
		if err != nil || len(sites) != 1 || sites[0].Date != date || sites[0].TotalDurationSecs != 1800 {
			t.Errorf("site day %s = %+v, %v; want 1800s", date, sites, err)
		}
		hours, err := GetHourlyUsageToday(date)
		wantHour := 23
		if date == "2026-11-23" {
			wantHour = 0
		}
		var total int
		for _, seconds := range hours {
			total += seconds
		}
		if err != nil || hours[wantHour] != 1800 || total != 1800 {
			t.Errorf("hourly day %s = %+v, %v; want hour %d with 1800s", date, hours, err, wantHour)
		}
	}
}

func TestCalendarDayBoundsPreserveDSTDayLength(t *testing.T) {
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip(err)
	}
	useTestCalendar(t, time.Time{}, zone)
	for _, fixture := range []struct {
		date  string
		hours int
	}{{"2026-03-08", 23}, {"2026-11-01", 25}} {
		start, end, err := calendarBoundsForDate(fixture.date)
		if err != nil || end.Sub(start) != time.Duration(fixture.hours)*time.Hour {
			t.Errorf("bounds %s = %s to %s, %v; want %dh", fixture.date, start, end, err, fixture.hours)
		}
	}
}

func TestGuestCalendarRetentionUsesChangedDateAndSkipsOnFailure(t *testing.T) {
	requireDestructiveGuest(t)
	setupStorageEnv(t)
	useTestCalendar(t, time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC), time.FixedZone("changed-zone", 14*3600))
	if err := SetRetentionDays(7); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO apps_daily(date,app_name,exe_name,app_id,total_duration_secs)
		VALUES ('2026-11-16','Editor','editor.exe','editor|zone',60), ('2026-11-17','Editor','editor.exe','editor|zone',60)`); err != nil {
		t.Fatal(err)
	}
	EnforceRetention()
	var date string
	if err := db.QueryRow(`SELECT date FROM apps_daily`).Scan(&date); err != nil || date != "2026-11-17" {
		t.Errorf("changed-date retention kept %s, %v; want 2026-11-17", date, err)
	}
	calendarLocalTime = func(time.Time) (time.Time, error) { return time.Time{}, errors.New("calendar unavailable") }
	if got := Today(); got != "0000-00-00" {
		t.Errorf("failure today = %s", got)
	}
	if _, err := db.Exec(`INSERT INTO apps_daily(date,app_name,exe_name,app_id,total_duration_secs) VALUES ('2000-01-01','Old','old.exe','old|zone',60)`); err != nil {
		t.Fatal(err)
	}
	EnforceRetention()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM apps_daily`).Scan(&count); err != nil || count != 2 {
		t.Errorf("calendar failure pruned history: %d rows, %v", count, err)
	}
}
