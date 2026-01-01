package cli

import (
	"fmt"
	"focusd/core"
	"focusd/storage"
	"focusd/system"
	"focusd/ui"
	"os"
	"time"
)

func RunStatus() {
	if err := storage.Init(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to initialize: %v", err))
		os.Exit(1)
	}
	defer storage.Close()

	if !storage.IsConsentGranted() {
		ui.PrintError("focusd is not initialized. Run 'focusd init' first.")
		os.Exit(1)
	}

	ui.PrintHeader()

	isRunning := false
	if system.GetProcessCount(system.DaemonProcessName) > 1 {
		isRunning = true
	}

	if isRunning {
		ui.PrintOK("Daemon: RUNNING")
	} else {
		ui.PrintWarn("Daemon: STOPPED")
		fmt.Println("  Run 'focusd start' to start tracking")
	}

	if storage.IsPaused() {
		ui.PrintWarn("Tracking: PAUSED")
	} else if isRunning {
		ui.PrintOK("Tracking: ACTIVE")
	} else {
		ui.PrintInfo("Tracking: INACTIVE (daemon not running)")
	}

	fmt.Println()

	summary, err := core.GetDailySummary(storage.Today())
	if err != nil {
		ui.PrintWarn("No data for today yet")
		return
	}

	fmt.Printf("Date: %s\n", time.Now().Format("Monday, January 2, 2006"))
	fmt.Println()

	ui.PrintSectionHeader("Today's Summary")
	fmt.Println()
	fmt.Printf("  Total App Time:     %s\n", ui.FormatDuration(summary.TotalAppTime))
	fmt.Printf("  Apps Used:          %d\n", summary.AppCount)
	fmt.Println()

	if len(summary.TopApps) > 0 {
		ui.PrintSectionHeader("Top Apps")
		columns := []ui.TableColumn{
			{Header: "App", Width: 20},
			{Header: "Time", Width: 10},
			{Header: "Opens", Width: 6},
		}
		var rows [][]string
		for i, app := range summary.TopApps {
			if i >= 5 {
				break
			}
			rows = append(rows, []string{
				app.AppName,
				ui.FormatDurationShort(app.TotalDurationSecs),
				fmt.Sprintf("%d", app.OpenCount),
			})
		}
		ui.PrintTable(columns, rows)
		fmt.Println()
	}

}
