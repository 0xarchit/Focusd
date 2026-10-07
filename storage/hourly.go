package storage

import "time"

// GetHourlyUsageToday returns seconds of tracked time per clock hour for one
// day.
//
// A session is distributed across the hours it actually covers. Assigning a
// whole session to its start hour, as this used to, made a single bar report
// more than sixty minutes while the following hours read zero. There is also no
// row cap, so a busy day is counted in full rather than truncated at the first
// 500 sessions.
func GetHourlyUsageToday(date string) ([24]int, error) {
	var buckets [24]int

	rows, err := db.Query(`
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
		for hour, secs := range splitSessionHours(start, end) {
			if hour >= 0 && hour < 24 {
				buckets[hour] += secs
			}
		}
	}

	return buckets, rows.Err()
}

// splitSessionHours returns the seconds of a session that fall in each clock
// hour, so a long session contributes to every hour it covers.
func splitSessionHours(start, end int64) map[int]int {
	out := map[int]int{}
	if end <= start {
		// A zero or negative span contributes nothing rather than wrapping.
		return out
	}

	cursor := start
	for cursor < end {
		ts := time.Unix(cursor, 0)
		next := time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour(), 0, 0, 0, ts.Location()).Add(time.Hour).Unix()
		if next > end {
			next = end
		}
		out[ts.Hour()] += int(next - cursor)
		cursor = next
	}
	return out
}
