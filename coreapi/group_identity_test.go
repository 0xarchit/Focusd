package coreapi

import (
	"testing"

	"focusd/storage"
)

func TestBrowserGroupsPreserveInstallIdentity(t *testing.T) {
	groups := groupBrowserStats([]storage.AppDailyStat{
		{AppID: "firefox|work", AppName: "GitHub", TotalDurationSecs: 10},
		{AppID: "firefox|personal", AppName: "GitHub", TotalDurationSecs: 20},
		{AppID: "firefox|work", AppName: "Pull request - GitHub", TotalDurationSecs: 30},
	})
	if len(groups) != 2 {
		t.Fatalf("groups = %+v; want distinct installs", groups)
	}
	totals := make(map[string]int)
	for _, group := range groups {
		if group.Category != "GitHub" {
			t.Fatalf("category = %q", group.Category)
		}
		totals[group.AppID] = group.TotalSecs
	}
	if totals["firefox|work"] != 40 || totals["firefox|personal"] != 20 {
		t.Fatalf("group totals = %+v", totals)
	}
}
