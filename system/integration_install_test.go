package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRequireInstalledBinaryNeverCopiesPortableFiles(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	if err := requireInstalledBinary("focusd.exe"); err == nil || !strings.Contains(err.Error(), "install Focusd") {
		t.Fatalf("missing installation error = %v", err)
	}
	if _, err := os.Stat(getInstallDir()); !os.IsNotExist(err) {
		t.Fatalf("installation check created a directory: %v", err)
	}
}

func TestRequireInstalledBinaryPreservesExistingCopy(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	if err := os.MkdirAll(getInstallDir(), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(getInstallDir(), "focusd.exe")
	if err := os.WriteFile(path, []byte("installed version"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := requireInstalledBinary("focusd.exe"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "installed version" {
		t.Fatalf("installed binary changed: %q, %v", data, err)
	}
}

func TestIntegrationRequiresBinaryRatherThanDirectory(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	if err := os.MkdirAll(filepath.Join(getInstallDir(), "focusd.exe"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := requireInstalledBinary("focusd.exe"); err == nil {
		t.Fatal("directory accepted as executable")
	}
}
