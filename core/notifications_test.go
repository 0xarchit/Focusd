package core

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestTruncateRunesKeepsValidUTF8 is the regression guard for the byte-slice
// truncation, which split multi-byte runes and produced malformed toast XML.
func TestTruncateRunesKeepsValidUTF8(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		max      int
		wantRune int
	}{
		{"short ascii", "hello", 10, 5},
		{"truncates ascii", "hello world", 5, 5},
		{"emoji boundary", "aaaa😀bbbb", 5, 5},
		{"emoji only", "😀😀😀", 2, 2},
		{"zero", "hello", 0, 0},
		{"negative", "hello", -1, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateRunes(tc.in, tc.max)
			if !utf8.ValidString(got) {
				t.Errorf("truncateRunes(%q, %d) produced invalid UTF-8", tc.in, tc.max)
			}
			if n := utf8.RuneCountInString(got); n != tc.wantRune {
				t.Errorf("rune count = %d; want %d (result %q)", n, tc.wantRune, got)
			}
		})
	}
}

func TestTruncateRunesLongTitle(t *testing.T) {
	long := strings.Repeat("é", 500)
	got := truncateRunes(long, 200)

	if !utf8.ValidString(got) {
		t.Error("result is not valid UTF-8")
	}
	if utf8.RuneCountInString(got) != 200 {
		t.Errorf("rune count = %d; want 200", utf8.RuneCountInString(got))
	}
}

func TestXMLEscape(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"plain", "plain"},
		{"a & b", "a &amp; b"},
		{"<tag>", "&lt;tag&gt;"},
		{`quote " here`, "quote &quot; here"},
		{"it's", "it&apos;s"},
		{"a<b>&c\"d'e", "a&lt;b&gt;&amp;c&quot;d&apos;e"},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := xmlEscape(tc.in); got != tc.want {
				t.Errorf("xmlEscape(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestToastXMLIsWellFormed checks the templates contain no stray characters that
// would break the document.
func TestToastXMLIsWellFormed(t *testing.T) {
	for name, tmpl := range map[string]string{
		"plain":  toastXMLTemplate,
		"action": actionToastXMLTemplate,
	} {
		t.Run(name, func(t *testing.T) {
			if !strings.HasPrefix(tmpl, "<toast>") || !strings.HasSuffix(tmpl, "</toast>") {
				t.Errorf("template is not a single toast element: %q", tmpl)
			}
			if strings.Count(tmpl, "<text") != strings.Count(tmpl, "</text>") {
				t.Errorf("unbalanced text elements: %q", tmpl)
			}
		})
	}
}

func TestToastUsesOwnAppID(t *testing.T) {
	if toastAUMID == "" {
		t.Fatal("toastAUMID must not be empty")
	}
	// The PowerShell GUID made notifications appear under Windows PowerShell.
	if strings.Contains(toastAUMID, "1AC14E77") {
		t.Errorf("toastAUMID still uses the PowerShell AppID: %q", toastAUMID)
	}
	if !strings.Contains(plainToastScript, toastAUMID) {
		t.Error("the toast script must reference the focusd AppID")
	}
}

// TestNotificationRateLimit confirms the throttle still suppresses bursts.
func TestNotificationRateLimit(t *testing.T) {
	resetNotificationState()

	if !notificationRateLimited() {
		t.Error("the first notification should be allowed")
	}
	if notificationRateLimited() {
		t.Error("an immediate second notification should be suppressed")
	}
}
