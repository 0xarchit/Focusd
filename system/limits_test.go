package system

import "testing"

// TestMatchLimitKeyWildcard is the regression guard for a silent failure: limits
// are stored under a bare exe name but usage is queried by app id, so an exact
// lookup would never match and no limit would ever fire.
func TestMatchLimitKeyWildcard(t *testing.T) {
	limits := map[string]int{"firefox": 60}

	key, ok := MatchLimitKey(limits, "firefox|4a40a58c", "firefox.exe")
	if !ok {
		t.Fatal("a bare config key should match a live app id")
	}
	if key != "firefox" {
		t.Errorf("matched key = %q; want firefox", key)
	}
}

func TestMatchLimitKeyPrefersSpecific(t *testing.T) {
	limits := map[string]int{
		"firefox":          60,
		"firefox|4a40a58c": 30,
	}

	key, ok := MatchLimitKey(limits, "firefox|4a40a58c", "firefox.exe")
	if !ok {
		t.Fatal("expected a match")
	}
	if key != "firefox|4a40a58c" {
		t.Errorf("matched key = %q; want the specific install key", key)
	}
}

func TestMatchLimitKeyAcceptsLegacySpelling(t *testing.T) {
	// Config written by older versions stored the name with the extension.
	limits := map[string]int{"firefox.exe": 45}

	key, ok := MatchLimitKey(limits, "firefox|4a40a58c", "firefox.exe")
	if !ok {
		t.Fatal("the historical .exe spelling should still match")
	}
	if key != "firefox.exe" {
		t.Errorf("matched key = %q; want firefox.exe", key)
	}
}

func TestMatchLimitKeyNoMatch(t *testing.T) {
	limits := map[string]int{"chrome": 60}

	if key, ok := MatchLimitKey(limits, "firefox|4a40a58c", "firefox.exe"); ok {
		t.Errorf("unexpected match on %q", key)
	}

	if _, ok := MatchLimitKey(nil, "firefox|abc", "firefox.exe"); ok {
		t.Error("empty limits should not match")
	}

	if _, ok := MatchLimitKey(map[string]int{}, "", ""); ok {
		t.Error("empty identity should not match")
	}
}

func TestDisplayOverrideRoundTrip(t *testing.T) {
	setupConfigEnv(t)

	const appID = "firefox|4a40a58c"
	if got := GetDisplayOverride(appID); got != "" {
		t.Errorf("expected no override initially, got %q", got)
	}

	if err := SetDisplayOverride(appID, "Work Firefox"); err != nil {
		t.Fatalf("SetDisplayOverride failed: %v", err)
	}
	if got := GetDisplayOverride(appID); got != "Work Firefox" {
		t.Errorf("override = %q; want Work Firefox", got)
	}

	// An empty name removes it.
	if err := SetDisplayOverride(appID, ""); err != nil {
		t.Fatalf("SetDisplayOverride(empty) failed: %v", err)
	}
	if got := GetDisplayOverride(appID); got != "" {
		t.Errorf("override = %q; want it removed", got)
	}
}

func TestSetDisplayOverrideRequiresID(t *testing.T) {
	setupConfigEnv(t)
	if err := SetDisplayOverride("", "Name"); err == nil {
		t.Error("expected an error for an empty app id")
	}
}

func TestResolveDisplayNameOrder(t *testing.T) {
	setupConfigEnv(t)

	const appID = "firefox|4a40a58c"
	if err := SetDisplayOverride(appID, "Work Firefox"); err != nil {
		t.Fatalf("SetDisplayOverride failed: %v", err)
	}

	// User override wins over everything.
	if got := ResolveDisplayName(appID, "firefox.exe", "Mozilla Firefox"); got != "Work Firefox" {
		t.Errorf("ResolveDisplayName = %q; want the user override", got)
	}

	// Without an override, version info wins.
	if got := ResolveDisplayName("other|abc", "firefox.exe", "Mozilla Firefox"); got != "Mozilla Firefox" {
		t.Errorf("ResolveDisplayName = %q; want the version label", got)
	}

	// Without version info, the built-in map wins.
	if got := ResolveDisplayName("other|abc", "code.exe", ""); got != "VS Code" {
		t.Errorf("ResolveDisplayName = %q; want VS Code", got)
	}

	// Falling all the way through.
	if got := ResolveDisplayName("other|abc", "someapp.exe", ""); got != "Someapp" {
		t.Errorf("ResolveDisplayName = %q; want Someapp", got)
	}
}

// TestWhitelistAddRemoveSymmetry covers the bug where removal compared the raw
// argument while insertion normalized it, so "foo" never removed "foo.exe".
func TestWhitelistAddRemoveSymmetry(t *testing.T) {
	setupConfigEnv(t)

	if err := AddWhitelistApp("thing"); err != nil {
		t.Fatalf("AddWhitelistApp failed: %v", err)
	}
	if !IsWhitelisted("thing.exe") {
		t.Error("the app should be whitelisted under its .exe form")
	}

	// The exact bug: removing with the bare name must work.
	if err := RemoveWhitelistApp("thing"); err != nil {
		t.Fatalf("RemoveWhitelistApp failed: %v", err)
	}
	if IsWhitelisted("thing.exe") {
		t.Error("the app should no longer be whitelisted")
	}

	// And the other spelling must work too.
	if err := AddWhitelistApp("other.exe"); err != nil {
		t.Fatalf("AddWhitelistApp failed: %v", err)
	}
	if err := RemoveWhitelistApp("other"); err != nil {
		t.Fatalf("RemoveWhitelistApp failed: %v", err)
	}
	if IsWhitelisted("other.exe") {
		t.Error("the app should no longer be whitelisted")
	}
}

func TestSetAppTimeLimitNormalizesKey(t *testing.T) {
	setupConfigEnv(t)

	if err := SetAppTimeLimit("Firefox", 45); err != nil {
		t.Fatalf("SetAppTimeLimit failed: %v", err)
	}

	limits := GetAppTimeLimits()
	if _, ok := limits["firefox"]; !ok {
		t.Errorf("limits = %v; want a normalized firefox key", limits)
	}

	if err := SetAppTimeLimit("firefox.exe", 0); err != nil {
		t.Fatalf("removing the limit failed: %v", err)
	}
	if len(GetAppTimeLimits()) != 0 {
		t.Error("the limit should be gone")
	}
}

func TestLookupNameMap(t *testing.T) {
	tests := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"code.exe", "VS Code", true},
		{"CODE.EXE", "VS Code", true},
		{"explorer.exe", "File Explorer", true},
		{"nonexistent.exe", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, ok := lookupNameMap(tc.in)
			if ok != tc.wantOK {
				t.Errorf("lookupNameMap(%q) ok = %v; want %v", tc.in, ok, tc.wantOK)
			}
			if ok && got != tc.want {
				t.Errorf("lookupNameMap(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeConfigName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"firefox", "firefox"},
		{"firefox.exe", "firefox"},
		{"Firefox.EXE", "firefox"},
		{"  chrome.exe  ", "chrome"},
	}

	for _, tc := range tests {
		if got := normalizeConfigName(tc.in); got != tc.want {
			t.Errorf("normalizeConfigName(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}
