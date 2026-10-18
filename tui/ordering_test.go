package tui

import (
	"testing"
	"time"

	"focusd/storage"
)

// seedEqualStats points APPDATA at a temp dir, opens a sandbox database, and
// seeds two apps (and two browsing sites) that tie on total duration, so the
// duration sort has to fall back to its tie-break on every comparison.
func seedEqualStats(t *testing.T) {
	t.Helper()
	t.Setenv("APPDATA", t.TempDir())
	storage.Close()
	if err := storage.Init(); err != nil {
		t.Fatalf("storage init: %v", err)
	}
	t.Cleanup(func() { storage.Close() })

	start := time.Date(2026, 10, 10, 12, 0, 0, 0, time.Local)
	seed := func(app, exe, appID, site string) {
		s := &storage.Session{
			AppName:      app,
			ExeName:      exe,
			AppID:        appID,
			StartTime:    start,
			EndTime:      start.Add(300 * time.Second),
			DurationSecs: 300,
			Date:         "2026-10-10",
		}
		if err := storage.InsertSessionWithDaily(s, site); err != nil {
			t.Fatalf("seed session %s: %v", app, err)
		}
	}
	seed("Alpha", "alpha.exe", "alpha|0", "GitHub")
	seed("Beta", "beta.exe", "beta|0", "YouTube")
}

func statNames(stats []storage.AppDailyStat) []string {
	names := make([]string, 0, len(stats))
	for _, s := range stats {
		names = append(names, s.AppName)
	}
	return names
}

// The stats aggregators merge rows through a map, so equal totals used to be
// emitted in the map's randomized order and the visible list reshuffled
// between refreshes. The tie-break must keep the order stable across calls.
func TestAggregateStatsTieOrderIsStable(t *testing.T) {
	seedEqualStats(t)

	for i := 0; i < 20; i++ {
		apps, err := aggregateAppStats("2026-10-10", "2026-10-10")
		if err != nil {
			t.Fatalf("aggregateAppStats: %v", err)
		}
		if len(apps) != 2 || apps[0].AppName != "Alpha" || apps[1].AppName != "Beta" {
			t.Fatalf("call %d: app tie order = %v; want Alpha then Beta", i, statNames(apps))
		}
		if apps[0].TotalDurationSecs != 300 {
			t.Fatalf("aggregateAppStats merged %d seconds for Alpha; want 300", apps[0].TotalDurationSecs)
		}

		sites, err := aggregateBrowserStats("2026-10-10", "2026-10-10")
		if err != nil {
			t.Fatalf("aggregateBrowserStats: %v", err)
		}
		if len(sites) != 2 || sites[0].AppName != "GitHub" || sites[1].AppName != "YouTube" {
			t.Fatalf("call %d: site tie order = %v; want GitHub then YouTube", i, statNames(sites))
		}
	}
}
