package system

import (
	"fmt"
	"os"
	"testing"

	"golang.org/x/sys/windows/registry"
)

// TestEnsureToastAppIDWritesRegistration verifies the registry entry Windows
// needs before it will display toasts for an AppUserModelID. Unregistered ids
// are dropped silently, which made every alert invisible on fresh installs.
func TestEnsureToastAppIDWritesRegistration(t *testing.T) {
	appID := fmt.Sprintf("Focusd.Test.%d", os.Getpid())
	keyPath := toastAppIDKeyBase + `\` + appID
	t.Cleanup(func() { _ = registry.DeleteKey(registry.CURRENT_USER, keyPath) })

	if err := EnsureToastAppID(appID, "Focusd Test", `C:\focusd.exe`); err != nil {
		t.Fatalf("EnsureToastAppID: %v", err)
	}

	key, err := registry.OpenKey(registry.CURRENT_USER, keyPath, registry.QUERY_VALUE)
	if err != nil {
		t.Fatalf("registration key missing: %v", err)
	}
	defer key.Close()

	name, _, err := key.GetStringValue("DisplayName")
	if err != nil || name != "Focusd Test" {
		t.Errorf("DisplayName = %q (err %v); want %q", name, err, "Focusd Test")
	}
	icon, _, err := key.GetStringValue("IconUri")
	if err != nil || icon != `C:\focusd.exe` {
		t.Errorf("IconUri = %q (err %v); want %q", icon, err, `C:\focusd.exe`)
	}
}
