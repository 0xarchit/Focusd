package storage

import "testing"

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
