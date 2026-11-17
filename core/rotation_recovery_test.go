package core

import (
	"fmt"
	"testing"
	"time"
)

func TestRotateActiveSessionRetainsEveryCompletedDay(t *testing.T) {
	for _, dayCount := range []int{1, 2, 3} {
		t.Run(fmt.Sprintf("%d completed days", dayCount), func(t *testing.T) {
			setupTestDB(t)
			originalLocation := time.Local
			time.Local = time.FixedZone("RotationTest", (12-time.Now().UTC().Hour())*3600)
			t.Cleanup(func() { time.Local = originalLocation })
			now := time.Now()
			midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
			start := midnight.AddDate(0, 0, -dayCount).Add(22 * time.Hour)
			tracker := &Tracker{currentSession: &activeSession{
				AppName: "Editor", ExeName: "editor.exe", AppID: "editor|abcd1234",
				ExePath: `C:\Editor\editor.exe`, WindowTitle: "Document",
				StartTime: start, Date: dateStr(start),
			}}

			tracker.rotateActiveSessionAtMidnight()

			want := map[string]int{dateStr(start): 7200}
			for day := 1; day < dayCount; day++ {
				want[dateStr(start.AddDate(0, 0, day))] = 24 * 60 * 60
			}
			got := make(map[string]int)
			for _, session := range tracker.pendingSessions {
				got[session.Date] += session.DurationSecs
				if session.AppID != "editor|abcd1234" || session.ExePath != `C:\Editor\editor.exe` {
					t.Errorf("rotated slice changed app identity: %+v", session)
				}
			}
			if len(got) != len(want) {
				t.Errorf("rotation retained %d completed date buckets; want %d: got %v", len(got), len(want), got)
			}
			for date, seconds := range want {
				if got[date] != seconds {
					t.Errorf("completed %s = %ds; want %ds", date, got[date], seconds)
				}
			}
			if tracker.currentSession == nil {
				t.Fatal("rotation discarded today's live session")
			}
			if !tracker.currentSession.StartTime.Equal(midnight) || tracker.currentSession.Date != dateStr(midnight) {
				t.Errorf("live session starts at %s/%s; want today's midnight %s", tracker.currentSession.StartTime, tracker.currentSession.Date, midnight)
			}
			pendingCount := len(tracker.pendingSessions)
			tracker.rotateActiveSessionAtMidnight()
			if len(tracker.pendingSessions) != pendingCount {
				t.Errorf("repeated rotation queued a completed interval twice: %d then %d slices", pendingCount, len(tracker.pendingSessions))
			}
		})
	}
}
