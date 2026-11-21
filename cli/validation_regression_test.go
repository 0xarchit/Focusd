package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"focusd/coreapi"
	"focusd/storage"
	"focusd/system"
)

func TestCLIValidationHelper(t *testing.T) {
	switch os.Getenv("FOCUSD_CLI_VALIDATION_HELPER") {
	case "focus":
		runFocus([]string{"focusd", "focus", os.Getenv("FOCUSD_TEST_ARGUMENT")})
	case "retention":
		runRetentionSet(os.Getenv("FOCUSD_TEST_ARGUMENT"))
	default:
		return
	}
}

func runValidationCLI(t *testing.T, command, argument, appData string) (string, error) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestCLIValidationHelper$")
	cmd.Env = append(os.Environ(), "APPDATA="+appData,
		"FOCUSD_CLI_VALIDATION_HELPER="+command, "FOCUSD_TEST_ARGUMENT="+argument)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func TestRunFocusRejectsInvalidDurationWithoutStartingTimer(t *testing.T) {
	for _, argument := range []string{"abc", "0", "-1"} {
		t.Run(argument, func(t *testing.T) {
			appData := t.TempDir()
			output, err := runValidationCLI(t, "focus", argument, appData)
			if err == nil {
				t.Errorf("invalid duration %q exited successfully: %s", argument, output)
			}
			t.Setenv("APPDATA", appData)
			storage.Close()
			t.Cleanup(func() { storage.Close() })
			if err := storage.Init(); err != nil {
				t.Fatal(err)
			}
			state, _ := coreapi.LoadPomodoroStateRaw()
			if state.Active {
				t.Errorf("invalid duration %q started a timer", argument)
			}
		})
	}
}

func TestRunFocusUsesConfiguredDefault(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	storage.Close()
	t.Cleanup(func() { storage.Close() })
	if err := system.SetPomodoroMinutes(37); err != nil {
		t.Fatal(err)
	}
	runFocus([]string{"focusd", "focus"})
	if err := storage.Init(); err != nil {
		t.Fatal(err)
	}
	state, _ := coreapi.LoadPomodoroStateRaw()
	if !state.Active || state.Duration != 37 {
		t.Fatalf("timer = active %v, duration %d; want active 37 minutes", state.Active, state.Duration)
	}
}

func TestRunFocusSupportsMaximumAndClampsAboveIt(t *testing.T) {
	for _, argument := range []string{"1440", "1441"} {
		t.Run(argument, func(t *testing.T) {
			t.Setenv("APPDATA", t.TempDir())
			storage.Close()
			t.Cleanup(func() { storage.Close() })
			runFocus([]string{"focusd", "focus", argument})
			if err := storage.Init(); err != nil {
				t.Fatal(err)
			}
			state, _ := coreapi.LoadPomodoroStateRaw()
			if !state.Active || state.Duration != coreapi.MaxPomodoroMinutes {
				t.Fatalf("duration %q became active %v, minutes %d", argument, state.Active, state.Duration)
			}
		})
	}
}

func TestRetentionInvalidInputReportsActualSupportedRange(t *testing.T) {
	output, err := runValidationCLI(t, "retention", "abc", t.TempDir())
	if err == nil {
		t.Fatal("non-numeric retention exited successfully")
	}
	want := fmt.Sprintf("between %d and %d", storage.MinRetentionDays, storage.MaxRetentionDays)
	if !strings.Contains(output, want) {
		t.Fatalf("output %q does not contain %q", output, want)
	}
}

func TestCompareVersionsSemVerIdentifiers(t *testing.T) {
	tests := []struct {
		name, a, b string
		want       int
	}{
		{"metadata with hyphen", "1.2.3+meta-build", "1.2.3", 0},
		{"prerelease metadata ignored", "1.2.3-rc.2+build.9", "1.2.3-rc.2+other", 0},
		{"numeric prerelease", "1.2.3-rc.10", "1.2.3-rc.2", 1},
		{"large numeric prerelease", "1.2.3-999999999999999999999999", "1.2.3-99999999999999999999999", 1},
		{"numeric before alphanumeric", "1.2.3-10", "1.2.3-alpha", -1},
		{"additional identifier", "1.2.3-alpha.1", "1.2.3-alpha", 1},
		{"alphanumeric retains lexical precedence", "1.2.3-rc10", "1.2.3-rc2", -1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := compareVersions(tc.a, tc.b); got != tc.want {
				t.Errorf("compareVersions(%q, %q) = %d; want %d", tc.a, tc.b, got, tc.want)
			}
			if got := compareVersions(tc.b, tc.a); got != -tc.want {
				t.Errorf("reverse comparison = %d; want %d", got, -tc.want)
			}
		})
	}
}
