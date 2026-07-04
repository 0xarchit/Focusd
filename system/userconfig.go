package system

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type UserConfig struct {
	WhitelistApps         []string       `json:"whitelist_apps"`
	BreakReminderEnabled  bool           `json:"break_reminder_enabled"`
	BreakReminderMinutes  int            `json:"break_reminder_minutes"`
	AppTimeLimits         map[string]int `json:"app_time_limits"`
	PomodoroMinutes       int            `json:"pomodoro_minutes"`
	Password              string         `json:"password"`
	SnoozeDurationMinutes int            `json:"snooze_duration_minutes"`
	CustomBrowsers        []string       `json:"custom_browsers"`
}

var (
	userConfig *UserConfig
	configMu   sync.RWMutex
)

func getUserConfigPath() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", nil
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
		Password:              "",
		SnoozeDurationMinutes: 60,
		CustomBrowsers:        []string{},
	}
}

func loadFromDisk() {
	configMu.Lock()
	defer configMu.Unlock()
	if userConfig != nil {
		return
	}
	userConfig = defaultUserConfig()

	configPath, err := getUserConfigPath()
	if err == nil && configPath != "" {
		if data, err := os.ReadFile(configPath); err == nil {
			if err := json.Unmarshal(data, userConfig); err != nil {
				log.Printf("WARN: failed to parse user config %s: %v", configPath, err)
			}
		}
	}
	if userConfig.AppTimeLimits == nil {
		userConfig.AppTimeLimits = make(map[string]int)
	}
	if userConfig.CustomBrowsers == nil {
		userConfig.CustomBrowsers = []string{}
	}
}

func SaveUserConfig() error {
	configMu.Lock()
	defer configMu.Unlock()
	return saveUserConfigLocked()
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
	os.MkdirAll(dataDir, 0700)

	data, err := json.MarshalIndent(userConfig, "", "  ")
	if err != nil {
		return err
	}

	tempPath := configPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0600); err != nil {
		return err
	}

	return os.Rename(tempPath, configPath)
}

func GetWhitelistApps() []string {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	config := userConfig
	if config.WhitelistApps == nil {
		return nil
	}
	cloned := make([]string, len(config.WhitelistApps))
	copy(cloned, config.WhitelistApps)
	return cloned
}

func AddWhitelistApp(exeName string) error {
	loadFromDisk()
	configMu.Lock()
	defer configMu.Unlock()
	config := userConfig

	exeName = strings.ToLower(strings.TrimSpace(exeName))
	if exeName == "" {
		return nil
	}

	if !strings.HasSuffix(exeName, ".exe") {
		exeName += ".exe"
	}

	for _, a := range config.WhitelistApps {
		if strings.EqualFold(a, exeName) {
			return nil
		}
	}

	config.WhitelistApps = append(config.WhitelistApps, exeName)
	return saveUserConfigLocked()
}

func RemoveWhitelistApp(exeName string) error {
	loadFromDisk()
	configMu.Lock()
	defer configMu.Unlock()
	config := userConfig

	var updated []string
	for _, a := range config.WhitelistApps {
		if !strings.EqualFold(a, exeName) {
			updated = append(updated, a)
		}
	}
	config.WhitelistApps = updated
	return saveUserConfigLocked()
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

func ReloadUserConfig() {
	configMu.Lock()
	userConfig = nil
	configMu.Unlock()
	loadFromDisk()
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
	mins := userConfig.BreakReminderMinutes
	if mins < 1 {
		return 60
	}
	return mins
}

func SetBreakReminder(enabled bool, minutes int) error {
	loadFromDisk()
	configMu.Lock()
	defer configMu.Unlock()
	config := userConfig
	config.BreakReminderEnabled = enabled
	if minutes > 0 {
		config.BreakReminderMinutes = minutes
	}
	return saveUserConfigLocked()
}

func GetAppTimeLimits() map[string]int {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	config := userConfig
	if config.AppTimeLimits == nil {
		return nil
	}
	cloned := make(map[string]int, len(config.AppTimeLimits))
	for k, v := range config.AppTimeLimits {
		cloned[k] = v
	}
	return cloned
}

func SetAppTimeLimit(exeName string, minutes int) error {
	loadFromDisk()
	configMu.Lock()
	defer configMu.Unlock()
	config := userConfig
	exeName = strings.ToLower(strings.TrimSpace(exeName))
	if !strings.HasSuffix(exeName, ".exe") {
		exeName += ".exe"
	}
	if minutes <= 0 {
		delete(config.AppTimeLimits, exeName)
	} else {
		config.AppTimeLimits[exeName] = minutes
	}
	return saveUserConfigLocked()
}

func RemoveAppTimeLimit(exeName string) error {
	loadFromDisk()
	configMu.Lock()
	defer configMu.Unlock()
	config := userConfig
	exeName = strings.ToLower(strings.TrimSpace(exeName))
	delete(config.AppTimeLimits, exeName)
	return saveUserConfigLocked()
}

func GetPomodoroMinutes() int {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	mins := userConfig.PomodoroMinutes
	if mins < 1 {
		return 25
	}
	return mins
}

func SetPomodoroMinutes(minutes int) error {
	loadFromDisk()
	configMu.Lock()
	defer configMu.Unlock()
	config := userConfig
	config.PomodoroMinutes = minutes
	return saveUserConfigLocked()
}

func GetPassword() string {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.Password
}

func SetPassword(password string) error {
	loadFromDisk()
	configMu.Lock()
	defer configMu.Unlock()
	config := userConfig
	config.Password = password
	return saveUserConfigLocked()
}

func IsPasswordEnabled() bool {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.Password != ""
}

func CheckPassword(input string) bool {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.Password == input
}

func GetSnoozeDurationMinutes() int {
	loadFromDisk()
	configMu.RLock()
	defer configMu.RUnlock()
	mins := userConfig.SnoozeDurationMinutes
	if mins < 1 {
		return 60
	}
	return mins
}

func SetSnoozeDurationMinutes(minutes int) error {
	loadFromDisk()
	configMu.Lock()
	defer configMu.Unlock()
	config := userConfig
	config.SnoozeDurationMinutes = minutes
	return saveUserConfigLocked()
}

var defaultBrowsers = map[string]bool{
	"chrome.exe":    true,
	"firefox.exe":   true,
	"msedge.exe":    true,
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
	config := userConfig

	for _, b := range config.CustomBrowsers {
		if b == exeName {
			return fmt.Errorf("%s is already in custom list", exeName)
		}
	}

	if defaultBrowsers[exeName] {
		return fmt.Errorf("%s is already a default browser", exeName)
	}

	config.CustomBrowsers = append(config.CustomBrowsers, exeName)
	return saveUserConfigLocked()
}

func RemoveCustomBrowser(exeName string) error {
	exeName = strings.ToLower(strings.TrimSpace(exeName))

	loadFromDisk()
	configMu.Lock()
	defer configMu.Unlock()
	config := userConfig

	found := false
	var newList []string
	for _, b := range config.CustomBrowsers {
		if b == exeName {
			found = true
			continue
		}
		newList = append(newList, b)
	}

	if !found {
		return fmt.Errorf("%s not found in custom list (cannot remove default browsers)", exeName)
	}

	config.CustomBrowsers = newList
	return saveUserConfigLocked()
}
