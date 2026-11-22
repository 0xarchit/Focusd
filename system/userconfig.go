package system

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows"
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
	RedactTitles bool   `json:"redact_titles"`
	PrivacyMode  string `json:"privacy_mode,omitempty"`
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
	PrivacyMode           string            `json:"privacy_mode"`
	DisplayOverrides      map[string]string `json:"display_overrides"`
	SiteOverrides         map[string]string `json:"site_overrides"`
}

const (
	// DefaultIdleThresholdMinutes is how long the keyboard and mouse may be
	// idle before time stops accruing.
	DefaultIdleThresholdMinutes = 5
	MinIdleThresholdMinutes     = 0
	MaxIdleThresholdMinutes     = 240
	PrivacyModeTitles           = "titles"
	PrivacyModeCategories       = "categories"
	PrivacyModeStrict           = "strict"
	configFingerprintInterval   = 250 * time.Millisecond
	maxConfigBytes              = 1 << 20
)

var (
	userConfig         *UserConfig
	configMu           sync.RWMutex
	lastModTime        time.Time
	lastConfigPath     string
	lastConfigSize     int64
	lastConfigHash     [32]byte
	lastConfigCheck    time.Time
	configBeforeUpdate *UserConfig
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
	reloadFromDiskLocked(false)
}

// reloadFromDiskLocked refreshes the in-memory config from disk. Caller holds
// configMu. With force, the mtime fast path is skipped so a read-modify-write
// always starts from the freshest file.
func reloadFromDiskLocked(force bool) {
	configPath, err := getUserConfigPath()
	if configPath != lastConfigPath {
		userConfig = defaultUserConfig()
		lastConfigPath = configPath
		lastModTime = time.Time{}
		lastConfigSize = 0
		lastConfigHash = [32]byte{}
		lastConfigCheck = time.Time{}
	}
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

	if !force && userConfig != nil && fi.ModTime().Equal(lastModTime) && fi.Size() == lastConfigSize && time.Since(lastConfigCheck) < configFingerprintInterval {
		return
	}

	lastConfigCheck = time.Now()
	if userConfig == nil {
		userConfig = defaultUserConfig()
	}

	if data, err := readBoundedConfig(configPath); err == nil {
		fingerprint := sha256.Sum256(data)
		if !force && fingerprint == lastConfigHash {
			lastModTime, lastConfigSize = fi.ModTime(), fi.Size()
			return
		}
		var temp userConfigFile
		if err := json.Unmarshal(data, &temp); err == nil {
			lastModTime, lastConfigSize, lastConfigHash = fi.ModTime(), fi.Size(), fingerprint
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
			userConfig.PrivacyMode = temp.PrivacyMode
			// Overrides are replaced wholesale, never merged. The file omits
			// them when they are empty, and a nil-check merge resurrected
			// entries the file no longer had, so deleting an override in
			// another process never took effect here.
			userConfig.DisplayOverrides = temp.DisplayOverrides
			userConfig.SiteOverrides = temp.SiteOverrides
		} else {
			log.Printf("WARN: failed to parse user config %s: %v", configPath, err)
			return
		}
	} else {
		log.Printf("WARN: failed to read user config %s: %v", configPath, err)
		return
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

func readBoundedConfig(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxConfigBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxConfigBytes {
		return nil, fmt.Errorf("configuration exceeds %d bytes", maxConfigBytes)
	}
	return data, nil
}

func saveUserConfigLocked() (err error) {
	defer func() {
		if err != nil && configBeforeUpdate != nil {
			userConfig = configBeforeUpdate
		}
	}()
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
	if len(data) > maxConfigBytes {
		return fmt.Errorf("configuration exceeds %d bytes", maxConfigBytes)
	}

	tempPath := configPath + ".tmp"
	f, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tempPath)
		return err
	}
	// Flush to disk before the rename: otherwise a crash can leave the rename
	// pointing at a temp file whose contents never made it out of the cache.
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tempPath)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tempPath)
		return err
	}

	if err := os.Rename(tempPath, configPath); err != nil {
		os.Remove(tempPath)
		return err
	}

	if fi, err := os.Stat(configPath); err == nil {
		lastModTime = fi.ModTime()
		lastConfigSize = fi.Size()
	}
	lastConfigPath = configPath
	lastConfigHash = sha256.Sum256(data)
	lastConfigCheck = time.Now()
	return nil
}

// lockUserConfig takes an exclusive lock on a sidecar file next to the config,
// serializing read-modify-write updates across processes (TUI, CLI, daemon).
// Windows releases the lock when the handle closes or the process dies, so a
// crash cannot leave the config wedged.
func lockUserConfig(configPath string) (func(), error) {
	lockPath := configPath + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	overlapped := new(windows.Overlapped)
	if err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, overlapped); err != nil {
		f.Close()
		return nil, fmt.Errorf("failed to lock the user config: %w", err)
	}
	return func() {
		windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, overlapped)
		f.Close()
	}, nil
}

// beginConfigUpdate starts a cross-process-safe read-modify-write of the user
// config: it takes the in-process mutex plus the inter-process file lock and
// reloads the on-disk state, so a change another process made while we were
// idle is not silently overwritten. The caller must defer the returned
// release function.
func beginConfigUpdate() (func(), error) {
	configMu.Lock()

	configPath, err := getUserConfigPath()
	if err != nil {
		configMu.Unlock()
		return nil, err
	}
	release, err := lockUserConfig(configPath)
	if err != nil {
		configMu.Unlock()
		return nil, err
	}
	reloadFromDiskLocked(true)
	configBeforeUpdate = cloneUserConfig(userConfig)
	return func() {
		configBeforeUpdate = nil
		release()
		configMu.Unlock()
	}, nil
}

func cloneUserConfig(source *UserConfig) *UserConfig {
	out := *source
	out.WhitelistApps = append([]string{}, source.WhitelistApps...)
	out.CustomBrowsers = append([]string{}, source.CustomBrowsers...)
	out.AppTimeLimits = make(map[string]int, len(source.AppTimeLimits))
	for key, value := range source.AppTimeLimits {
		out.AppTimeLimits[key] = value
	}
	out.DisplayOverrides = make(map[string]string, len(source.DisplayOverrides))
	for key, value := range source.DisplayOverrides {
		out.DisplayOverrides[key] = value
	}
	out.SiteOverrides = make(map[string]string, len(source.SiteOverrides))
	for key, value := range source.SiteOverrides {
		out.SiteOverrides[key] = value
	}
	return &out
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
	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()
	exeName = normalizeConfigName(exeName)
	if exeName == "unknown" || exeName == "" {
		return fmt.Errorf("invalid app name")
	}
	for _, a := range userConfig.WhitelistApps {
		if normalizeConfigName(a) == exeName {
			return nil
		}
	}
	userConfig.WhitelistApps = append(userConfig.WhitelistApps, exeName)
	return saveUserConfigLocked()
}

func RemoveWhitelistApp(exeName string) error {
	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()

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
	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()
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
	return RenameAppTimeLimit(exeName, exeName, minutes)
}

// RenameAppTimeLimit edits or renames a limit in one cross-process-locked write.
func RenameAppTimeLimit(oldKey, newKey string, minutes int) error {
	oldKey, newKey = canonicalLimitKey(oldKey), canonicalLimitKey(newKey)
	if oldKey == "" || newKey == "" {
		return fmt.Errorf("app name is required")
	}
	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()
	previous := userConfig.AppTimeLimits
	updated := make(map[string]int, len(previous)+1)
	for key, value := range previous {
		canonical := canonicalLimitKey(key)
		if canonical != oldKey && canonical != newKey {
			updated[key] = value
		}
	}
	if minutes > 0 {
		updated[newKey] = minutes
	}
	userConfig.AppTimeLimits = updated
	if err := saveUserConfigLocked(); err != nil {
		userConfig.AppTimeLimits = previous
		return err
	}
	return nil
}

func canonicalLimitKey(key string) string {
	base, suffix, exact := strings.Cut(strings.TrimSpace(key), "|")
	base = normalizeExe(base)
	if base == "unknown" {
		return ""
	}
	if exact {
		return base + "|" + strings.ToLower(strings.TrimSpace(suffix))
	}
	return base
}

func GetPomodoroMinutes() int {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.PomodoroMinutes
}

func SetPomodoroMinutes(minutes int) error {
	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()
	userConfig.PomodoroMinutes = minutes
	return saveUserConfigLocked()
}

// MatchLimitKey finds the config entry that governs an app. A bare exe name
// acts as a wildcard across every install; a specific app key wins.
func MatchLimitKey(limits map[string]int, appID, exeName string) (string, bool) {
	for _, target := range []string{canonicalLimitKey(appID), canonicalLimitKey(exeName)} {
		if target == "" {
			continue
		}
		if _, ok := limits[target]; ok {
			return target, true
		}
		matched := ""
		for key := range limits {
			if canonicalLimitKey(key) == target && (matched == "" || key < matched) {
				matched = key
			}
		}
		if matched != "" {
			return matched, true
		}
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
	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()

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

// GetSiteOverrides returns a snapshot of user site labels.
func GetSiteOverrides() map[string]string {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	out := make(map[string]string, len(userConfig.SiteOverrides))
	for key, value := range userConfig.SiteOverrides {
		out[key] = value
	}
	return out
}

// SetSiteOverride stores a presentation label; an empty label removes it.
func SetSiteOverride(key, name string) error {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return fmt.Errorf("site name is required")
	}
	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()
	name = strings.TrimSpace(name)
	if name == "" {
		delete(userConfig.SiteOverrides, key)
	} else {
		userConfig.SiteOverrides[key] = name
	}
	return saveUserConfigLocked()
}

type PresentationOverrides struct {
	Display map[string]string
	Sites   map[string]string
}

// GetPresentationOverrides returns both label maps from the same config state.
func GetPresentationOverrides() PresentationOverrides {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	out := PresentationOverrides{Display: make(map[string]string, len(userConfig.DisplayOverrides)), Sites: make(map[string]string, len(userConfig.SiteOverrides))}
	for key, value := range userConfig.DisplayOverrides {
		out.Display[key] = value
	}
	for key, value := range userConfig.SiteOverrides {
		out.Sites[key] = value
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

func SetTrackWhileIdle(enabled bool) error {
	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()
	userConfig.TrackWhileIdle = enabled
	return saveUserConfigLocked()
}

func privacyModeLocked() string {
	switch userConfig.PrivacyMode {
	case PrivacyModeTitles, PrivacyModeCategories, PrivacyModeStrict:
		return userConfig.PrivacyMode
	}
	if userConfig.PrivacyMode != "" {
		return PrivacyModeStrict
	}
	if userConfig.RedactTitles {
		return PrivacyModeStrict
	}
	return PrivacyModeTitles
}

// GetPrivacyMode preserves legacy redact_titles when privacy_mode is absent.
func GetPrivacyMode() string {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	return privacyModeLocked()
}

func SetPrivacyMode(mode string) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != PrivacyModeTitles && mode != PrivacyModeCategories && mode != PrivacyModeStrict {
		return fmt.Errorf("privacy mode must be titles, categories, or strict")
	}
	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()
	userConfig.PrivacyMode = mode
	userConfig.RedactTitles = mode != PrivacyModeTitles
	return saveUserConfigLocked()
}

// GetRedactTitles reports whether window titles are stored as a placeholder
// instead of their text.
func GetRedactTitles() bool {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	return privacyModeLocked() != PrivacyModeTitles
}

// SetRedactTitles turns window-title redaction on or off.
func SetRedactTitles(enabled bool) error {
	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()
	userConfig.RedactTitles = enabled
	userConfig.PrivacyMode = ""
	return saveUserConfigLocked()
}

// SetIdleThreshold sets the idle cutoff. A negative value disables it, which is
// how a user opts back into counting unattended time.
func SetIdleThreshold(minutes int) error {
	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()
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

	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()

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

	release, err := beginConfigUpdate()
	if err != nil {
		return err
	}
	defer release()

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
