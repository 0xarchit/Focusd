package system

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type UserConfig struct {
	WhitelistApps         []string       `json:"whitelist_apps"`
	BreakReminderEnabled  bool           `json:"break_reminder_enabled"`
	BreakReminderMinutes  int            `json:"break_reminder_minutes"`
	AppTimeLimits         map[string]int `json:"app_time_limits"`
	PomodoroMinutes       int            `json:"pomodoro_minutes"`
	SnoozeDurationMinutes int            `json:"snooze_duration_minutes"`
	CustomBrowsers        []string       `json:"custom_browsers"`
	IdleThresholdMinutes  int            `json:"idle_threshold_minutes"`
	TrackWhileIdle        bool           `json:"track_while_idle"`
	// RedactTitles stores a placeholder instead of window title text.
	RedactTitles bool `json:"redact_titles"`
	// DisplayOverrides renames an app in the UI without touching history,
	// keyed by app id.
	DisplayOverrides map[string]string `json:"display_overrides,omitempty"`
	// SiteOverrides renames a site in the UI, keyed by the cleaned site name.
	SiteOverrides map[string]string `json:"site_overrides,omitempty"`
}

// userConfigFile mirrors UserConfig for decoding. IdleThresholdMinutes is a
// pointer so an absent key is distinguishable from an explicit zero, which
// means "never treat me as idle".
type userConfigFile struct {
	WhitelistApps         []string          `json:"whitelist_apps"`
	BreakReminderEnabled  bool              `json:"break_reminder_enabled"`
	BreakReminderMinutes  int               `json:"break_reminder_minutes"`
	AppTimeLimits         map[string]int    `json:"app_time_limits"`
	PomodoroMinutes       int               `json:"pomodoro_minutes"`
	SnoozeDurationMinutes int               `json:"snooze_duration_minutes"`
	CustomBrowsers        []string          `json:"custom_browsers"`
	IdleThresholdMinutes  *int              `json:"idle_threshold_minutes"`
	TrackWhileIdle        *bool             `json:"track_while_idle"`
	RedactTitles          bool              `json:"redact_titles"`
	DisplayOverrides      map[string]string `json:"display_overrides"`
	SiteOverrides         map[string]string `json:"site_overrides"`
}

const (
	// DefaultIdleThresholdMinutes is how long the keyboard and mouse may be
	// idle before time stops accruing.
	DefaultIdleThresholdMinutes = 5
	MinIdleThresholdMinutes     = 0
	MaxIdleThresholdMinutes     = 240
)

var (
	userConfig  *UserConfig
	configMu    sync.RWMutex
	lastModTime time.Time
)

func getUserConfigPath() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", fmt.Errorf("APPDATA environment variable is empty")
	}
	return filepath.Join(appData, "focusd", "config.json"), nil
}

func defaultUserConfig() *UserConfig {
	return &UserConfig{
		WhitelistApps:         []string{},
		BreakReminderEnabled:  false,
		BreakReminderMinutes:  60,
		AppTimeLimits:         make(map[string]int),
		PomodoroMinutes:       25,
		SnoozeDurationMinutes: 60,
		CustomBrowsers:        []string{},
		IdleThresholdMinutes:  DefaultIdleThresholdMinutes,
		TrackWhileIdle:        false,
		RedactTitles:          false,
		DisplayOverrides:      make(map[string]string),
		SiteOverrides:         make(map[string]string),
	}
}

func loadFromDisk() {
	configMu.Lock()
	defer configMu.Unlock()

	configPath, err := getUserConfigPath()
	if err != nil || configPath == "" {
		if userConfig == nil {
			userConfig = defaultUserConfig()
		}
		return
	}

	fi, err := os.Stat(configPath)
	if err != nil {
		if userConfig == nil {
			userConfig = defaultUserConfig()
		}
		return
	}

	if userConfig != nil && !fi.ModTime().After(lastModTime) {
		return
	}

	lastModTime = fi.ModTime()
	if userConfig == nil {
		userConfig = defaultUserConfig()
	}

	if data, err := os.ReadFile(configPath); err == nil {
		var temp userConfigFile
		if err := json.Unmarshal(data, &temp); err == nil {
			userConfig.WhitelistApps = temp.WhitelistApps
			userConfig.BreakReminderEnabled = temp.BreakReminderEnabled
			userConfig.BreakReminderMinutes = temp.BreakReminderMinutes
			userConfig.AppTimeLimits = temp.AppTimeLimits
			userConfig.PomodoroMinutes = temp.PomodoroMinutes
			userConfig.SnoozeDurationMinutes = temp.SnoozeDurationMinutes
			userConfig.CustomBrowsers = temp.CustomBrowsers

			// An absent key keeps the current value rather than decoding as
			// "disabled".
			if temp.IdleThresholdMinutes != nil {
				userConfig.IdleThresholdMinutes = *temp.IdleThresholdMinutes
			}
			if temp.TrackWhileIdle != nil {
				userConfig.TrackWhileIdle = *temp.TrackWhileIdle
			}
			userConfig.RedactTitles = temp.RedactTitles
			if temp.DisplayOverrides != nil {
				userConfig.DisplayOverrides = temp.DisplayOverrides
			}
			if temp.SiteOverrides != nil {
				userConfig.SiteOverrides = temp.SiteOverrides
			}
		} else {
			log.Printf("WARN: failed to parse user config %s: %v", configPath, err)
		}
	}

	if userConfig.AppTimeLimits == nil {
		userConfig.AppTimeLimits = make(map[string]int)
	}
	if userConfig.CustomBrowsers == nil {
		userConfig.CustomBrowsers = []string{}
	}
	if userConfig.DisplayOverrides == nil {
		userConfig.DisplayOverrides = make(map[string]string)
	}
	if userConfig.SiteOverrides == nil {
		userConfig.SiteOverrides = make(map[string]string)
	}
	if userConfig.BreakReminderMinutes < 1 {
		userConfig.BreakReminderMinutes = 60
	}
	if userConfig.PomodoroMinutes < 1 {
		userConfig.PomodoroMinutes = 25
	}
	if userConfig.SnoozeDurationMinutes < 1 {
		userConfig.SnoozeDurationMinutes = 60
	}
	if userConfig.IdleThresholdMinutes < MinIdleThresholdMinutes ||
		userConfig.IdleThresholdMinutes > MaxIdleThresholdMinutes {
		userConfig.IdleThresholdMinutes = DefaultIdleThresholdMinutes
	}
}

func saveUserConfigLocked() error {
	if userConfig == nil {
		userConfig = defaultUserConfig()
	}

	configPath, err := getUserConfigPath()
	if err != nil || configPath == "" {
		return err
	}

	dataDir := filepath.Dir(configPath)
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(userConfig, "", "  ")
	if err != nil {
		return err
	}

	tempPath := configPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0600); err != nil {
		return err
	}

	if err := os.Rename(tempPath, configPath); err != nil {
		return err
	}

	if fi, err := os.Stat(configPath); err == nil {
		lastModTime = fi.ModTime()
	}
	return nil
}

// IsWhitelisted reports whether an app should be excluded from tracking. The
// exe name is matched against the normalized config entries so both "foo" and
// "foo.exe" address the same app.
func IsWhitelisted(exeName string) bool {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()

	target := normalizeConfigName(exeName)
	for _, a := range userConfig.WhitelistApps {
		if normalizeConfigName(a) == target {
			return true
		}
	}
	return false
}

func GetWhitelistApps() []string {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	out := make([]string, len(userConfig.WhitelistApps))
	copy(out, userConfig.WhitelistApps)
	return out
}

func AddWhitelistApp(exeName string) error {
	loadFromDisk()
	configMu.Lock()
	defer configMu.Unlock()
	exeName = normalizeConfigName(exeName)
	if exeName == "unknown" || exeName == "" {
		return fmt.Errorf("invalid app name")
	}
	for _, a := range userConfig.WhitelistApps {
		if normalizeConfigName(a) == exeName {
			return nil // already exists
		}
	}
	userConfig.WhitelistApps = append(userConfig.WhitelistApps, exeName)
	return saveUserConfigLocked()
}

func RemoveWhitelistApp(exeName string) error {
	loadFromDisk()
	configMu.Lock()
	defer configMu.Unlock()

	// Normalize exactly like insertion, so "foo" removes the stored "foo.exe".
	target := normalizeConfigName(exeName)

	matched := false
	filtered := userConfig.WhitelistApps[:0]
	for _, a := range userConfig.WhitelistApps {
		if normalizeConfigName(a) == target {
			matched = true
			continue
		}
		filtered = append(filtered, a)
	}
	if !matched {
		return nil
	}
	userConfig.WhitelistApps = filtered
	return saveUserConfigLocked()
}

func GetBreakReminderEnabled() bool {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.BreakReminderEnabled
}

func GetBreakReminderMinutes() int {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.BreakReminderMinutes
}

func SetBreakReminder(enabled bool, minutes int) error {
	loadFromDisk()
	configMu.Lock()
	defer configMu.Unlock()
	userConfig.BreakReminderEnabled = enabled
	if minutes > 0 {
		userConfig.BreakReminderMinutes = minutes
	}
	return saveUserConfigLocked()
}

func GetAppTimeLimits() map[string]int {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	limits := make(map[string]int, len(userConfig.AppTimeLimits))
	for k, v := range userConfig.AppTimeLimits {
		limits[k] = v
	}
	return limits
}

func SetAppTimeLimit(exeName string, minutes int) error {
	loadFromDisk()
	configMu.Lock()
	defer configMu.Unlock()
	exeName = normalizeConfigName(exeName)
	if minutes <= 0 {
		delete(userConfig.AppTimeLimits, exeName)
	} else {
		userConfig.AppTimeLimits[exeName] = minutes
	}
	return saveUserConfigLocked()
}

func GetPomodoroMinutes() int {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.PomodoroMinutes
}

func SetPomodoroMinutes(minutes int) error {
	loadFromDisk()
	configMu.Lock()
	defer configMu.Unlock()
	userConfig.PomodoroMinutes = minutes
	return saveUserConfigLocked()
}

// MatchLimitKey finds the config entry that governs an app. A bare exe name
// acts as a wildcard across every install; a specific app key wins.
func MatchLimitKey(limits map[string]int, appID, exeName string) (string, bool) {
	if appID != "" {
		if _, ok := limits[appID]; ok {
			return appID, true
		}
	}

	bare := normalizeExe(exeName)
	if bare == "" || bare == "unknown" {
		return "", false
	}
	if _, ok := limits[bare]; ok {
		return bare, true
	}

	// Accept the historical ".exe" spelling too.
	withExt := bare + ".exe"
	if _, ok := limits[withExt]; ok {
		return withExt, true
	}

	return "", false
}

// ResolveDisplayName returns the human label for an app, preferring a user
// override, then version info, then the built-in map, then the exe name.
func ResolveDisplayName(appID, exeName, versionLabel string) string {
	if override := GetDisplayOverride(appID); override != "" {
		return override
	}
	if versionLabel != "" {
		return versionLabel
	}
	if mapped, ok := lookupNameMap(exeName); ok {
		return mapped
	}
	return TitleCaseExe(exeName)
}

// GetDisplayOverride returns a user-supplied label for an app, if any.
func GetDisplayOverride(appID string) string {
	if appID == "" {
		return ""
	}
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.DisplayOverrides[appID]
}

// SetDisplayOverride renames an app in the UI. An empty name removes the
// override.
func SetDisplayOverride(appID, name string) error {
	if appID == "" {
		return fmt.Errorf("app id is required")
	}
	loadFromDisk()
	configMu.Lock()
	defer configMu.Unlock()

	name = strings.TrimSpace(name)
	if name == "" {
		delete(userConfig.DisplayOverrides, appID)
	} else {
		userConfig.DisplayOverrides[appID] = name
	}
	return saveUserConfigLocked()
}

// GetDisplayOverrides returns a copy of every override, safe to iterate.
func GetDisplayOverrides() map[string]string {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	out := make(map[string]string, len(userConfig.DisplayOverrides))
	for k, v := range userConfig.DisplayOverrides {
		out[k] = v
	}
	return out
}

// lookupNameMap resolves the built-in friendly names.
func lookupNameMap(exeName string) (string, bool) {
	name := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(exeName)), ".exe")
	mapped, ok := displayNameMap[name]
	return mapped, ok
}

func GetSnoozeDurationMinutes() int {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.SnoozeDurationMinutes
}

// GetIdleThresholdMinutes returns how many minutes of keyboard and mouse
// inactivity stop time from accruing. Zero means idle time is still tracked.
func GetIdleThresholdMinutes() int {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.IdleThresholdMinutes
}

func GetTrackWhileIdle() bool {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.TrackWhileIdle
}

// GetRedactTitles reports whether window titles are stored as a placeholder
// instead of their text.
func GetRedactTitles() bool {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.RedactTitles
}

// SetRedactTitles turns window-title redaction on or off.
func SetRedactTitles(enabled bool) error {
	loadFromDisk()
	configMu.Lock()
	defer configMu.Unlock()
	userConfig.RedactTitles = enabled
	return saveUserConfigLocked()
}

// SetIdleThreshold sets the idle cutoff. A negative value disables it, which is
// how a user opts back into counting unattended time.
func SetIdleThreshold(minutes int) error {
	loadFromDisk()
	configMu.Lock()
	defer configMu.Unlock()
	if minutes < MinIdleThresholdMinutes {
		minutes = MinIdleThresholdMinutes
	}
	if minutes > MaxIdleThresholdMinutes {
		minutes = MaxIdleThresholdMinutes
	}
	userConfig.IdleThresholdMinutes = minutes
	return saveUserConfigLocked()
}

var defaultBrowsers = map[string]bool{
	"chrome.exe":     true,
	"firefox.exe":    true,
	"msedge.exe":     true,
	"edge.exe":       true,
	"brave.exe":      true,
	"opera.exe":      true,
	"operagx.exe":    true,
	"sidekick.exe":   true,
	"orion.exe":      true,
	"tor.exe":        true,
	"torbrowser.exe": true,
	"coccoc.exe":     true,
	"vivaldi.exe":    true,
	"waterfox.exe":   true,
	"arc.exe":        true,
	"iexplore.exe":   true,
	"safari.exe":     true,
	"whale.exe":      true,
	"yandex.exe":     true,
	"thorium.exe":    true,
	"librewolf.exe":  true,
	"chromium.exe":   true,
	"floorp.exe":     true,
	"zen.exe":        true,
}

func IsBrowser(exeName string) bool {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()

	exeLower := strings.ToLower(exeName)
	if defaultBrowsers[exeLower] {
		return true
	}
	for _, cb := range userConfig.CustomBrowsers {
		if strings.ToLower(cb) == exeLower {
			return true
		}
	}
	return false
}

func GetCustomBrowsersList() []string {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()

	res := make([]string, len(userConfig.CustomBrowsers))
	copy(res, userConfig.CustomBrowsers)
	return res
}

func AddCustomBrowser(exeName string) error {
	exeName = strings.ToLower(strings.TrimSpace(exeName))
	if exeName == "" {
		return fmt.Errorf("invalid browser name")
	}
	if !strings.HasSuffix(exeName, ".exe") {
		exeName += ".exe"
	}

	loadFromDisk()
	configMu.Lock()
	defer configMu.Unlock()

	for _, b := range userConfig.CustomBrowsers {
		if b == exeName {
			return fmt.Errorf("%s is already in custom list", exeName)
		}
	}

	if defaultBrowsers[exeName] {
		return fmt.Errorf("%s is already a default browser", exeName)
	}

	userConfig.CustomBrowsers = append(userConfig.CustomBrowsers, exeName)
	return saveUserConfigLocked()
}

func RemoveCustomBrowser(exeName string) error {
	exeName = strings.ToLower(strings.TrimSpace(exeName))

	loadFromDisk()
	configMu.Lock()
	defer configMu.Unlock()

	if !strings.HasSuffix(exeName, ".exe") {
		exeName += ".exe"
	}
	found := false
	var newList []string
	for _, b := range userConfig.CustomBrowsers {
		if b == exeName {
			found = true
			continue
		}
		newList = append(newList, b)
	}

	if !found {
		return fmt.Errorf("%s not found in custom list (cannot remove default browsers)", exeName)
	}

	userConfig.CustomBrowsers = newList
	return saveUserConfigLocked()
}
