package system

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfigReloadAcceptsOlderModificationTime(t *testing.T) {
	dir := setupConfigEnv(t)
	if err := SetPomodoroMinutes(31); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "focusd", "config.json")
	stamp := lastModTime.Add(-time.Hour)
	if err := os.WriteFile(path, []byte(`{"pomodoro_minutes":47}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if got := GetPomodoroMinutes(); got != 47 {
		t.Fatalf("pomodoro after clock rollback edit = %d; want 47", got)
	}
}

func TestConfigReloadAcceptsChangedContentAtSameModificationTime(t *testing.T) {
	dir := setupConfigEnv(t)
	if err := SetPomodoroMinutes(31); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "focusd", "config.json")
	stamp := lastModTime
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte(`"pomodoro_minutes": 31`), []byte(`"pomodoro_minutes": 47`), 1)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if got := GetPomodoroMinutes(); got != 47 {
		t.Fatalf("pomodoro after same-mtime edit = %d; want 47", got)
	}
}

func TestDeletingLegacyLimitRemovesEquivalentSpellings(t *testing.T) {
	dir := setupConfigEnv(t)
	path := filepath.Join(dir, "focusd", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"app_time_limits":{"Firefox.exe":45,"firefox":60,"firefox|aabb":90}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := SetAppTimeLimit("firefox.exe", 0); err != nil {
		t.Fatal(err)
	}
	limits := GetAppTimeLimits()
	if len(limits) != 1 || limits["firefox|aabb"] != 90 {
		t.Fatalf("limits after wildcard deletion = %#v; want only exact install", limits)
	}
}

func TestUpdatingLegacyExactLimitKeepsInstallSuffix(t *testing.T) {
	dir := setupConfigEnv(t)
	path := filepath.Join(dir, "focusd", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"app_time_limits":{"Firefox.exe|aabb":45,"firefox":60}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := SetAppTimeLimit("Firefox.exe|aabb", 90); err != nil {
		t.Fatal(err)
	}
	limits := GetAppTimeLimits()
	if len(limits) != 2 || limits["firefox|aabb"] != 90 || limits["firefox"] != 60 {
		t.Fatalf("limits after exact update = %#v; want normalized exact and unchanged wildcard", limits)
	}
}
