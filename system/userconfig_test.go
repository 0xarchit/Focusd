package system

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestRedactTitlesRoundTrip(t *testing.T) {
	setupConfigEnv(t)

	if GetRedactTitles() {
		t.Error("title redaction should be off by default")
	}
	if err := SetRedactTitles(true); err != nil {
		t.Fatalf("SetRedactTitles failed: %v", err)
	}
	if !GetRedactTitles() {
		t.Error("title redaction should be on after enabling")
	}

	// The switch persists to config.json.
	data, err := os.ReadFile(filepath.Join(os.Getenv("APPDATA"), "focusd", "config.json"))
	if err != nil {
		t.Fatalf("config.json unreadable: %v", err)
	}
	if !strings.Contains(string(data), `"redact_titles": true`) {
		t.Errorf("config.json does not carry the switch:\n%s", data)
	}

	if err := SetRedactTitles(false); err != nil {
		t.Fatalf("SetRedactTitles failed: %v", err)
	}
	if GetRedactTitles() {
		t.Error("title redaction should be off after disabling")
	}
}

// TestConfigUpdatesSerializeOnTheFileLock pins that a setter must wait for the
// cross-process lock, so two processes cannot interleave their
// read-modify-write and silently drop one change.
func TestConfigUpdatesSerializeOnTheFileLock(t *testing.T) {
	dir := setupConfigEnv(t)
	configPath := filepath.Join(dir, "focusd", "config.json")

	release, err := lockUserConfig(configPath)
	if err != nil {
		t.Fatalf("lockUserConfig failed: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- SetPomodoroMinutes(30)
	}()

	select {
	case <-done:
		release()
		t.Fatal("the setter completed while the config lock was held")
	case <-time.After(200 * time.Millisecond):
	}

	release()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("setter failed after the lock was released: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the setter never finished after the lock was released")
	}
}

// TestConfigUpdateMergesConcurrentChange pins the fresh-reload half: a
// read-modify-write starts from the on-disk state, so a field another process
// changed is preserved.
func TestConfigUpdateMergesConcurrentChange(t *testing.T) {
	dir := setupConfigEnv(t)

	if err := SetPomodoroMinutes(30); err != nil {
		t.Fatalf("SetPomodoroMinutes failed: %v", err)
	}

	// Another process rewrites config.json, touching a field we never set.
	configPath := filepath.Join(dir, "focusd", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	raw["redact_titles"] = true
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent failed: %v", err)
	}
	if err := os.WriteFile(configPath, out, 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if err := SetIdleThreshold(45); err != nil {
		t.Fatalf("SetIdleThreshold failed: %v", err)
	}

	if !GetRedactTitles() {
		t.Error("the other process's change was lost")
	}
	if got := GetIdleThresholdMinutes(); got != 45 {
		t.Errorf("idle threshold = %d; want 45", got)
	}
	if got := GetPomodoroMinutes(); got != 30 {
		t.Errorf("pomodoro minutes = %d; want 30", got)
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
