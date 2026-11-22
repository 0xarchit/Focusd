package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func checkpointSession(id string, start time.Time) *Session {
	return &Session{
		SessionID: id, AppName: "Editor", ExeName: "editor.exe", AppID: "editor|abcd1234",
		StartTime: start, EndTime: start.Add(time.Minute), DurationSecs: 60,
		Date: start.Format("2006-01-02"), SiteKnown: true, SiteName: "GitHub",
	}
}

func TestWriteCheckpointCountsRetriedSliceOnce(t *testing.T) {
	setupStorageEnv(t)
	start := time.Now().Add(-time.Hour).Truncate(time.Second)
	session := checkpointSession("session-one", start)
	for range 2 {
		if err := WriteCheckpoint([]*Session{session}, nil); err != nil {
			t.Fatalf("WriteCheckpoint: %v", err)
		}
	}
	var count, duration, opens int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("completed rows = %d; want one retried slice", count)
	}
	if err := db.QueryRow(`SELECT total_duration_secs, open_count FROM apps_daily`).Scan(&duration, &opens); err != nil {
		t.Fatal(err)
	}
	if duration != 60 || opens != 1 {
		t.Errorf("daily app = %ds/%d opens; want 60s/1", duration, opens)
	}
	if err := db.QueryRow(`SELECT total_duration_secs, open_count FROM browsing_daily`).Scan(&duration, &opens); err != nil {
		t.Fatal(err)
	}
	if duration != 60 || opens != 1 {
		t.Errorf("daily site = %ds/%d opens; want 60s/1", duration, opens)
	}
}

func TestSessionIDsDistinguishRepeatedWallTimestamp(t *testing.T) {
	setupStorageEnv(t)
	start := time.Now().Add(-time.Hour).Truncate(time.Second)
	first := checkpointSession("before-clock-reset", start)
	second := checkpointSession("after-clock-reset", start)
	if err := WriteCheckpoint([]*Session{first}, nil); err != nil {
		t.Fatal(err)
	}
	if exists, err := SessionExists(second); err != nil || exists {
		t.Fatalf("new session at previous timestamp exists = %v, err = %v; want false", exists, err)
	}
	if err := WriteCheckpoint([]*Session{second}, nil); err != nil {
		t.Fatal(err)
	}
	var count, seconds int
	if err := db.QueryRow(`SELECT COUNT(*), SUM(duration_secs) FROM sessions`).Scan(&count, &seconds); err != nil {
		t.Fatal(err)
	}
	if count != 2 || seconds != 120 {
		t.Errorf("distinct sessions = %d/%ds; want 2/120s", count, seconds)
	}
}

func TestWriteCheckpointRollsBackHistoryOnInvalidActiveRow(t *testing.T) {
	setupStorageEnv(t)
	start := time.Now().Add(-time.Hour).Truncate(time.Second)
	active := &ActiveSessionRecord{
		SessionID: "live", AppName: "Editor", ExeName: "editor.exe", AppID: "editor|abcd1234",
		StartTime: start, LastSeen: start.Add(time.Minute), Date: start.Format("2006-01-02"),
	}
	if err := SaveActiveSession(active); err != nil {
		t.Fatal(err)
	}
	invalid := *active
	invalid.LastSeen = start.Add(-time.Second)
	if err := WriteCheckpoint([]*Session{checkpointSession("closed", start)}, &invalid); err == nil {
		t.Fatal("invalid active interval must reject the whole checkpoint")
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("rollback left %d completed rows", count)
	}
	got, err := GetActiveSession()
	if err != nil || got == nil || !got.LastSeen.Equal(active.LastSeen) {
		t.Errorf("rollback changed old heartbeat: %+v, err %v", got, err)
	}
}

func TestWriteCheckpointRejectsPreviousDataGeneration(t *testing.T) {
	setupStorageEnv(t)
	if err := SetConfig(dataGenerationKey, "1"); err != nil {
		t.Fatal(err)
	}
	session := checkpointSession("old-generation", time.Now().Add(-time.Hour))
	if err := WriteCheckpoint([]*Session{session}, nil); !errors.Is(err, ErrStaleGeneration) {
		t.Fatalf("old checkpoint error = %v; want stale generation", err)
	}
	active := &ActiveSessionRecord{AppName: "Editor", ExeName: "editor.exe", StartTime: time.Now(), LastSeen: time.Now()}
	if err := SaveActiveSession(active); !errors.Is(err, ErrStaleGeneration) {
		t.Fatalf("old active error = %v; want stale generation", err)
	}
	session.Generation = 1
	if err := WriteCheckpoint([]*Session{session}, nil); err != nil {
		t.Fatalf("current checkpoint: %v", err)
	}
}

func TestExplicitNoSiteDoesNotUseLegacyBrowserArgument(t *testing.T) {
	setupStorageEnv(t)
	session := checkpointSession("strict-private", time.Now().Add(-time.Hour))
	session.SiteName = ""
	if err := InsertSessionWithDaily(session, "Private Browsing"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM browsing_daily`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("explicit no-site persisted %d browsing rows", count)
	}
}

func TestLiveBrowserMetadataTransitionsWithoutDoubleCounting(t *testing.T) {
	now := setupLiveAccountingDB(t)
	start := now.Add(-10 * time.Minute)
	active := &ActiveSessionRecord{
		SessionID: "browser-live", AppName: "Browser", ExeName: "browser.exe", AppID: "browser|abcd1234",
		WindowTitle: "[redacted]", SiteKnown: true, SiteName: "GitHub",
		StartTime: start, LastSeen: start.Add(time.Minute), Date: Today(),
	}
	if err := SaveActiveSession(active); err != nil {
		t.Fatal(err)
	}
	before, err := GetBrowserStatsInRange(Today(), Today())
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 1 || before[0].AppName != "GitHub" || before[0].TotalDurationSecs != 60 {
		t.Fatalf("live browser = %+v; want GitHub/60s", before)
	}
	session := checkpointSession(active.SessionID, start)
	session.AppID, session.ExeName = active.AppID, active.ExeName
	if err := WriteCheckpoint([]*Session{session}, nil); err != nil {
		t.Fatal(err)
	}
	after, err := GetBrowserStatsInRange(Today(), Today())
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 || after[0] != before[0] {
		t.Errorf("browser totals changed on handoff: before %+v, after %+v", before, after)
	}
}

func TestCheckpointSchemaRepairsOldColumnsWithoutMergingHistory(t *testing.T) {
	dir := setupStorageEnv(t)
	for _, table := range []string{"sessions", "active_session"} {
		for _, column := range []string{"generation", "site_known", "site_name", "slice_id", "session_id"} {
			if column == "slice_id" && table == "sessions" {
				if _, err := db.Exec(`DROP INDEX idx_session_slice`); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := db.Exec(`ALTER TABLE ` + table + ` DROP COLUMN ` + column); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := SetConfig(schemaVersionKey, "3"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO apps_daily(date, app_name, exe_name, app_id, total_duration_secs) VALUES ('2026-01-01','Browser','firefox.exe','firefox|real-install',123)`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(); err != nil {
		t.Fatal(err)
	}
	var appID string
	var seconds int
	if err := db.QueryRow(`SELECT app_id,total_duration_secs FROM apps_daily`).Scan(&appID, &seconds); err != nil {
		t.Fatal(err)
	}
	if appID != "firefox|real-install" || seconds != 123 {
		t.Errorf("migration changed existing install data: %s/%d", appID, seconds)
	}
	backups, err := filepath.Glob(filepath.Join(dir, "focusd", "focusd.db.*pre-v3"))
	if err != nil || len(backups) == 0 {
		t.Errorf("checkpoint migration backup = %v, err %v", backups, err)
	}
	if version, err := GetConfig(schemaVersionKey); err != nil || version != fmt.Sprint(schemaVersion) {
		t.Errorf("schema version = %s, err %v", version, err)
	}
}

func TestTrackingBackupIsConsistentAndPreservesSource(t *testing.T) {
	setupStorageEnv(t)
	session := checkpointSession("backup-test", time.Now().Add(-time.Hour))
	if err := WriteCheckpoint([]*Session{session}, nil); err != nil {
		t.Fatal(err)
	}
	path, err := BackupTrackingDatabase()
	if err != nil {
		t.Fatal(err)
	}
	backup, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	var backupCount, sourceCount int
	if err := backup.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&backupCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&sourceCount); err != nil {
		t.Fatal(err)
	}
	if backupCount != 1 || sourceCount != 1 {
		t.Errorf("backup/source rows = %d/%d; want 1/1", backupCount, sourceCount)
	}
}
