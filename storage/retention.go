package storage

import (
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

	_, err := db.Exec("VACUUM")
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

func ClearAllTrackingData() error {
	if _, err := db.Exec("DELETE FROM sessions"); err != nil {
		return err
	}
	if _, err := db.Exec("DELETE FROM apps_daily"); err != nil {
		return err
	}

	if _, err := db.Exec("DELETE FROM active_session"); err != nil {
		return err
	}
	_, err := db.Exec("VACUUM")
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
