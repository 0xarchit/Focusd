package system

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type UserConfig struct {
	WhitelistApps         []string       `json:"whitelist_apps"`
	BreakReminderEnabled  bool           `json:"break_reminder_enabled"`
	BreakReminderMinutes  int            `json:"break_reminder_minutes"`
	AppTimeLimits         map[string]int `json:"app_time_limits"`
	PomodoroMinutes       int            `json:"pomodoro_minutes"`
	Password              string         `json:"password"`
	SnoozeDurationMinutes int            `json:"snooze_duration_minutes"`
}

var userConfig *UserConfig

func getUserConfigPath() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", nil
	}
	return filepath.Join(appData, "focusd", "config.json"), nil
}

func loadUserConfig() *UserConfig {
	if userConfig != nil {
		return userConfig
	}

	userConfig = &UserConfig{

		WhitelistApps:         []string{},
		BreakReminderEnabled:  false,
		BreakReminderMinutes:  60,
		AppTimeLimits:         make(map[string]int),
		PomodoroMinutes:       25,
		Password:              "",
		SnoozeDurationMinutes: 60,
	}

	configPath, err := getUserConfigPath()
	if err != nil || configPath == "" {
		return userConfig
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return userConfig
	}

	json.Unmarshal(data, userConfig)
	if userConfig.AppTimeLimits == nil {
		userConfig.AppTimeLimits = make(map[string]int)
	}
	return userConfig
}

func SaveUserConfig() error {
	if userConfig == nil {
		userConfig = &UserConfig{

			WhitelistApps:        []string{},
			BreakReminderEnabled: false,
			BreakReminderMinutes: 60,
			AppTimeLimits:        make(map[string]int),
			PomodoroMinutes:      25,
			Password:             "",
		}
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
	config := loadUserConfig()
	return config.WhitelistApps
}

func AddWhitelistApp(exeName string) error {
	config := loadUserConfig()

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
	return SaveUserConfig()
}

func RemoveWhitelistApp(exeName string) error {
	config := loadUserConfig()

	var updated []string
	for _, a := range config.WhitelistApps {
		if !strings.EqualFold(a, exeName) {
			updated = append(updated, a)
		}
	}
	config.WhitelistApps = updated
	return SaveUserConfig()
}

func IsWhitelisted(exeName string) bool {
	config := loadUserConfig()

	exeName = strings.ToLower(exeName)
	for _, a := range config.WhitelistApps {
		if strings.EqualFold(a, exeName) {
			return true
		}
	}
	return false
}

func ReloadUserConfig() {
	userConfig = nil
	loadUserConfig()
}

func GetBreakReminderEnabled() bool {
	return loadUserConfig().BreakReminderEnabled
}

func GetBreakReminderMinutes() int {
	mins := loadUserConfig().BreakReminderMinutes
	if mins < 1 {
		return 60
	}
	return mins
}

func SetBreakReminder(enabled bool, minutes int) error {
	config := loadUserConfig()
	config.BreakReminderEnabled = enabled
	if minutes > 0 {
		config.BreakReminderMinutes = minutes
	}
	return SaveUserConfig()
}

func GetAppTimeLimits() map[string]int {
	return loadUserConfig().AppTimeLimits
}

func SetAppTimeLimit(exeName string, minutes int) error {
	config := loadUserConfig()
	exeName = strings.ToLower(strings.TrimSpace(exeName))
	if !strings.HasSuffix(exeName, ".exe") {
		exeName += ".exe"
	}
	if minutes <= 0 {
		delete(config.AppTimeLimits, exeName)
	} else {
		config.AppTimeLimits[exeName] = minutes
	}
	return SaveUserConfig()
}

func RemoveAppTimeLimit(exeName string) error {
	config := loadUserConfig()
	exeName = strings.ToLower(strings.TrimSpace(exeName))
	delete(config.AppTimeLimits, exeName)
	return SaveUserConfig()
}

func GetPomodoroMinutes() int {
	mins := loadUserConfig().PomodoroMinutes
	if mins < 1 {
		return 25
	}
	return mins
}

func SetPomodoroMinutes(minutes int) error {
	config := loadUserConfig()
	config.PomodoroMinutes = minutes
	return SaveUserConfig()
}

func GetPassword() string {
	return loadUserConfig().Password
}

func SetPassword(password string) error {
	config := loadUserConfig()
	config.Password = password
	return SaveUserConfig()
}

func IsPasswordEnabled() bool {
	return loadUserConfig().Password != ""
}

func CheckPassword(input string) bool {
	return loadUserConfig().Password == input
}

func ClearPassword() error {
	config := loadUserConfig()
	config.Password = ""
	return SaveUserConfig()
}

func GetSnoozeDurationMinutes() int {
	mins := loadUserConfig().SnoozeDurationMinutes
	if mins < 1 {
		return 60
	}
	return mins
}

func SetSnoozeDurationMinutes(minutes int) error {
	config := loadUserConfig()
	config.SnoozeDurationMinutes = minutes
	return SaveUserConfig()
}
