package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	configKeyRetentionDays    = "retention_days"
	configKeyPaused           = "tracking_paused"
	configKeyTrackingInterval = "tracking_interval_seconds"

	DefaultRetentionDays = 7
	MaxRetentionDays     = 365
	MinRetentionDays     = 1

	DefaultTrackingIntervalSeconds = 5
	MinTrackingIntervalSeconds     = 1
	MaxTrackingIntervalSeconds     = 60
)

func GetConfig(key string) (string, error) {
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

func SetConfig(key, value string) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	_, err := db.Exec(`
		INSERT INTO config (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, value, time.Now().Unix())
	return err
}

func CompareAndSwapConfig(key, oldValue, newValue string) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("database not initialized")
	}

	// If the row doesn't exist yet, try to insert it if oldValue is empty
	if oldValue == "" {
		res, err := db.Exec(`
			INSERT INTO config (key, value, updated_at)
			VALUES (?, ?, ?)
			ON CONFLICT(key) DO NOTHING
		`, key, newValue, time.Now().Unix())
		if err != nil {
			return false, err
		}
		rows, err := res.RowsAffected()
		if err != nil {
			return false, err
		}
		if rows > 0 {
			return true, nil
		}
		// If insert did nothing, row exists. Fall through to update check
	}

	res, err := db.Exec(`
		UPDATE config SET value = ?, updated_at = ? WHERE key = ? AND value = ?
	`, newValue, time.Now().Unix(), key, oldValue)
	if err != nil {
		return false, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// GetRetentionDays returns the configured retention window for display. A
// failed read falls back to the default; enforcement must use
// resolveRetentionDays instead, so a transient failure never prunes history
// down to the default window.
func GetRetentionDays() int {
	days, err := resolveRetentionDays()
	if err != nil {
		return DefaultRetentionDays
	}
	return days
}

// resolveRetentionDays reads the retention window and distinguishes "not set"
// from "read failed". A missing row (or an unreadable value) means the
// default; a failed read returns an error and callers must skip enforcement.
func resolveRetentionDays() (int, error) {
	value, err := GetConfig(configKeyRetentionDays)
	if errors.Is(err, sql.ErrNoRows) {
		return DefaultRetentionDays, nil
	}
	if err != nil {
		return 0, err
	}
	days, perr := strconv.Atoi(strings.TrimSpace(value))
	if perr != nil || days < MinRetentionDays || days > MaxRetentionDays {
		return DefaultRetentionDays, nil
	}
	return days, nil
}

func SetRetentionDays(days int) error {
	if days < MinRetentionDays {
		days = MinRetentionDays
	}
	if days > MaxRetentionDays {
		days = MaxRetentionDays
	}
	return SetConfig(configKeyRetentionDays, strconv.Itoa(days))
}

func IsPaused() bool {
	value, err := GetConfig(configKeyPaused)
	return err == nil && value == "true"
}

func SetPaused(paused bool) error {
	return SetConfig(configKeyPaused, strconv.FormatBool(paused))
}

func GetTrackingIntervalSeconds() int {
	value, err := GetConfig(configKeyTrackingInterval)
	if err != nil {
		return DefaultTrackingIntervalSeconds
	}
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds < MinTrackingIntervalSeconds || seconds > MaxTrackingIntervalSeconds {
		return DefaultTrackingIntervalSeconds
	}
	return seconds
}
