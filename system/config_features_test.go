package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPrivacyModePreservesLegacyRedaction(t *testing.T) {
	dir := setupConfigEnv(t)
	path := filepath.Join(dir, "focusd", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"redact_titles":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got := GetPrivacyMode(); got != PrivacyModeStrict {
		t.Fatalf("legacy redaction mode = %q; want strict", got)
	}
	for _, mode := range []string{PrivacyModeCategories, PrivacyModeTitles, PrivacyModeStrict} {
		if err := SetPrivacyMode(mode); err != nil {
			t.Fatal(err)
		}
		if got := GetPrivacyMode(); got != mode {
			t.Fatalf("mode = %q; want %q", got, mode)
		}
		if GetRedactTitles() != (mode != PrivacyModeTitles) {
			t.Fatalf("redaction flag disagrees with %s", mode)
		}
	}
	if err := SetPrivacyMode("unknown"); err == nil {
		t.Fatal("invalid privacy mode was accepted")
	}
	if GetPrivacyMode() != PrivacyModeStrict {
		t.Fatal("invalid mode changed the saved policy")
	}
	if err := SetRedactTitles(false); err != nil {
		t.Fatal(err)
	}
	if GetPrivacyMode() != PrivacyModeTitles {
		t.Fatal("legacy disable did not clear explicit strict mode")
	}
}

func TestPresentationOverrideSnapshotsAreIndependent(t *testing.T) {
	setupConfigEnv(t)
	if err := SetDisplayOverride("code|aabb", "Editor"); err != nil {
		t.Fatal(err)
	}
	if err := SetSiteOverride(" GitHub ", "Work"); err != nil {
		t.Fatal(err)
	}
	snapshot := GetPresentationOverrides()
	snapshot.Display["code|aabb"] = "Changed"
	snapshot.Sites["github"] = "Changed"
	if GetDisplayOverride("code|aabb") != "Editor" || GetSiteOverrides()["github"] != "Work" {
		t.Fatal("returned maps alias config state")
	}
	if err := SetSiteOverride("GITHUB", ""); err != nil {
		t.Fatal(err)
	}
	if len(GetSiteOverrides()) != 0 {
		t.Fatal("site override was not removed")
	}
	if err := SetTrackWhileIdle(true); err != nil {
		t.Fatal(err)
	}
	if !GetTrackWhileIdle() {
		t.Fatal("track-while-idle did not persist")
	}
}

func TestLimitRenameKeepsWildcardAndRollsBackFailedSave(t *testing.T) {
	dir := setupConfigEnv(t)
	if err := SetAppTimeLimit("Firefox.exe", 60); err != nil {
		t.Fatal(err)
	}
	if err := SetAppTimeLimit("Firefox.exe|aabb", 30); err != nil {
		t.Fatal(err)
	}
	if err := RenameAppTimeLimit("Firefox.exe|aabb", "Firefox.exe|ccdd", 40); err != nil {
		t.Fatal(err)
	}
	limits := GetAppTimeLimits()
	if len(limits) != 2 || limits["firefox"] != 60 || limits["firefox|ccdd"] != 40 {
		t.Fatalf("renamed limits = %#v", limits)
	}
	path := filepath.Join(dir, "focusd", "config.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path+".tmp", 0700); err != nil {
		t.Fatal(err)
	}
	if err := RenameAppTimeLimit("firefox|ccdd", "chrome|1122", 50); err == nil {
		t.Fatal("expected blocked temp-file write")
	}
	if got := GetAppTimeLimits(); len(got) != 2 || got["firefox|ccdd"] != 40 || got["firefox"] != 60 {
		t.Fatalf("failed rename changed limits: %#v", got)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("failed save changed on-disk config")
	}
}

func TestConfigProfileChangeResetsMissingFileDefaults(t *testing.T) {
	setupConfigEnv(t)
	if err := SetAppTimeLimit("code", 15); err != nil {
		t.Fatal(err)
	}
	if err := SetPomodoroMinutes(40); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("APPDATA", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if len(GetAppTimeLimits()) != 0 || GetPomodoroMinutes() != 25 {
		t.Fatal("new profile inherited prior config")
	}
}

func TestInvalidConfigKeepsLastGoodAndRecoversAtSameTime(t *testing.T) {
	dir := setupConfigEnv(t)
	if err := SetPomodoroMinutes(35); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "focusd", "config.json")
	stamp := time.Now().Add(-time.Hour)
	if err := os.WriteFile(path, []byte(`{"pomodoro_minutes":oops}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if GetPomodoroMinutes() != 35 {
		t.Fatal("invalid config replaced last good state")
	}
	if err := os.WriteFile(path, []byte(`{"pomodoro_minutes":55}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if GetPomodoroMinutes() != 55 {
		t.Fatal("corrected older-mtime file did not reload")
	}
	if err := os.WriteFile(path, []byte(strings.Repeat(" ", maxConfigBytes+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if GetPomodoroMinutes() != 55 {
		t.Fatal("oversized config replaced last good state")
	}
}

func TestLegacyCaseExactLimitWinsOverWildcard(t *testing.T) {
	limits := map[string]int{"Firefox.exe": 60, "Firefox.exe|AABB": 30}
	if key, ok := MatchLimitKey(limits, "firefox|aabb", "firefox.exe"); !ok || key != "Firefox.exe|AABB" {
		t.Fatalf("match = %q, %v; want exact legacy key", key, ok)
	}
}

func TestConfigSettersRollbackWhenSaveFails(t *testing.T) {
	dir := setupConfigEnv(t)
	if err := SetPrivacyMode(PrivacyModeStrict); err != nil {
		t.Fatal(err)
	}
	if err := SetSiteOverride("github", "Work"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "focusd", "config.json")
	if err := os.Mkdir(path+".tmp", 0700); err != nil {
		t.Fatal(err)
	}
	if err := SetPrivacyMode(PrivacyModeTitles); err == nil {
		t.Fatal("expected denied config write")
	}
	if GetPrivacyMode() != PrivacyModeStrict {
		t.Fatal("failed privacy save relaxed in-memory privacy")
	}
	if err := SetSiteOverride("github", "Changed"); err == nil {
		t.Fatal("expected denied site override write")
	}
	if GetSiteOverrides()["github"] != "Work" {
		t.Fatal("failed site save changed in-memory label")
	}
}

func TestInvalidExplicitPrivacyModeFailsClosed(t *testing.T) {
	dir := setupConfigEnv(t)
	path := filepath.Join(dir, "focusd", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"privacy_mode":"strcit","redact_titles":false}`), 0600); err != nil {
		t.Fatal(err)
	}
	if GetPrivacyMode() != PrivacyModeStrict || !GetRedactTitles() {
		t.Fatal("invalid explicit privacy mode allowed title capture")
	}
}
