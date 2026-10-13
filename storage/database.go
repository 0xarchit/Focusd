package storage

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	// busyRetryAttempts and busyRetryDelay bound the retry budget for a read
	// that races the daemon's writer.
	busyRetryAttempts = 4
	busyRetryDelay    = 20 * time.Millisecond
)

var db *sql.DB

func getDataDir() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", fmt.Errorf("APPDATA environment variable not set")
	}
	return filepath.Join(appData, "focusd"), nil
}

func GetDBPath() (string, error) {
	dataDir, err := getDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "focusd.db"), nil
}

func Init() error {
	if db != nil {
		if err := db.Ping(); err == nil {
			return nil
		}
		db.Close()
		db = nil
	}
	return initDB()
}

func initDB() error {
	dataDir, err := getDataDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath, err := GetDBPath()
	if err != nil {
		return err
	}

	q := url.Values{}
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "synchronous(NORMAL)")
	q.Add("_pragma", "auto_vacuum(INCREMENTAL)")
	q.Add("_pragma", "cache_size(-200)")
	q.Add("_pragma", "temp_store(MEMORY)")

	uPath := filepath.ToSlash(dbPath)
	if len(uPath) >= 2 && uPath[1] == ':' {
		uPath = "/" + uPath
	}
	u := &url.URL{
		Scheme:   "file",
		Path:     uPath,
		RawQuery: q.Encode(),
	}
	dsn := u.String()

	var lastErr error
	db, lastErr = sql.Open("sqlite", dsn)
	if lastErr != nil {
		db = nil
		return fmt.Errorf("failed to open database: %w", lastErr)
	}
	if pingErr := db.Ping(); pingErr != nil {
		db.Close()
		db = nil
		return fmt.Errorf("failed to ping database: %w", pingErr)
	}

	// One connection serialises access within a process and avoids self
	// contention on the WAL writer. Cross-process contention is handled by
	// busy_timeout plus withBusyRetry.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := createSchema(); err != nil {
		db.Close()
		db = nil
		return err
	}

	if err := Migrate(); err != nil {
		db.Close()
		db = nil
		return err
	}
	return nil
}

func createSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS config (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		app_name TEXT NOT NULL,
		exe_name TEXT NOT NULL,
		exe_path TEXT,
		app_id TEXT,
		window_title TEXT,
		start_time INTEGER NOT NULL,
		end_time INTEGER,
		duration_secs INTEGER,
		date TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS apps_daily (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		date TEXT NOT NULL,
		app_name TEXT NOT NULL,
		exe_name TEXT NOT NULL,
		exe_path TEXT,
		app_id TEXT,
		total_duration_secs INTEGER DEFAULT 0,
		open_count INTEGER DEFAULT 0,
		UNIQUE(date, app_id)
	);

	CREATE TABLE IF NOT EXISTS active_session (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		app_name TEXT NOT NULL,
		exe_name TEXT NOT NULL,
		exe_path TEXT,
		app_id TEXT,
		window_title TEXT,
		start_time INTEGER NOT NULL,
		last_seen INTEGER NOT NULL,
		date TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS browsing_daily (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		date TEXT NOT NULL,
		app_id TEXT,
		site_name TEXT NOT NULL,
		total_duration_secs INTEGER DEFAULT 0,
		open_count INTEGER DEFAULT 0,
		UNIQUE(date, app_id, site_name)
	);

	CREATE INDEX IF NOT EXISTS idx_sessions_date ON sessions(date);
	CREATE INDEX IF NOT EXISTS idx_sessions_start ON sessions(start_time);

	`

	_, err := db.Exec(schema)
	return err
}

// withBusyRetry retries an operation that hit SQLITE_BUSY. The daemon and the
// CLI hold separate handles to the same file, so a read can lose a race with the
// daemon's writer despite busy_timeout. The budget is kept short because this
// runs on the UI's read path.
func withBusyRetry(operation func() error) error {
	var lastErr error
	for attempt := 0; attempt < busyRetryAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * busyRetryDelay)
		}
		lastErr = operation()
		if lastErr == nil {
			return nil
		}
		if !isBusyError(lastErr) {
			return lastErr
		}
	}
	return lastErr
}

func isBusyError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "database table is locked") ||
		strings.Contains(msg, "sqlite_busy")
}

func Close() error {
	if db != nil {
		err := db.Close()
		db = nil
		return err
	}
	return nil
}

func Today() string {
	return time.Now().Format("2006-01-02")
}
