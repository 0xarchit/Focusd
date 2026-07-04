package core

import (
	"context"
	"focusd/storage"
	"os"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "focusd_test_appdata_*")
	if err != nil {
		t.Fatalf("failed to create temp appdata: %v", err)
	}

	originalAppData := os.Getenv("APPDATA")
	os.Setenv("APPDATA", tempDir)

	t.Cleanup(func() {
		if originalAppData != "" {
			os.Setenv("APPDATA", originalAppData)
		} else {
			os.Unsetenv("APPDATA")
		}
		os.RemoveAll(tempDir)
	})

	if err := storage.Init(); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	return tempDir
}

func TestGetAppName(t *testing.T) {
	tests := []struct {
		exe      string
		expected string
	}{
		{"code.exe", "VS Code"},
		{"explorer.exe", "File Explorer"},
		{"notepad++.exe", "Notepad++"},
		{"cmd.EXE", "Command Prompt"},
		{"unknownapp.exe", "Unknownapp"},
		{"CapitalEXE.EXE", "CapitalEXE"},
	}

	for _, tc := range tests {
		got := getAppName(tc.exe)
		if got != tc.expected {
			t.Errorf("getAppName(%q) = %q; want %q", tc.exe, got, tc.expected)
		}
	}
}

func TestCloseCurrentSession(t *testing.T) {
	tracker := &Tracker{
		pollInterval: time.Second,
	}

	tracker.closeCurrentSession()
	if len(tracker.pendingSessions) != 0 {
		t.Errorf("expected 0 pending sessions, got %d", len(tracker.pendingSessions))
	}

	tracker.currentSession = &ActiveSession{
		AppName:   "TestApp",
		ExeName:   "test.exe",
		StartTime: time.Now(),
		Date:      "2026-06-26",
	}
	tracker.closeCurrentSession()
	if len(tracker.pendingSessions) != 0 {
		t.Errorf("expected session shorter than 1s to be discarded, but got %d pending sessions", len(tracker.pendingSessions))
	}
	if tracker.currentSession != nil {
		t.Error("expected currentSession to be nil after close")
	}

	tracker.currentSession = &ActiveSession{
		AppName:   "TestApp",
		ExeName:   "test.exe",
		StartTime: time.Now().Add(-2 * time.Second),
		Date:      "2026-06-26",
	}
	tracker.closeCurrentSession()
	if len(tracker.pendingSessions) != 1 {
		t.Fatalf("expected 1 pending session, got %d", len(tracker.pendingSessions))
	}
	s := tracker.pendingSessions[0]
	if s.AppName != "TestApp" || s.DurationSecs < 2 {
		t.Errorf("invalid saved session: %+v", s)
	}
}

func TestFlushPendingSessions(t *testing.T) {
	setupTestDB(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tracker := &Tracker{
		pollInterval: time.Second,
		ctx:          ctx,
		cancel:       cancel,
	}

	s1 := &storage.Session{
		AppName:      "App1",
		ExeName:      "app1.exe",
		WindowTitle:  "Title1",
		StartTime:    time.Now().Add(-10 * time.Second),
		EndTime:      time.Now(),
		DurationSecs: 10,
		Date:         storage.Today(),
	}

	tracker.pendingSessions = append(tracker.pendingSessions, s1)
	tracker.flushPendingSessions()

	if len(tracker.pendingSessions) != 0 {
		t.Errorf("pending sessions should be cleared, got %d", len(tracker.pendingSessions))
	}

	sessions, err := storage.GetSessionsPaginated(10, 0, storage.Today(), storage.Today())
	if err != nil {
		t.Fatalf("failed to retrieve sessions: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("expected 1 session in DB, got %d", len(sessions))
	}
	if sessions[0].AppName != "App1" {
		t.Errorf("expected AppName to be 'App1', got %q", sessions[0].AppName)
	}
}

func TestRetentionDays(t *testing.T) {
	setupTestDB(t)

	if got := storage.GetRetentionDays(); got != storage.DefaultRetentionDays {
		t.Errorf("expected default retention days to be %d, got %d", storage.DefaultRetentionDays, got)
	}

	customDays := 15
	if err := storage.SetRetentionDays(customDays); err != nil {
		t.Fatalf("failed to set retention days: %v", err)
	}

	if got := storage.GetRetentionDays(); got != customDays {
		t.Errorf("expected custom retention days to be %d, got %d", customDays, got)
	}
}
