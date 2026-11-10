package core

import "testing"

func TestClassifyObservationKeepsPrivateAppTimeWithoutSite(t *testing.T) {
	for _, mode := range []string{"titles", "categories", "strict"} {
		for _, title := range []string{
			"Bank - Private Browsing - Mozilla Firefox",
			"Bank - Google Chrome (Incognito)",
		} {
			got := classifyObservation("firefox.exe", title, mode, true, false)
			if !got.Private || !got.Browser || got.StoredTitle != "[redacted]" || got.SiteName != "" {
				t.Errorf("mode %s, title %q: %+v", mode, title, got)
			}
		}
	}
}

func TestClassifyObservationDoesNotTreatNonBrowserAsPrivate(t *testing.T) {
	title := "test-private.txt - Visual Studio Code"
	got := classifyObservation("code.exe", title, "titles", false, false)
	if got.Private || got.Browser || got.StoredTitle != title || got.SiteName != "" {
		t.Fatalf("non-browser observation = %+v", got)
	}
	got = classifyObservation("code.exe", title, "strict", false, false)
	if got.Private || got.StoredTitle != "[redacted]" {
		t.Fatalf("strict non-browser observation = %+v", got)
	}
}

func TestClassifyObservationSeparatesPrivacyModes(t *testing.T) {
	tests := []struct {
		mode, title, stored, site string
	}{
		{"titles", "Pull request - GitHub - Google Chrome", "Pull request - GitHub - Google Chrome", "Pull request - GitHub"},
		{"categories", "Pull request - GitHub - Google Chrome", "[redacted]", "GitHub"},
		{"categories", "Secret document on unknown host - Google Chrome", "[redacted]", "Unknown Site"},
		{"strict", "Pull request - GitHub - Google Chrome", "[redacted]", ""},
		{"invalid", "Pull request - GitHub - Google Chrome", "[redacted]", ""},
	}
	for _, tc := range tests {
		got := classifyObservation("chrome.exe", tc.title, tc.mode, true, false)
		if got.StoredTitle != tc.stored || got.SiteName != tc.site || got.Private {
			t.Errorf("%s: %+v; want stored %q, site %q", tc.mode, got, tc.stored, tc.site)
		}
	}
}

func TestClassifyObservationTorHasNoPageOrSite(t *testing.T) {
	for _, title := range []string{"Secret Page — Tor Browser", ""} {
		got := classifyObservation("firefox.exe", title, "titles", true, true)
		if !got.Private || got.StoredTitle != "[redacted]" || got.SiteName != "" {
			t.Fatalf("Tor observation = %+v", got)
		}
	}
}

func TestTorBrowserIdentityUsesExactPathComponents(t *testing.T) {
	tests := []struct {
		exe, path string
		want      bool
	}{
		{"torbrowser.exe", "", true},
		{"tor.exe", "", true},
		{"firefox.exe", `C:\Users\User\Desktop\Tor Browser\Browser\firefox.exe`, true},
		{"firefox.exe", "C:/Apps/Tor Browser/Browser/firefox.exe", true},
		{"firefox.exe", `C:\Tor Browser Notes\Browser\firefox.exe`, false},
		{"firefox.exe", `C:\Tor Browser\Other\firefox.exe`, false},
		{"code.exe", `C:\Tor Browser\Browser\code.exe`, false},
	}
	for _, tc := range tests {
		if got := isTorBrowserIdentity(tc.exe, tc.path); got != tc.want {
			t.Errorf("isTorBrowserIdentity(%q, %q) = %v; want %v", tc.exe, tc.path, got, tc.want)
		}
	}
}
