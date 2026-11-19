package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"focusd/system"
)

func TestSettingsCommandsPersistAndRemoveValues(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	var output bytes.Buffer
	commands := [][]string{
		{"privacy", "set", "strict"},
		{"idle", "set", "17"},
		{"track-while-idle", "enable"},
		{"display", "set", "code|aabb", "Work Editor"},
		{"site", "set", "github", "Work GitHub"},
	}
	for _, args := range commands {
		if err := settingsCommand(args, &output); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	if system.GetPrivacyMode() != system.PrivacyModeStrict || system.GetIdleThresholdMinutes() != 17 || !system.GetTrackWhileIdle() {
		t.Fatal("CLI settings did not reach config")
	}
	if system.GetDisplayOverride("code|aabb") != "Work Editor" || system.GetSiteOverrides()["github"] != "Work GitHub" {
		t.Fatal("CLI labels did not reach config")
	}
	if err := settingsCommand([]string{"display", "remove", "code|aabb"}, &output); err != nil {
		t.Fatal(err)
	}
	if err := settingsCommand([]string{"site", "remove", "github"}, &output); err != nil {
		t.Fatal(err)
	}
	if len(system.GetDisplayOverrides()) != 0 || len(system.GetSiteOverrides()) != 0 {
		t.Fatal("CLI removals left labels")
	}
}

func TestSettingsCommandInvalidInputDoesNotCreateConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	for _, args := range [][]string{{"privacy", "set", "unknown"}, {"idle", "set", "-1"}, {"idle", "set", "241"}, {"idle", "set", "abc"}, {"track-while-idle", "true"}, {"display", "set", "code"}, {"site", "set", "github", ""}, {"privacy", "status", "extra"}} {
		var output bytes.Buffer
		if err := settingsCommand(args, &output); err == nil {
			t.Fatalf("accepted invalid input %v", args)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "focusd", "config.json")); !os.IsNotExist(err) {
		t.Fatalf("invalid commands created config: %v", err)
	}
}

func TestSettingsListsAndStatusAreDeterministic(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	var output bytes.Buffer
	for _, args := range [][]string{{"site", "set", "zeta", "Z"}, {"site", "set", "alpha", "A"}} {
		if err := settingsCommand(args, &output); err != nil {
			t.Fatal(err)
		}
	}
	output.Reset()
	if err := settingsCommand([]string{"site", "list"}, &output); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != "alpha: A\nzeta: Z\n" {
		t.Fatalf("site list = %q", got)
	}
	output.Reset()
	if err := settingsCommand([]string{"privacy"}, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "titles") {
		t.Fatalf("privacy status = %q", output.String())
	}
}
