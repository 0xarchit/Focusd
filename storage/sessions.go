package storage

import (
	"database/sql"
	"errors"
	"log"
	"sort"
	"strings"
	"time"
)

type Session struct {
	SessionID    string
	SliceID      string
	SiteName     string
	SiteKnown    bool
	Generation   uint64
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

	if err := insertSessionWithDaily(tx, s, cleanBrowserTitle); err != nil {
		return err
	}

	return tx.Commit()
}

// SessionExists reports whether a session with the same app identity and start
// time is already in history. Orphan recovery uses it to avoid re-inserting an
// interval that was flushed before the active row could be cleared.
func SessionExists(s *Session) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM sessions WHERE app_id = ? AND start_time = ? AND session_id = ''`
	args := []any{sessionAppID(s), s.StartTime.Unix()}
	if s.SessionID != "" {
		query = `SELECT COUNT(*) FROM sessions WHERE slice_id = ?`
		args = []any{sessionSliceID(s)}
	}
	err := db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
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
// Rotation is the tracker's job, so read paths never insert.
type ActiveSessionSnapshot = ActiveSessionRecord

// GetActiveSession returns the persisted active session, or nil when there is
// none. It never writes.
func GetActiveSession() (*ActiveSessionSnapshot, error) {
	return getActiveSession(db)
}

type activeSessionReader interface {
	QueryRow(query string, args ...any) *sql.Row
}

func getActiveSession(reader activeSessionReader) (*ActiveSessionSnapshot, error) {
	var s ActiveSessionSnapshot
	var startTime, lastSeen int64

	err := reader.QueryRow(`
		SELECT app_name, exe_name, COALESCE(exe_path, ''), COALESCE(app_id, ''), window_title, start_time, last_seen, date,
		       session_id, slice_id, site_name, site_known, generation
		FROM active_session WHERE id = 1
	`).Scan(&s.AppName, &s.ExeName, &s.ExePath, &s.AppID, &s.WindowTitle, &startTime, &lastSeen, &s.Date,
		&s.SessionID, &s.SliceID, &s.SiteName, &s.SiteKnown, &s.Generation)
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

// liveElapsedToday clips a session that began before local midnight at
// midnight, so yesterday's time is never counted toward today even before the
// tracker rotates it.
func liveElapsedToday(start time.Time, now time.Time) int {
	midnight := startOfLocalDay(now)
	begin := start
	if begin.Before(midnight) {
		begin = midnight
	}
	elapsed := int(now.Sub(begin).Seconds())
	if elapsed > 0 {
		return elapsed
	}
	return 0
}

// activeElapsedOn bounds persisted usage by the last observed heartbeat.
func activeElapsedOn(active *ActiveSessionSnapshot, date string) int {
	if active == nil || active.LastSeen.Before(active.StartTime) {
		return 0
	}
	day, next, err := calendarBoundsForDate(date)
	if err != nil {
		return 0
	}
	start, end := active.StartTime, active.LastSeen
	if start.Before(day) {
		start = day
	}
	if end.After(next) {
		end = next
	}
	if !end.After(start) {
		return 0
	}
	return int(end.Sub(start).Seconds())
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
	stats, err := GetAppStatsInRange(today, today)
	if err != nil {
		log.Printf("WARN: GetAppUsageTodaySeconds failed: %v", err)
		return 0
	}
	for _, stat := range stats {
		if stat.AppID == appID {
			secs += stat.TotalDurationSecs
		}
	}
	return secs
}

// GetAppUsageTodaySecondsForConfig returns today's tracked seconds for a
// config limit key with the same wildcard semantics enforcement uses: a bare
// exe name covers every install of that executable and a full app key matches
// one install. The live active session is included when it matches.
func GetAppUsageTodaySecondsForConfig(configKey string) int {
	base, hash := splitConfigKeyForUsage(configKey)
	if hash != "" {
		return GetAppUsageTodaySeconds(base + "|" + hash)
	}

	var secs int
	stats, err := GetAppStatsInRange(Today(), Today())
	if err != nil {
		log.Printf("WARN: GetAppUsageTodaySecondsForConfig failed: %v", err)
		return 0
	}
	for _, stat := range stats {
		if normalizeExeName(appIDBaseForUsage(stat.AppID)) == base {
			secs += stat.TotalDurationSecs
		}
	}
	return secs
}

// splitConfigKeyForUsage splits a config key into its normalized exe base and
// its install hash. A bare name has no hash and matches every install.
func splitConfigKeyForUsage(configKey string) (base, hash string) {
	trimmed := strings.TrimSpace(configKey)
	if i := strings.Index(trimmed, "|"); i >= 0 {
		return normalizeExeName(trimmed[:i]), trimmed[i+1:]
	}
	return normalizeExeName(trimmed), ""
}

// appIDBaseForUsage extracts the exe part of a live app id.
func appIDBaseForUsage(appID string) string {
	if i := strings.Index(appID, "|"); i >= 0 {
		return appID[:i]
	}
	return appID
}

func GetAppUsageTodayMinutesMap() (map[string]int, error) {
	stats, err := GetAppStatsInRange(Today(), Today())
	if err != nil {
		return nil, err
	}
	secs := make(map[string]int)
	for _, stat := range stats {
		secs[stat.AppID] += stat.TotalDurationSecs
	}

	res := make(map[string]int, len(secs))
	for appID, s := range secs {
		res[appID] = s / 60
	}
	return res, nil
}

func GetSessionsPaginated(limit, offset int, startDate, endDate string) ([]Session, error) {
	if limit < 0 {
		limit = 0
	}
	if offset < 0 {
		offset = 0
	}
	dataQuery := `
		SELECT app_name, exe_name, COALESCE(exe_path, ''), COALESCE(app_id, ''), window_title, start_time, end_time, duration_secs, date,
		       session_id, slice_id, site_name, site_known, generation
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
		if err := rows.Scan(&s.AppName, &s.ExeName, &s.ExePath, &s.AppID, &windowTitle, &startTime, &endTime, &s.DurationSecs, &s.Date,
			&s.SessionID, &s.SliceID, &s.SiteName, &s.SiteKnown, &s.Generation); err != nil {
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
	query += ` WHERE (? = '' OR date >= ?) AND (? = '' OR date <= ?)`
	args = append(args, startDate, startDate, endDate, endDate)
	query += ` ORDER BY date DESC, total_duration_secs DESC`

	var stats []AppDailyStat
	err := withBusyRetry(func() error {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		rows, err := tx.Query(query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()

		stats = stats[:0]
		for rows.Next() {
			var s AppDailyStat
			if err := rows.Scan(&s.Date, &s.AppName, &s.ExeName, &s.AppID, &s.TotalDurationSecs, &s.OpenCount); err != nil {
				return err
			}
			stats = append(stats, s)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		rows.Close()
		active, err := getActiveSession(tx)
		if err != nil {
			return err
		}
		for _, stat := range activeAppStats(active, startDate, endDate) {
			stats = appendActiveStat(stats, stat, false)
		}
		return tx.Commit()
	})
	if err != nil {
		return nil, err
	}

	sortDailyStats(stats)
	return stats, nil
}

func GetBrowserStatsInRange(startDate, endDate string) ([]AppDailyStat, error) {
	var stats []AppDailyStat
	err := withBusyRetry(func() error {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		rows, err := tx.Query(`
			SELECT b.date, b.site_name, COALESCE(b.app_id, ''), COALESCE(a.exe_name, ''),
			       SUM(b.total_duration_secs), SUM(b.open_count)
			FROM browsing_daily AS b
			LEFT JOIN apps_daily AS a ON a.date = b.date AND a.app_id = b.app_id
			WHERE (? = '' OR b.date >= ?) AND (? = '' OR b.date <= ?)
			GROUP BY b.date, b.app_id, b.site_name
			ORDER BY b.date DESC, SUM(b.total_duration_secs) DESC
		`, startDate, startDate, endDate, endDate)
		if err != nil {
			return err
		}
		defer rows.Close()

		stats = stats[:0]
		for rows.Next() {
			var s AppDailyStat
			if err := rows.Scan(&s.Date, &s.AppName, &s.AppID, &s.ExeName, &s.TotalDurationSecs, &s.OpenCount); err != nil {
				return err
			}
			if s.ExeName == "" && s.AppID != "" {
				s.ExeName = normalizeExeName(appIDBaseForUsage(s.AppID)) + ".exe"
			}
			stats = append(stats, s)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		rows.Close()
		active, err := getActiveSession(tx)
		if err != nil {
			return err
		}
		if active != nil && active.SiteKnown && active.SiteName != "" {
			for _, stat := range activeAppStats(active, startDate, endDate) {
				stat.AppName = active.SiteName
				stats = appendActiveStat(stats, stat, true)
			}
		}
		return tx.Commit()
	})
	if err != nil {
		return nil, err
	}
	sortDailyStats(stats)
	return stats, nil
}

func activeAppStats(active *ActiveSessionSnapshot, startDate, endDate string) []AppDailyStat {
	if active == nil || !active.LastSeen.After(active.StartTime) {
		return nil
	}
	for _, date := range []string{startDate, endDate} {
		if date != "" {
			if _, err := time.Parse("2006-01-02", date); err != nil {
				return nil
			}
		}
	}
	start, end := active.StartTime, active.LastSeen
	var stats []AppDailyStat
	for start.Before(end) {
		date, err := calendarDate(start)
		if err != nil {
			return nil
		}
		_, next, err := calendarDayBounds(start)
		if err != nil || !next.After(start) {
			return nil
		}
		if next.After(end) {
			next = end
		}
		if duration := int(next.Sub(start).Seconds()); duration > 0 && (startDate == "" || date >= startDate) && (endDate == "" || date <= endDate) {
			stats = append(stats, AppDailyStat{
				Date: date, AppName: active.AppName, ExeName: active.ExeName,
				AppID: active.AppID, TotalDurationSecs: duration, OpenCount: 1,
			})
		}
		start = next
	}
	return stats
}

func appendActiveStat(stats []AppDailyStat, active AppDailyStat, site bool) []AppDailyStat {
	for index, stat := range stats {
		matches := stat.Date == active.Date && stat.AppID == active.AppID
		if site {
			matches = matches && stat.AppName == active.AppName
		}
		if matches {
			stats[index].TotalDurationSecs += active.TotalDurationSecs
			stats[index].OpenCount += active.OpenCount
			return stats
		}
	}
	return append(stats, active)
}

func sortDailyStats(stats []AppDailyStat) {
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Date != stats[j].Date {
			return stats[i].Date > stats[j].Date
		}
		if stats[i].TotalDurationSecs != stats[j].TotalDurationSecs {
			return stats[i].TotalDurationSecs > stats[j].TotalDurationSecs
		}
		if stats[i].AppName != stats[j].AppName {
			return stats[i].AppName < stats[j].AppName
		}
		return stats[i].AppID < stats[j].AppID
	})
}

type ActiveSessionRecord struct {
	SessionID   string
	SliceID     string
	SiteName    string
	SiteKnown   bool
	Generation  uint64
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
	return WriteCheckpoint(nil, s)
}

func ClearActiveSession() error {
	_, err := db.Exec("DELETE FROM active_session WHERE id = 1")
	return err
}

func RecoverActiveSession() (*Session, error) {
	s, err := GetActiveSession()
	if err != nil || s == nil {
		return nil, err
	}
	duration := int(s.LastSeen.Unix() - s.StartTime.Unix())
	if duration < 1 {
		return nil, nil
	}

	return &Session{
		SessionID:    s.SessionID,
		SliceID:      s.SliceID,
		SiteName:     s.SiteName,
		SiteKnown:    s.SiteKnown,
		Generation:   s.Generation,
		AppName:      s.AppName,
		ExeName:      s.ExeName,
		ExePath:      s.ExePath,
		AppID:        s.AppID,
		WindowTitle:  s.WindowTitle,
		StartTime:    s.StartTime,
		EndTime:      s.LastSeen,
		DurationSecs: duration,
		Date:         s.Date,
	}, nil
}

// normalizeExeName duplicates system.normalizeExe so storage does not import
// the windowing package.
func normalizeExeName(exeName string) string {
	name := strings.ToLower(strings.TrimSpace(exeName))
	name = strings.TrimSuffix(name, ".exe")
	if name == "" {
		name = "unknown"
	}
	return name
}

// legacyAppIDFor is the identity key for rows that predate app identity.
func legacyAppIDFor(exeName string) string {
	return normalizeExeName(exeName) + "|legacy"
}
