package storage

import (
	"strconv"
	"time"
)

const (
	ConfigKeyRetentionDays    = "retention_days"
	ConfigKeyAutostart        = "autostart_enabled"
	ConfigKeyPathEnabled      = "path_enabled"
	ConfigKeyPaused           = "tracking_paused"
	ConfigKeyTrackingInterval = "tracking_interval_seconds"

	DefaultRetentionDays = 7
	MaxRetentionDays     = 30
	MinRetentionDays     = 1

	DefaultTrackingIntervalSeconds = 5
	MinTrackingIntervalSeconds     = 1
	MaxTrackingIntervalSeconds     = 60
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

func GetRetentionDays() int {
	value, err := GetConfig(ConfigKeyRetentionDays)
	if err != nil {
		return DefaultRetentionDays
	}
	days, err := strconv.Atoi(value)
	if err != nil || days < MinRetentionDays || days > MaxRetentionDays {
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
	return SetConfig(ConfigKeyRetentionDays, strconv.Itoa(days))
}

func IsPaused() bool {
	value, err := GetConfig(ConfigKeyPaused)
	return err == nil && value == "true"
}

func SetPaused(paused bool) error {
	return SetConfig(ConfigKeyPaused, strconv.FormatBool(paused))
}

func GetTrackingIntervalSeconds() int {
	value, err := GetConfig(ConfigKeyTrackingInterval)
	if err != nil {
		return DefaultTrackingIntervalSeconds
	}
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds < MinTrackingIntervalSeconds || seconds > MaxTrackingIntervalSeconds {
		return DefaultTrackingIntervalSeconds
	}
	return seconds
}
