package system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallFileCreatesDestination(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")

	if err := os.WriteFile(src, []byte("new contents"), 0600); err != nil {
		t.Fatalf("write src failed: %v", err)
	}
	if err := installFile(src, dst); err != nil {
		t.Fatalf("installFile failed: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst failed: %v", err)
	}
	if string(got) != "new contents" {
		t.Errorf("dst = %q; want the source contents", got)
	}
}

// TestInstallFileReplacesExisting covers the update path, which is where the
// previous truncate-in-place version could leave the user with no binary.
func TestInstallFileReplacesExisting(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")

	if err := os.WriteFile(src, []byte("version 2"), 0600); err != nil {
		t.Fatalf("write src failed: %v", err)
	}
	if err := os.WriteFile(dst, []byte("version 1"), 0600); err != nil {
		t.Fatalf("write dst failed: %v", err)
	}

	if err := installFile(src, dst); err != nil {
		t.Fatalf("installFile failed: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst failed: %v", err)
	}
	if string(got) != "version 2" {
		t.Errorf("dst = %q; want version 2", got)
	}

	// No temporary or backup files may be left behind.
	assertNoStrayFiles(t, dir, "dst.txt")
}

// TestInstallFileSourceMissing checks a bad source leaves the destination alone
// rather than truncating it.
func TestInstallFileSourceMissing(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "missing.txt")
	dst := filepath.Join(dir, "dst.txt")

	if err := os.WriteFile(dst, []byte("original"), 0600); err != nil {
		t.Fatalf("write dst failed: %v", err)
	}

	if err := installFile(src, dst); err == nil {
		t.Error("expected an error for a missing source")
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("the destination must survive a failed install: %v", err)
	}
	if string(got) != "original" {
		t.Errorf("dst = %q; want the original contents preserved", got)
	}
}

func TestInstallFileLarge(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	dst := filepath.Join(dir, "dst.bin")

	payload := make([]byte, 512*1024)
	for i := range payload {
		payload[i] = byte(i % 251)
	}
	if err := os.WriteFile(src, payload, 0600); err != nil {
		t.Fatalf("write src failed: %v", err)
	}

	if err := installFile(src, dst); err != nil {
		t.Fatalf("installFile failed: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst failed: %v", err)
	}
	if len(got) != len(payload) {
		t.Fatalf("copied %d bytes; want %d", len(got), len(payload))
	}
	for i := range payload {
		if got[i] != payload[i] {
			t.Fatalf("content differs at byte %d", i)
		}
	}
}

// assertNoStrayFiles fails if staging or backup files remain.
func assertNoStrayFiles(t *testing.T, dir, base string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir failed: %v", err)
	}
	for _, e := range entries {
		if e.Name() == base {
			continue
		}
		if filepath.Ext(e.Name()) == ".old" || filepath.Ext(e.Name()) == ".new" {
			t.Errorf("stray file left behind: %s", e.Name())
		}
	}
}
