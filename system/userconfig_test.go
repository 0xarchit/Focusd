package system

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupConfigEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	original, had := os.LookupEnv("APPDATA")
	if err := os.Setenv("APPDATA", dir); err != nil {
		t.Fatalf("failed to set APPDATA: %v", err)
	}
	t.Cleanup(func() {
		if had {
			os.Setenv("APPDATA", original)
		} else {
			os.Unsetenv("APPDATA")
		}
		userConfig = nil
		lastModTime = time.Time{}
	})
	return dir
}

func TestIdleThresholdDefaults(t *testing.T) {
	setupConfigEnv(t)

	if got := GetIdleThresholdMinutes(); got != DefaultIdleThresholdMinutes {
		t.Errorf("idle threshold = %d; want default %d", got, DefaultIdleThresholdMinutes)
	}
	if GetTrackWhileIdle() {
		t.Error("idle tracking should be off by default so time stops accruing")
	}
}

func TestSetIdleThresholdClamps(t *testing.T) {
	setupConfigEnv(t)

	if err := SetIdleThreshold(12); err != nil {
		t.Fatalf("SetIdleThreshold failed: %v", err)
	}
	if got := GetIdleThresholdMinutes(); got != 12 {
		t.Errorf("idle threshold = %d; want 12", got)
	}

	// Zero disables the cutoff.
	if err := SetIdleThreshold(0); err != nil {
		t.Fatalf("SetIdleThreshold(0) failed: %v", err)
	}
	if got := GetIdleThresholdMinutes(); got != 0 {
		t.Errorf("idle threshold = %d; want 0", got)
	}

	// Negative clamps up to zero rather than becoming a negative cutoff that
	// would make everything look idle.
	if err := SetIdleThreshold(-5); err != nil {
		t.Fatalf("SetIdleThreshold(-5) failed: %v", err)
	}
	if got := GetIdleThresholdMinutes(); got != 0 {
		t.Errorf("idle threshold = %d; want 0 after clamping", got)
	}

	// Oversized clamps to the maximum.
	if err := SetIdleThreshold(MaxIdleThresholdMinutes + 1000); err != nil {
		t.Fatalf("SetIdleThreshold(oversized) failed: %v", err)
	}
	if got := GetIdleThresholdMinutes(); got != MaxIdleThresholdMinutes {
		t.Errorf("idle threshold = %d; want %d", got, MaxIdleThresholdMinutes)
	}
}

func TestIdleThresholdOutOfRangeInFileIsRepaired(t *testing.T) {
	dir := setupConfigEnv(t)

	configDir := filepath.Join(dir, "focusd")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	body := `{"idle_threshold_minutes": 99999, "track_while_idle": true}`
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte(body), 0600); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if got := GetIdleThresholdMinutes(); got != DefaultIdleThresholdMinutes {
		t.Errorf("idle threshold = %d; want the default after repair, got %d", got, DefaultIdleThresholdMinutes)
	}
}

func TestConfigFileWithoutIdleFieldsKeepsDefaults(t *testing.T) {
	dir := setupConfigEnv(t)

	configDir := filepath.Join(dir, "focusd")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	// An existing user's config predates these fields.
	body := `{"pomodoro_minutes": 50, "custom_browsers": ["thing.exe"]}`
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte(body), 0600); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if got := GetIdleThresholdMinutes(); got != DefaultIdleThresholdMinutes {
		t.Errorf("idle threshold = %d; want default %d for a legacy config", got, DefaultIdleThresholdMinutes)
	}
	if got := GetPomodoroMinutes(); got != 50 {
		t.Errorf("pomodoro minutes = %d; want 50 to survive", got)
	}
}
