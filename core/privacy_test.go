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
	if stored != privateWindowLabel("firefox.exe") {
		t.Errorf("stored title = %q; want %q", stored, privateWindowLabel("firefox.exe"))
	}
}
