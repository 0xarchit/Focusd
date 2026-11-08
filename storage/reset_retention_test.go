package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func requireDestructiveGuest(t *testing.T) {
	t.Helper()
	computer := os.Getenv("COMPUTERNAME")
	expected := os.Getenv("FOCUSD_SANDBOX_COMPUTER")
	if os.Getenv("FOCUSD_SANDBOX_GUEST") != "1" || expected == "" || !strings.EqualFold(computer, expected) || strings.EqualFold(computer, "0XARCHIT") {
		t.Skip("destructive storage integration requires the verified Windows Sandbox guest")
	}
}

func TestGetResetResultIsReadOnlyAndValidatesRequestID(t *testing.T) {
	setupStorageEnv(t)
	for _, id := range []string{"", strings.Repeat("x", 129), "bad\x00id", "bad\rid", "bad\nid"} {
		if _, found, err := GetResetResult(id); err == nil || found {
			t.Errorf("invalid ID %q: found %v, error %v", id, found, err)
		}
	}
	if result, found, err := GetResetResult("not-recorded"); err != nil || found || result != (ResetResult{}) {
		t.Errorf("missing receipt = %+v, found %v, error %v", result, found, err)
	}
	if generation, err := GetDataGeneration(); err != nil || generation != 0 {
		t.Errorf("lookup changed generation = %d, error %v", generation, err)
	}
	if err := SetConfig("reset_result:corrupt", "not JSON"); err != nil {
		t.Fatal(err)
	}
	if _, found, err := GetResetResult("corrupt"); err == nil || found {
		t.Errorf("corrupt receipt: found %v, error %v", found, err)
	}
}

func TestGuestResetReceiptSurvivesRestartAndPreservesNewRows(t *testing.T) {
	requireDestructiveGuest(t)
	setupStorageEnv(t)
	start := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := WriteCheckpoint([]*Session{checkpointSession("pre-restart-reset", start)}, nil); err != nil {
		t.Fatal(err)
	}
	result, err := ResetTrackingData("durable-operation-id", true)
	if err != nil {
		t.Fatal(err)
	}
	newSession := checkpointSession("after-reset-before-restart", start.Add(10*time.Minute))
	newSession.Generation = result.Generation
	active := &ActiveSessionRecord{
		SessionID: "new-active", AppName: "Editor", ExeName: "editor.exe", AppID: "editor|abcd1234",
		StartTime: start.Add(20 * time.Minute), LastSeen: start.Add(21 * time.Minute),
		Date: start.Format("2006-01-02"), Generation: result.Generation,
	}
	if err := WriteCheckpoint([]*Session{newSession}, active); err != nil {
		t.Fatal(err)
	}
	if err := SetConfig("pomodoro_state", `{"active":true,"fixture":"post-reset"}`); err != nil {
		t.Fatal(err)
	}
	if err := Close(); err != nil {
		t.Fatal(err)
	}
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	if receipt, found, err := GetResetResult("durable-operation-id"); err != nil || !found || receipt != result {
		t.Fatalf("restart receipt = %+v, found %v, error %v; want %+v", receipt, found, err, result)
	}
	if retry, err := ResetTrackingData("durable-operation-id", true); err != nil || retry != result {
		t.Fatalf("restart retry = %+v, error %v; want %+v", retry, err, result)
	}
	for _, table := range []string{"sessions", "apps_daily", "browsing_daily", "active_session"} {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil || count != 1 {
			t.Errorf("restart retry changed %s: %d rows, error %v", table, count, err)
		}
	}
	if timer, err := GetConfig("pomodoro_state"); err != nil || timer != `{"active":true,"fixture":"post-reset"}` {
		t.Errorf("restart retry changed new timer: %q, error %v", timer, err)
	}
	if generation, err := GetDataGeneration(); err != nil || generation != result.Generation {
		t.Errorf("restart retry changed generation: %d, error %v", generation, err)
	}
}

func TestGuestResetHistoryPreservesPreferencesAndIsIdempotent(t *testing.T) {
	requireDestructiveGuest(t)
	setupStorageEnv(t)
	start := time.Now().Add(-time.Hour)
	if err := WriteCheckpoint([]*Session{checkpointSession("old-history", start)}, &ActiveSessionRecord{
		SessionID: "active", AppName: "Editor", ExeName: "editor.exe", AppID: "editor|abcd1234",
		StartTime: start, LastSeen: start.Add(time.Minute), Date: Today(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := SetConfig("preferences_fixture", "keep"); err != nil {
		t.Fatal(err)
	}
	if err := SetConfig("pomodoro_state", `{"active":true}`); err != nil {
		t.Fatal(err)
	}
	result, err := ResetTrackingData("reset-once", true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Generation != 1 || result.BackupPath == "" {
		t.Fatalf("reset result = %+v; want generation 1 and verified backup", result)
	}
	for _, table := range []string{"sessions", "apps_daily", "browsing_daily", "active_session"} {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil || count != 0 {
			t.Errorf("reset %s count = %d, err %v", table, count, err)
		}
	}
	if value, err := GetConfig("preferences_fixture"); err != nil || value != "keep" {
		t.Errorf("preferences = %q, err %v", value, err)
	}
	var timer struct {
		Active bool `json:"active"`
	}
	if raw, err := GetConfig("pomodoro_state"); err != nil || json.Unmarshal([]byte(raw), &timer) != nil || timer.Active {
		t.Errorf("reset timer tombstone invalid: %q, %v", raw, err)
	}
	backup, err := sql.Open("sqlite", result.BackupPath)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := backup.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&count); err != nil || count != 1 {
		t.Errorf("backup session count = %d, err %v", count, err)
	}
	backup.Close()
	newSession := checkpointSession("post-reset", start)
	newSession.Generation = 1
	if err := WriteCheckpoint([]*Session{newSession}, nil); err != nil {
		t.Fatal(err)
	}
	if retry, err := ResetTrackingData("reset-once", true); err != nil || retry != result {
		t.Errorf("retry outcome = %+v, err %v; want original %+v", retry, err, result)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&count); err != nil || count != 1 {
		t.Errorf("retry deleted new session: count %d, err %v", count, err)
	}
	if err := WriteCheckpoint([]*Session{checkpointSession("old-generation", start)}, nil); !errors.Is(err, ErrStaleGeneration) {
		t.Errorf("old generation accepted after reset: %v", err)
	}
}

func TestGuestFailedResetBackupDoesNotDeleteHistory(t *testing.T) {
	requireDestructiveGuest(t)
	dir := setupStorageEnv(t)
	if err := WriteCheckpoint([]*Session{checkpointSession("keep-history", time.Now().Add(-time.Hour))}, nil); err != nil {
		t.Fatal(err)
	}
	const timer = `{"active":true,"duration_minutes":60}`
	if err := SetConfig("pomodoro_state", timer); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "focusd", "backups"), []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ResetTrackingData("failed-backup", true); err == nil {
		t.Fatal("backup failure must reject reset")
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&count); err != nil || count != 1 {
		t.Errorf("failed backup changed history: %d, %v", count, err)
	}
	if generation, err := GetDataGeneration(); err != nil || generation != 0 {
		t.Errorf("failed reset generation = %d, %v", generation, err)
	}
	if raw, err := GetConfig("pomodoro_state"); err != nil || raw != timer {
		t.Errorf("failed backup changed timer: %q, %v", raw, err)
	}
}

func TestGuestFailedResetTransactionPreservesTimerAndHistory(t *testing.T) {
	requireDestructiveGuest(t)
	setupStorageEnv(t)
	if err := WriteCheckpoint([]*Session{checkpointSession("transaction-preserve", time.Now().Add(-time.Hour))}, nil); err != nil {
		t.Fatal(err)
	}
	const timer = `{"active":true,"duration_minutes":60}`
	if err := SetConfig("pomodoro_state", timer); err != nil {
		t.Fatal(err)
	}
	// Fail after history deletes and the timer tombstone, before commit.
	if _, err := db.Exec(`CREATE TRIGGER reject_reset_generation BEFORE INSERT ON config
		WHEN NEW.key = 'data_generation' BEGIN SELECT RAISE(ABORT, 'injected reset failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := ResetTrackingData("failed-transaction", false); err == nil {
		t.Fatal("injected transaction failure must reject reset")
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&count); err != nil || count != 1 {
		t.Errorf("rollback changed history: %d rows, %v", count, err)
	}
	if raw, err := GetConfig("pomodoro_state"); err != nil || raw != timer {
		t.Errorf("rollback changed timer: %q, %v", raw, err)
	}
	if _, found, err := GetResetResult("failed-transaction"); err != nil || found {
		t.Errorf("rollback stored receipt: found %v, %v", found, err)
	}
}

func TestGuestRetentionHoldRequiresUnchangedPreview(t *testing.T) {
	requireDestructiveGuest(t)
	setupStorageEnv(t)
	state := ClockState{Version: clockStateVersion, RetentionHold: true, Epoch: 7}
	if err := SaveClockState(state); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO apps_daily(date,app_name,exe_name,app_id,total_duration_secs) VALUES ('2000-01-01','Old','old.exe','old|legacy',60)`); err != nil {
		t.Fatal(err)
	}
	EnforceRetention()
	preview, err := PreviewRetention(Today())
	if err != nil {
		t.Fatal(err)
	}
	if preview.Rows != 1 || preview.Oldest != "2000-01-01" || preview.Newest != "2000-01-01" || preview.Token == "" {
		t.Fatalf("preview = %+v", preview)
	}
	if _, err := db.Exec(`INSERT INTO apps_daily(date,app_name,exe_name,app_id,total_duration_secs) VALUES ('2000-01-02','Old','old.exe','old|legacy',60)`); err != nil {
		t.Fatal(err)
	}
	if err := ConfirmRetention(preview); !errors.Is(err, ErrRetentionPreviewChanged) {
		t.Fatalf("changed cleanup allowed: %v", err)
	}
	preview, err = PreviewRetention(Today())
	if err != nil {
		t.Fatal(err)
	}
	if err := ConfirmRetention(preview); err != nil {
		t.Fatal(err)
	}
	if RetentionHeld() {
		t.Error("confirmed cleanup retained hold")
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM apps_daily`).Scan(&count); err != nil || count != 0 {
		t.Errorf("confirmed cleanup left %d old rows, err %v", count, err)
	}
}

func TestGuestRetentionRejectsNewClockIncident(t *testing.T) {
	requireDestructiveGuest(t)
	setupStorageEnv(t)
	if err := SaveClockState(ClockState{Version: clockStateVersion, RetentionHold: true}); err != nil {
		t.Fatal(err)
	}
	preview, err := PreviewRetention(Today())
	if err != nil {
		t.Fatal(err)
	}
	state := ClockState{Version: clockStateVersion, RetentionHold: true, Incident: &ClockIncident{
		ID: "new-incident", Revision: 1, Current: ClockCheckpoint{WallUTC: time.Now()},
	}}
	if err := SaveClockState(state); err != nil {
		t.Fatal(err)
	}
	if err := ConfirmRetention(preview); !errors.Is(err, ErrRetentionPreviewChanged) {
		t.Errorf("new clock incident permits cleanup: %v", err)
	}
}
