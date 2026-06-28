package storage

import (
	"database/sql"
	"fmt"
	"time"
)

func EnforceRetention() error {
	days := GetRetentionDays()
	cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	if _, err := db.Exec("DELETE FROM sessions WHERE date < ?", cutoff); err != nil {
		return err
	}
	if _, err := db.Exec("DELETE FROM apps_daily WHERE date < ?", cutoff); err != nil {
		return err
	}
	if _, err := db.Exec("DELETE FROM browsing_daily WHERE date < ?", cutoff); err != nil {
		return err
	}

	_, err := db.Exec("PRAGMA incremental_vacuum")
	return err
}

func GetOldestDate() (string, error) {
	var date string
	err := db.QueryRow(`
		SELECT MIN(date) FROM (
			SELECT date FROM sessions
			UNION
			SELECT date FROM apps_daily

		)
	`).Scan(&date)
	return date, err
}

func GetTotalSessionCount() (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count)
	return count, err
}

func execOrRollback(tx *sql.Tx, query string) error {
	if _, err := tx.Exec(query); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("query %q failed: %v (rollback also failed: %v)", query, err, rbErr)
		}
		return err
	}
	return nil
}

func ClearAllTrackingData() error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	if err := execOrRollback(tx, "DELETE FROM sessions"); err != nil {
		return err
	}
	if err := execOrRollback(tx, "DELETE FROM apps_daily"); err != nil {
		return err
	}
	if err := execOrRollback(tx, "DELETE FROM browsing_daily"); err != nil {
		return err
	}
	if err := execOrRollback(tx, "DELETE FROM active_session"); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	_, err = db.Exec("VACUUM")
	return err
}

func ClearTodayData() error {
	today := Today()
	if _, err := db.Exec("DELETE FROM sessions WHERE date = ?", today); err != nil {
		return err
	}
	if _, err := db.Exec("DELETE FROM apps_daily WHERE date = ?", today); err != nil {
		return err
	}

	return nil
}

func ClearLastHourData() error {
	cutoff := time.Now().Add(-1 * time.Hour).Unix()
	if _, err := db.Exec("DELETE FROM sessions WHERE start_time >= ?", cutoff); err != nil {
		return err
	}
	return nil
}

func ClearLast24HoursData() error {
	cutoff := time.Now().Add(-24 * time.Hour).Unix()
	cutoffDate := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	if _, err := db.Exec("DELETE FROM sessions WHERE start_time >= ?", cutoff); err != nil {
		return err
	}
	if _, err := db.Exec("DELETE FROM apps_daily WHERE date >= ?", cutoffDate); err != nil {
		return err
	}

	return nil
}

func ClearLastNHoursData(hours int) error {
	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour).Unix()
	if _, err := db.Exec("DELETE FROM sessions WHERE start_time >= ?", cutoff); err != nil {
		return err
	}
	return nil
}
