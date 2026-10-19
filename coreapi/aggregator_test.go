package coreapi

import (
	"testing"
	"time"

	"focusd/storage"
)

// GetDailySummary sorts rows that tie on total duration; the tie order is
// pinned by name so the top lists never depend on row order coming out of
// SQLite, which is not part of the query contract.
func TestGetDailySummaryTieOrderIsStable(t *testing.T) {
	setupPomodoroEnv(t)

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

	for i := 0; i < 20; i++ {
		summary, err := GetDailySummary("2026-10-10")
		if err != nil {
			t.Fatalf("GetDailySummary: %v", err)
		}
		if len(summary.TopApps) != 2 || summary.TopApps[0].AppName != "Alpha" || summary.TopApps[1].AppName != "Beta" {
			t.Fatalf("call %d: TopApps tie order = %v; want Alpha then Beta", i, summary.TopApps)
		}
		if len(summary.TopSites) != 2 || summary.TopSites[0].AppName != "GitHub" || summary.TopSites[1].AppName != "YouTube" {
			t.Fatalf("call %d: TopSites tie order = %v; want GitHub then YouTube", i, summary.TopSites)
		}
	}
}
