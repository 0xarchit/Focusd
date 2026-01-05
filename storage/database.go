package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

var db *sql.DB

func GetDataDir() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", fmt.Errorf("APPDATA environment variable not set")
	}
	return filepath.Join(appData, "focusd"), nil
}

func GetDBPath() (string, error) {
	dataDir, err := GetDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "focusd.db"), nil
}

func Init() error {
	dataDir, err := GetDataDir()
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

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		db, lastErr = sql.Open("sqlite", dbPath)
		if lastErr == nil {
			if pingErr := db.Ping(); pingErr == nil {
				break
			} else {
				lastErr = pingErr
				db.Close()
				db = nil
			}
		}
		time.Sleep(time.Duration(100*(attempt+1)) * time.Millisecond)
	}
	if lastErr != nil {
		return fmt.Errorf("failed to open database after retries: %w", lastErr)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)

	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA busy_timeout=5000")
	db.Exec("PRAGMA synchronous=NORMAL")
	db.Exec("PRAGMA auto_vacuum=INCREMENTAL")

	db.Exec("PRAGMA cache_size = -2000")
	db.Exec("PRAGMA mmap_size = 0")
	db.Exec("PRAGMA temp_store = FILE")

	return createSchema()
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

func GetDB() *sql.DB {
	return db
}

func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

func DeleteAllData() error {
	dataDir, err := GetDataDir()
	if err != nil {
		return err
	}

	if db != nil {
		db.Close()
		db = nil
	}

	return os.RemoveAll(dataDir)
}

func Today() string {
	return time.Now().Format("2006-01-02")
}
