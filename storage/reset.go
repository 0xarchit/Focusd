package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

type ResetResult struct {
	Generation uint64 `json:"generation"`
	BackupPath string `json:"backup_path,omitempty"`
	Warning    string `json:"warning,omitempty"`
}

// GetResetResult reads a durable reset receipt without modifying tracking data.
func GetResetResult(requestID string) (ResetResult, bool, error) {
	if requestID == "" || len(requestID) > 128 || strings.ContainsAny(requestID, "\x00\r\n") {
		return ResetResult{}, false, errors.New("invalid reset request ID")
	}
	return storedResetResult(db, "reset_result:"+requestID)
}

// ResetTrackingData requires the caller to quiesce the profile's writer.
func ResetTrackingData(requestID string, withBackup bool) (ResetResult, error) {
	if result, found, err := GetResetResult(requestID); err != nil || found {
		return result, err
	}
	key := "reset_result:" + requestID
	result := ResetResult{}
	if withBackup {
		path, err := BackupTrackingDatabase()
		if err != nil {
			return result, err
		}
		result.BackupPath = path
	}
	tx, err := db.Begin()
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	if previous, found, err := storedResetResult(tx, key); err != nil || found {
		return previous, err
	}
	generation, err := dataGeneration(tx)
	if err != nil {
		return result, err
	}
	if generation >= math.MaxInt64 {
		return result, errors.New("tracking data generation exhausted")
	}
	result.Generation = generation + 1
	for _, table := range []string{"sessions", "apps_daily", "browsing_daily", "active_session"} {
		if _, err := tx.Exec(`DELETE FROM ` + table); err != nil {
			return result, err
		}
	}
	// A nonempty inactive state prevents lazy import of leftover pomodoro.json.
	if err := setConfigTx(tx, "pomodoro_state", `{"active":false,"duration_minutes":25,"notified":false}`); err != nil {
		return result, err
	}
	if err := setConfigTx(tx, dataGenerationKey, strconv.FormatUint(result.Generation, 10)); err != nil {
		return result, err
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return result, err
	}
	if err := setConfigTx(tx, key, string(encoded)); err != nil {
		return result, err
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	if _, err := db.Exec(`VACUUM`); err != nil {
		result.Warning = "History was deleted; database compaction failed: " + err.Error()
		if encoded, encodeErr := json.Marshal(result); encodeErr == nil {
			_ = SetConfig(key, string(encoded))
		}
	}
	return result, nil
}

func storedResetResult(reader activeSessionReader, key string) (ResetResult, bool, error) {
	var raw string
	err := reader.QueryRow(`SELECT value FROM config WHERE key = ?`, key).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return ResetResult{}, false, nil
	}
	if err != nil {
		return ResetResult{}, false, err
	}
	var result ResetResult
	if json.Unmarshal([]byte(raw), &result) != nil || result.Generation == 0 {
		return result, false, fmt.Errorf("invalid stored reset outcome")
	}
	return result, true, nil
}

func setConfigTx(tx *sql.Tx, key, value string) error {
	_, err := tx.Exec(`INSERT INTO config(key,value,updated_at) VALUES (?,?,?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`, key, value, time.Now().Unix())
	return err
}
