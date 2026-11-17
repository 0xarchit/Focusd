package core

import (
	"testing"
	"time"

	"focusd/system"
)

func TestIsPrivateWindowTitle(t *testing.T) {
	private := []string{
		"Amazon - InPrivate",
		"Banking - Microsoft Edge InPrivate",
		"Something - Incognito",
		"Mail - Private Browsing",
		"Docs - Private Window",
		"Page - Guest Mode",
		"page -private",
	}
	for _, title := range private {
		if !IsPrivateWindowTitle(title) {
			t.Errorf("expected %q to be treated as private", title)
		}
	}

	public := []string{
		"GitHub - Google Chrome",
		"Privacy Policy - Example",
		"Inbox - Gmail",
		"",
		"Confidential Report Draft",
	}
	for _, title := range public {
		if IsPrivateWindowTitle(title) {
			t.Errorf("expected %q not to be treated as private", title)
		}
	}
}

func TestPrivateTitleMarkersAreAnchored(t *testing.T) {
	for _, title := range []string{
		"How incognito works - Google Chrome",
		"test-private.txt - Visual Studio Code",
		"Private browsing tutorial - Mozilla Firefox",
		"Guest mode documentation - Google Chrome",
	} {
		if IsPrivateWindowTitle(title) {
			t.Errorf("public title %q classified as private", title)
		}
	}
	for _, title := range []string{
		"Bank - Private Browsing - Mozilla Firefox",
		"Bank - Google Chrome (Incognito)",
		"Bank - Privates Fenster - Mozilla Firefox",
		"Bank - Navigation privée - Mozilla Firefox",
		"Bank - Incógnito - Google Chrome",
	} {
		if !IsPrivateWindowTitle(title) {
			t.Errorf("private suffix in %q was not recognized", title)
		}
	}
}

func TestStorableTitleDropsPrivate(t *testing.T) {
	setupTestDB(t)
	if err := system.SetRedactTitles(false); err != nil {
		t.Fatalf("SetRedactTitles failed: %v", err)
	}
	if got := storableTitle("Bank - InPrivate"); got != "" {
		t.Errorf("storableTitle of a private title = %q; want empty", got)
	}
	if got := storableTitle("GitHub - Chrome"); got != "GitHub - Chrome" {
		t.Errorf("storableTitle of a normal title = %q", got)
	}
	if got := storableTitle("   "); got != "" {
		t.Errorf("storableTitle of whitespace = %q; want empty", got)
	}
}

func TestStorableTitleRedaction(t *testing.T) {
	setupTestDB(t)
	if err := system.SetRedactTitles(true); err != nil {
		t.Fatalf("SetRedactTitles failed: %v", err)
	}
	t.Cleanup(func() { system.SetRedactTitles(false) })

	if got := storableTitle("GitHub - Chrome"); got != "[redacted]" {
		t.Errorf("with redaction on, storableTitle = %q; want [redacted]", got)
	}
	// A private title is still withheld rather than labelled.
	if got := storableTitle("Bank - InPrivate"); got != "" {
		t.Errorf("a private title must not be relabelled, got %q", got)
	}
}

// TestPrivateSessionStoresNoPageText is the end-to-end guard: a private session
// must not carry its page text into the database.
func TestPrivateSessionStoresNoPageText(t *testing.T) {
	setupTestDB(t)
	tracker := &Tracker{pollInterval: time.Second}

	tracker.applyPoll(&pollObservation{
		exeName: "firefox.exe",
		appID:   "firefox|abc12345",
		title:   "Online Banking - Mozilla Firefox",
	})
	tracker.applyPoll(&pollObservation{
		exeName: "firefox.exe",
		appID:   "firefox|abc12345",
		title:   "Online Banking - Private Browsing - Mozilla Firefox",
	})

	if tracker.currentSession == nil {
		t.Fatal("expected a session to be tracked")
	}
	stored := tracker.currentSession.WindowTitle
	if stored == "Online Banking - Private Browsing - Mozilla Firefox" {
		t.Error("the private page title was stored verbatim")
	}
	if stored != "[redacted]" || tracker.currentSession.SiteName != "" || !tracker.currentSession.SiteKnown {
		t.Errorf("private metadata = %q/%q; want redacted title and no site", stored, tracker.currentSession.SiteName)
	}
}

// TestConsecutivePrivatePollsDoNotSplit guards the stored-title comparison: the
// private placeholder must compare equal to itself, or every private poll would
// close and reopen the session.
func TestConsecutivePrivatePollsDoNotSplit(t *testing.T) {
	setupTestDB(t)
	if err := system.SetRedactTitles(false); err != nil {
		t.Fatalf("SetRedactTitles failed: %v", err)
	}

	tracker := &Tracker{pollInterval: time.Second}
	for i, title := range []string{
		"Search - Private Browsing - Mozilla Firefox",
		"Mail - Private Browsing - Mozilla Firefox",
	} {
		tracker.applyPoll(&pollObservation{
			exeName: "firefox.exe",
			appID:   "firefox|abc12345",
			title:   title,
		})
		if tracker.currentSession == nil {
			t.Fatalf("poll %d: expected a session", i)
		}
		if len(tracker.pendingSessions) != 0 {
			t.Fatalf("poll %d: private polls must not split the session", i)
		}
	}
}

// TestConsecutiveRedactedPollsDoNotSplit is the same guard with title redaction
// on: every page stores "[redacted]", which must compare equal.
func TestConsecutiveRedactedPollsDoNotSplit(t *testing.T) {
	setupTestDB(t)
	if err := system.SetRedactTitles(true); err != nil {
		t.Fatalf("SetRedactTitles failed: %v", err)
	}
	t.Cleanup(func() { system.SetRedactTitles(false) })

	tracker := &Tracker{pollInterval: time.Second}
	for i, title := range []string{
		"Page One - Google Chrome",
		"Page Two - Google Chrome",
	} {
		tracker.applyPoll(&pollObservation{
			exeName: "chrome.exe",
			appID:   "chrome|abc12345",
			title:   title,
		})
		if tracker.currentSession == nil {
			t.Fatalf("poll %d: expected a session", i)
		}
		if tracker.currentSession.WindowTitle != "[redacted]" {
			t.Fatalf("poll %d: stored title = %q; want [redacted]", i, tracker.currentSession.WindowTitle)
		}
		if len(tracker.pendingSessions) != 0 {
			t.Fatalf("poll %d: redacted polls must not split the session", i)
		}
	}
}

func TestPrivacyModeChangeRotatesNonBrowserInterval(t *testing.T) {
	setupTestDB(t)
	if err := system.SetPrivacyMode("titles"); err != nil {
		t.Fatal(err)
	}
	tracker := &Tracker{pollInterval: time.Second}
	tracker.applyPoll(&pollObservation{exeName: "notepad.exe", appID: "notepad|test", title: "Sensitive document"})
	tracker.currentSession.StartTime = time.Now().Add(-time.Minute)
	if err := system.SetPrivacyMode("strict"); err != nil {
		t.Fatal(err)
	}
	tracker.applyPoll(&pollObservation{exeName: "notepad.exe", appID: "notepad|test", title: ""})
	if tracker.currentSession == nil || tracker.currentSession.WindowTitle != "[redacted]" {
		t.Fatal("stricter policy retained raw title")
	}
	if len(tracker.pendingSessions) != 1 {
		t.Fatal("prior valid interval was lost")
	}
}
