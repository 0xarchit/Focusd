package storage

import (
	"fmt"
	"strconv"
	"time"
)

const (
	configKeyRetentionDays    = "retention_days"
	configKeyPaused           = "tracking_paused"
	configKeyTrackingInterval = "tracking_interval_seconds"

	DefaultRetentionDays = 7
	MaxRetentionDays     = 30
	MinRetentionDays     = 1

	DefaultTrackingIntervalSeconds = 5
	MinTrackingIntervalSeconds     = 1
	MaxTrackingIntervalSeconds     = 60
)

func getConfig(key string) (string, error) {
	if db == nil {
		return "", fmt.Errorf("database not initialized")
	}
	var value string
	err := db.QueryRow("SELECT value FROM config WHERE key = ?", key).Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

func setConfig(key, value string) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	_, err := db.Exec(`
		INSERT INTO config (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, value, time.Now().Unix())
	return err
}

func GetRetentionDays() int {
	value, err := getConfig(configKeyRetentionDays)
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
	return setConfig(configKeyRetentionDays, strconv.Itoa(days))
}

func IsPaused() bool {
	value, err := getConfig(configKeyPaused)
	return err == nil && value == "true"
}

func SetPaused(paused bool) error {
	return setConfig(configKeyPaused, strconv.FormatBool(paused))
}

func GetTrackingIntervalSeconds() int {
	value, err := getConfig(configKeyTrackingInterval)
	if err != nil {
		return DefaultTrackingIntervalSeconds
	}
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds < MinTrackingIntervalSeconds || seconds > MaxTrackingIntervalSeconds {
		return DefaultTrackingIntervalSeconds
	}
	return seconds
}
