//go:build !race

package system

import (
	"os"
	"path/filepath"
	"testing"
)

// TestQueryVersionLabelOnRealExecutables checks the version.dll reader against
// binaries that definitely ship version resources. These are read-only probes.
func TestQueryVersionLabelOnRealExecutables(t *testing.T) {
	candidates := []string{
		filepath.Join(os.Getenv("SystemRoot"), "System32", "notepad.exe"),
		filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe"),
	}

	found := 0
	for _, path := range candidates {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err != nil {
			continue
		}
		found++

		label := queryVersionLabel(path)
		if label == "" {
			t.Logf("no version label for %s (may legitimately lack a resource)", path)
			continue
		}
		if isBlocklistedLabel(label) {
			t.Errorf("%s returned blocklisted label %q", path, label)
		}
	}

	if found == 0 {
		t.Skip("no known executables found to probe")
	}
}

func TestQueryVersionLabelMissingFile(t *testing.T) {
	label := queryVersionLabel(filepath.Join(t.TempDir(), "does-not-exist.exe"))
	if label != "" {
		t.Errorf("expected no label for a missing file, got %q", label)
	}
}

func TestReadVersionLabelCaches(t *testing.T) {
	path := filepath.Join(os.Getenv("SystemRoot"), "System32", "notepad.exe")
	if _, err := os.Stat(path); err != nil {
		t.Skip("notepad.exe not available")
	}

	versionInfoMu.Lock()
	versionInfoCache = map[string]string{}
	versionInfoMu.Unlock()

	first := readVersionLabel(path)
	versionInfoMu.Lock()
	_, cached := versionInfoCache[path]
	versionInfoMu.Unlock()

	if first != "" && !cached {
		t.Error("a non-empty label should have been cached")
	}
}

func TestIsBlocklistedLabel(t *testing.T) {
	blocklisted := []string{
		"Microsoft® Windows® Operating System",
		"microsoft windows",
		"WINDOWS",
		"",
		"   ",
	}
	for _, label := range blocklisted {
		if !isBlocklistedLabel(label) {
			t.Errorf("expected %q to be blocklisted", label)
		}
	}

	allowed := []string{"Google Chrome", "Mozilla Firefox", "Visual Studio Code", "Zen"}
	for _, label := range allowed {
		if isBlocklistedLabel(label) {
			t.Errorf("expected %q to be allowed", label)
		}
	}
}

func TestLangHex(t *testing.T) {
	tests := []struct {
		in   uint16
		want string
	}{
		{0x0409, "0409"},
		{0x04B0, "04B0"},
		{0x0000, "0000"},
		{0xFFFF, "FFFF"},
	}
	for _, tc := range tests {
		if got := langHex(tc.in); got != tc.want {
			t.Errorf("langHex(%#04x) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

func TestExpandVersionKey(t *testing.T) {
	got := expandVersionKey("\\StringFileInfo\\%04X%04X\\FileDescription", [2]uint16{0x0409, 0x04B0})
	want := "\\StringFileInfo\\040904B0\\FileDescription"
	if got != want {
		t.Errorf("expandVersionKey() = %q; want %q", got, want)
	}
}
