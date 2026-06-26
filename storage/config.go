package storage

import (
	"strconv"
	"time"
)

const (
	ConfigKeyConsent          = "consent_granted"
	ConfigKeyConsentTimestamp = "consent_timestamp"
	ConfigKeyRetentionDays    = "retention_days"
	ConfigKeyAutostart        = "autostart_enabled"
	ConfigKeyPathEnabled      = "path_enabled"
	ConfigKeyPaused           = "tracking_paused"
	ConfigKeyTrackingInterval = "tracking_interval_seconds"
	ConfigKeyIdleThreshold    = "idle_threshold_seconds"
	ConfigKeyWarningThreshold = "warning_threshold_percent"

	DefaultRetentionDays = 7
	MaxRetentionDays     = 30
	MinRetentionDays     = 1

	DefaultTrackingIntervalSeconds = 5
	MinTrackingIntervalSeconds     = 1
	MaxTrackingIntervalSeconds     = 60

	DefaultIdleThresholdSeconds = 60
	MinIdleThresholdSeconds     = 15
	MaxIdleThresholdSeconds     = 600

	DefaultWarningThresholdPercent = 80
	MinWarningThresholdPercent     = 50
	MaxWarningThresholdPercent     = 100
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

func getBoolConfig(key string) bool {
	value, err := GetConfig(key)
	return err == nil && value == "true"
}

func setBoolConfig(key string, val bool) error {
	return SetConfig(key, strconv.FormatBool(val))
}

func IsConsentGranted() bool {
	return true
}

func SetConsent(granted bool) error {
	return nil
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
	return getBoolConfig(ConfigKeyPaused)
}

func SetPaused(paused bool) error {
	return setBoolConfig(ConfigKeyPaused, paused)
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

func SetTrackingIntervalSeconds(seconds int) error {
	if seconds < MinTrackingIntervalSeconds {
		seconds = MinTrackingIntervalSeconds
	}
	if seconds > MaxTrackingIntervalSeconds {
		seconds = MaxTrackingIntervalSeconds
	}
	return SetConfig(ConfigKeyTrackingInterval, strconv.Itoa(seconds))
}

func GetIdleThresholdSeconds() int {
	value, err := GetConfig(ConfigKeyIdleThreshold)
	if err != nil {
		return DefaultIdleThresholdSeconds
	}
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds < MinIdleThresholdSeconds || seconds > MaxIdleThresholdSeconds {
		return DefaultIdleThresholdSeconds
	}
	return seconds
}

func SetIdleThresholdSeconds(seconds int) error {
	if seconds < MinIdleThresholdSeconds {
		seconds = MinIdleThresholdSeconds
	}
	if seconds > MaxIdleThresholdSeconds {
		seconds = MaxIdleThresholdSeconds
	}
	return SetConfig(ConfigKeyIdleThreshold, strconv.Itoa(seconds))
}

func GetWarningThresholdPercent() int {
	value, err := GetConfig(ConfigKeyWarningThreshold)
	if err != nil {
		return DefaultWarningThresholdPercent
	}
	pct, err := strconv.Atoi(value)
	if err != nil || pct < MinWarningThresholdPercent || pct > MaxWarningThresholdPercent {
		return DefaultWarningThresholdPercent
	}
	return pct
}

func SetWarningThresholdPercent(pct int) error {
	if pct < MinWarningThresholdPercent {
		pct = MinWarningThresholdPercent
	}
	if pct > MaxWarningThresholdPercent {
		pct = MaxWarningThresholdPercent
	}
	return SetConfig(ConfigKeyWarningThreshold, strconv.Itoa(pct))
}
