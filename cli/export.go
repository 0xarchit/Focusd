package cli

import (
	"encoding/csv"
	"fmt"
	"focusd/storage"
	"focusd/ui"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func RunExport() {
	if err := storage.Init(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to initialize: %v", err))
		os.Exit(1)
	}
	defer storage.Close()

	if !storage.IsConsentGranted() {
		ui.PrintError("focusd is not initialized. Run 'focusd init' first.")
		os.Exit(1)
	}

	userProfile := os.Getenv("USERPROFILE")
	exportDir := filepath.Join(userProfile, "Downloads")
	if userProfile == "" {
		wd, err := os.Getwd()
		if err != nil {
			exportDir = "."
		} else {
			exportDir = wd
		}
	}

	timestamp := time.Now().Format("2006-01-02_150405")

	appsFile := filepath.Join(exportDir, fmt.Sprintf("focusd_apps_%s.csv", timestamp))
	sessionsFile := filepath.Join(exportDir, fmt.Sprintf("focusd_sessions_%s.csv", timestamp))

	if err := exportApps(appsFile); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to export apps: %v", err))
	} else {
		ui.PrintOK(fmt.Sprintf("Exported apps to: %s", appsFile))
	}

	if err := exportSessions(sessionsFile); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to export sessions: %v", err))
	} else {
		ui.PrintOK(fmt.Sprintf("Exported sessions to: %s", sessionsFile))
	}

	fmt.Println()
	ui.PrintInfo("Export complete!")
}

func exportApps(filename string) error {
	apps, err := storage.GetAllAppStats()
	if err != nil {
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"Date", "App Name", "Executable", "Duration (seconds)", "Open Count"})

	for _, app := range apps {
		writer.Write([]string{
			app.Date,
			app.AppName,
			app.ExeName,
			strconv.Itoa(app.TotalDurationSecs),
			strconv.Itoa(app.OpenCount),
		})
	}

	return nil
}

func exportSessions(filename string) error {
	sessions, err := storage.GetAllSessions()
	if err != nil {
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"Date", "App Name", "Executable", "Window Title", "Start Time", "End Time", "Duration (seconds)"})

	for _, s := range sessions {
		endTime := ""
		if !s.EndTime.IsZero() {
			endTime = s.EndTime.Format(time.RFC3339)
		}
		writer.Write([]string{
			s.Date,
			s.AppName,
			s.ExeName,
			s.WindowTitle,
			s.StartTime.Format(time.RFC3339),
			endTime,
			strconv.Itoa(s.DurationSecs),
		})
	}

	return nil
}
