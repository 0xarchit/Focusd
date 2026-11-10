package coreapi

import (
	"reflect"
	"testing"

	"focusd/storage"
	"focusd/system"
)

func TestDisplayOverridesApplyToCopiedHistoricalRows(t *testing.T) {
	setupPomodoroEnv(t)
	stats := []storage.AppDailyStat{
		{AppID: "firefox|work", AppName: "Firefox", TotalDurationSecs: 10},
		{AppID: "firefox|personal", AppName: "Firefox", TotalDurationSecs: 20},
	}
	original := append([]storage.AppDailyStat(nil), stats...)
	if err := system.SetDisplayOverride("firefox|work", "Work Browser"); err != nil {
		t.Fatal(err)
	}
	got := ApplyDisplayOverrides(stats)
	if got[0].AppName != "Work Browser" || got[1].AppName != "Firefox" || !reflect.DeepEqual(stats, original) {
		t.Fatalf("override result = %+v, original = %+v", got, stats)
	}
	if err := system.SetDisplayOverride("firefox|work", ""); err != nil {
		t.Fatal(err)
	}
	if got := ApplyDisplayOverrides(stats); !reflect.DeepEqual(got, original) {
		t.Fatalf("removed override result = %+v; want %+v", got, original)
	}
}

func TestConfiguredSiteOverridesReachLookupAndReadModel(t *testing.T) {
	setupPomodoroEnv(t)
	ClearSiteOverrides()
	t.Cleanup(ClearSiteOverrides)
	if err := system.SetSiteOverride("GitHub", "Work GitHub"); err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"GitHub", "github.com", "Pull request - GitHub"} {
		if got := SiteNameFor(title); got != "Work GitHub" {
			t.Errorf("SiteNameFor(%q) = %q", title, got)
		}
	}
	stats := []storage.AppDailyStat{{AppName: "GitHub", TotalDurationSecs: 50}}
	got := ApplySiteOverrides(stats)
	if got[0].AppName != "Work GitHub" || stats[0].AppName != "GitHub" {
		t.Fatalf("read result = %+v; original = %+v", got, stats)
	}
	if got := CanonicalSiteNameFor("GitHub"); got != "GitHub" {
		t.Fatalf("canonical classification must not persist user label; got %q", got)
	}
	if err := system.SetSiteOverride("GitHub", ""); err != nil {
		t.Fatal(err)
	}
	if got := SiteNameFor("GitHub"); got != "GitHub" {
		t.Fatalf("removed override result = %q", got)
	}
}

func TestDistinctInstallLabelsKeepIdentityBeforeLongNames(t *testing.T) {
	stats := []storage.AppDailyStat{
		{AppName: "A page title long enough to hide a trailing install label", AppID: "firefox|work"},
		{AppName: "A page title long enough to hide a trailing install label", AppID: "firefox|personal"},
		{AppName: "Chrome", AppID: "chrome|work"},
		{ExeName: "notepad.exe", AppID: "notepad|work"},
	}
	original := append([]storage.AppDailyStat(nil), stats...)
	got := DistinctInstallLabels(stats)
	want := []string{
		"[firefox|work] A page title long enough to hide a trailing install label",
		"[firefox|personal] A page title long enough to hide a trailing install label",
		"Chrome", "Notepad",
	}
	if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(stats, original) {
		t.Fatalf("labels = %q; original = %+v", got, stats)
	}
	if got := DistinctInstallLabels([]storage.AppDailyStat{{AppName: "GitHub", AppID: "firefox|work"}, {AppName: "GitHub", AppID: "firefox|work"}}); got[0] != "GitHub" || got[1] != "GitHub" {
		t.Fatalf("same install acquired collision labels: %q", got)
	}
}
