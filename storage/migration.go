package storage

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// schemaVersion is the current data layout. A fresh database is created at this
// version directly; an older one is migrated.
const schemaVersion = 4

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

// legacyShape reports whether apps_daily exists without the v3 columns. Row
// counts are deliberately ignored: an empty v2 database still needs its tables
// rebuilt. A missing table means a fresh database, where the schema creation
// that follows builds the current layout.
func legacyShape() (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(apps_daily)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	exists := false
	hasAppID := false
	for rows.Next() {
		var (
			cid          int
			name         string
			columnType   string
			notNull      int
			defaultValue sql.NullString
			primaryKey   int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, err
		}
		exists = true
		if strings.EqualFold(name, "app_id") {
			hasAppID = true
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return exists && !hasAppID, nil
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

	id, err := backupID()
	if err != nil {
		return "", err
	}
	return snapshotDatabase(dbPath + "." + id + migrationBackupSuffix)
}

func backupID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(id[:]), nil
}

// BackupTrackingDatabase returns a verified snapshot before a history reset.
func BackupTrackingDatabase() (string, error) {
	dir, err := getDataDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, "backups")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	id, err := backupID()
	if err != nil {
		return "", err
	}
	return snapshotDatabase(filepath.Join(dir, "focusd-"+id+".db"))
}

func snapshotDatabase(dest string) (string, error) {
	if _, err := db.Exec(`VACUUM INTO ?`, dest); err != nil {
		return "", fmt.Errorf("database snapshot: %w", err)
	}
	f, err := os.OpenFile(dest, os.O_RDWR, 0600)
	if err == nil {
		err = f.Sync()
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
	}
	if err != nil {
		os.Remove(dest)
		return "", err
	}
	backupPath := filepath.ToSlash(dest)
	if len(backupPath) >= 2 && backupPath[1] == ':' {
		backupPath = "/" + backupPath
	}
	uri := &url.URL{Scheme: "file", Path: backupPath, RawQuery: "mode=ro"}
	backup, err := sql.Open("sqlite", uri.String())
	if err == nil {
		var check string
		err = backup.QueryRow(`PRAGMA integrity_check`).Scan(&check)
		if err == nil && check != "ok" {
			err = fmt.Errorf("backup integrity check: %s", check)
		}
		if closeErr := backup.Close(); err == nil {
			err = closeErr
		}
	}
	if err != nil {
		os.Remove(dest)
		return "", err
	}
	return dest, nil
}

// copyFile copies srcPath to destPath, creating the destination exclusively so
// a partial copy can never be mistaken for a good backup.
func copyFile(srcPath, destPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		os.Remove(destPath)
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		os.Remove(destPath)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(destPath)
		return err
	}
	return nil
}

// copyFileIfExists is copyFile for sidecar files (the WAL): a missing source is
// fine, anything else is an error.
func copyFileIfExists(srcPath, destPath string) error {
	if _, err := os.Stat(srcPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return copyFile(srcPath, destPath)
}

// Migrate brings the schema up to the current version. It is called during
// startup, before any tracking work begins.
func Migrate() error {
	version, err := currentSchemaVersion()
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	legacy, err := legacyShape()
	if err != nil {
		return fmt.Errorf("inspect schema shape: %w", err)
	}

	// The shape of apps_daily decides, not the stored version or the row
	// counts: a v2 database that never recorded anything still needs its
	// tables rebuilt, and a database the old data-counting gate stamped v3
	// without rebuilding is repaired here.
	if !legacy {
		return migrateCheckpointSchema(version)
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
	return migrateCheckpointSchema(3)
}

func migrateCheckpointSchema(version int) error {
	columns := []struct{ name, definition string }{
		{"session_id", `TEXT NOT NULL DEFAULT ''`},
		{"slice_id", `TEXT NOT NULL DEFAULT ''`},
		{"site_name", `TEXT NOT NULL DEFAULT ''`},
		{"site_known", `INTEGER NOT NULL DEFAULT 0`},
		{"generation", `INTEGER NOT NULL DEFAULT 0`},
	}
	missing := make(map[string][]string)
	for _, table := range []string{"sessions", "active_session"} {
		rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
		if err != nil {
			return err
		}
		found := make(map[string]bool)
		for rows.Next() {
			var cid, notNull, primaryKey int
			var name, kind string
			var defaultValue sql.NullString
			if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
				rows.Close()
				return err
			}
			found[name] = true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, column := range columns {
			if !found[column.name] {
				missing[table] = append(missing[table], column.name+" "+column.definition)
			}
		}
	}
	if len(missing) > 0 {
		if _, err := backupDatabaseFile(); err != nil {
			return fmt.Errorf("refusing checkpoint migration without a backup: %w", err)
		}
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for table, columns := range missing {
		for _, column := range columns {
			if _, err := tx.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column); err != nil {
				return err
			}
		}
	}
	if _, err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_session_slice ON sessions(slice_id) WHERE slice_id <> ''`); err != nil {
		return err
	}
	if version <= schemaVersion {
		if _, err := tx.Exec(`INSERT INTO config(key, value, updated_at) VALUES (?, ?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`, schemaVersionKey, strconv.Itoa(schemaVersion), time.Now().Unix()); err != nil {
			return err
		}
	}
	return tx.Commit()
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
	`, schemaVersionKey, "3", time.Now().Unix()); err != nil {
		return err
	}

	return tx.Commit()
}

// legacyAppIDExpr reproduces legacyAppIDFor exactly. A divergence here puts the
// same app under two keys and double-counts transition-day totals.
//
// rtrim(x, '.exe') is not usable: it strips any trailing characters from that
// set, so "firefox.exe" would become "firefo".
const legacyAppIDExpr = `CASE WHEN lower(%s) LIKE '%%.exe' ` +
	`THEN substr(lower(%s), 1, length(lower(%s)) - 4) ELSE lower(%s) END || '|legacy'`

// migrateAppsDaily rebuilds the table keyed by app identity. GROUP BY is
// required twice over: the old UNIQUE(date, exe_name) was case sensitive, so
// case-variant rows would otherwise abort the insert; and "chrome" plus
// "chrome.exe" collapse to one app id, so grouping them apart would collide on
// UNIQUE(date, app_id) and roll the whole migration back.
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

	appIDExpr := fmt.Sprintf(legacyAppIDExpr, "exe_name", "exe_name", "exe_name", "exe_name")
	if _, err := tx.Exec(fmt.Sprintf(`
		INSERT INTO apps_daily_v3 (date, app_name, exe_name, app_id, total_duration_secs, open_count)
		SELECT date,
		       MIN(app_name),
		       MIN(exe_name),
		       %s,
		       SUM(total_duration_secs),
		       SUM(open_count)
		FROM apps_daily
		GROUP BY date, %s
	`, appIDExpr, appIDExpr)); err != nil {
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

// migrateBrowsingDaily rebuilds the site table keyed by app and site. The v2
// table never recorded an executable, so migrated rows get a generic browser
// key rather than being attributed to an install.
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
