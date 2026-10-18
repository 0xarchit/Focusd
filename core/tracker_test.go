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

	// storage.Init returns early when a handle is already open, so a previous
	// test's database would be reused here and writes would go into a deleted
	// temp directory. Closing first guarantees each test gets its own file.
	storage.Close()

	t.Cleanup(func() {
		storage.Close()
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

// TestPollRetainsSessionOnBlankTitle covers the case where a child window with
// an empty caption takes focus. The session must continue rather than split.
func TestPollRetainsSessionOnBlankTitle(t *testing.T) {
	tracker := &Tracker{pollInterval: time.Second}

	tracker.currentSession = &activeSession{
		AppName:     "TestApp",
		ExeName:     "test.exe",
		WindowTitle: "Real Window",
		StartTime:   time.Now(),
		Date:        "2026-06-26",
	}
	originalStart := tracker.currentSession.StartTime

	tracker.applyPoll(&pollObservation{exeName: "test.exe", title: ""})

	if tracker.currentSession == nil {
		t.Fatal("session was closed by a blank title")
	}
	if tracker.currentSession.StartTime != originalStart {
		t.Error("session start time changed, which means the session was split")
	}
	if tracker.currentSession.WindowTitle != "Real Window" {
		t.Errorf("window title was overwritten with a blank: %q", tracker.currentSession.WindowTitle)
	}
	if len(tracker.pendingSessions) != 0 {
		t.Errorf("expected no flushed sessions, got %d", len(tracker.pendingSessions))
	}
}

// TestPollBlankTitleWithoutSessionStartsNothing covers the complementary case:
// a blank title with no session to retain has nothing worth recording.
func TestPollBlankTitleWithoutSessionStartsNothing(t *testing.T) {
	tracker := &Tracker{pollInterval: time.Second}

	tracker.applyPoll(&pollObservation{exeName: "test.exe", title: ""})

	if tracker.currentSession != nil {
		t.Error("expected no session to start from a blank title")
	}
	if len(tracker.pendingSessions) != 0 {
		t.Errorf("expected no sessions, got %d", len(tracker.pendingSessions))
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

	tracker.currentSession = &activeSession{
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

	tracker.currentSession = &activeSession{
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

// TestFlushForReadKeepsLiveSession pins that the IPC flush behind `focusd
// status`, `focusd stats`, and the TUI refresh only persists state: closing
// the session there fragmented it, bumped open counts, and cleared
// active_session until the next poll.
func TestFlushForReadKeepsLiveSession(t *testing.T) {
	setupTestDB(t)

	tracker := &Tracker{pollInterval: time.Second}
	tracker.currentSession = &activeSession{
		AppName:     "TestApp",
		ExeName:     "test.exe",
		AppID:       "test|abcd1234",
		WindowTitle: "Window",
		StartTime:   time.Now().Add(-time.Minute),
		Date:        storage.Today(),
	}

	tracker.flushForRead()

	if tracker.currentSession == nil {
		t.Fatal("flush for a read closed the live session")
	}

	active, err := storage.GetActiveSession()
	if err != nil {
		t.Fatalf("GetActiveSession failed: %v", err)
	}
	if active == nil {
		t.Fatal("active_session row missing after a read flush")
	}
	if active.ExeName != "test.exe" {
		t.Errorf("active_session exe = %q; want test.exe", active.ExeName)
	}
}

// TestPersistedActiveSessionKeepsIdentity pins that the active_session row
// carries the session's real app identity: without it the row falls back to
// the legacy key, and the tracker's own GetAppUsageTodaySeconds lookup never
// matches the live session against its install.
func TestPersistedActiveSessionKeepsIdentity(t *testing.T) {
	setupTestDB(t)

	tracker := &Tracker{pollInterval: time.Second}
	tracker.currentSession = &activeSession{
		AppName:     "Chrome",
		ExeName:     "chrome.exe",
		ExePath:     `C:\Program Files\Google\Chrome\Application\chrome.exe`,
		AppID:       "chrome|abcd1234",
		WindowTitle: "Tab",
		StartTime:   time.Now().Add(-30 * time.Second),
		Date:        storage.Today(),
	}

	tracker.forcePersistActiveSession()

	active, err := storage.GetActiveSession()
	if err != nil {
		t.Fatalf("GetActiveSession failed: %v", err)
	}
	if active == nil {
		t.Fatal("active_session row missing")
	}
	if active.AppID != "chrome|abcd1234" {
		t.Errorf("active_session app_id = %q; want chrome|abcd1234", active.AppID)
	}
	if active.ExePath != `C:\Program Files\Google\Chrome\Application\chrome.exe` {
		t.Errorf("active_session exe_path = %q; want the full path", active.ExePath)
	}

	// Enforcement asks for the live session under its real key; the row must
	// answer it.
	if usage := storage.GetAppUsageTodaySeconds("chrome|abcd1234"); usage <= 0 {
		t.Errorf("live session not counted for its own install (usage = %d)", usage)
	}
}
