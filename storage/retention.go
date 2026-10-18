package storage

import (
	"log"
	"time"
)

func EnforceRetention() {
	days, err := resolveRetentionDays()
	if err != nil {
		// Never fall back to the default here: one transient read failure
		// (for example SQLITE_BUSY) would silently prune the history down to
		// the default window.
		log.Printf("WARN: retention skipped, configured window unreadable: %v", err)
		return
	}
	cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	tx, err := db.Begin()
	if err != nil {
		log.Printf("WARN: EnforceRetention begin tx: %v", err)
		return
	}
	defer tx.Rollback()

	queries := []string{
		"DELETE FROM sessions WHERE date < ?",
		"DELETE FROM apps_daily WHERE date < ?",
		"DELETE FROM browsing_daily WHERE date < ?",
	}
	for _, q := range queries {
		if _, err := tx.Exec(q, cutoff); err != nil {
			log.Printf("WARN: EnforceRetention exec: %v", err)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("WARN: EnforceRetention commit: %v", err)
		return
	}

	if _, err := db.Exec("PRAGMA incremental_vacuum"); err != nil {
		log.Printf("WARN: incremental_vacuum failed: %v", err)
	}
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
