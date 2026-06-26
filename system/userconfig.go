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

func loadUserConfigLocked() *UserConfig {
	if userConfig == nil {
		userConfig = defaultUserConfig()

		configPath, err := getUserConfigPath()
		if err != nil || configPath == "" {
			return userConfig
		}

		data, err := os.ReadFile(configPath)
		if err != nil {
			return userConfig
		}

		if err := json.Unmarshal(data, userConfig); err != nil {
			log.Printf("WARN: failed to parse user config %s: %v", configPath, err)
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
	configMu.RLock()
	defer configMu.RUnlock()
	config := loadUserConfigLocked()
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
	configMu.RLock()
	defer configMu.RUnlock()
	config := loadUserConfigLocked()

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
	configMu.RLock()
	defer configMu.RUnlock()
	return loadUserConfigLocked().BreakReminderEnabled
}

func GetBreakReminderMinutes() int {
	configMu.RLock()
	defer configMu.RUnlock()
	mins := loadUserConfigLocked().BreakReminderMinutes
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
	configMu.RLock()
	defer configMu.RUnlock()
	config := loadUserConfigLocked()
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
	configMu.RLock()
	defer configMu.RUnlock()
	mins := loadUserConfigLocked().PomodoroMinutes
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
	configMu.RLock()
	defer configMu.RUnlock()
	return loadUserConfigLocked().Password
}

func SetPassword(password string) error {
	configMu.Lock()
	defer configMu.Unlock()
	config := loadUserConfigLocked()
	config.Password = password
	return saveUserConfigLocked()
}

func IsPasswordEnabled() bool {
	configMu.RLock()
	defer configMu.RUnlock()
	return loadUserConfigLocked().Password != ""
}

func CheckPassword(input string) bool {
	configMu.RLock()
	defer configMu.RUnlock()
	return loadUserConfigLocked().Password == input
}

func ClearPassword() error {
	configMu.Lock()
	defer configMu.Unlock()
	config := loadUserConfigLocked()
	config.Password = ""
	return saveUserConfigLocked()
}

func GetSnoozeDurationMinutes() int {
	configMu.RLock()
	defer configMu.RUnlock()
	mins := loadUserConfigLocked().SnoozeDurationMinutes
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
	configMu.RLock()
	defer configMu.RUnlock()
	return loadUserConfigLocked().SmartGroupingEnabled
}

func SetSmartGroupingEnabled(enabled bool) error {
	configMu.Lock()
	defer configMu.Unlock()
	config := loadUserConfigLocked()
	config.SmartGroupingEnabled = enabled
	return saveUserConfigLocked()
}

