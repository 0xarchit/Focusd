package system

import (
	"encoding/json"
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
	SmartGroupingEnabled  bool           `json:"smart_grouping_enabled"`
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
		SmartGroupingEnabled:  true,
	}
}

func ensureLoaded() {
	configMu.RLock()
	if userConfig != nil {
		configMu.RUnlock()
		return
	}
	configMu.RUnlock()

	configMu.Lock()
	defer configMu.Unlock()
	if userConfig == nil {
		userConfig = defaultUserConfig()

		configPath, err := getUserConfigPath()
		if err == nil && configPath != "" {
			data, err := os.ReadFile(configPath)
			if err == nil {
				if err := json.Unmarshal(data, userConfig); err != nil {
					log.Printf("WARN: failed to parse user config %s: %v", configPath, err)
				}
			}
		}
		if userConfig.AppTimeLimits == nil {
			userConfig.AppTimeLimits = make(map[string]int)
		}
	}
}

func loadUserConfigLocked() *UserConfig {
	if userConfig == nil {
		userConfig = defaultUserConfig()

		configPath, err := getUserConfigPath()
		if err == nil && configPath != "" {
			data, err := os.ReadFile(configPath)
			if err == nil {
				if err := json.Unmarshal(data, userConfig); err != nil {
					log.Printf("WARN: failed to parse user config %s: %v", configPath, err)
				}
			}
		}
		if userConfig.AppTimeLimits == nil {
			userConfig.AppTimeLimits = make(map[string]int)
		}
	}
	return userConfig
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
	ensureLoaded()
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
	configMu.Lock()
	defer configMu.Unlock()
	config := loadUserConfigLocked()

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
	configMu.Lock()
	defer configMu.Unlock()
	config := loadUserConfigLocked()

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
	ensureLoaded()
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
	defer configMu.Unlock()
	userConfig = nil
	loadUserConfigLocked()
}

func GetBreakReminderEnabled() bool {
	ensureLoaded()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.BreakReminderEnabled
}

func GetBreakReminderMinutes() int {
	ensureLoaded()
	configMu.RLock()
	defer configMu.RUnlock()
	mins := userConfig.BreakReminderMinutes
	if mins < 1 {
		return 60
	}
	return mins
}

func SetBreakReminder(enabled bool, minutes int) error {
	configMu.Lock()
	defer configMu.Unlock()
	config := loadUserConfigLocked()
	config.BreakReminderEnabled = enabled
	if minutes > 0 {
		config.BreakReminderMinutes = minutes
	}
	return saveUserConfigLocked()
}

func GetAppTimeLimits() map[string]int {
	ensureLoaded()
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
	configMu.Lock()
	defer configMu.Unlock()
	config := loadUserConfigLocked()
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
	configMu.Lock()
	defer configMu.Unlock()
	config := loadUserConfigLocked()
	exeName = strings.ToLower(strings.TrimSpace(exeName))
	delete(config.AppTimeLimits, exeName)
	return saveUserConfigLocked()
}

func GetPomodoroMinutes() int {
	ensureLoaded()
	configMu.RLock()
	defer configMu.RUnlock()
	mins := userConfig.PomodoroMinutes
	if mins < 1 {
		return 25
	}
	return mins
}

func SetPomodoroMinutes(minutes int) error {
	configMu.Lock()
	defer configMu.Unlock()
	config := loadUserConfigLocked()
	config.PomodoroMinutes = minutes
	return saveUserConfigLocked()
}

func GetPassword() string {
	ensureLoaded()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.Password
}

func SetPassword(password string) error {
	configMu.Lock()
	defer configMu.Unlock()
	config := loadUserConfigLocked()
	config.Password = password
	return saveUserConfigLocked()
}

func IsPasswordEnabled() bool {
	ensureLoaded()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.Password != ""
}

func CheckPassword(input string) bool {
	ensureLoaded()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.Password == input
}

func ClearPassword() error {
	configMu.Lock()
	defer configMu.Unlock()
	config := loadUserConfigLocked()
	config.Password = ""
	return saveUserConfigLocked()
}

func GetSnoozeDurationMinutes() int {
	ensureLoaded()
	configMu.RLock()
	defer configMu.RUnlock()
	mins := userConfig.SnoozeDurationMinutes
	if mins < 1 {
		return 60
	}
	return mins
}

func SetSnoozeDurationMinutes(minutes int) error {
	configMu.Lock()
	defer configMu.Unlock()
	config := loadUserConfigLocked()
	config.SnoozeDurationMinutes = minutes
	return saveUserConfigLocked()
}

func GetSmartGroupingEnabled() bool {
	ensureLoaded()
	configMu.RLock()
	defer configMu.RUnlock()
	return userConfig.SmartGroupingEnabled
}

func SetSmartGroupingEnabled(enabled bool) error {
	configMu.Lock()
	defer configMu.Unlock()
	config := loadUserConfigLocked()
	config.SmartGroupingEnabled = enabled
	return saveUserConfigLocked()
}
