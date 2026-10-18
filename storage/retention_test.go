package storage

import (
	"testing"
	"time"
)

func TestResolveRetentionDays(t *testing.T) {
	setupStorageEnv(t)

	// Unset row: the default, and no error.
	days, err := resolveRetentionDays()
	if err != nil {
		t.Fatalf("unset row returned error: %v", err)
	}
	if days != DefaultRetentionDays {
		t.Errorf("unset row = %d; want default %d", days, DefaultRetentionDays)
	}

	// A configured value wins.
	if err := SetRetentionDays(90); err != nil {
		t.Fatalf("SetRetentionDays failed: %v", err)
	}
	if days, err = resolveRetentionDays(); err != nil || days != 90 {
		t.Errorf("configured row = %d, %v; want 90, nil", days, err)
	}

	// A garbage value falls back to the default rather than an error.
	if err := SetConfig(configKeyRetentionDays, "banana"); err != nil {
		t.Fatalf("SetConfig failed: %v", err)
	}
	if days, err = resolveRetentionDays(); err != nil || days != DefaultRetentionDays {
		t.Errorf("garbage row = %d, %v; want default %d, nil", days, err, DefaultRetentionDays)
	}
}

// TestEnforceRetentionSkipsOnReadFailure pins that a failed read is not
// treated as the default window: one SQLITE_BUSY would otherwise silently
// prune a 365-day history down to 7 days.
func TestEnforceRetentionSkipsOnReadFailure(t *testing.T) {
	setupStorageEnv(t)

	if _, err := db.Exec(`
		INSERT INTO apps_daily (date, app_name, exe_name, app_id, total_duration_secs, open_count)
		VALUES ('2000-01-01', 'Old', 'old.exe', 'old|legacy', 60, 1)
	`); err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	db.Close() // reads now fail; EnforceRetention must not prune with the default

	EnforceRetention()

	// Reopen the same file and check the row survived.
	if err := Init(); err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM apps_daily WHERE date = '2000-01-01'`).Scan(&count); err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 1 {
		t.Errorf("old row was pruned despite the read failure (count = %d; want 1)", count)
	}
}

// TestEnforceRetentionKeepsExactlyNDays pins the off-by-one: "7 days" means
// seven date rows - today plus the six days before it, not today plus seven.
func TestEnforceRetentionKeepsExactlyNDays(t *testing.T) {
	setupStorageEnv(t)

	if err := SetRetentionDays(7); err != nil {
		t.Fatalf("SetRetentionDays failed: %v", err)
	}

	now := time.Now()
	for i := 0; i <= 8; i++ {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		if _, err := db.Exec(`
			INSERT INTO apps_daily (date, app_name, exe_name, app_id, total_duration_secs, open_count)
			VALUES (?, 'App', 'app.exe', 'app|key', 60, 1)
		`, date); err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}

	EnforceRetention()

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM apps_daily`).Scan(&count); err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 7 {
		t.Errorf("kept %d date rows; want exactly 7", count)
	}

	var kept int
	if err := db.QueryRow(`SELECT COUNT(*) FROM apps_daily WHERE date = ?`, now.AddDate(0, 0, -6).Format("2006-01-02")).Scan(&kept); err != nil {
		t.Fatalf("check failed: %v", err)
	}
	if kept != 1 {
		t.Error("the row six days back should be kept")
	}

	var pruned int
	if err := db.QueryRow(`SELECT COUNT(*) FROM apps_daily WHERE date = ?`, now.AddDate(0, 0, -7).Format("2006-01-02")).Scan(&pruned); err != nil {
		t.Fatalf("check failed: %v", err)
	}
	if pruned != 0 {
		t.Error("the row seven days back should be pruned")
	}
}
