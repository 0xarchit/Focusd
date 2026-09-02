package coreapi

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func setupTempAppData(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	original, hadOriginal := os.LookupEnv("APPDATA")
	if err := os.Setenv("APPDATA", dir); err != nil {
		t.Fatalf("failed to set APPDATA: %v", err)
	}
	t.Cleanup(func() {
		if hadOriginal {
			os.Setenv("APPDATA", original)
		} else {
			os.Unsetenv("APPDATA")
		}
	})
	return dir
}

func TestParseIPCCommand(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantCmd string
		wantTok string
		wantErr bool
	}{
		{"stop with token", "stop abc123\n", "stop", "abc123", false},
		{"flush with token", "flush abc123", "flush", "abc123", false},
		{"stop without token is rejected", "stop\n", "", "", true},
		{"flush without token is rejected", "flush\n", "", "", true},
		{"bare stop word without token", "stop", "", "", true},
		{"unknown command without token", "bogus\n", "bogus", "", false},
		{"unknown command with token is rejected", "bogus abc\n", "", "", true},
		{"trailing data is rejected", "stop abc extra\n", "", "", true},
		{"empty payload is rejected", "\n", "", "", true},
		{"whitespace only is rejected", "   \n", "", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd, tok, err := ParseIPCCommand(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got cmd=%q token=%q", cmd, tok)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cmd != tc.wantCmd {
				t.Errorf("cmd = %q; want %q", cmd, tc.wantCmd)
			}
			if tok != tc.wantTok {
				t.Errorf("token = %q; want %q", tok, tc.wantTok)
			}
		})
	}
}

func TestTokensMatch(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
		ok   bool
	}{
		{"identical", "abc123", "abc123", true},
		{"different", "abc123", "def456", false},
		{"prefix is not a match", "abc", "abc123", false},
		{"longer is not a match", "abc123", "abc", false},
		{"empty received", "", "abc123", false},
		{"empty expected", "abc123", "", false},
		{"both empty", "", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := TokensMatch(tc.got, tc.want); got != tc.ok {
				t.Errorf("TokensMatch(%q, %q) = %v; want %v", tc.got, tc.want, got, tc.ok)
			}
		})
	}
}

func TestIPCCommandNeedsToken(t *testing.T) {
	if !IPCCommandNeedsToken("stop") {
		t.Error("stop must require a token")
	}
	if !IPCCommandNeedsToken("flush") {
		t.Error("flush must require a token")
	}
	if IPCCommandNeedsToken("status") {
		t.Error("read-only commands must not require a token")
	}
}

func TestEncodeIPCCmd(t *testing.T) {
	if got := encodeIPCCmd("stop", "tok"); got != "stop tok" {
		t.Errorf("encodeIPCCmd(stop) = %q; want %q", got, "stop tok")
	}
	if got := encodeIPCCmd("status", "tok"); got != "status" {
		t.Errorf("encodeIPCCmd(status) = %q; want %q", got, "status")
	}
}

func TestEnsureIPCTokenIsStableAndPrivate(t *testing.T) {
	dir := setupTempAppData(t)

	first, err := EnsureIPCToken()
	if err != nil {
		t.Fatalf("EnsureIPCToken failed: %v", err)
	}
	if len(first) != ipcTokenBytes*2 {
		t.Errorf("token length = %d; want %d hex chars", len(first), ipcTokenBytes*2)
	}

	second, err := EnsureIPCToken()
	if err != nil {
		t.Fatalf("second EnsureIPCToken failed: %v", err)
	}
	if first != second {
		t.Error("token changed between calls")
	}

	tokenPath := filepath.Join(dir, "focusd", ipcTokenFile)
	if _, err := os.Stat(tokenPath); err != nil {
		t.Fatalf("token file missing: %v", err)
	}

	// The 0600 mode argument only takes effect on non-Windows platforms. On
	// Windows the file inherits the ACL of %APPDATA%, which is per-user and
	// not readable by other accounts by default. Asserting POSIX bits here
	// would be asserting something Windows does not implement.
	if runtime.GOOS != "windows" {
		info, err := os.Stat(tokenPath)
		if err != nil {
			t.Fatalf("stat failed: %v", err)
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Errorf("token file is readable beyond the owner: mode %v", info.Mode().Perm())
		}
	}
}

func TestEnsureIPCTokenDistinctPerInstall(t *testing.T) {
	setupTempAppData(t)
	first, err := EnsureIPCToken()
	if err != nil {
		t.Fatalf("EnsureIPCToken failed: %v", err)
	}

	// A second install directory means a different token.
	os.Setenv("APPDATA", t.TempDir())
	second, err := EnsureIPCToken()
	if err != nil {
		t.Fatalf("EnsureIPCToken failed: %v", err)
	}
	if first == second {
		t.Error("expected distinct tokens for distinct install directories")
	}
}

func TestLoadIPCTokenTrimsWhitespace(t *testing.T) {
	dir := setupTempAppData(t)

	installDir := filepath.Join(dir, "focusd")
	if err := os.MkdirAll(installDir, 0700); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	want := "0123456789abcdef"
	if err := os.WriteFile(filepath.Join(installDir, ipcTokenFile), []byte("  "+want+"\r\n"), 0600); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	got, err := loadIPCToken()
	if err != nil {
		t.Fatalf("loadIPCToken failed: %v", err)
	}
	if got != want {
		t.Errorf("loadIPCToken() = %q; want %q", got, want)
	}
}

func TestLoadIPCTokenMissing(t *testing.T) {
	setupTempAppData(t)

	if _, err := loadIPCToken(); err == nil {
		t.Error("expected an error when no token file exists")
	}
}

func TestReadIPCTokenEmptyFile(t *testing.T) {
	setupTempAppData(t)

	installDir := filepath.Join(t.TempDir(), "focusd")
	if err := os.MkdirAll(installDir, 0700); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	path := filepath.Join(installDir, ipcTokenFile)
	if err := os.WriteFile(path, []byte("   \n"), 0600); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if _, err := readIPCToken(path); err == nil {
		t.Error("expected an error for an empty token file")
	}
}

// TestParseRejectsBareStop covers the original unauthenticated protocol: a bare
// "stop" on the loopback port must no longer be accepted.
func TestParseRejectsBareStop(t *testing.T) {
	if _, _, err := ParseIPCCommand("stop\n"); err == nil {
		t.Fatal("bare stop must be rejected")
	}
	if _, _, err := ParseIPCCommand("flush\n"); err == nil {
		t.Fatal("bare flush must be rejected")
	}
	if _, _, err := ParseIPCCommand("stop " + strings.Repeat("f", 64) + "\n"); err != nil {
		t.Fatalf("tokenised stop should parse: %v", err)
	}
}
