package storage

import (
	"fmt"
	"focusd/system"
	"time"
)

var (
	calendarNow       = time.Now
	calendarLocalTime = system.LocalTimeAt
	calendarDayBounds = system.LocalDayBounds
)

func calendarDate(at time.Time) (string, error) {
	local, err := calendarLocalTime(at)
	if err != nil {
		return "", err
	}
	return local.Format("2006-01-02"), nil
}

// calendarBoundsForDate resolves an explicit date using the current Windows rules.
func calendarBoundsForDate(date string) (time.Time, time.Time, error) {
	day, err := time.Parse("2006-01-02", date)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	candidate := day.Add(12 * time.Hour)
	for range 3 {
		local, err := calendarLocalTime(candidate)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		localDay := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
		if localDay.Equal(day) {
			return calendarDayBounds(candidate)
		}
		candidate = candidate.Add(day.Sub(localDay))
	}
	return time.Time{}, time.Time{}, fmt.Errorf("cannot resolve local calendar date %s", date)
}

func retentionCutoff(days int) (string, error) {
	date, err := calendarDate(calendarNow())
	if err != nil {
		return "", err
	}
	day, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "", err
	}
	return day.AddDate(0, 0, -(days - 1)).Format("2006-01-02"), nil
}
