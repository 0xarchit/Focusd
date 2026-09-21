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
	ExePath      string
	AppID        string
	WindowTitle  string
	StartTime    time.Time
	EndTime      time.Time
	DurationSecs int
	Date         string
}

// sessionAppID returns the identity key to store, falling back to the exe name
// for callers that predate identity.
func sessionAppID(s *Session) string {
	if s.AppID != "" {
		return s.AppID
	}
	return legacyAppIDFor(s.ExeName)
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

	appID := sessionAppID(s)

	_, err = tx.Exec(`
		INSERT INTO sessions (app_name, exe_name, exe_path, app_id, window_title, start_time, end_time, duration_secs, date)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, s.AppName, s.ExeName, s.ExePath, appID, s.WindowTitle, s.StartTime.Unix(), endTime, s.DurationSecs, s.Date)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO apps_daily (date, app_name, exe_name, exe_path, app_id, total_duration_secs, open_count)
		VALUES (?, ?, ?, ?, ?, ?, 1)
		ON CONFLICT(date, app_id) DO UPDATE SET
			app_name = excluded.app_name,
			exe_name = excluded.exe_name,
			exe_path = excluded.exe_path,
			total_duration_secs = total_duration_secs + excluded.total_duration_secs,
			open_count = open_count + 1
	`, s.Date, s.AppName, s.ExeName, s.ExePath, appID, s.DurationSecs)
	if err != nil {
		return err
	}

	if cleanBrowserTitle != "" {
		_, err = tx.Exec(`
			INSERT INTO browsing_daily (date, app_id, site_name, total_duration_secs, open_count)
			VALUES (?, ?, ?, ?, 1)
			ON CONFLICT(date, app_id, site_name) DO UPDATE SET
				total_duration_secs = total_duration_secs + excluded.total_duration_secs,
				open_count = open_count + 1
		`, s.Date, appID, cleanBrowserTitle, s.DurationSecs)
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
	AppID             string
	TotalDurationSecs int
	OpenCount         int
}

// ActiveSessionSnapshot is a read-only view of the persisted active session.
// Read paths use it instead of rotating, because rotating inserted a slice and
// every reader could trigger it, which double-counted while a session spanned
// midnight.
type ActiveSessionSnapshot struct {
	AppName     string
	ExeName     string
	ExePath     string
	AppID       string
	WindowTitle string
	StartTime   time.Time
	LastSeen    time.Time
	Date        string
}

// GetActiveSession returns the persisted active session, or nil when there is
// none. It never writes.
func GetActiveSession() (*ActiveSessionSnapshot, error) {
	var s ActiveSessionSnapshot
	var startTime, lastSeen int64

	err := db.QueryRow(`
		SELECT app_name, exe_name, COALESCE(exe_path, ''), COALESCE(app_id, ''), window_title, start_time, last_seen, date
		FROM active_session WHERE id = 1
	`).Scan(&s.AppName, &s.ExeName, &s.ExePath, &s.AppID, &s.WindowTitle, &startTime, &lastSeen, &s.Date)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	s.StartTime = time.Unix(startTime, 0)
	s.LastSeen = time.Unix(lastSeen, 0)
	return &s, nil
}

// startOfLocalDay returns local midnight for the given time.
func startOfLocalDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// liveElapsedToday returns how many seconds of the active session count toward
// today. A session that began before local midnight is clipped at midnight so
// yesterday's time is never counted toward today, regardless of whether the
// tracker has rotated it yet.
func liveElapsedToday(start time.Time, now time.Time) int {
	midnight := startOfLocalDay(now)
	if start.Before(midnight) {
		end := midnight
		if now.Before(midnight) {
			return 0
		}
		elapsed := int(end.Sub(start).Seconds())
		if elapsed > 0 {
			return elapsed
		}
		return 0
	}
	elapsed := int(now.Sub(start).Seconds())
	if elapsed > 0 {
		return elapsed
	}
	return 0
}

// RebaseActiveSessionStart moves the persisted start time forward. Only the
// tracker calls this, after it has written the prior slice, which makes
// rotation single-writer.
func RebaseActiveSessionStart(start time.Time, date string) error {
	_, err := db.Exec(`
		UPDATE active_session
		SET start_time = ?, date = ?
		WHERE id = 1
	`, start.Unix(), date)
	return err
}

func GetAppUsageTodaySeconds(appID string) int {
	today := Today()
	var secs int
	err := db.QueryRow(`
		SELECT COALESCE(total_duration_secs, 0) FROM apps_daily
		WHERE date = ? AND app_id = ?
	`, today, appID).Scan(&secs)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Printf("WARN: GetAppUsageTodaySeconds query failed for %s: %v", appID, err)
	}

	active, aerr := GetActiveSession()
	if aerr != nil {
		log.Printf("WARN: GetActiveSession failed: %v", aerr)
		return secs
	}
	if active != nil && active.AppID == appID {
		secs += liveElapsedToday(active.StartTime, time.Now())
	}
	return secs
}

func GetAppUsageTodayMinutesMap() (map[string]int, error) {
	today := Today()
	rows, err := db.Query(`
		SELECT app_id, total_duration_secs FROM apps_daily
		WHERE date = ?
	`, today)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]int)
	for rows.Next() {
		var appID string
		var secs int
		if err := rows.Scan(&appID, &secs); err != nil {
			return nil, err
		}
		res[appID] = secs / 60
	}

	active, aerr := GetActiveSession()
	if aerr != nil {
		log.Printf("WARN: GetActiveSession failed: %v", aerr)
		return res, rows.Err()
	}
	if active != nil {
		if elapsed := liveElapsedToday(active.StartTime, time.Now()); elapsed > 0 {
			res[active.AppID] += elapsed / 60
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
		SELECT app_name, exe_name, COALESCE(exe_path, ''), COALESCE(app_id, ''), window_title, start_time, end_time, duration_secs, date
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
		if err := rows.Scan(&s.AppName, &s.ExeName, &s.ExePath, &s.AppID, &windowTitle, &startTime, &endTime, &s.DurationSecs, &s.Date); err != nil {
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
	query := `SELECT date, app_name, exe_name, app_id, total_duration_secs, open_count FROM apps_daily`
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
		if err := rows.Scan(&s.Date, &s.AppName, &s.ExeName, &s.AppID, &s.TotalDurationSecs, &s.OpenCount); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}

	// Dynamic active session injection if date range includes today
	today := Today()
	inRange := (startDate == "" && endDate == "") || (today >= startDate && today <= endDate)
	if inRange {
		active, aerr := GetActiveSession()
		if aerr != nil {
			log.Printf("WARN: GetActiveSession failed: %v", aerr)
		} else if active != nil {
			elapsed := liveElapsedToday(active.StartTime, time.Now())
			if elapsed > 0 {
				found := false
				for i, s := range stats {
					if s.Date == today && s.AppID == active.AppID {
						stats[i].TotalDurationSecs += elapsed
						stats[i].OpenCount += 1
						found = true
						break
					}
				}
				if !found {
					stats = append(stats, AppDailyStat{
						Date:              today,
						AppName:           active.AppName,
						AppID:             active.AppID,
						ExeName:           active.ExeName,
						TotalDurationSecs: elapsed,
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
		SELECT date, site_name, app_id, total_duration_secs, open_count
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
		if err := rows.Scan(&s.Date, &s.AppName, &s.AppID, &s.TotalDurationSecs, &s.OpenCount); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

type ActiveSessionRecord struct {
	AppName     string
	ExeName     string
	ExePath     string
	AppID       string
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
	appID := s.AppID
	if appID == "" {
		appID = legacyAppIDFor(s.ExeName)
	}
	_, err := db.Exec(`
		INSERT OR REPLACE INTO active_session (id, app_name, exe_name, exe_path, app_id, window_title, start_time, last_seen, date)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?)
	`, s.AppName, s.ExeName, s.ExePath, appID, s.WindowTitle, s.StartTime.Unix(), lastSeen.Unix(), s.Date)
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
		SELECT app_name, exe_name, COALESCE(exe_path, ''), COALESCE(app_id, ''), window_title, start_time, last_seen, date
		FROM active_session WHERE id = 1
	`).Scan(&s.AppName, &s.ExeName, &s.ExePath, &s.AppID, &s.WindowTitle, &startTime, &lastSeen, &s.Date)

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
		ExePath:      s.ExePath,
		AppID:        s.AppID,
		WindowTitle:  s.WindowTitle,
		StartTime:    time.Unix(startTime, 0),
		EndTime:      time.Unix(lastSeen, 0),
		DurationSecs: duration,
		Date:         s.Date,
	}, nil
}

// legacyAppIDFor builds the key used for rows that predate identity. It lives
// here rather than importing the system package so storage stays independent of
// windowing concerns.
func legacyAppIDFor(exeName string) string {
	name := strings.ToLower(strings.TrimSpace(exeName))
	name = strings.TrimSuffix(name, ".exe")
	if name == "" {
		name = "unknown"
	}
	return name + "|legacy"
}
