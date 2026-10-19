package core

import "testing"

func TestSanitizeTitle(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "GitHub - Pull Request", "GitHub - Pull Request"},
		{"zero width space removed", "Microsoft\u200b Edge", "Microsoft Edge"},
		{"zero width joiner removed", "a\u200db", "ab"},
		{"soft hyphen removed", "some\u00adthing", "something"},
		{"control characters become spaces", "a\x00b", "a b"},
		{"whitespace collapsed", "  a   b  ", "a b"},
		{"non ascii preserved", "Café — naïve", "Café — naïve"},
		{"empty", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitizeTitle(tc.in); got != tc.want {
				t.Errorf("sanitizeTitle(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestStripDecorations(t *testing.T) {
	// These run after the browser suffix has already been removed, which is
	// why the anchored patterns apply.
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"leading counter", "(3) Inbox - Gmail", "Inbox - Gmail"},
		{"trailing counter", "Inbox [5]", "Inbox"},
		{"braced counter", "{12} Something", "Something"},
		{"and more pages", "Tab and 4 more pages", "Tab"},
		{"person segment", "Inbox - Person 2 -", "Inbox"},
		{"counter in middle", "Inbox (7) - Gmail", "Inbox - Gmail"},
		{"nothing to strip", "Plain Title", "Plain Title"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripDecorations(tc.in); got != tc.want {
				t.Errorf("stripDecorations(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestCleanWindowTitleBrowserSuffix covers the ordinary cases.
func TestCleanWindowTitleBrowserSuffix(t *testing.T) {
	tests := []struct {
		name string
		in   string
		exe  string
		want string
	}{
		{"chrome", "GitHub - Google Chrome", "chrome.exe", "GitHub"},
		{"firefox em dash", "Example Page — Mozilla Firefox", "firefox.exe", "Example Page"},
		{"edge zero width space", "Microsoft\u200b Edge", "msedge.exe", "Microsoft Edge"},
		{"pipe separator", "YouTube | Brave", "brave.exe", "YouTube"},
		{"empty title", "", "chrome.exe", "Unknown Tab"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := cleanWindowTitle(tc.in, tc.exe); got != tc.want {
				t.Errorf("cleanWindowTitle(%q, %q) = %q; want %q", tc.in, tc.exe, got, tc.want)
			}
		})
	}
}

// TestCleanWindowTitleKeepsPageText checks that a short exe name never strips
// text out of the middle of an unrelated title.
func TestCleanWindowTitleKeepsPageText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		exe  string
		want string
	}{
		{"arc does not match search", "Search results", "arc.exe", "Search results"},
		{"code does not match LeetCode", "LeetCode - Two Sum", "code.exe", "LeetCode - Two Sum"},
		{"word arc kept", "Arcadia Rising", "arc.exe", "Arcadia Rising"},
		{"person not stripped as browser", "Person of Interest", "chrome.exe", "Person of Interest"},
		{"work kept mid title", "Work - Employee Handbook", "chrome.exe", "Work - Employee Handbook"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := cleanWindowTitle(tc.in, tc.exe); got != tc.want {
				t.Errorf("cleanWindowTitle(%q, %q) = %q; want %q", tc.in, tc.exe, got, tc.want)
			}
		})
	}
}

func TestCleanWindowTitleStripsProfileSegments(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"firefox work profile", "Add-ons - Firefox Developer Edition", "Add-ons"},
		{"trailing work", "Inbox - Work", "Inbox"},
		{"trailing profile 3", "Docs - Profile 3", "Docs"},
		{"trailing personal", "Mail - Personal", "Mail"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := cleanWindowTitle(tc.in, "firefox.exe"); got != tc.want {
				t.Errorf("cleanWindowTitle(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestCleanWindowTitleEdgeZeroWidth proves the fix for the reported Edge
// mismatch: the suffix only matched once the zero-width space was removed.
func TestCleanWindowTitleEdgeZeroWidth(t *testing.T) {
	withZWS := "Stack Overflow\u200b - Microsoft\u200b Edge"
	got := cleanWindowTitle(withZWS, "msedge.exe")
	if got != "Stack Overflow" {
		t.Errorf("cleanWindowTitle = %q; want %q", got, "Stack Overflow")
	}
}

// TestCleanWindowTitleCollapsesURLTitles proves a loading or redirecting tab
// is recorded as its host, not as the raw tracking link.
func TestCleanWindowTitleCollapsesURLTitles(t *testing.T) {
	tests := []struct {
		in   string
		exe  string
		want string
	}{
		{"bing.com/ck/a?!&&p=f7e19469 - Microsoft Edge", "msedge.exe", "bing.com"},
		{"https://www.bing.com/search?q=focusd - Google Chrome", "chrome.exe", "www.bing.com"},
		{"Docs / Home - Microsoft Edge", "msedge.exe", "Docs / Home"},
		{"about:blank - Microsoft Edge", "msedge.exe", "about:blank"},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := cleanWindowTitle(tc.in, tc.exe); got != tc.want {
				t.Errorf("cleanWindowTitle(%q, %q) = %q; want %q", tc.in, tc.exe, got, tc.want)
			}
		})
	}
}

func TestIsKnownBrowserSuffixAnchored(t *testing.T) {
	shouldMatch := []string{"google chrome", "chrome", "firefox", "edge", "brave", "opera", "vivaldi"}
	for _, s := range shouldMatch {
		if !isKnownBrowserSuffix(s) {
			t.Errorf("expected %q to be a browser suffix", s)
		}
	}

	// These must NOT match: matching is on the whole segment, not a substring.
	shouldNotMatch := []string{"search", "leetcode", "architecture", "my documents", "inbox"}
	for _, s := range shouldNotMatch {
		if isKnownBrowserSuffix(s) {
			t.Errorf("expected %q not to be a browser suffix", s)
		}
	}
}

func TestLooksLikeProfileSegment(t *testing.T) {
	shouldMatch := []string{"profile", "profile 3", "work", "personal", "private", "person 2", "guest"}
	for _, s := range shouldMatch {
		if !looksLikeProfileSegment(s) {
			t.Errorf("expected %q to look like a profile segment", s)
		}
	}

	shouldNotMatch := []string{"inbox", "github", "my long page title", "", "weekly report"}
	for _, s := range shouldNotMatch {
		if looksLikeProfileSegment(s) {
			t.Errorf("expected %q not to look like a profile segment", s)
		}
	}
}

func TestBaseExeName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"chrome.exe", "chrome"},
		{"CHROME.EXE", "chrome"},
		{"chrome", "chrome"},
		{"  Chrome.exe  ", "chrome"},
	}

	for _, tc := range tests {
		if got := baseExeName(tc.in); got != tc.want {
			t.Errorf("baseExeName(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}
