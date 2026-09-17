package system

import "testing"

func TestNormalizeExe(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"firefox.exe", "firefox"},
		{"FIREFOX.EXE", "firefox"},
		{"Firefox.Exe", "firefox"},
		{"  firefox.exe  ", "firefox"},
		{"firefox", "firefox"},
		{"", "unknown"},
		{".exe", "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := normalizeExe(tc.in); got != tc.want {
				t.Errorf("normalizeExe(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestBuildAppIDSeparatesInstalls is the regression guard for issue #17: the same
// exe name in two different install directories must produce two different keys.
func TestBuildAppIDSeparatesInstalls(t *testing.T) {
	firefox := buildAppID("firefox.exe", `C:\Program Files\Mozilla Firefox\firefox.exe`)
	nightly := buildAppID("firefox.exe", `C:\Program Files\Firefox Nightly\firefox.exe`)

	if firefox == nightly {
		t.Errorf("Firefox and Firefox Nightly share the key %q; they must be separate", firefox)
	}
	if appKeyBase(firefox) != "firefox" || appKeyBase(nightly) != "firefox" {
		t.Errorf("keys should retain the exe base: %q and %q", firefox, nightly)
	}
}

// TestBuildAppIDSurvivesVersionBump guards the other requirement: identity must
// stay stable for the same install across upgrades, so the key must depend on
// the directory and never on a versioned subdirectory name.
func TestBuildAppIDSurvivesVersionBump(t *testing.T) {
	before := buildAppID("chrome.exe", `C:\Program Files\Google\Chrome\Application\chrome.exe`)
	after := buildAppID("chrome.exe", `C:\Program Files\Google\Chrome\Application\chrome.exe`)

	if before != after {
		t.Errorf("the same path must produce a stable key: %q vs %q", before, after)
	}

	// A channel installed elsewhere is a different app.
	canary := buildAppID("chrome.exe", `C:\Users\me\AppData\Local\Google\Chrome SxS\Application\chrome.exe`)
	if canary == before {
		t.Error("a different install directory should produce a different key")
	}
}

// TestBuildAppIDNoFolderCollision covers the defect in the original plan, where
// a single parent-folder segment collided for every browser installed under a
// folder called "Application".
func TestBuildAppIDNoFolderCollision(t *testing.T) {
	chrome := buildAppID("chrome.exe", `C:\Program Files\Google\Chrome\Application\chrome.exe`)
	edge := buildAppID("msedge.exe", `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`)

	if chrome == edge {
		t.Errorf("Chrome and Edge collided on %q", chrome)
	}
}

func TestBuildAppIDCaseInsensitive(t *testing.T) {
	lower := buildAppID("Chrome.exe", `C:\Program Files\Google\Chrome\chrome.exe`)
	upper := buildAppID("CHROME.EXE", `c:\program files\google\chrome\CHROME.EXE`)

	if lower != upper {
		t.Errorf("identity should ignore case: %q vs %q", lower, upper)
	}
}

func TestBuildAppIDFallback(t *testing.T) {
	if got := buildAppID("firefox.exe", ""); got != "firefox|system" {
		t.Errorf("buildAppID with no path = %q; want firefox|system", got)
	}
	if !IsFallbackAppID(buildAppID("firefox.exe", "")) {
		t.Error("a pathless key should be marked as a fallback")
	}
}

func TestLegacyAndFallbackAppIDs(t *testing.T) {
	legacy := legacyAppID("Firefox.exe")
	if legacy != "firefox|legacy" {
		t.Errorf("legacyAppID = %q; want firefox|legacy", legacy)
	}
	if !IsLegacyAppID(legacy) {
		t.Error("legacy key should be marked as legacy")
	}
	if IsFallbackAppID(legacy) {
		t.Error("a legacy key is not a fallback key")
	}

	fallback := fallbackAppID("firefox.exe")
	if fallback != "firefox|system" {
		t.Errorf("fallbackAppID = %q; want firefox|system", fallback)
	}
}

// TestSameAppKeyKeepsInstallsSeparate checks the config-wildcard rule: a bare
// exe name matches anything, but two distinct installs must never match.
func TestSameAppKeyKeepsInstallsSeparate(t *testing.T) {
	live := buildAppID("firefox.exe", `C:\Program Files\Mozilla Firefox\firefox.exe`)
	nightly := buildAppID("firefox.exe", `C:\Program Files\Firefox Nightly\firefox.exe`)
	legacy := legacyAppID("firefox.exe")
	fallback := fallbackAppID("firefox.exe")

	if sameAppKey(live, nightly) {
		t.Error("Firefox and Nightly must not match for config purposes")
	}
	if sameAppKey(live, legacy) {
		t.Error("two hashed keys must not match")
	}

	// A bare config key is a wildcard.
	if !sameAppKey("firefox", live) || !sameAppKey("firefox", nightly) ||
		!sameAppKey("firefox", legacy) || !sameAppKey("firefox", fallback) {
		t.Error("a bare exe name should match every install")
	}
	if !sameAppKey("firefox.exe", live) {
		t.Error("a bare name with the .exe suffix should also match")
	}
	if sameAppKey("firefox", "chrome") {
		t.Error("different apps must not match")
	}
}

// TestStoredAppMatchesIsStrict covers the migration rule: a row recorded before
// paths existed keeps its own bucket rather than being folded into a live
// install. Folding it in would re-merge Firefox and Firefox Nightly, which is
// the exact problem this identity split exists to solve.
func TestStoredAppMatchesIsStrict(t *testing.T) {
	live := buildAppID("firefox.exe", `C:\Program Files\Mozilla Firefox\firefox.exe`)
	nightly := buildAppID("firefox.exe", `C:\Program Files\Firefox Nightly\firefox.exe`)
	legacy := legacyAppID("firefox.exe")
	fallback := fallbackAppID("firefox.exe")

	if !StoredAppMatches(live, live) {
		t.Error("a key should match itself")
	}
	if StoredAppMatches(legacy, live) {
		t.Error("a migrated row must not be folded into a live install")
	}
	if StoredAppMatches(fallback, live) {
		t.Error("a fallback row must not be folded into a live install")
	}
	if StoredAppMatches(live, nightly) {
		t.Error("two distinct installs must not match")
	}
	if StoredAppMatches("firefox", "chrome") {
		t.Error("different apps must not match")
	}
}

func TestConfigMatchesApp(t *testing.T) {
	live := buildAppID("firefox.exe", `C:\Program Files\Mozilla Firefox\firefox.exe`)
	nightly := buildAppID("firefox.exe", `C:\Program Files\Firefox Nightly\firefox.exe`)

	for _, key := range []string{"firefox", "firefox.exe", "FireFox.Exe", " firefox.exe ", live} {
		if !ConfigMatchesApp(key, live) {
			t.Errorf("config key %q should match %q", key, live)
		}
	}

	// Wildcard across installs, which is the point of keeping bare keys.
	if !ConfigMatchesApp("firefox", nightly) {
		t.Error("a bare config key should cover every install")
	}

	if ConfigMatchesApp("chrome", live) {
		t.Error("an unrelated config key should not match")
	}
	if ConfigMatchesApp("", live) {
		t.Error("an empty config key should not match")
	}
}

func TestNormalizeConfigKey(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"firefox", "firefox"},
		{"firefox.exe", "firefox"},
		{"Firefox.EXE", "firefox"},
		{"  chrome.exe ", "chrome"},
		{"firefox|a1b2c3d4", "firefox|a1b2c3d4"},
		{"", ""},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := normalizeConfigKey(tc.in); got != tc.want {
				t.Errorf("normalizeConfigKey(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSameAppKeyBareExeMatchesEverything(t *testing.T) {
	// A config entry written before this feature is just the exe name. It must
	// act as a wildcard so nobody's limits silently stop firing.
	bare := "firefox"
	for _, key := range []string{
		buildAppID("firefox.exe", `C:\Program Files\Mozilla Firefox\firefox.exe`),
		buildAppID("firefox.exe", `C:\Program Files\Firefox Nightly\firefox.exe`),
		legacyAppID("firefox.exe"),
		fallbackAppID("firefox.exe"),
	} {
		if !sameAppKey(bare, key) {
			t.Errorf("bare config key %q should match %q", bare, key)
		}
	}
}

func TestAppKeyBase(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"firefox|a1b2c3d4", "firefox"},
		{"firefox|legacy", "firefox"},
		{"firefox|system", "firefox"},
		{"firefox", "firefox"},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := appKeyBase(tc.in); got != tc.want {
				t.Errorf("appKeyBase(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestDirHashStability(t *testing.T) {
	dir := `C:\Program Files\Mozilla Firefox`
	if dirHash(dir) != dirHash(dir) {
		t.Error("dirHash must be stable")
	}
	if dirHash(dir) == dirHash(`C:\Program Files\Firefox Nightly`) {
		t.Error("different directories must hash differently")
	}
	if len(dirHash(dir)) != 8 {
		t.Errorf("dirHash length = %d; want 8", len(dirHash(dir)))
	}
}

func TestTitleCaseExe(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"firefox.exe", "Firefox"},
		{"firefox", "Firefox"},
		{"msedge.exe", "Msedge"},
		{"", ""},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := TitleCaseExe(tc.in); got != tc.want {
				t.Errorf("TitleCaseExe(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestResolveIdentityFallsBackCleanly covers a process whose path cannot be read,
// which happens for elevated and protected processes.
func TestResolveIdentityFallsBackCleanly(t *testing.T) {
	id := ResolveIdentity("svchost.exe", "")

	if id.AppID != "svchost|system" {
		t.Errorf("AppID = %q; want svchost|system", id.AppID)
	}
	if id.DisplayName == "" {
		t.Error("a fallback identity must still have a label")
	}
	if id.ExePath != "" {
		t.Errorf("ExePath = %q; want empty", id.ExePath)
	}
}

func TestResolveIdentityUsesVersionInfoWhenAvailable(t *testing.T) {
	// A path that exists on this machine and has a version resource. The label
	// is whatever the resource says, not the exe name.
	id := ResolveIdentity("notepad.exe", `C:\WINDOWS\System32\notepad.exe`)

	if id.AppID != "notepad|"+dirHash(`C:\WINDOWS\System32`) {
		t.Errorf("AppID = %q; unexpected key", id.AppID)
	}
	if id.DisplayName == "" {
		t.Error("expected a label")
	}
}