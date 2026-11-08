package storage

import "time"

// GetHourlyUsageToday returns seconds of tracked time per clock hour. A session
// is spread across the hours it covers, so no bucket can exceed sixty minutes.
func GetHourlyUsageToday(date string) ([24]int, error) {
	var buckets [24]int
	err := withBusyRetry(func() error {
		var err error
		buckets, err = getHourlyUsageTodayOnce(date)
		return err
	})
	return buckets, err
}

func getHourlyUsageTodayOnce(date string) ([24]int, error) {
	var buckets [24]int
	tx, err := db.Begin()
	if err != nil {
		return buckets, err
	}
	defer tx.Rollback()
	rows, err := tx.Query(`
		SELECT start_time, COALESCE(end_time, start_time) AS end_time
		FROM sessions
		WHERE date = ?
	`, date)
	if err != nil {
		return buckets, err
	}
	defer rows.Close()

	for rows.Next() {
		var start, end int64
		if err := rows.Scan(&start, &end); err != nil {
			return buckets, err
		}
		split, err := splitSessionHoursCalendar(start, end)
		if err != nil {
			return buckets, err
		}
		for hour, secs := range split {
			if hour >= 0 && hour < 24 {
				buckets[hour] += secs
			}
		}
	}

	if err := rows.Err(); err != nil {
		return buckets, err
	}
	if err := rows.Close(); err != nil {
		return buckets, err
	}
	active, err := getActiveSession(tx)
	if err != nil {
		return buckets, err
	}
	if active != nil && active.LastSeen.After(active.StartTime) {
		day, next, err := calendarBoundsForDate(date)
		if err != nil {
			return buckets, err
		}
		start, end := active.StartTime, active.LastSeen
		if start.Before(day) {
			start = day
		}
		if end.After(next) {
			end = next
		}
		split, err := splitSessionHoursCalendar(start.Unix(), end.Unix())
		if err != nil {
			return buckets, err
		}
		for hour, secs := range split {
			buckets[hour] += secs
		}
	}
	return buckets, tx.Commit()
}

// splitSessionHours returns the seconds of a session that fall in each clock
// hour, so a long session contributes to every hour it covers.
func splitSessionHours(start, end int64) map[int]int {
	out, _ := splitSessionHoursCalendar(start, end)
	return out
}

func splitSessionHoursCalendar(start, end int64) (map[int]int, error) {
	out := map[int]int{}
	if end <= start {
		// A zero or negative span contributes nothing rather than wrapping.
		return out, nil
	}

	cursor := start
	for cursor < end {
		ts, err := calendarLocalTime(time.Unix(cursor, 0))
		if err != nil {
			return nil, err
		}
		next := cursor + int64(3600-ts.Minute()*60-ts.Second())
		if next > end {
			next = end
		}
		out[ts.Hour()] += int(next - cursor)
		cursor = next
	}
	return out, nil
}
