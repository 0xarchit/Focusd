package storage

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// schemaVersion is the current data layout. A fresh database is created at this
// version directly; an older one is migrated.
const schemaVersion = 3

const schemaVersionKey = "schema_version"

// migrationBackups keeps every pre-migration copy rather than overwriting one
// name, so a botched migration can still be recovered after a second attempt.
const migrationBackupSuffix = ".pre-v3"

// currentSchemaVersion reads the stored version, treating an absent key as a
// legacy database.
func currentSchemaVersion() (int, error) {
	value, err := GetConfig(schemaVersionKey)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	v, err := strconv.Atoi(value)
	if err != nil {
		return 0, nil
	}
	return v, nil
}

// hasAnyData reports whether the legacy tables hold anything. An empty database
// needs no migration, only the new schema.
func hasAnyData() (bool, error) {
	for _, table := range []string{"sessions", "apps_daily", "browsing_daily"} {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			return false, fmt.Errorf("count %s: %w", table, err)
		}
		if count > 0 {
			return true, nil
		}
	}
	return false, nil
}

// backupDatabaseFile copies focusd.db aside. The copy must succeed, otherwise
// the migration refuses to run: this is the user's only copy of their history.
func backupDatabaseFile() (string, error) {
	dbPath, err := GetDBPath()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			// Nothing to back up on a fresh install.
			return "", nil
		}
		return "", err
	}

	stamp := time.Now().Format("20060102-150405")
	dest := dbPath + "." + stamp + migrationBackupSuffix

	src, err := os.Open(dbPath)
	if err != nil {
		return "", err
	}
	defer src.Close()

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		os.Remove(dest)
		return "", err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		os.Remove(dest)
		return "", err
	}
	if err := out.Close(); err != nil {
		os.Remove(dest)
		return "", err
	}

	info, err := os.Stat(dest)
	if err != nil || info.Size() == 0 {
		os.Remove(dest)
		return "", fmt.Errorf("backup verification failed")
	}

	return dest, nil
}

// Migrate brings the schema up to the current version. It is called during
// startup, before any tracking work begins.
func Migrate() error {
	version, err := currentSchemaVersion()
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	if version >= schemaVersion {
		return nil
	}

	needsData, err := hasAnyData()
	if err != nil {
		// A brand new database has no legacy tables yet; creating them is the
		// only step needed.
		needsData = false
	}

	if !needsData {
		return setSchemaVersion(schemaVersion)
	}

	backupPath, err := backupDatabaseFile()
	if err != nil {
		return fmt.Errorf("refusing to migrate without a database backup: %w", err)
	}
	if backupPath != "" {
		log.Printf("INFO: database backed up to %s before migration", backupPath)
	}

	if err := runV3Migration(); err != nil {
		return fmt.Errorf("migration to v3 failed, backup kept at %s: %w", backupPath, err)
	}

	log.Printf("INFO: migrated database to schema v%d (backup: %s)", schemaVersion, backupPath)
	return nil
}

func setSchemaVersion(v int) error {
	return SetConfig(schemaVersionKey, strconv.Itoa(v))
}

// runV3Migration performs the conversion inside a single transaction. SQLite
// cannot alter a UNIQUE constraint, so the affected tables are rebuilt.
func runV3Migration() error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	steps := []struct {
		name string
		run  func(tx *sql.Tx) error
	}{
		{"apps_daily", migrateAppsDaily},
		{"browsing_daily", migrateBrowsingDaily},
		{"sessions", migrateSessions},
		{"active_session", migrateActiveSession},
	}

	for _, step := range steps {
		if err := step.run(tx); err != nil {
			return fmt.Errorf("%s: %w", step.name, err)
		}
	}

	if _, err := tx.Exec(`
		INSERT INTO config (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, schemaVersionKey, strconv.Itoa(schemaVersion), time.Now().Unix()); err != nil {
		return err
	}

	return tx.Commit()
}

// legacyAppIDExpr is the SQL that reproduces the live legacyAppIDFor derivation
// exactly: lowercase, then drop a trailing ".exe" and nothing else.
//
// Getting this byte-identical to what live code produces is not cosmetic. If
// the migration writes "firefox.exe|legacy" while live code writes
// "firefox|legacy", the same app appears under two keys and transition-day
// totals double-count.
//
// Note that rtrim(x, '.exe') is NOT usable here: rtrim strips any trailing
// characters from that set, so "firefox.exe" would become "firefo".
const legacyAppIDExpr = `CASE WHEN lower(%s) LIKE '%%.exe' ` +
	`THEN substr(lower(%s), 1, length(lower(%s)) - 4) ELSE lower(%s) END || '|legacy'`

// migrateAppsDaily rebuilds the table keyed by app identity. GROUP BY is
// mandatory: the old UNIQUE(date, exe_name) was case sensitive, so a machine
// that recorded both Firefox.exe and firefox.exe has two rows that would abort
// the insert against the new key.
func migrateAppsDaily(tx *sql.Tx) error {
	if _, err := tx.Exec(`
		CREATE TABLE apps_daily_v3 (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date TEXT NOT NULL,
			app_name TEXT NOT NULL,
			exe_name TEXT NOT NULL,
			exe_path TEXT,
			app_id TEXT,
			total_duration_secs INTEGER DEFAULT 0,
			open_count INTEGER DEFAULT 0,
			UNIQUE(date, app_id)
		)
	`); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		INSERT INTO apps_daily_v3 (date, app_name, exe_name, app_id, total_duration_secs, open_count)
		SELECT date,
		       MIN(app_name),
		       MIN(exe_name),
		       CASE WHEN lower(exe_name) LIKE '%.exe'
		            THEN substr(lower(exe_name), 1, length(lower(exe_name)) - 4)
		            ELSE lower(exe_name) END || '|legacy',
		       SUM(total_duration_secs),
		       SUM(open_count)
		FROM apps_daily
		GROUP BY date, lower(exe_name)
	`); err != nil {
		return err
	}

	if _, err := tx.Exec(`DROP TABLE apps_daily`); err != nil {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE apps_daily_v3 RENAME TO apps_daily`); err != nil {
		return err
	}
	// The old explicit date index duplicated the one the UNIQUE constraint
	// already creates.
	_, err := tx.Exec(`DROP INDEX IF EXISTS idx_apps_daily_date`)
	return err
}

// migrateBrowsingDaily rebuilds the site table keyed by app and site. The
// legacy table never recorded an executable, so migrated rows carry a generic
// browser sentinel rather than pretending to belong to a specific install.
// Referencing exe_name here is a real error: the v2 table has no such column.
func migrateBrowsingDaily(tx *sql.Tx) error {
	if _, err := tx.Exec(`
		CREATE TABLE browsing_daily_v3 (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date TEXT NOT NULL,
			app_id TEXT,
			site_name TEXT NOT NULL,
			total_duration_secs INTEGER DEFAULT 0,
			open_count INTEGER DEFAULT 0,
			UNIQUE(date, app_id, site_name)
		)
	`); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		INSERT INTO browsing_daily_v3 (date, app_id, site_name, total_duration_secs, open_count)
		SELECT date, 'browser|legacy', domain_or_title, SUM(total_duration_secs), SUM(open_count)
		FROM browsing_daily
		GROUP BY date, domain_or_title
	`); err != nil {
		return err
	}

	if _, err := tx.Exec(`DROP TABLE browsing_daily`); err != nil {
		return err
	}
	_, err := tx.Exec(`ALTER TABLE browsing_daily_v3 RENAME TO browsing_daily`)
	return err
}

func migrateSessions(tx *sql.Tx) error {
	for _, stmt := range []string{
		`ALTER TABLE sessions ADD COLUMN exe_path TEXT`,
		`ALTER TABLE sessions ADD COLUMN app_id TEXT`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}

	_, err := tx.Exec(`
		UPDATE sessions SET app_id = CASE WHEN lower(exe_name) LIKE '%.exe'
		                                   THEN substr(lower(exe_name), 1, length(lower(exe_name)) - 4)
		                                   ELSE lower(exe_name) END || '|legacy'
		WHERE app_id IS NULL
	`)
	return err
}

func migrateActiveSession(tx *sql.Tx) error {
	for _, stmt := range []string{
		`ALTER TABLE active_session ADD COLUMN exe_path TEXT`,
		`ALTER TABLE active_session ADD COLUMN app_id TEXT`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(`
		UPDATE active_session SET app_id = CASE WHEN lower(exe_name) LIKE '%.exe'
		                                         THEN substr(lower(exe_name), 1, length(lower(exe_name)) - 4)
		                                         ELSE lower(exe_name) END || '|legacy'
		WHERE app_id IS NULL AND exe_name IS NOT NULL AND exe_name != ''
	`); err != nil {
		return err
	}

	_, err := tx.Exec(`DELETE FROM active_session WHERE app_id IS NULL`)
	return err
}
