package system

import (
	"strings"
	"testing"
)

func TestSplitPathEntries(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty", "", nil},
		{"whitespace only", "   ", nil},
		{"single", `C:\Tools`, []string{`C:\Tools`}},
		{"multiple", `C:\A;C:\B`, []string{`C:\A`, `C:\B`}},
		{"trims spaces", ` C:\A ; C:\B `, []string{`C:\A`, `C:\B`}},
		{"drops empty segments", `C:\A;;C:\B;`, []string{`C:\A`, `C:\B`}},
		{"keeps var refs intact", `%USERPROFILE%\go\bin;C:\A`, []string{`%USERPROFILE%\go\bin`, `C:\A`}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := splitPathEntries(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("splitPathEntries(%q) = %v; want %v", tc.input, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("splitPathEntries(%q)[%d] = %q; want %q", tc.input, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestPathContains(t *testing.T) {
	tests := []struct {
		name  string
		value string
		entry string
		want  bool
	}{
		{"exact", `C:\A;C:\B`, `C:\B`, true},
		{"case insensitive", `C:\A;C:\B`, `c:\b`, true},
		{"absent", `C:\A`, `C:\B`, false},
		{"substring must not match", `C:\Apps\focusd`, `C:\Apps`, false},
		{"empty value", ``, `C:\A`, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := pathContains(tc.value, tc.entry); got != tc.want {
				t.Errorf("pathContains(%q, %q) = %v; want %v", tc.value, tc.entry, got, tc.want)
			}
		})
	}
}

func TestPathAdd(t *testing.T) {
	tests := []struct {
		name  string
		value string
		entry string
		want  string
	}{
		{"into empty", ``, `C:\focusd`, `C:\focusd`},
		{"into whitespace", `  `, `C:\focusd`, `C:\focusd`},
		{"appends with separator", `C:\A`, `C:\focusd`, `C:\A;C:\focusd`},
		{"normalizes trailing semicolon", `C:\A;`, `C:\focusd`, `C:\A;C:\focusd`},
		{"idempotent when present", `C:\A;C:\focusd`, `C:\focusd`, `C:\A;C:\focusd`},
		{"idempotent ignores case", `C:\A;c:\focusd`, `C:\focusd`, `C:\A;c:\focusd`},
		{"preserves existing entries verbatim", `%USERPROFILE%\go\bin;C:\A`, `C:\focusd`, `%USERPROFILE%\go\bin;C:\A;C:\focusd`},
		{"empty entry is a no-op", `C:\A`, `   `, `C:\A`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := pathAdd(tc.value, tc.entry); got != tc.want {
				t.Errorf("pathAdd(%q, %q) = %q; want %q", tc.value, tc.entry, got, tc.want)
			}
		})
	}
}

func TestPathRemove(t *testing.T) {
	tests := []struct {
		name  string
		value string
		entry string
		want  string
	}{
		{"only entry", `C:\focusd`, `C:\focusd`, ``},
		{"from middle", `C:\A;C:\focusd;C:\B`, `C:\focusd`, `C:\A;C:\B`},
		{"from start", `C:\focusd;C:\A`, `C:\focusd`, `C:\A`},
		{"from end", `C:\A;C:\focusd`, `C:\focusd`, `C:\A`},
		{"ignores case", `C:\A;C:\FOCUSD`, `C:\focusd`, `C:\A`},
		{"absent is a no-op", `C:\A;C:\B`, `C:\focusd`, `C:\A;C:\B`},
		{"removes every duplicate", `C:\A;C:\focusd;C:\focusd;C:\B`, `C:\focusd`, `C:\A;C:\B`},
		{"substring must not match", `C:\Apps\focusd;C:\A`, `C:\Apps`, `C:\Apps\focusd;C:\A`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := pathRemove(tc.value, tc.entry); got != tc.want {
				t.Errorf("pathRemove(%q, %q) = %q; want %q", tc.value, tc.entry, got, tc.want)
			}
		})
	}
}

// TestPathRemovePreservesUserEntries is the regression guard for issue #18:
// removing the focusd entry must never cost the user another entry.
func TestPathRemovePreservesUserEntries(t *testing.T) {
	original := `%USERPROFILE%\go\bin;C:\Python312;C:\Program Files\Git\cmd;C:\focusd;C:\Users\me\bin`

	after := pathRemove(original, `C:\focusd`)

	if !pathRetainsAll(original, after, `C:\focusd`) {
		t.Fatalf("pathRemove dropped pre-existing entries: %q -> %q", original, after)
	}
	if pathContains(after, `C:\focusd`) {
		t.Errorf("focusd entry still present after removal: %q", after)
	}
	if after == original {
		t.Errorf("pathRemove returned the value unchanged")
	}
}

func TestPathRemoveNotPresentRetainsAll(t *testing.T) {
	original := `%USERPROFILE%\go\bin;C:\Python312`

	after := pathRemove(original, `C:\focusd`)

	if after != original {
		t.Errorf("pathRemove of an absent entry changed the value: %q -> %q", original, after)
	}
	if !pathRetainsAll(original, after, `C:\focusd`) {
		t.Errorf("pathRetainsAll rejected a valid no-op result")
	}
}

// TestPathRoundTrip covers the install/uninstall cycle #18 reported as lossy.
func TestPathRoundTrip(t *testing.T) {
	original := `%USERPROFILE%\go\bin;C:\Python312;C:\Users\me\bin`

	withFocusd := pathAdd(original, `C:\focusd`)
	if !pathRetainsAll(original, withFocusd, ``) {
		t.Fatalf("pathAdd dropped pre-existing entries: %q -> %q", original, withFocusd)
	}
	if !pathContains(withFocusd, `C:\focusd`) {
		t.Fatalf("pathAdd did not add the focusd entry: %q", withFocusd)
	}

	restored := pathRemove(withFocusd, `C:\focusd`)
	if restored != original {
		t.Errorf("round trip lost data: want %q, got %q", original, restored)
	}
}

// TestPathWriteAllowed is the guard that would have stopped #18 from writing a
// PATH that dropped the user's entries.
func TestPathWriteAllowed(t *testing.T) {
	before := `%USERPROFILE%\go\bin;C:\Python312;C:\Users\me\bin`

	tests := []struct {
		name    string
		before  string
		after   string
		removed string
		want    bool
	}{
		{"add keeps everything", before, pathAdd(before, `C:\focusd`), "", true},
		{"remove only focusd", before + `;C:\focusd`, before, `C:\focusd`, true},
		{"remove when absent is a no-op", before, before, `C:\focusd`, true},
		{"rejects cleared path", before, ``, "", false},
		{"rejects path holding only focusd", before, `C:\focusd`, "", false},
		{"rejects dropped user entry", before, `C:\focusd`, "", false},
		{"rejects dropping one of many", before, `C:\focusd;C:\Python312`, "", false},
		{"empty before allows anything", ``, `C:\focusd`, "", true},
		{"removing the sole entry is allowed", `C:\focusd`, ``, `C:\focusd`, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := pathWriteAllowed(tc.before, tc.after, tc.removed); got != tc.want {
				t.Errorf("pathWriteAllowed(%q -> %q, removed %q) = %v; want %v",
					tc.before, tc.after, tc.removed, got, tc.want)
			}
		})
	}
}

// TestPathWriteAllowedPreservesVarRefs guards the specific #18 symptom: a PATH
// that still holds every entry but lost its %VAR% expansion.
func TestPathWriteAllowedPreservesVarRefs(t *testing.T) {
	before := `%USERPROFILE%\go\bin;C:\focusd`
	after := pathRemove(before, `C:\focusd`)

	if !pathWriteAllowed(before, after, `C:\focusd`) {
		t.Fatalf("refused a legitimate removal: %q -> %q", before, after)
	}
	if !strings.Contains(after, `%USERPROFILE%`) {
		t.Errorf("removal lost the variable reference: %q", after)
	}
}
