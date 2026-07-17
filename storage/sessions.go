package storage

import (
	"database/sql"
	"errors"
	"log"
	"strings"
	"time"
)

type Session struct {
	AppName      string
	ExeName      string
	WindowTitle  string
	StartTime    time.Time
	EndTime      time.Time
	DurationSecs int
	Date         string
}

func InsertSessionWithDaily(s *Session, cleanBrowserTitle string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var endTime *int64
	if !s.EndTime.IsZero() {
		t := s.EndTime.Unix()
		endTime = &t
	}

	_, err = tx.Exec(`
		INSERT INTO sessions (app_name, exe_name, window_title, start_time, end_time, duration_secs, date)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, s.AppName, s.ExeName, s.WindowTitle, s.StartTime.Unix(), endTime, s.DurationSecs, s.Date)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO apps_daily (date, app_name, exe_name, total_duration_secs, open_count)
		VALUES (?, ?, ?, ?, 1)
		ON CONFLICT(date, exe_name) DO UPDATE SET
			total_duration_secs = total_duration_secs + excluded.total_duration_secs,
			open_count = open_count + 1
	`, s.Date, s.AppName, s.ExeName, s.DurationSecs)
	if err != nil {
		return err
	}

	if cleanBrowserTitle != "" {
		_, err = tx.Exec(`
			INSERT INTO browsing_daily (date, domain_or_title, total_duration_secs, open_count)
			VALUES (?, ?, ?, 1)
			ON CONFLICT(date, domain_or_title) DO UPDATE SET
				total_duration_secs = total_duration_secs + excluded.total_duration_secs,
				open_count = open_count + 1
		`, s.Date, cleanBrowserTitle, s.DurationSecs)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

type AppDailyStat struct {
	Date              string
	AppName           string
	ExeName           string
	TotalDurationSecs int
	OpenCount         int
}


func checkAndRotateActiveSession(today string) (string, string, int64) {
	var appName, activeExe, windowTitle, activeDate string
	var startTime int64
	err := db.QueryRow(`
		SELECT app_name, exe_name, window_title, start_time, date
		FROM active_session WHERE id = 1
	`).Scan(&appName, &activeExe, &windowTitle, &startTime, &activeDate)
	if err != nil || activeExe == "" {
		return "", "", 0
	}

	now := time.Now()
	localStartOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	localStartOfTodayUnix := localStartOfToday.Unix()

	if startTime < localStartOfTodayUnix {
		priorDuration := localStartOfTodayUnix - startTime
		if priorDuration > 0 {
			priorSession := &Session{
				AppName:      appName,
				ExeName:      activeExe,
				WindowTitle:  windowTitle,
				StartTime:    time.Unix(startTime, 0),
				EndTime:      time.Unix(localStartOfTodayUnix, 0),
				DurationSecs: int(priorDuration),
				Date:         activeDate,
			}
			_ = InsertSessionWithDaily(priorSession, "")
		}

		_, _ = db.Exec(`
			UPDATE active_session
			SET start_time = ?, date = ?
			WHERE id = 1
		`, localStartOfTodayUnix, today)

		return appName, activeExe, localStartOfTodayUnix
	}

	return appName, activeExe, startTime
}

func GetAppUsageTodaySeconds(exeName string) int {
	today := Today()
	var secs int
	err := db.QueryRow(`
		SELECT COALESCE(total_duration_secs, 0) FROM apps_daily
		WHERE date = ? AND exe_name = ? COLLATE NOCASE
	`, today, exeName).Scan(&secs)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Printf("WARN: GetAppUsageTodaySeconds query failed for %s: %v", exeName, err)
	}

	_, activeExe, startTime := checkAndRotateActiveSession(today)
	if activeExe != "" && strings.EqualFold(activeExe, exeName) {
		elapsed := time.Now().Unix() - startTime
		if elapsed > 0 {
			secs += int(elapsed)
		}
	}
	return secs
}

func GetAppUsageTodayMinutesMap() (map[string]int, error) {
	today := Today()
	rows, err := db.Query(`
		SELECT exe_name, total_duration_secs FROM apps_daily
		WHERE date = ?
	`, today)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]int)
	for rows.Next() {
		var exeName string
		var secs int
		if err := rows.Scan(&exeName, &secs); err != nil {
			return nil, err
		}
		res[strings.ToLower(exeName)] = secs / 60
	}

	_, activeExe, startTime := checkAndRotateActiveSession(today)
	if activeExe != "" {
		elapsed := time.Now().Unix() - startTime
		if elapsed > 0 {
			res[strings.ToLower(activeExe)] += int(elapsed) / 60
		}
	}

	return res, rows.Err()
}

func GetSessionsPaginated(limit, offset int, startDate, endDate string) ([]Session, error) {
	if limit < 0 {
		limit = 0
	}
	if offset < 0 {
		offset = 0
	}
	dataQuery := `
		SELECT app_name, exe_name, window_title, start_time, end_time, duration_secs, date
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

	dataQuery += whereClause + " ORDER BY start_time DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.Query(dataQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		var startTime int64
		var endTime *int64
		var windowTitle *string
		if err := rows.Scan(&s.AppName, &s.ExeName, &windowTitle, &startTime, &endTime, &s.DurationSecs, &s.Date); err != nil {
			return nil, err
		}
		if windowTitle != nil {
			s.WindowTitle = *windowTitle
		}
		s.StartTime = time.Unix(startTime, 0)
		if endTime != nil {
			s.EndTime = time.Unix(*endTime, 0)
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func GetAppStatsInRange(startDate, endDate string) ([]AppDailyStat, error) {
	query := `SELECT date, app_name, exe_name, total_duration_secs, open_count FROM apps_daily`
	var args []any
	if startDate != "" && endDate != "" {
		query += ` WHERE date >= ? AND date <= ?`
		args = append(args, startDate, endDate)
	}
	query += ` ORDER BY date DESC, total_duration_secs DESC`
	rows, err := db.Query(query, args...)
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

	// Dynamic active session injection if date range includes today
	today := Today()
	inRange := (startDate == "" && endDate == "") || (today >= startDate && today <= endDate)
	if inRange {
		activeApp, activeExe, startTime := checkAndRotateActiveSession(today)
		if activeExe != "" {
			elapsed := time.Now().Unix() - startTime
			if elapsed > 0 {
				found := false
				for i, s := range stats {
					if s.Date == today && strings.EqualFold(s.ExeName, activeExe) {
						stats[i].TotalDurationSecs += int(elapsed)
						stats[i].OpenCount += 1
						found = true
						break
					}
				}
				if !found {
					stats = append(stats, AppDailyStat{
						Date:              today,
						AppName:           activeApp,
						ExeName:           activeExe,
						TotalDurationSecs: int(elapsed),
						OpenCount:         1,
					})
				}
			}
		}
	}

	return stats, rows.Err()
}

func GetBrowserStatsInRange(startDate, endDate string) ([]AppDailyStat, error) {
	rows, err := db.Query(`
		SELECT date, domain_or_title, '', total_duration_secs, open_count
		FROM browsing_daily
		WHERE date >= ? AND date <= ?
		ORDER BY date DESC, total_duration_secs DESC
	`, startDate, endDate)
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
	lastSeen := s.LastSeen
	if lastSeen.IsZero() {
		lastSeen = time.Now()
	}
	_, err := db.Exec(`
		INSERT OR REPLACE INTO active_session (id, app_name, exe_name, window_title, start_time, last_seen, date)
		VALUES (1, ?, ?, ?, ?, ?, ?)
	`, s.AppName, s.ExeName, s.WindowTitle, s.StartTime.Unix(), lastSeen.Unix(), s.Date)
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

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
