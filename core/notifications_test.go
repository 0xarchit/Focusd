package core

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"
	"unicode/utf8"
)

// TestTruncateRunesKeepsValidUTF8 checks truncation never splits a multi-byte
// rune, which would produce malformed toast XML.
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

// TestToastEnvPassesRawValues pins the contract with the PowerShell script:
// LoadXml needs the literal template, and title/message reach the XML through
// InnerText, which escapes on its own. Escaping here killed every toast.
func TestToastEnvPassesRawValues(t *testing.T) {
	title := "Focus & <limits>"
	message := `"Time's up" & co`
	env := toastEnv(title, message, toastXMLTemplate)

	values := map[string]string{}
	for _, kv := range env {
		if key, val, ok := strings.Cut(kv, "="); ok {
			switch key {
			case "FOCUS_TITLE", "FOCUS_MSG", "FOCUS_TOAST_XML":
				values[key] = val
			}
		}
	}

	if values["FOCUS_TITLE"] != title {
		t.Errorf("FOCUS_TITLE = %q; want raw %q", values["FOCUS_TITLE"], title)
	}
	if values["FOCUS_MSG"] != message {
		t.Errorf("FOCUS_MSG = %q; want raw %q", values["FOCUS_MSG"], message)
	}
	if values["FOCUS_TOAST_XML"] != toastXMLTemplate {
		t.Errorf("FOCUS_TOAST_XML = %q; want the literal template", values["FOCUS_TOAST_XML"])
	}
}

// TestToastXMLIsWellFormed parses the templates the way LoadXml does: any
// stray character is a load error, and a failed load means no toast at all.
func TestToastXMLIsWellFormed(t *testing.T) {
	for name, tmpl := range map[string]string{
		"plain":  toastXMLTemplate,
		"action": actionToastXMLTemplate,
	} {
		t.Run(name, func(t *testing.T) {
			dec := xml.NewDecoder(strings.NewReader(tmpl))
			for {
				if _, err := dec.Token(); err != nil {
					if err == io.EOF {
						break
					}
					t.Fatalf("template is not well-formed XML: %v", err)
				}
			}
		})
	}
}

// TestToastScriptStopsOnError keeps the failure path loud: under the default
// Continue preference a LoadXml failure would not reliably set a non-zero exit
// code, so runToast would have nothing to log.
func TestToastScriptStopsOnError(t *testing.T) {
	if !strings.Contains(plainToastScript, "$ErrorActionPreference = 'Stop'") {
		t.Error("toast script must fail fast so runToast can log the failure")
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
