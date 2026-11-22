package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type RetentionPreview struct {
	Date   string `json:"date"`
	Cutoff string `json:"cutoff"`
	Rows   int    `json:"rows"`
	Oldest string `json:"oldest,omitempty"`
	Newest string `json:"newest,omitempty"`
	Token  string `json:"token"`
	clock  string
}

var ErrRetentionPreviewChanged = errors.New("retention preview changed; review cleanup again")

// PreviewRetention captures the exact proposed deletion and clock policy.
func PreviewRetention(date string) (RetentionPreview, error) {
	if date != Today() {
		return RetentionPreview{}, ErrRetentionPreviewChanged
	}
	day, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return RetentionPreview{}, err
	}
	days, err := resolveRetentionDays()
	if err != nil {
		return RetentionPreview{}, err
	}
	tx, err := db.Begin()
	if err != nil {
		return RetentionPreview{}, err
	}
	defer tx.Rollback()
	state, policy, err := readRetentionClock(tx)
	if err != nil {
		return RetentionPreview{}, err
	}
	if state.Incident != nil {
		return RetentionPreview{}, errors.New("resolve the clock warning before reviewing cleanup")
	}
	preview, err := retentionRows(tx, date, day.AddDate(0, 0, -(days-1)).Format("2006-01-02"))
	if err != nil {
		return preview, err
	}
	preview.Token, err = backupID()
	if err != nil {
		return preview, err
	}
	preview.clock = policy
	return preview, tx.Commit()
}

// ConfirmRetention requires the preview cached by the authenticated controller.
func ConfirmRetention(preview RetentionPreview) error {
	if preview.Token == "" || preview.Date != Today() {
		return ErrRetentionPreviewChanged
	}
	days, err := resolveRetentionDays()
	if err != nil {
		return err
	}
	day, err := time.ParseInLocation("2006-01-02", preview.Date, time.Local)
	if err != nil || day.AddDate(0, 0, -(days-1)).Format("2006-01-02") != preview.Cutoff {
		return ErrRetentionPreviewChanged
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	state, policy, err := readRetentionClock(tx)
	if err != nil {
		return err
	}
	if state.Incident != nil || policy != preview.clock {
		return ErrRetentionPreviewChanged
	}
	current, err := retentionRows(tx, preview.Date, preview.Cutoff)
	if err != nil {
		return err
	}
	if current.Rows != preview.Rows || current.Oldest != preview.Oldest || current.Newest != preview.Newest {
		return ErrRetentionPreviewChanged
	}
	if err := pruneRetention(tx, preview.Cutoff); err != nil {
		return err
	}
	state.RetentionHold = false
	encoded, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if err := setConfigTx(tx, clockStateKey, string(encoded)); err != nil {
		return err
	}
	return tx.Commit()
}

func readRetentionClock(reader activeSessionReader) (ClockState, string, error) {
	var raw string
	err := reader.QueryRow(`SELECT value FROM config WHERE key = ?`, clockStateKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return ClockState{Version: clockStateVersion}, "", nil
	}
	if err != nil {
		return ClockState{}, "", err
	}
	var state ClockState
	if len(raw) > 32*1024 || json.Unmarshal([]byte(raw), &state) != nil || state.Version != clockStateVersion {
		return state, "", ErrMalformedClockState
	}
	if state.Accepted != nil && state.Accepted.WallUTC.IsZero() || state.Incident != nil && (state.Incident.ID == "" || state.Incident.Revision < 1 || state.Incident.Current.WallUTC.IsZero()) {
		return state, "", ErrMalformedClockState
	}
	// A normal heartbeat must not invalidate a cleanup preview.
	policy := struct {
		Anchor *ClockCheckpoint
		Epoch  uint64
		Hold   bool
	}{state.Anchor, state.Epoch, state.RetentionHold}
	encoded, err := json.Marshal(policy)
	return state, string(encoded), err
}

func retentionRows(reader activeSessionReader, date, cutoff string) (RetentionPreview, error) {
	preview := RetentionPreview{Date: date, Cutoff: cutoff}
	err := reader.QueryRow(`SELECT COUNT(*), COALESCE(MIN(date),''), COALESCE(MAX(date),'') FROM (
		SELECT date FROM sessions WHERE date < ? UNION ALL
		SELECT date FROM apps_daily WHERE date < ? UNION ALL
		SELECT date FROM browsing_daily WHERE date < ?
	)`, cutoff, cutoff, cutoff).Scan(&preview.Rows, &preview.Oldest, &preview.Newest)
	return preview, err
}

func pruneRetention(tx *sql.Tx, cutoff string) error {
	for _, table := range []string{"sessions", "apps_daily", "browsing_daily"} {
		if _, err := tx.Exec(`DELETE FROM `+table+` WHERE date < ?`, cutoff); err != nil {
			return err
		}
	}
	return nil
}

// LatestHistoryDate includes retained completed rows and the bounded checkpoint.
func LatestHistoryDate() (string, error) {
	rows, err := db.Query(`SELECT DISTINCT date FROM (
		SELECT date FROM sessions UNION ALL SELECT date FROM apps_daily UNION ALL
		SELECT date FROM browsing_daily UNION ALL SELECT date FROM active_session
	) WHERE length(date) = 10 ORDER BY date DESC`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var date string
		if err := rows.Scan(&date); err != nil {
			return "", err
		}
		if parsed, err := time.Parse("2006-01-02", date); err == nil && parsed.Format("2006-01-02") == date {
			return date, nil
		}
	}
	return "", rows.Err()
}
