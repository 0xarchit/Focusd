package system

import (
	"os"
	"path/filepath"
	"testing"
)

func currentPID() int {
	return os.Getpid()
}

// TestSameDir covers the matching rule that keeps the update flow from
// terminating an unrelated focusd.exe that merely shares the file name.
// The kill path itself is never exercised in tests; only this predicate is.
func TestSameDir(t *testing.T) {
	tests := []struct {
		name    string
		exePath string
		dir     string
		want    bool
	}{
		{"exact match", `C:\Users\me\AppData\Roaming\focusd\focusd.exe`, `C:\Users\me\AppData\Roaming\focusd`, true},
		{"case insensitive", `c:\users\me\appdata\roaming\focusd\focusd.exe`, `C:\Users\me\AppData\Roaming\focusd`, true},
		{"trailing separator in dir", `C:\a\focusd.exe`, `C:\a\`, true},
		{"forward slash in dir", `C:\a\focusd.exe`, `C:/a`, true},
		{"different dir", `C:\other\focusd.exe`, `C:\Users\me\AppData\Roaming\focusd`, false},
		{"sibling with same suffix", `C:\Users\me\AppData\Roaming\focusd-old\focusd.exe`, `C:\Users\me\AppData\Roaming\focusd`, false},
		{"parent dir is not a match", `C:\Users\me\focusd.exe`, `C:\Users\me\AppData\Roaming\focusd`, false},
		{"unresolved exe path", ``, `C:\a`, false},
		{"empty dir", `C:\a\focusd.exe`, ``, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := sameDir(tc.exePath, tc.dir); got != tc.want {
				t.Errorf("sameDir(%q, %q) = %v; want %v", tc.exePath, tc.dir, got, tc.want)
			}
		})
	}
}

func TestNormalizeDir(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`C:\A`, `c:\a`},
		{`C:\A\`, `c:\a`},
		{`C:\A\\`, `c:\a`},
		{`C:/A`, `c:\a`},
		{`  C:\A  `, `c:\a`},
		{``, ``},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			if got := normalizeDir(tc.input); got != tc.want {
				t.Errorf("normalizeDir(%q) = %q; want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestGetProcessPathSelf checks the path lookup against a process that is
// guaranteed to exist and be readable: this one.
func TestGetProcessPathSelf(t *testing.T) {
	path, err := getProcessPath(uint32(currentPID()))
	if err != nil {
		t.Skipf("process path unavailable for self: %v", err)
	}
	if path == "" {
		t.Fatal("expected a non-empty image path for the current process")
	}
	if !sameDir(path, filepath.Dir(path)) {
		t.Errorf("path %q does not match its own directory", path)
	}
}
