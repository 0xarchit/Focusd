package coreapi

import (
	"testing"

	"focusd/storage"
)

func TestSiteNameForTitles(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"GitHub", "GitHub"},
		{"YouTube", "YouTube"},
		{"LeetCode - Two Sum", "LeetCode"},
		{"Inbox (3) - Gmail", "Gmail"},
		{"Pull requests · GitHub", "GitHub"},
		{"Claude", "Claude"},
		{"Netflix", "Netflix"},
		{"Visual Studio Code", "VS Code"},
		{"MDN Web Docs", "MDN"},
		{"Stack Overflow", "Stack Overflow"},
		{"Microsoft Teams", "Microsoft Teams"},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := SiteNameFor(tc.in); got != tc.want {
				t.Errorf("SiteNameFor(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSiteNameForDomains(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"github.com", "GitHub"},
		{"https://github.com/0xarchit/focusd", "GitHub"},
		{"gist.github.com", "GitHub"},
		{"www.youtube.com", "YouTube"},
		{"music.youtube.com", "YouTube"},
		{"mail.google.com", "Gmail"},
		{"x.com", "Twitter/X"},
		{"https://x.com/user/status/1", "Twitter/X"},
		{"mail.google.co.uk", ""},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := SiteNameFor(tc.in); got != tc.want {
				t.Errorf("SiteNameFor(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestSiteNameForPrefersTheMostSpecificDomain pins the domain-first pass:
// amazon.com must stay Amazon while its consoles keep their own names, and a
// title match from an earlier rule must not steal a longer domain match from a
// later rule.
func TestSiteNameForPrefersTheMostSpecificDomain(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"https://www.amazon.com/dp/B0CHX1W1XY", "Amazon"},
		{"primevideo.com", "Prime Video"},
		{"Prime Video", "Prime Video"},
		{"aws.amazon.com", "AWS"},
		{"console.aws.amazon.com", "AWS"},
		{"cloud.google.com", "Google Cloud"},
		{"console.cloud.google.com", "Google Cloud"},
		{"youtube.github.io", "GitHub"},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := SiteNameFor(tc.in); got != tc.want {
				t.Errorf("SiteNameFor(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestSiteNameForNoFalsePositives checks that ordinary English words do not
// resolve to a site.
func TestSiteNameForNoFalsePositives(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"Expense Report - Xylophone", ""},
		{"Report | Xerox", ""},
		{"Meeting with the team", ""},
		{"Inbox Cleanup Guide", ""},
		{"Boxing practice", ""},
		{"Xbox settings", ""},
		{"Zoom out shortcut", ""},
		{"Arcadia Rising", ""},
		{"Code review checklist", ""},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := SiteNameFor(tc.in); got != tc.want {
				t.Errorf("SiteNameFor(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestDomainMatchesHost(t *testing.T) {
	tests := []struct {
		host   string
		domain string
		want   bool
	}{
		{"youtube.com", "youtube.com", true},
		{"www.youtube.com", "youtube.com", true},
		{"music.youtube.com", "youtube.com", true},
		{"youtube.com.", "youtube.com", true},
		{"notyoutube.com", "youtube.com", false},
		{"youtube.com.evil.net", "youtube.com", false},
		{"", "youtube.com", false},
	}

	for _, tc := range tests {
		t.Run(tc.host+"|"+tc.domain, func(t *testing.T) {
			if got := domainMatchesHost(tc.host, tc.domain); got != tc.want {
				t.Errorf("domainMatchesHost(%q, %q) = %v; want %v", tc.host, tc.domain, got, tc.want)
			}
		})
	}
}

func TestDomainOf(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"https://github.com/user/repo", "github.com"},
		{"github.com", "github.com"},
		{"github.com/", "github.com"},
		{"github.com:443/path", "github.com"},
		{"  WWW.YouTube.com  ", "www.youtube.com"},
		{"", ""},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := domainOf(tc.in); got != tc.want {
				t.Errorf("domainOf(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestCollapseURLTitle(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"bing.com/ck/a?!&&p=f7e19469", "bing.com"},
		{"https://www.bing.com/search?q=focusd", "www.bing.com"},
		{"github.com/0xarchit/focusd", "github.com"},
		{"mail.google.com/mail/u/0/#inbox", "mail.google.com"},
		{"Example Domain", "Example Domain"},
		{"Docs / Home", "Docs / Home"},
		{"about:blank", "about:blank"},
		{"localhost:3000/dashboard", "localhost:3000/dashboard"},
		{"192.168.1.1/admin", "192.168.1.1/admin"},
		{"TCP/IP", "TCP/IP"},
		{"", ""},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := CollapseURLTitle(tc.in); got != tc.want {
				t.Errorf("CollapseURLTitle(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSiteOverride(t *testing.T) {
	ClearSiteOverrides()
	t.Cleanup(ClearSiteOverrides)

	if got := SiteNameFor("GitHub"); got != "GitHub" {
		t.Fatalf("baseline = %q; want GitHub", got)
	}

	SetSiteOverride("GitHub", "Work GitHub")
	if got := SiteNameFor("GitHub"); got != "Work GitHub" {
		t.Errorf("override not applied: got %q", got)
	}

	// Overriding an uncatalogued title works too.
	SetSiteOverride("My Wiki", "Team Wiki")
	if got := SiteNameFor("My Wiki"); got != "Team Wiki" {
		t.Errorf("uncatalogued override not applied: got %q", got)
	}

	SetSiteOverride("GitHub", "")
	if got := SiteNameFor("GitHub"); got != "GitHub" {
		t.Errorf("override removal failed: got %q", got)
	}
}

// TestSiteOverrideNormalizesKey pins that an override key is matched
// case-insensitively and whitespace-insensitively, because the lookup lowercases
// the site name.
func TestSiteOverrideNormalizesKey(t *testing.T) {
	ClearSiteOverrides()
	t.Cleanup(ClearSiteOverrides)

	SetSiteOverride("  GitHub ", "Work GitHub")
	for _, in := range []string{"GitHub", "github", "GITHUB", "  github  "} {
		if got := SiteNameFor(in); got != "Work GitHub" {
			t.Errorf("SiteNameFor(%q) = %q; want the override", in, got)
		}
	}

	// Removal uses the same normalized key.
	SetSiteOverride("GitHub", "")
	if got := SiteNameFor("GitHub"); got != "GitHub" {
		t.Errorf("override removal failed: got %q", got)
	}
}

func TestExtractAppCategoryIdleTitles(t *testing.T) {
	for _, title := range []string{"New Tab", "new tab", "Mozilla Firefox"} {
		if got := extractAppCategory(title); got != "Browser (Idle)" {
			t.Errorf("extractAppCategory(%q) = %q; want Browser (Idle)", title, got)
		}
	}
	if got := extractAppCategory(""); got != "" {
		t.Errorf("extractAppCategory(\"\") = %q; want empty", got)
	}
}

// TestGroupBrowserStatsKeepsEverything checks the rollup conserves total time
// and separates catalogued sites from uncatalogued page titles.
func TestGroupBrowserStatsKeepsEverything(t *testing.T) {
	stats := []storage.AppDailyStat{
		{AppName: "GitHub", TotalDurationSecs: 600},
		{AppName: "gitlab", TotalDurationSecs: 300},
		{AppName: "Some Random Page", TotalDurationSecs: 120},
	}

	groups := groupBrowserStats(stats)

	total := 0
	byCategory := map[string]int{}
	for _, g := range groups {
		total += g.TotalSecs
		byCategory[g.Category] = g.TotalSecs
	}

	if total != 1020 {
		t.Errorf("total = %d; want 1020, rollup must not lose time", total)
	}
	if byCategory["GitHub"] != 600 {
		t.Errorf("GitHub group = %d; want 600", byCategory["GitHub"])
	}
	if byCategory["GitLab"] != 300 {
		t.Errorf("GitLab group = %d; want 300", byCategory["GitLab"])
	}
	if byCategory["Some Random Page"] != 120 {
		t.Errorf("uncatalogued title should stand alone, got %d", byCategory["Some Random Page"])
	}

	// The uncatalogued entry is its own group with no sub-entries.
	for _, g := range groups {
		if g.Category == "Some Random Page" && len(g.SubEntries) != 0 {
			t.Errorf("an uncatalogued title should not gain sub-entries: %+v", g.SubEntries)
		}
	}
}

func TestGroupBrowserStatsEmpty(t *testing.T) {
	if groups := groupBrowserStats(nil); len(groups) != 0 {
		t.Errorf("expected no groups, got %d", len(groups))
	}
}
