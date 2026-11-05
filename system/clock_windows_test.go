package system

import (
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func usEasternTestZone() dynamicTimezoneInformation {
	return dynamicTimezoneInformation{
		Bias: 300, StandardBias: 0, DaylightBias: -60,
		StandardDate: windows.Systemtime{Month: 11, Day: 1, DayOfWeek: 0, Hour: 2},
		DaylightDate: windows.Systemtime{Month: 3, Day: 2, DayOfWeek: 0, Hour: 2},
	}
}

func TestLocalCalendarUsesRefreshedZoneRules(t *testing.T) {
	zone := dynamicTimezoneInformation{Bias: -330}
	at := time.Date(2026, 11, 21, 19, 0, 0, 0, time.UTC)
	local, err := localTimeInZone(at, &zone)
	if err != nil {
		t.Fatal(err)
	}
	if local.Format("2006-01-02 15:04") != "2026-11-22 00:30" {
		t.Fatal(local)
	}
	start, end, err := localDayBoundsInZone(local, &zone)
	if err != nil {
		t.Fatal(err)
	}
	if !start.Equal(time.Date(2026, 11, 21, 18, 30, 0, 0, time.UTC)) || end.Sub(start) != 24*time.Hour {
		t.Fatalf("bounds = %v .. %v", start, end)
	}
}

func TestLocalCalendarBoundsDSTDays(t *testing.T) {
	zone := usEasternTestZone()
	for _, tc := range []struct {
		date  string
		hours time.Duration
	}{{"2026-03-08", 23}, {"2026-11-01", 25}, {"2026-11-02", 24}} {
		local, _ := time.Parse("2006-01-02", tc.date)
		start, end, err := localDayBoundsInZone(local, &zone)
		if err != nil {
			t.Fatal(err)
		}
		if got := end.Sub(start); got != tc.hours*time.Hour {
			t.Errorf("%s duration = %s; want %dh", tc.date, got, tc.hours)
		}
	}
}

func TestClockFFIStructLayouts(t *testing.T) {
	if got := unsafe.Sizeof(dynamicTimezoneInformation{}); got != 432 {
		t.Errorf("dynamic timezone size = %d; want 432", got)
	}
	if got := unsafe.Sizeof(bootEnvironmentInformation{}); got != 32 {
		t.Errorf("boot information size = %d; want 32", got)
	}
}

func TestClockSampleReadOnlyObservation(t *testing.T) {
	first, err := ReadClockSample()
	if err != nil {
		t.Fatal(err)
	}
	second, err := ReadClockSample()
	if err != nil {
		t.Fatal(err)
	}
	if first.WallUTC.IsZero() || first.InterruptTicks == 0 || second.InterruptTicks < first.InterruptTicks || second.UnbiasedTicks < first.UnbiasedTicks {
		t.Fatalf("invalid clock samples: %+v then %+v", first, second)
	}
	if first.BootID != second.BootID {
		t.Errorf("boot ID changed without restart: %q -> %q", first.BootID, second.BootID)
	}
	t.Logf("boot identity available: %v, zone: %s", first.BootID != "", first.ZoneKey)
	if first.ZoneKey == "" {
		t.Error("current Windows timezone key missing")
	}
}
