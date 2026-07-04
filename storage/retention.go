package storage

import (
	"time"
)

func EnforceRetention() error {
	days := GetRetentionDays()
	cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	queries := []string{
		"DELETE FROM sessions WHERE date < ?",
		"DELETE FROM apps_daily WHERE date < ?",
		"DELETE FROM browsing_daily WHERE date < ?",
	}
	for _, q := range queries {
		if _, err := tx.Exec(q, cutoff); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	_, err = db.Exec("PRAGMA incremental_vacuum")
	return err
}

func ClearAllTrackingData() error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	queries := []string{
		"DELETE FROM sessions",
		"DELETE FROM apps_daily",
		"DELETE FROM browsing_daily",
		"DELETE FROM active_session",
	}
	for _, q := range queries {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	_, err = db.Exec("VACUUM")
	return err
}
