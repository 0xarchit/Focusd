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
	q.Add("_pragma", "cache_size(-1000)")
	q.Add("_pragma", "temp_store(MEMORY)")
	escapedPath := filepath.ToSlash(dbPath)
	escapedPath = strings.ReplaceAll(escapedPath, " ", "%20")
	escapedPath = strings.ReplaceAll(escapedPath, "?", "%3F")
	escapedPath = strings.ReplaceAll(escapedPath, "#", "%23")
	dsn := fmt.Sprintf("file:%s?%s", escapedPath, q.Encode())

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

	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := createSchema(); err != nil {
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
		total_duration_secs INTEGER DEFAULT 0,
		open_count INTEGER DEFAULT 0,
		UNIQUE(date, exe_name)
	);

	CREATE TABLE IF NOT EXISTS active_session (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		app_name TEXT NOT NULL,
		exe_name TEXT NOT NULL,
		window_title TEXT,
		start_time INTEGER NOT NULL,
		last_seen INTEGER NOT NULL,
		date TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS browsing_daily (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		date TEXT NOT NULL,
		domain_or_title TEXT NOT NULL,
		total_duration_secs INTEGER DEFAULT 0,
		open_count INTEGER DEFAULT 0,
		UNIQUE(date, domain_or_title)
	);

	CREATE INDEX IF NOT EXISTS idx_sessions_date ON sessions(date);
	CREATE INDEX IF NOT EXISTS idx_sessions_start ON sessions(start_time);
	CREATE INDEX IF NOT EXISTS idx_apps_daily_date ON apps_daily(date);

	`

	_, err := db.Exec(schema)
	return err
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
