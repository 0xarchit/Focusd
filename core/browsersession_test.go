package core

import (
	"testing"
	"time"

	"focusd/system"
)

// browserTracker builds a tracker whose config makes chrome a browser, so the
// cleaned-title comparison path is exercised.
func browserTracker(t *testing.T) *Tracker {
	t.Helper()
	setupTestDB(t)

	// IsBrowser reads config.json; without an entry chrome.exe is already a
	// default browser, so no setup is needed beyond the tracker itself.
	return &Tracker{pollInterval: time.Second}
}

func newSession(appID, exeName, title string) *activeSession {
	return &activeSession{
		AppName:     system.TitleCaseExe(exeName),
		ExeName:     exeName,
		AppID:       appID,
		WindowTitle: title,
		// Started well in the past: closeCurrentSession discards sessions
		// shorter than a second, which would otherwise make these tests pass
		// for the wrong reason.
		StartTime: time.Now().Add(-time.Hour),
		Date:      "2026-06-26",
	}
}

// TestBrowserTitleNoiseDoesNotSplitSession is the regression guard for session
// churn: a title that ticks must not close and reopen the session.
func TestBrowserTitleNoiseDoesNotSplitSession(t *testing.T) {
	tracker := browserTracker(t)

	const appID = "chrome|abc12345"
	tracker.currentSession = newSession(appID, "chrome.exe", "GitHub - Google Chrome")
	originalStart := tracker.currentSession.StartTime

	// The same page, with the decorations a browser adds and removes.
	for _, noisy := range []string{
		"(3) GitHub - Google Chrome",
		"GitHub (12) - Google Chrome",
		"GitHub - Google Chrome",
		"GitHub and 4 more pages - Google Chrome",
		"GitHub - Person 2 - Microsoft\u200b Edge",
	} {
		tracker.applyPoll(&pollObservation{
			exeName: "chrome.exe",
			appID:   appID,
			title:   noisy,
		})

		if tracker.currentSession == nil {
			t.Fatalf("session was closed by noisy title %q", noisy)
		}
		if tracker.currentSession.StartTime != originalStart {
			t.Fatalf("noisy title %q split the session", noisy)
		}
	}
	if len(tracker.pendingSessions) != 0 {
		t.Errorf("expected no flushed sessions, got %d", len(tracker.pendingSessions))
	}
}

func TestBrowserRealNavigationSplitsSession(t *testing.T) {
	tracker := browserTracker(t)

	const appID = "chrome|abc12345"
	tracker.currentSession = newSession(appID, "chrome.exe", "GitHub - Google Chrome")

	tracker.applyPoll(&pollObservation{
		exeName: "chrome.exe",
		appID:   appID,
		title:   "YouTube - Google Chrome",
	})

	if tracker.currentSession == nil {
		t.Fatal("navigating to another site should start a new session")
	}
	if tracker.currentSession.WindowTitle != "YouTube - Google Chrome" {
		t.Errorf("new session title = %q", tracker.currentSession.WindowTitle)
	}
	if len(tracker.pendingSessions) != 1 {
		t.Errorf("expected the previous session to be flushed, got %d", len(tracker.pendingSessions))
	}
}

// TestNonBrowserIgnoresTitleChanges documents that a non-browser app keeps one
// session across title changes.
func TestNonBrowserIgnoresTitleChanges(t *testing.T) {
	setupTestDB(t)
	tracker := &Tracker{pollInterval: time.Second}

	const appID = "notepad|abc12345"
	tracker.currentSession = newSession(appID, "notepad.exe", "notes - Untitled")

	// A cosmetic difference in the raw title does not close a non-browser
	// session. A file being renamed is not new activity, and splitting on it
	// would inflate open counts.
	tracker.applyPoll(&pollObservation{
		exeName: "notepad.exe",
		appID:   appID,
		title:   "notes - Edited",
	})

	if len(tracker.pendingSessions) != 0 {
		t.Errorf("a title change should not close a non-browser session, got %d pending", len(tracker.pendingSessions))
	}
	if tracker.currentSession == nil {
		t.Error("the session should still be running")
	}
}

// TestNonBrowserSwitchesOnDifferentExe confirms the app transition still
// happens for non-browsers.
func TestNonBrowserSwitchesOnDifferentExe(t *testing.T) {
	setupTestDB(t)
	tracker := &Tracker{pollInterval: time.Second}

	tracker.currentSession = newSession("notepad|abc12345", "notepad.exe", "notes")

	tracker.applyPoll(&pollObservation{
		exeName: "word.exe",
		appID:   "word|def67890",
		title:   "Document",
	})

	if len(tracker.pendingSessions) != 1 {
		t.Errorf("switching apps should flush the session, got %d pending", len(tracker.pendingSessions))
	}
	if tracker.currentSession.ExeName != "word.exe" {
		t.Errorf("current exe = %q; want word.exe", tracker.currentSession.ExeName)
	}
}

// TestDifferentInstallsAlwaysSplit covers issue #17: the same exe name from two
// installs must not continue one session.
func TestDifferentInstallsAlwaysSplit(t *testing.T) {
	setupTestDB(t)
	tracker := &Tracker{pollInterval: time.Second}

	tracker.currentSession = newSession("firefox|11111111", "firefox.exe", "Page — Mozilla Firefox")

	tracker.applyPoll(&pollObservation{
		exeName: "firefox.exe",
		appID:   "firefox|22222222",
		title:   "Page — Mozilla Firefox",
	})

	if len(tracker.pendingSessions) != 1 {
		t.Errorf("switching install should close the session, got %d pending", len(tracker.pendingSessions))
	}
	if tracker.currentSession.AppID != "firefox|22222222" {
		t.Errorf("new session app id = %q; want the nightly install", tracker.currentSession.AppID)
	}
}
