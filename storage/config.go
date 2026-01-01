package storage

import (
	"time"
)

const (
	ConfigKeyConsent          = "consent_granted"
	ConfigKeyConsentTimestamp = "consent_timestamp"
	ConfigKeyRetentionDays    = "retention_days"
	ConfigKeyAutostart        = "autostart_enabled"
	ConfigKeyPathEnabled      = "path_enabled"
	ConfigKeyPaused           = "tracking_paused"

	DefaultRetentionDays = 7
	MaxRetentionDays     = 30
	MinRetentionDays     = 1
)

func GetConfig(key string) (string, error) {
	var value string
	err := db.QueryRow("SELECT value FROM config WHERE key = ?", key).Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

func SetConfig(key, value string) error {
	_, err := db.Exec(`
		INSERT INTO config (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, value, time.Now().Unix())
	return err
}

func DeleteConfig(key string) error {
	_, err := db.Exec("DELETE FROM config WHERE key = ?", key)
	return err
}

func IsConsentGranted() bool {
	value, err := GetConfig(ConfigKeyConsent)
	if err != nil {
		return false
	}
	return value == "true"
}

func SetConsent(granted bool) error {
	value := "false"
	if granted {
		value = "true"
	}
	if err := SetConfig(ConfigKeyConsent, value); err != nil {
		return err
	}
	return SetConfig(ConfigKeyConsentTimestamp, time.Now().Format(time.RFC3339))
}

func GetRetentionDays() int {
	value, err := GetConfig(ConfigKeyRetentionDays)
	if err != nil {
		return DefaultRetentionDays
	}
	var days int
	if _, err := time.ParseDuration(value + "h"); err == nil {
		return DefaultRetentionDays
	}
	if n, err := parseInt(value); err == nil {
		days = n
	} else {
		return DefaultRetentionDays
	}
	if days < MinRetentionDays || days > MaxRetentionDays {
		return DefaultRetentionDays
	}
	return days
}

func SetRetentionDays(days int) error {
	if days < MinRetentionDays {
		days = MinRetentionDays
	}
	if days > MaxRetentionDays {
		days = MaxRetentionDays
	}
	return SetConfig(ConfigKeyRetentionDays, intToStr(days))
}

func IsPaused() bool {
	value, err := GetConfig(ConfigKeyPaused)
	if err != nil {
		return false
	}
	return value == "true"
}

func SetPaused(paused bool) error {
	value := "false"
	if paused {
		value = "true"
	}
	return SetConfig(ConfigKeyPaused, value)
}

func parseInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, nil
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
