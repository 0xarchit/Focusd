//go:build windows

package system

import (
	"errors"
	"testing"
)

func TestProfileLockExcludesSecondWriterAndReleasesOnClose(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	first, err := AcquireProfileLock()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { first.Close() })
	if second, err := AcquireProfileLock(); !errors.Is(err, ErrProfileInUse) {
		if second != nil {
			second.Close()
		}
		t.Fatalf("second writer error=%v; want profile-in-use", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := AcquireProfileLock()
	if err != nil {
		t.Fatalf("lock survived close: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
}
