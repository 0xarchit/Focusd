package storage

import (
	"testing"
	"time"
)

func TestSplitSessionHoursSingle(t *testing.T) {
	start := time.Date(2026, 6, 26, 14, 30, 0, 0, time.Local).Unix()
	end := start + 600

	got := splitSessionHours(start, end)
	if len(got) != 1 {
		t.Fatalf("expected one hour bucket, got %v", got)
	}
	if got[14] != 600 {
		t.Errorf("hour 14 = %d; want 600", got[14])
	}
}

func TestSplitSessionHoursCrossesHour(t *testing.T) {
	start := time.Date(2026, 6, 26, 14, 30, 0, 0, time.Local).Unix()
	end := start + 3600

	got := splitSessionHours(start, end)

	if len(got) != 2 {
		t.Fatalf("expected two hour buckets, got %v", got)
	}
	if got[14] != 1800 {
		t.Errorf("hour 14 = %d; want 1800 (30 minutes remaining)", got[14])
	}
	if got[15] != 1800 {
		t.Errorf("hour 15 = %d; want 1800 (30 minutes elapsed)", got[15])
	}
}

// TestSplitSessionHoursLongSession checks that a three hour session cannot put
// more than sixty minutes into any one bar.
func TestSplitSessionHoursLongSession(t *testing.T) {
	start := time.Date(2026, 6, 26, 10, 30, 0, 0, time.Local).Unix()
	end := time.Date(2026, 6, 26, 13, 30, 0, 0, time.Local).Unix()

	got := splitSessionHours(start, end)

	total := 0
	for hour, secs := range got {
		if secs > 3600 {
			t.Errorf("hour %d holds %d seconds, which cannot fit in one hour", hour, secs)
		}
		total += secs
	}
	if total != int(end-start) {
		t.Errorf("total = %d; want %d", total, end-start)
	}
	if len(got) != 4 {
		t.Errorf("expected 4 hour buckets, got %v", got)
	}
}

func TestSplitSessionHoursOpenEnded(t *testing.T) {
	start := time.Date(2026, 6, 26, 22, 0, 0, 0, time.Local).Unix()

	// end equal to start contributes nothing rather than a negative span.
	if got := splitSessionHours(start, start); len(got) != 0 {
		t.Errorf("expected no buckets for a zero-length session, got %v", got)
	}
	if got := splitSessionHours(start, start-60); len(got) != 0 {
		t.Errorf("expected no buckets for an inverted session, got %v", got)
	}
}

func TestSplitSessionHoursSpansMidnight(t *testing.T) {
	start := time.Date(2026, 6, 26, 23, 30, 0, 0, time.Local).Unix()
	end := time.Date(2026, 6, 27, 0, 30, 0, 0, time.Local).Unix()

	got := splitSessionHours(start, end)

	if got[23] != 1800 {
		t.Errorf("hour 23 = %d; want 1800", got[23])
	}
	if got[0] != 1800 {
		t.Errorf("hour 0 = %d; want 1800", got[0])
	}
}

// TestGetHourlyUsageTodayKeepsEverySession checks the row cap is gone: more
// sessions than the old 500 limit must all be counted.
func TestGetHourlyUsageTodayKeepsEverySession(t *testing.T) {
	setupStorageEnv(t)

	const date = "2026-06-26"
	base := time.Date(2026, 6, 26, 9, 0, 0, 0, time.Local).Unix()

	const sessions = 600
	for i := 0; i < sessions; i++ {
		start := base + int64(i)*10
		end := start + 10
		s := &Session{
			AppName:      "App",
			ExeName:      "app.exe",
			AppID:        "app|12345678",
			StartTime:    time.Unix(start, 0),
			EndTime:      time.Unix(end, 0),
			DurationSecs: 10,
			Date:         date,
		}
		if err := InsertSessionWithDaily(s, ""); err != nil {
			t.Fatalf("insert %d failed: %v", i, err)
		}
	}

	buckets, err := GetHourlyUsageToday(date)
	if err != nil {
		t.Fatalf("GetHourlyUsageToday failed: %v", err)
	}

	total := 0
	for _, secs := range buckets {
		if secs > 3600 {
			t.Errorf("an hour bucket holds %d seconds, which cannot fit in one hour", secs)
		}
		total += secs
	}
	if total != sessions*10 {
		t.Errorf("total = %d; want %d, sessions were truncated", total, sessions*10)
	}
}

func TestGetHourlyUsageTodayEmpty(t *testing.T) {
	setupStorageEnv(t)

	buckets, err := GetHourlyUsageToday("2026-01-01")
	if err != nil {
		t.Fatalf("GetHourlyUsageToday failed: %v", err)
	}
	for hour, secs := range buckets {
		if secs != 0 {
			t.Errorf("hour %d = %d; want 0 for an empty day", hour, secs)
		}
	}
}

// TestSplitSessionHoursDSTFallBack pins that a session crossing a DST
// fall-back still terminates: the second occurrence of the repeated hour
// re-resolves to its first start, and without a forward-progress clamp the
// cursor never advanced (an infinite loop that hung the hourly view).
func TestSplitSessionHoursDSTFallBack(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("zoneinfo unavailable: %v", err)
	}
	old := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = old })

	// 2026-11-01: 01:00-02:00 EDT repeats as 01:00-02:00 EST.
	start := time.Date(2026, 11, 1, 0, 30, 0, 0, loc)
	end := start.Add(2 * time.Hour)

	buckets := splitSessionHours(start.Unix(), end.Unix())

	total := 0
	for _, secs := range buckets {
		total += secs
	}
	if want := int(end.Sub(start).Seconds()); total != want {
		t.Errorf("bucket total = %d; want %d (no time lost, no hang)", total, want)
	}
}
