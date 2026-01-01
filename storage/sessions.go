package storage

import (
	"time"
)

type Session struct {
	ID           int64
	AppName      string
	ExeName      string
	WindowTitle  string
	StartTime    time.Time
	EndTime      time.Time
	DurationSecs int
	Date         string
}

func InsertSession(s *Session) error {
	var endTime *int64
	if !s.EndTime.IsZero() {
		t := s.EndTime.Unix()
		endTime = &t
	}

	_, err := db.Exec(`
		INSERT INTO sessions (app_name, exe_name, window_title, start_time, end_time, duration_secs, date)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, s.AppName, s.ExeName, s.WindowTitle, s.StartTime.Unix(), endTime, s.DurationSecs, s.Date)
	return err
}

func InsertSessionsBatch(sessions []*Session) error {
	if len(sessions) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO sessions (app_name, exe_name, window_title, start_time, end_time, duration_secs, date)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, s := range sessions {
		var endTime *int64
		if !s.EndTime.IsZero() {
			t := s.EndTime.Unix()
			endTime = &t
		}
		if _, err := stmt.Exec(s.AppName, s.ExeName, s.WindowTitle, s.StartTime.Unix(), endTime, s.DurationSecs, s.Date); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func UpdateAppDaily(date, appName, exeName string, durationSecs int) error {
	_, err := db.Exec(`
		INSERT INTO apps_daily (date, app_name, exe_name, total_duration_secs, open_count)
		VALUES (?, ?, ?, ?, 1)
		ON CONFLICT(date, exe_name) DO UPDATE SET
			total_duration_secs = total_duration_secs + excluded.total_duration_secs,
			open_count = open_count + 1
	`, date, appName, exeName, durationSecs)
	return err
}

func IncrementAppOpenCount(date, exeName string) error {
	_, err := db.Exec(`
		UPDATE apps_daily SET open_count = open_count + 1
		WHERE date = ? AND exe_name = ?
	`, date, exeName)
	return err
}

type AppDailyStat struct {
	Date              string
	AppName           string
	ExeName           string
	TotalDurationSecs int
	OpenCount         int
}

func GetAppStatsForDate(date string) ([]AppDailyStat, error) {
	rows, err := db.Query(`
		SELECT date, app_name, exe_name, total_duration_secs, open_count
		FROM apps_daily
		WHERE date = ?
		ORDER BY total_duration_secs DESC
	`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []AppDailyStat
	for rows.Next() {
		var s AppDailyStat
		if err := rows.Scan(&s.Date, &s.AppName, &s.ExeName, &s.TotalDurationSecs, &s.OpenCount); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

func GetAppUsageTodayMinutes(exeName string) int {
	today := Today()
	var secs int
	err := db.QueryRow(`
		SELECT COALESCE(total_duration_secs, 0) FROM apps_daily
		WHERE date = ? AND exe_name = ?
	`, today, exeName).Scan(&secs)
	if err != nil {
		return 0
	}
	return secs / 60
}

func GetAllSessions() ([]Session, error) {
	rows, err := db.Query(`
		SELECT id, app_name, exe_name, window_title, start_time, end_time, duration_secs, date
		FROM sessions
		ORDER BY start_time DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		var startTime int64
		var endTime *int64
		if err := rows.Scan(&s.ID, &s.AppName, &s.ExeName, &s.WindowTitle, &startTime, &endTime, &s.DurationSecs, &s.Date); err != nil {
			return nil, err
		}
		s.StartTime = time.Unix(startTime, 0)
		if endTime != nil {
			s.EndTime = time.Unix(*endTime, 0)
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func GetSessionsPaginated(limit, offset int, startDate, endDate string) ([]Session, int, error) {
	countQuery := `SELECT COUNT(*) FROM sessions`
	dataQuery := `
		SELECT id, app_name, exe_name, window_title, start_time, end_time, duration_secs, date
		FROM sessions
	`

	var args []interface{}
	whereClause := ""

	if startDate != "" && endDate != "" {
		whereClause = " WHERE date >= ? AND date <= ?"
		args = append(args, startDate, endDate)
	} else if startDate != "" {
		whereClause = " WHERE date >= ?"
		args = append(args, startDate)
	} else if endDate != "" {
		whereClause = " WHERE date <= ?"
		args = append(args, endDate)
	}

	var total int
	err := db.QueryRow(countQuery+whereClause, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	dataQuery += whereClause + " ORDER BY start_time DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.Query(dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		var startTime int64
		var endTime *int64
		if err := rows.Scan(&s.ID, &s.AppName, &s.ExeName, &s.WindowTitle, &startTime, &endTime, &s.DurationSecs, &s.Date); err != nil {
			return nil, 0, err
		}
		s.StartTime = time.Unix(startTime, 0)
		if endTime != nil {
			s.EndTime = time.Unix(*endTime, 0)
		}
		sessions = append(sessions, s)
	}
	return sessions, total, rows.Err()
}

func GetSessionsByDateRange(startDate, endDate string) ([]Session, error) {
	rows, err := db.Query(`
		SELECT id, app_name, exe_name, window_title, start_time, end_time, duration_secs, date
		FROM sessions
		WHERE date >= ? AND date <= ?
		ORDER BY start_time DESC
	`, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		var startTime int64
		var endTime *int64
		if err := rows.Scan(&s.ID, &s.AppName, &s.ExeName, &s.WindowTitle, &startTime, &endTime, &s.DurationSecs, &s.Date); err != nil {
			return nil, err
		}
		s.StartTime = time.Unix(startTime, 0)
		if endTime != nil {
			s.EndTime = time.Unix(*endTime, 0)
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func GetAllAppStats() ([]AppDailyStat, error) {
	rows, err := db.Query(`
		SELECT date, app_name, exe_name, total_duration_secs, open_count
		FROM apps_daily
		ORDER BY date DESC, total_duration_secs DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []AppDailyStat
	for rows.Next() {
		var s AppDailyStat
		if err := rows.Scan(&s.Date, &s.AppName, &s.ExeName, &s.TotalDurationSecs, &s.OpenCount); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

func GetAllBrowserStats() ([]AppDailyStat, error) {
	rows, err := db.Query(`
		SELECT date, domain_or_title, '', total_duration_secs, open_count
		FROM browsing_daily
		ORDER BY date DESC, total_duration_secs DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []AppDailyStat
	for rows.Next() {
		var s AppDailyStat
		if err := rows.Scan(&s.Date, &s.AppName, &s.ExeName, &s.TotalDurationSecs, &s.OpenCount); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

func UpdateBrowserDaily(date, domainOrTitle string, durationSecs int) error {
	_, err := db.Exec(`
		INSERT INTO browsing_daily (date, domain_or_title, total_duration_secs, open_count)
		VALUES (?, ?, ?, 1)
		ON CONFLICT(date, domain_or_title) DO UPDATE SET
			total_duration_secs = total_duration_secs + excluded.total_duration_secs,
			open_count = open_count + 1
	`, date, domainOrTitle, durationSecs)
	return err
}

func GetBrowserStatsForDate(date string) ([]AppDailyStat, error) {
	rows, err := db.Query(`
		SELECT date, domain_or_title, '', total_duration_secs, open_count
		FROM browsing_daily
		WHERE date = ?
		ORDER BY total_duration_secs DESC
	`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []AppDailyStat
	for rows.Next() {
		var s AppDailyStat
		if err := rows.Scan(&s.Date, &s.AppName, &s.ExeName, &s.TotalDurationSecs, &s.OpenCount); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

type ActiveSessionRecord struct {
	AppName     string
	ExeName     string
	WindowTitle string
	StartTime   time.Time
	LastSeen    time.Time
	Date        string
}

func SaveActiveSession(s *ActiveSessionRecord) error {
	if s == nil {
		return nil
	}
	_, err := db.Exec(`
		INSERT OR REPLACE INTO active_session (id, app_name, exe_name, window_title, start_time, last_seen, date)
		VALUES (1, ?, ?, ?, ?, ?, ?)
	`, s.AppName, s.ExeName, s.WindowTitle, s.StartTime.Unix(), s.LastSeen.Unix(), s.Date)
	return err
}

func ClearActiveSession() error {
	_, err := db.Exec("DELETE FROM active_session WHERE id = 1")
	return err
}

func RecoverActiveSession() (*Session, error) {
	var s ActiveSessionRecord
	var startTime, lastSeen int64

	err := db.QueryRow(`
		SELECT app_name, exe_name, window_title, start_time, last_seen, date
		FROM active_session WHERE id = 1
	`).Scan(&s.AppName, &s.ExeName, &s.WindowTitle, &startTime, &lastSeen, &s.Date)

	if err != nil {
		return nil, nil
	}

	ClearActiveSession()

	duration := int(lastSeen - startTime)
	if duration < 1 {
		return nil, nil
	}

	return &Session{
		AppName:      s.AppName,
		ExeName:      s.ExeName,
		WindowTitle:  s.WindowTitle,
		StartTime:    time.Unix(startTime, 0),
		EndTime:      time.Unix(lastSeen, 0),
		DurationSecs: duration,
		Date:         s.Date,
	}, nil
}
