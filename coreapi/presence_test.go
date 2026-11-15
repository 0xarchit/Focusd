package coreapi

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"focusd/system"
)

const testPresenceID = "00112233445566778899aabbccddeeff"

func stubPresenceIdentity(t *testing.T) {
	t.Helper()
	original := readPresenceOwner
	t.Cleanup(func() { readPresenceOwner = original })
	readPresenceOwner = func(pid int) (string, uint64, error) {
		return `C:\Focusd\focusd.exe`, uint64(pid) + 1000, nil
	}
}

func TestTUIPresencePublishesBeforeDaemonAndExpiresByElapsedTime(t *testing.T) {
	setupTempAppData(t)
	stubPresenceIdentity(t)
	sample := system.ClockSample{BootID: "boot-1", InterruptTicks: 500_000_000}
	if err := PublishTUIPresence(testPresenceID, sample); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadEndpoint(); !os.IsNotExist(err) {
		t.Fatalf("unexpected daemon endpoint: %v", err)
	}
	active, err := ReadTUIPresences(sample)
	if err != nil || len(active) != 1 || active[0].ClientID != testPresenceID {
		t.Fatalf("active=%+v err=%v", active, err)
	}
	sample.InterruptTicks += tuiLeaseTicks
	if active, err := ReadTUIPresences(sample); err != nil || len(active) != 1 {
		t.Fatalf("boundary active=%+v err=%v", active, err)
	}
	sample.InterruptTicks++
	if active, err := ReadTUIPresences(sample); err != nil || len(active) != 0 {
		t.Fatalf("expired active=%+v err=%v", active, err)
	}
	path, _ := presencePath(testPresenceID)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("read removed expired record: %v", err)
	}
	if err := RemoveTUIPresence(testPresenceID); err != nil {
		t.Fatal(err)
	}
	if err := RemoveTUIPresence(testPresenceID); err != nil {
		t.Fatal(err)
	}
}

func TestTUIPresenceIgnoresOtherBootAndReusedPID(t *testing.T) {
	setupTempAppData(t)
	stubPresenceIdentity(t)
	sample := system.ClockSample{BootID: "boot-1", InterruptTicks: 500_000_000}
	if err := PublishTUIPresence(testPresenceID, sample); err != nil {
		t.Fatal(err)
	}
	for _, other := range []system.ClockSample{{BootID: "boot-2", InterruptTicks: sample.InterruptTicks}, {BootID: "", InterruptTicks: sample.InterruptTicks}, {BootID: sample.BootID, InterruptTicks: sample.InterruptTicks - 1}} {
		if active, err := ReadTUIPresences(other); err != nil || len(active) != 0 {
			t.Fatalf("active=%+v err=%v", active, err)
		}
	}
	readPresenceOwner = func(pid int) (string, uint64, error) { return `C:\Focusd\focusd.exe`, uint64(pid) + 2000, nil }
	if active, err := ReadTUIPresences(sample); err != nil || len(active) != 0 {
		t.Fatalf("reused PID active=%+v err=%v", active, err)
	}
	if err := RemoveTUIPresence(testPresenceID); err == nil {
		t.Fatal("new process identity removed an old process lease")
	}
}

func TestTUIPresenceRejectsInvalidClientIDsAndUnknownBoot(t *testing.T) {
	setupTempAppData(t)
	stubPresenceIdentity(t)
	for _, id := range []string{"", "../profile", strings.Repeat("a", 33), strings.ToUpper(testPresenceID), "client-id"} {
		if err := PublishTUIPresence(id, system.ClockSample{BootID: "boot-1"}); err == nil {
			t.Errorf("invalid id %q accepted", id)
		}
	}
	if err := PublishTUIPresence(testPresenceID, system.ClockSample{}); err == nil {
		t.Fatal("unknown boot published a fresh lease")
	}
}

func TestTUIPresenceRejectsUnprotectedAndMismatchedRecords(t *testing.T) {
	setupTempAppData(t)
	stubPresenceIdentity(t)
	sample := system.ClockSample{BootID: "boot-1", InterruptTicks: 500_000_000}
	if err := PublishTUIPresence(testPresenceID, sample); err != nil {
		t.Fatal(err)
	}
	path, _ := presencePath(testPresenceID)
	presence, err := readPresenceFile(path)
	if err != nil {
		t.Fatal(err)
	}
	presence.ClientID = "ffeeddccbbaa99887766554433221100"
	if err := system.WritePrivateFileAtomic(path, func(w io.Writer) error { return json.NewEncoder(w).Encode(presence) }); err != nil {
		t.Fatal(err)
	}
	if active, err := ReadTUIPresences(sample); err != nil || len(active) != 0 {
		t.Fatalf("mismatched filename active=%+v err=%v", active, err)
	}
	unprotected := filepath.Join(filepath.Dir(path), "1234567890abcdef1234567890abcdef.json")
	if err := os.WriteFile(unprotected, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readPresenceFile(unprotected); err == nil {
		t.Fatal("unprotected file accepted as owned presence")
	}
}

func TestTUIPresenceRejectsOversizedInventory(t *testing.T) {
	setupTempAppData(t)
	dir, _ := presenceDirectory()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for i := 0; i <= maxTUIPresences; i++ {
		if err := os.WriteFile(filepath.Join(dir, strings.Repeat("a", i+1)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ReadTUIPresences(system.ClockSample{BootID: "boot-1"}); err == nil {
		t.Fatal("oversized inventory accepted")
	}
}
