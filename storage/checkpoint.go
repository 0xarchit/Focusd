package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"
)

const dataGenerationKey = "data_generation"

var ErrStaleGeneration = errors.New("tracking data generation changed")

// GetDataGeneration returns the epoch that owns newly collected intervals.
func GetDataGeneration() (uint64, error) {
	return dataGeneration(db)
}

func dataGeneration(reader activeSessionReader) (uint64, error) {
	var raw string
	err := reader.QueryRow(`SELECT value FROM config WHERE key = ?`, dataGenerationKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	value, err := strconv.ParseUint(raw, 10, 63)
	if err != nil {
		return 0, fmt.Errorf("invalid tracking data generation: %w", err)
	}
	return value, nil
}

func sessionSliceID(s *Session) string {
	if s.SliceID != "" {
		return s.SliceID
	}
	if s.SessionID == "" {
		return ""
	}
	return s.SessionID + ":" + strconv.FormatInt(s.StartTime.Unix(), 10)
}

// WriteCheckpoint commits completed intervals and the active row together.
func WriteCheckpoint(sessions []*Session, active *ActiveSessionRecord) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, session := range sessions {
		if err := insertSessionWithDaily(tx, session, ""); err != nil {
			return err
		}
	}
	if active == nil {
		_, err = tx.Exec(`DELETE FROM active_session WHERE id = 1`)
	} else {
		err = saveActiveSession(tx, active)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func insertSessionWithDaily(tx *sql.Tx, s *Session, legacySite string) error {
	if s == nil {
		return errors.New("nil completed session")
	}
	generation, err := dataGeneration(tx)
	if err != nil {
		return err
	}
	if generation != s.Generation {
		return ErrStaleGeneration
	}
	if s.DurationSecs < 0 || (!s.EndTime.IsZero() && s.EndTime.Before(s.StartTime)) {
		return errors.New("invalid completed session interval")
	}
	if s.Generation > math.MaxInt64 {
		return errors.New("tracking data generation overflow")
	}
	if !s.EndTime.IsZero() {
		boundarySeconds := s.EndTime.Unix() - s.StartTime.Unix()
		if delta := int64(s.DurationSecs) - boundarySeconds; delta < -1 || delta > 1 {
			return errors.New("completed duration does not match its boundaries")
		}
	}
	site := legacySite
	if s.SiteKnown {
		site = s.SiteName
	}
	var endTime *int64
	if !s.EndTime.IsZero() {
		seconds := s.EndTime.Unix()
		endTime = &seconds
	}
	appID := sessionAppID(s)
	result, err := tx.Exec(`
		INSERT INTO sessions (session_id, slice_id, site_name, site_known, generation,
		                      app_name, exe_name, exe_path, app_id, window_title, start_time, end_time, duration_secs, date)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT DO NOTHING
	`, s.SessionID, sessionSliceID(s), site, s.SiteKnown, s.Generation,
		s.AppName, s.ExeName, s.ExePath, appID, s.WindowTitle, s.StartTime.Unix(), endTime, s.DurationSecs, s.Date)
	if err != nil {
		return err
	}
	inserted, err := result.RowsAffected()
	if err != nil || inserted == 0 {
		return err
	}
	_, err = tx.Exec(`
		INSERT INTO apps_daily (date, app_name, exe_name, exe_path, app_id, total_duration_secs, open_count)
		VALUES (?, ?, ?, ?, ?, ?, 1)
		ON CONFLICT(date, app_id) DO UPDATE SET
		 app_name = excluded.app_name, exe_name = excluded.exe_name, exe_path = excluded.exe_path,
		 total_duration_secs = total_duration_secs + excluded.total_duration_secs, open_count = open_count + 1
	`, s.Date, s.AppName, s.ExeName, s.ExePath, appID, s.DurationSecs)
	if err != nil {
		return err
	}
	if site == "" {
		return nil
	}
	_, err = tx.Exec(`
		INSERT INTO browsing_daily (date, app_id, site_name, total_duration_secs, open_count)
		VALUES (?, ?, ?, ?, 1)
		ON CONFLICT(date, app_id, site_name) DO UPDATE SET
		 total_duration_secs = total_duration_secs + excluded.total_duration_secs, open_count = open_count + 1
	`, s.Date, appID, site, s.DurationSecs)
	return err
}

func saveActiveSession(tx *sql.Tx, s *ActiveSessionRecord) error {
	generation, err := dataGeneration(tx)
	if err != nil {
		return err
	}
	if generation != s.Generation {
		return ErrStaleGeneration
	}
	lastSeen := s.LastSeen
	if lastSeen.IsZero() {
		lastSeen = time.Now()
	}
	if lastSeen.Before(s.StartTime) {
		return errors.New("active heartbeat precedes session start")
	}
	appID := s.AppID
	if appID == "" {
		appID = legacyAppIDFor(s.ExeName)
	}
	_, err = tx.Exec(`
		INSERT OR REPLACE INTO active_session
		 (id, session_id, slice_id, site_name, site_known, generation, app_name, exe_name, exe_path, app_id, window_title, start_time, last_seen, date)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, s.SessionID, s.SliceID, s.SiteName, s.SiteKnown, s.Generation,
		s.AppName, s.ExeName, s.ExePath, appID, s.WindowTitle, s.StartTime.Unix(), lastSeen.Unix(), s.Date)
	return err
}
