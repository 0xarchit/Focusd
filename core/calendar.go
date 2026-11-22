package core

import (
	"fmt"
	"focusd/storage"
	"focusd/system"
	"time"
)

type sessionCalendar struct {
	localTime func(time.Time) (time.Time, error)
	dayBounds func(time.Time) (time.Time, time.Time, error)
}

func captureWindowsCalendar() (*sessionCalendar, error) {
	calendar, err := system.CaptureLocalCalendar()
	if err != nil {
		return nil, err
	}
	return &sessionCalendar{localTime: calendar.LocalTimeAt, dayBounds: calendar.DayBounds}, nil
}

func locationCalendar(location *time.Location) *sessionCalendar {
	return &sessionCalendar{
		localTime: func(at time.Time) (time.Time, error) { return at.In(location), nil },
		dayBounds: func(at time.Time) (time.Time, time.Time, error) {
			local := at.In(location)
			start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
			return start, start.AddDate(0, 0, 1), nil
		},
	}
}

func (c *sessionCalendar) date(at time.Time) (string, error) {
	local, err := c.localTime(at)
	if err != nil {
		return "", err
	}
	return local.Format("2006-01-02"), nil
}

func splitSessionWithCalendar(s *activeSession, end time.Time, calendar *sessionCalendar) ([]*storage.Session, error) {
	if s == nil || !end.After(s.StartTime) {
		return nil, nil
	}
	var out []*storage.Session
	cursor := s.StartTime
	date := s.Date
	if date == "" {
		var err error
		date, err = calendar.date(cursor)
		if err != nil {
			return nil, err
		}
	}
	for cursor.Before(end) {
		_, boundary, err := calendar.dayBounds(cursor)
		if err != nil {
			return nil, err
		}
		if !boundary.After(cursor) {
			return nil, fmt.Errorf("local calendar boundary did not advance")
		}
		until := end
		if boundary.Before(end) {
			until = boundary
		}
		if duration := int(until.Sub(cursor).Seconds()); duration >= 1 {
			out = append(out, &storage.Session{SessionID: s.SessionID, SliceID: sliceID(s.SessionID, cursor), SiteName: s.SiteName, SiteKnown: s.SiteKnown, Generation: s.Generation, AppName: s.AppName, ExeName: s.ExeName, ExePath: s.ExePath, AppID: s.AppID, WindowTitle: s.WindowTitle, StartTime: cursor, EndTime: until, DurationSecs: duration, Date: date})
		}
		cursor = until
		if cursor.Before(end) {
			date, err = calendar.date(cursor)
			if err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}
