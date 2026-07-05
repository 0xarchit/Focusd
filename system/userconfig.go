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
}

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
		var temp UserConfig
		if err := json.Unmarshal(data, &temp); err == nil {
			userConfig.WhitelistApps = temp.WhitelistApps
			userConfig.BreakReminderEnabled = temp.BreakReminderEnabled
			userConfig.BreakReminderMinutes = temp.BreakReminderMinutes
			userConfig.AppTimeLimits = temp.AppTimeLimits
			userConfig.PomodoroMinutes = temp.PomodoroMinutes
			userConfig.SnoozeDurationMinutes = temp.SnoozeDurationMinutes
			userConfig.CustomBrowsers = temp.CustomBrowsers
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
	if userConfig.BreakReminderMinutes < 1 {
		userConfig.BreakReminderMinutes = 60
	}
	if userConfig.PomodoroMinutes < 1 {
		userConfig.PomodoroMinutes = 25
	}
	if userConfig.SnoozeDurationMinutes < 1 {
		userConfig.SnoozeDurationMinutes = 60
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

func IsWhitelisted(exeName string) bool {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	config := userConfig

	exeName = strings.ToLower(exeName)
	for _, a := range config.WhitelistApps {
		if strings.EqualFold(a, exeName) {
			return true
		}
	}
	return false
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
	exeName = strings.ToLower(strings.TrimSpace(exeName))
	if !strings.HasSuffix(exeName, ".exe") {
		exeName += ".exe"
	}
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

func GetSnoozeDurationMinutes() int {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.SnoozeDurationMinutes
}

var defaultBrowsers = map[string]bool{
	"chrome.exe":    true,
	"firefox.exe":   true,
	"msedge.exe":    true,
	"edge.exe":      true,
	"brave.exe":     true,
	"opera.exe":     true,
	"vivaldi.exe":   true,
	"waterfox.exe":  true,
	"arc.exe":       true,
	"iexplore.exe":  true,
	"safari.exe":    true,
	"whale.exe":     true,
	"yandex.exe":    true,
	"thorium.exe":   true,
	"librewolf.exe": true,
	"chromium.exe":  true,
	"floorp.exe":    true,
	"zen.exe":       true,
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
