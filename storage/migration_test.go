package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// setupStorageEnv points APPDATA at a temp directory and opens a database there.
// Nothing in these tests ever touches the real user database.
func setupStorageEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	original, had := os.LookupEnv("APPDATA")
	if err := os.Setenv("APPDATA", dir); err != nil {
		t.Fatalf("failed to set APPDATA: %v", err)
	}
	t.Cleanup(func() {
		Close()
		if had {
			os.Setenv("APPDATA", original)
		} else {
			os.Unsetenv("APPDATA")
		}
	})

	Close()
	if err := Init(); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	return dir
}

// createLegacySchema builds a v2 database by hand, so the migration is tested
// against the shape it will actually encounter rather than a mock. Init has
// already created the current tables, so they are dropped first.
func createLegacySchema(t *testing.T) {
	t.Helper()

	for _, table := range []string{"browsing_daily", "active_session", "apps_daily", "sessions", "config"} {
		if _, err := db.Exec("DROP TABLE IF EXISTS " + table); err != nil {
			t.Fatalf("failed to drop %s: %v", table, err)
		}
	}

	legacy := `
		CREATE TABLE config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at INTEGER NOT NULL
		);
		CREATE TABLE sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			app_name TEXT NOT NULL,
			exe_name TEXT NOT NULL,
			window_title TEXT,
			start_time INTEGER NOT NULL,
			end_time INTEGER,
			duration_secs INTEGER,
			date TEXT NOT NULL
		);
		CREATE TABLE apps_daily (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date TEXT NOT NULL,
			app_name TEXT NOT NULL,
			exe_name TEXT NOT NULL,
			total_duration_secs INTEGER DEFAULT 0,
			open_count INTEGER DEFAULT 0,
			UNIQUE(date, exe_name)
		);
		CREATE TABLE active_session (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			app_name TEXT NOT NULL,
			exe_name TEXT NOT NULL,
			window_title TEXT,
			start_time INTEGER NOT NULL,
			last_seen INTEGER NOT NULL,
			date TEXT NOT NULL
		);
		CREATE TABLE browsing_daily (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date TEXT NOT NULL,
			domain_or_title TEXT NOT NULL,
			total_duration_secs INTEGER DEFAULT 0,
			open_count INTEGER DEFAULT 0,
			UNIQUE(date, domain_or_title)
		);
		CREATE INDEX idx_apps_daily_date ON apps_daily(date);
	`
	if _, err := db.Exec(legacy); err != nil {
		t.Fatalf("failed to create legacy schema: %v", err)
	}
}

func seedLegacyRows(t *testing.T) {
	t.Helper()
	stmts := []string{
		`INSERT INTO config (key, value, updated_at) VALUES ('retention_days', '7', 1)`,
		`INSERT INTO apps_daily (date, app_name, exe_name, total_duration_secs, open_count) VALUES ('2026-06-26', 'Firefox', 'firefox.exe', 3600, 5)`,
		// Case variant of the same app: the old UNIQUE was case sensitive, so
		// both rows can exist and the migration must collapse them.
		`INSERT INTO apps_daily (date, app_name, exe_name, total_duration_secs, open_count) VALUES ('2026-06-26', 'Firefox', 'Firefox.exe', 600, 1)`,
		`INSERT INTO apps_daily (date, app_name, exe_name, total_duration_secs, open_count) VALUES ('2026-06-25', 'Chrome', 'chrome.exe', 1200, 2)`,
		`INSERT INTO sessions (app_name, exe_name, window_title, start_time, end_time, duration_secs, date) VALUES ('Firefox', 'firefox.exe', 'Window', 100, 200, 100, '2026-06-26')`,
		`INSERT INTO browsing_daily (date, domain_or_title, total_duration_secs, open_count) VALUES ('2026-06-26', 'GitHub', 900, 3)`,
		`INSERT INTO browsing_daily (date, domain_or_title, total_duration_secs, open_count) VALUES ('2026-06-26', 'YouTube', 1800, 4)`,
		`INSERT INTO active_session (id, app_name, exe_name, window_title, start_time, last_seen, date) VALUES (1, 'Chrome', 'chrome.exe', 'Tab', 100, 200, '2026-06-26')`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("seed failed for %q: %v", s, err)
		}
	}
}

func TestMigrateV2ToV3(t *testing.T) {
	setupStorageEnv(t)
	createLegacySchema(t)
	seedLegacyRows(t)

	if err := Migrate(); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	version, err := currentSchemaVersion()
	if err != nil {
		t.Fatalf("currentSchemaVersion failed: %v", err)
	}
	if version != schemaVersion {
		t.Errorf("schema version = %d; want %d", version, schemaVersion)
	}

	// Case-variant rows must be collapsed, with durations summed.
	var secs, opens int
	var appName, appID, exeName string
	err = db.QueryRow(`
		SELECT app_name, exe_name, app_id, total_duration_secs, open_count
		FROM apps_daily WHERE date = '2026-06-26'
	`).Scan(&appName, &exeName, &appID, &secs, &opens)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if secs != 4200 {
		t.Errorf("duration = %d; want 4200 after collapsing case variants", secs)
	}
	if opens != 6 {
		t.Errorf("open count = %d; want 6", opens)
	}
	if appID != "firefox|legacy" {
		t.Errorf("app_id = %q; want firefox|legacy", appID)
	}

	// No row may be lost.
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM apps_daily`).Scan(&count); err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 2 {
		t.Errorf("apps_daily rows = %d; want 2 after collapsing duplicates", count)
	}
}

func TestMigrateBackfillsSessionIdentity(t *testing.T) {
	setupStorageEnv(t)
	createLegacySchema(t)
	seedLegacyRows(t)

	if err := Migrate(); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	var appID string
	if err := db.QueryRow(`SELECT COALESCE(app_id, '') FROM sessions LIMIT 1`).Scan(&appID); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if appID != "firefox|legacy" {
		t.Errorf("session app_id = %q; want firefox|legacy", appID)
	}

	var activeID string
	if err := db.QueryRow(`SELECT COALESCE(app_id, '') FROM active_session WHERE id = 1`).Scan(&activeID); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if activeID != "chrome|legacy" {
		t.Errorf("active_session app_id = %q; want chrome|legacy", activeID)
	}
}

func TestMigrateRebuildsBrowsingDaily(t *testing.T) {
	setupStorageEnv(t)
	createLegacySchema(t)
	seedLegacyRows(t)

	if err := Migrate(); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	var site, appID string
	var secs int
	err := db.QueryRow(`
		SELECT site_name, COALESCE(app_id, ''), total_duration_secs
		FROM browsing_daily WHERE site_name = 'GitHub'
	`).Scan(&site, &appID, &secs)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if secs != 900 {
		t.Errorf("duration = %d; want 900", secs)
	}
	if appID != "browser|legacy" {
		t.Errorf("browsing app_id = %q; want browser|legacy", appID)
	}
}

// TestMigrateKeepsBackup proves the safety net: a copy must exist afterwards.
func TestMigrateKeepsBackup(t *testing.T) {
	dir := setupStorageEnv(t)
	createLegacySchema(t)
	seedLegacyRows(t)

	if err := Migrate(); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(dir, "focusd", "focusd.db.*pre-v3"))
	if err != nil {
		t.Fatalf("glob failed: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("expected a pre-migration backup file to be written")
	}
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil {
			t.Fatalf("backup stat failed: %v", err)
		}
		if info.Size() == 0 {
			t.Errorf("backup %s is empty", m)
		}
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	setupStorageEnv(t)
	createLegacySchema(t)
	seedLegacyRows(t)

	if err := Migrate(); err != nil {
		t.Fatalf("first Migrate failed: %v", err)
	}
	var countBefore int
	if err := db.QueryRow(`SELECT COUNT(*) FROM apps_daily`).Scan(&countBefore); err != nil {
		t.Fatalf("count failed: %v", err)
	}

	// Running again must be a no-op, not a second transformation.
	if err := Migrate(); err != nil {
		t.Fatalf("second Migrate failed: %v", err)
	}
	var countAfter int
	if err := db.QueryRow(`SELECT COUNT(*) FROM apps_daily`).Scan(&countAfter); err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if countAfter != countBefore {
		t.Errorf("row count changed on a repeat migration: %d then %d", countBefore, countAfter)
	}
}

func TestMigrateFreshDatabaseSkipsBackup(t *testing.T) {
	dir := setupStorageEnv(t)

	if err := Migrate(); err != nil {
		t.Fatalf("Migrate failed on a fresh database: %v", err)
	}

	matches, _ := filepath.Glob(filepath.Join(dir, "focusd", "focusd.db.*pre-v3"))
	if len(matches) != 0 {
		t.Errorf("a fresh database should not produce a backup, found %v", matches)
	}
}

// TestMigrationKeyMatchesLiveDerivation is the invariant that matters most: the
// migration SQL and legacyAppIDFor must produce identical keys, or the same app
// lands under two keys on upgrade day and totals double-count.
func TestMigrationKeyMatchesLiveDerivation(t *testing.T) {
	setupStorageEnv(t)

	if _, err := db.Exec(`CREATE TABLE _keycheck (exe_name TEXT)`); err != nil {
		t.Fatalf("failed to create helper table: %v", err)
	}
	defer db.Exec(`DROP TABLE IF EXISTS _keycheck`)

	cases := []string{"firefox.exe", "Firefox.exe", "FIREFOX.EXE", "chrome.exe", "notepad++.exe", "firefox"}

	for _, exe := range cases {
		if _, err := db.Exec(`INSERT INTO _keycheck (exe_name) VALUES (?)`, exe); err != nil {
			t.Fatalf("insert failed: %v", err)
		}
	}

	rows, err := db.Query(`SELECT exe_name, ` + fmt.Sprintf(legacyAppIDExpr, "exe_name", "exe_name", "exe_name", "exe_name") + ` FROM _keycheck`)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var exe, sqlKey string
		if err := rows.Scan(&exe, &sqlKey); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
		if want := legacyAppIDFor(exe); sqlKey != want {
			t.Errorf("for %q the migration produces %q but live code produces %q", exe, sqlKey, want)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows error: %v", err)
	}
}

func TestLegacyAppIDFor(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"firefox.exe", "firefox|legacy"},
		{"Firefox.exe", "firefox|legacy"},
		{"firefox", "firefox|legacy"},
		{"", "unknown|legacy"},
	}
	for _, tc := range tests {
		if got := legacyAppIDFor(tc.in); got != tc.want {
			t.Errorf("legacyAppIDFor(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

// TestMigrateEmptyLegacyDatabase covers the install that upgraded but never
// recorded anything: deciding on row counts stamped such a database v3 and
// left the legacy tables in place, so every later query hit missing columns.
func TestMigrateEmptyLegacyDatabase(t *testing.T) {
	setupStorageEnv(t)
	createLegacySchema(t)

	if err := Migrate(); err != nil {
		t.Fatalf("Migrate failed on an empty legacy database: %v", err)
	}

	for _, query := range []string{
		`SELECT COUNT(app_id) FROM apps_daily`,
		`SELECT COUNT(exe_path) FROM apps_daily`,
		`SELECT COUNT(app_id) FROM sessions`,
		`SELECT COUNT(site_name) FROM browsing_daily`,
	} {
		var count int
		if err := db.QueryRow(query).Scan(&count); err != nil {
			t.Errorf("empty legacy database was not rebuilt (%s failed): %v", query, err)
		}
	}
}

// TestMigrateRepairsStampedLegacyDatabase reproduces the brick itself: the old
// gate stamped v3 on an empty v2 database without rebuilding it. The shape has
// to win over the stored version so the next start repairs the database.
func TestMigrateRepairsStampedLegacyDatabase(t *testing.T) {
	dir := setupStorageEnv(t)
	createLegacySchema(t)

	if err := setSchemaVersion(schemaVersion); err != nil {
		t.Fatalf("failed to stamp the broken version: %v", err)
	}

	if err := Migrate(); err != nil {
		t.Fatalf("Migrate failed to repair a stamped legacy database: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(app_id) FROM apps_daily`).Scan(&count); err != nil {
		t.Fatalf("apps_daily still legacy shaped after repair: %v", err)
	}

	matches, _ := filepath.Glob(filepath.Join(dir, "focusd", "focusd.db.*pre-v3"))
	if len(matches) == 0 {
		t.Errorf("repairing a stamped database should keep a backup")
	}
}

// TestMigrateCorruptVersionKeepsV3Data pins F26: an unparseable schema_version
// is read as 0, so without the shape gate the migration would re-run against
// already-v3 tables - collapsing per-install rows into |legacy buckets. The
// shape decides, so a corrupt version on a v3 database is only re-stamped.
func TestMigrateCorruptVersionKeepsV3Data(t *testing.T) {
	dir := setupStorageEnv(t)

	if _, err := db.Exec(`
		INSERT INTO apps_daily (date, app_name, exe_name, exe_path, app_id, total_duration_secs, open_count)
		VALUES ('2026-10-18', 'Chrome', 'chrome.exe', 'C:\chrome.exe', 'chrome|aaaa1111', 120, 1)
	`); err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	if err := SetConfig(schemaVersionKey, "not-a-number"); err != nil {
		t.Fatalf("SetConfig failed: %v", err)
	}

	if err := Migrate(); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	var appID string
	if err := db.QueryRow(`SELECT app_id FROM apps_daily WHERE exe_name = 'chrome.exe'`).Scan(&appID); err != nil {
		t.Fatalf("read back failed: %v", err)
	}
	if appID != "chrome|aaaa1111" {
		t.Errorf("app_id = %q; want the per-install key preserved", appID)
	}

	version, err := GetConfig(schemaVersionKey)
	if err != nil {
		t.Fatalf("GetConfig failed: %v", err)
	}
	if version != "3" {
		t.Errorf("schema_version = %q; want a re-stamp to 3", version)
	}

	// No rebuild means no backup was taken.
	matches, err := filepath.Glob(filepath.Join(dir, "focusd", "focusd.db.*pre-v3"))
	if err != nil {
		t.Fatalf("glob failed: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("unexpected pre-migration backup(s): %v", matches)
	}
}

// TestBackupRestoresEveryCommittedRow pins the backup contents: frames
// committed by another connection can still live only in focusd.db-wal, so a
// restore from the backup must replay them (regression: only the main file was
// copied, silently dropping the tail).
func TestBackupRestoresEveryCommittedRow(t *testing.T) {
	setupStorageEnv(t)

	if _, err := db.Exec(`
		INSERT INTO apps_daily (date, app_name, exe_name, app_id, total_duration_secs, open_count)
		VALUES ('2026-10-18', 'Chrome', 'chrome.exe', 'chrome|aaaa1111', 60, 1)
	`); err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	// A second connection commits a row; while it stays open the frame may
	// not have reached the main file yet.
	dbPath, err := GetDBPath()
	if err != nil {
		t.Fatalf("GetDBPath failed: %v", err)
	}
	other, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("second open failed: %v", err)
	}
	defer other.Close()
	if _, err := other.Exec(`
		INSERT INTO apps_daily (date, app_name, exe_name, app_id, total_duration_secs, open_count)
		VALUES ('2026-10-18', 'Firefox', 'firefox.exe', 'firefox|bbbb2222', 30, 1)
	`); err != nil {
		t.Fatalf("second connection insert failed: %v", err)
	}

	backup, err := backupDatabaseFile()
	if err != nil {
		t.Fatalf("backupDatabaseFile failed: %v", err)
	}
	if backup == "" {
		t.Fatal("no backup was produced")
	}

	// Restore into a fresh directory: the main file plus the WAL sidecar when
	// one was taken with it.
	restored := filepath.Join(t.TempDir(), "focusd.db")
	if err := copyFile(backup, restored); err != nil {
		t.Fatalf("restore copy failed: %v", err)
	}
	if _, err := os.Stat(backup + "-wal"); err == nil {
		if err := copyFile(backup+"-wal", restored+"-wal"); err != nil {
			t.Fatalf("restore WAL copy failed: %v", err)
		}
	}

	restoredDB, err := sql.Open("sqlite", restored)
	if err != nil {
		t.Fatalf("restore open failed: %v", err)
	}
	defer restoredDB.Close()

	var count int
	if err := restoredDB.QueryRow(`SELECT COUNT(*) FROM apps_daily`).Scan(&count); err != nil {
		t.Fatalf("restore query failed: %v", err)
	}
	if count != 2 {
		t.Errorf("restored %d committed rows; want 2", count)
	}
}

// TestMigrateMergesExeSuffixVariants covers rows that differ only by the .exe
// suffix. Both compute the same legacy app id, so grouping by the raw
// exe_name put them in separate groups and UNIQUE(date, app_id) rejected the
// second row - aborting the migration on every retry.
func TestMigrateMergesExeSuffixVariants(t *testing.T) {
	setupStorageEnv(t)
	createLegacySchema(t)

	for _, row := range []struct {
		appName string
		exeName string
		secs    int
		opens   int
	}{
		{"Chrome", "chrome.exe", 1200, 2},
		{"Chrome", "chrome", 300, 1},
	} {
		if _, err := db.Exec(
			`INSERT INTO apps_daily (date, app_name, exe_name, total_duration_secs, open_count) VALUES ('2026-06-26', ?, ?, ?, ?)`,
			row.appName, row.exeName, row.secs, row.opens,
		); err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}

	if err := Migrate(); err != nil {
		t.Fatalf("Migrate aborted on .exe suffix variants: %v", err)
	}

	var appID string
	var secs, opens int
	if err := db.QueryRow(
		`SELECT COALESCE(app_id, ''), total_duration_secs, open_count FROM apps_daily WHERE date = '2026-06-26'`,
	).Scan(&appID, &secs, &opens); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if secs != 1500 || opens != 3 {
		t.Errorf("merged row = %ds/%d opens; want 1500s/3 opens", secs, opens)
	}
	if appID != "chrome|legacy" {
		t.Errorf("app_id = %q; want chrome|legacy", appID)
	}
}
