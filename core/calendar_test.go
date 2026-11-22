package core

import (
	"fmt"
	"focusd/storage"
	"focusd/system"
	"testing"
	"time"
	_ "time/tzdata"
)

func TestSessionCalendarCapturesZoneAndPreservesAttributedDate(t *testing.T) {
	old := locationCalendar(time.FixedZone("old", 0))
	newCalendar := locationCalendar(time.FixedZone("new", 2*3600))
	start := time.Date(2026, 11, 21, 23, 30, 0, 0, time.UTC)
	session := &activeSession{SessionID: "session", StartTime: start, Date: "2026-11-21", calendar: old}
	slices, err := splitSessionWithCalendar(session, start.Add(time.Hour), old)
	if err != nil {
		t.Fatal(err)
	}
	if len(slices) != 2 || slices[0].Date != "2026-11-21" || slices[1].Date != "2026-11-22" || slices[0].DurationSecs != 1800 {
		t.Fatalf("old timezone interval relabeled: %+v", slices)
	}
	date, err := newCalendar.date(start)
	if err != nil || date != "2026-11-22" {
		t.Fatalf("new observed date=%s err=%v", date, err)
	}
}

func TestSessionCalendarDSTKeepsActualDayLength(t *testing.T) {
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	calendar := locationCalendar(zone)
	start := time.Date(2026, 3, 8, 0, 0, 0, 0, zone)
	end := start.AddDate(0, 0, 1)
	slices, err := splitSessionWithCalendar(&activeSession{StartTime: start, Date: "2026-03-08"}, end, calendar)
	if err != nil {
		t.Fatal(err)
	}
	if len(slices) != 1 || slices[0].DurationSecs != 23*3600 {
		t.Fatalf("DST day incorrectly billed: %+v", slices)
	}
}

func TestControllerZoneChangeClosesUnderCapturedCalendar(t *testing.T) {
	c := testController(t)
	start := time.Date(2026, 11, 21, 23, 30, 0, 0, time.UTC)
	initial := system.ClockSample{WallUTC: start.Add(15 * time.Minute), InterruptTicks: 100000000, UnbiasedTicks: 100000000, BootID: "boot", ZoneKey: "old"}
	if _, err := c.guard.Observe(initial, true); err != nil {
		t.Fatal(err)
	}
	c.sample = initial
	c.tracker.currentSession = &activeSession{SessionID: "session", SliceID: sliceID("session", start), AppName: "App", ExeName: "app.exe", StartTime: start, Date: "2026-11-21", calendar: locationCalendar(time.UTC), SiteKnown: true}
	changed := initial
	changed.WallUTC = changed.WallUTC.Add(time.Second)
	changed.InterruptTicks += 10000000
	changed.UnbiasedTicks += 10000000
	changed.ZoneKey = "new"
	changed.OffsetSeconds = 2 * 3600
	c.readClockSample = func() (system.ClockSample, error) { return changed, nil }
	c.observe(false)
	if c.guarded || c.tracker.currentSession != nil {
		t.Fatal("ordinary timezone change did not safely rotate interval")
	}
	sessions, err := storage.GetSessionsPaginated(10, 0, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].Date != "2026-11-21" || sessions[0].DurationSecs != 900 {
		t.Fatalf("old interval relabeled: %+v", sessions)
	}
}

func TestCalendarConversionFailureGuardsCollection(t *testing.T) {
	c := testController(t)
	c.tracker.calendarFactory = func() (*sessionCalendar, error) { return nil, fmt.Errorf("conversion unavailable") }
	c.tracker.applyPoll(&pollObservation{exeName: "app.exe", appID: "app|test", title: "Window"})
	if c.tracker.currentSession != nil {
		t.Fatal("failed conversion started guessed-date interval")
	}
	if !c.checkCalendarFault() || !c.guarded || c.guard.State().Incident == nil {
		t.Fatal("conversion failure did not guard collection")
	}
}
