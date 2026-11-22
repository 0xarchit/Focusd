package storage_test

import (
	"focusd/coreapi"
	"focusd/storage"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGuestResetDoesNotReimportLegacyPomodoro(t *testing.T) {
	expected := os.Getenv("FOCUSD_SANDBOX_COMPUTER")
	computer := os.Getenv("COMPUTERNAME")
	if os.Getenv("FOCUSD_SANDBOX_GUEST") != "1" || expected == "" || !strings.EqualFold(computer, expected) || strings.EqualFold(computer, "0XARCHIT") {
		t.Skip("destructive storage integration requires the verified Windows Sandbox guest")
	}
	_ = storage.Close()
	t.Setenv("APPDATA", t.TempDir())
	if err := storage.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	legacyPath := filepath.Join(os.Getenv("APPDATA"), "focusd", "pomodoro.json")
	legacy := []byte(`{"active":true,"duration_minutes":60,"start_time":"2026-11-22T12:00:00Z"}`)
	if err := os.WriteFile(legacyPath, legacy, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.ResetTrackingData("legacy-offline-reset", true); err != nil {
		t.Fatal(err)
	}
	if err := storage.Close(); err != nil {
		t.Fatal(err)
	}
	if err := storage.Init(); err != nil {
		t.Fatal(err)
	}
	state := coreapi.LoadPomodoroStateFresh()
	if state.Active || state.Frozen || state.RemainingNS != nil || !state.StartTime.IsZero() {
		t.Errorf("legacy timer resurrected after reset: %+v", state)
	}
	if content, err := os.ReadFile(legacyPath); err != nil || string(content) != string(legacy) {
		t.Errorf("reset/read mutated legacy file: content %q, error %v", content, err)
	}
}
