package cli

import (
	"encoding/json"
	"fmt"
	"focusd/core"
	"focusd/coreapi"
	"focusd/storage"
	"focusd/ui"
	"os"
	"time"
)

func runStatus() {
	if err := storage.Init(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to initialize: %v", err))
		os.Exit(1)
	}
	defer storage.Close()

	response, stateErr := coreapi.Call(coreapi.Request{Command: "status"})
	isRunning := stateErr == nil && response.OK
	var daemon struct {
		Clock storage.ClockState `json:"clock"`
		Error string             `json:"error"`
	}
	if isRunning {
		stateErr = json.Unmarshal(response.Data, &daemon)
		coreapi.SendIPCCmd("flush")
	}
	if _, err := coreapi.ReadEndpoint(); os.IsNotExist(err) {
		stateErr = nil
	}

	// Finish an expired timer only while this user's daemon is stopped: a
	// running daemon announces completion itself, so checking here too
	// could toast the same timer twice.
	if stateErr == nil && !isRunning {
		core.CheckPomodoroCompletion()
	}

	ui.PrintHeader()

	if stateErr != nil {
		ui.PrintWarn(fmt.Sprintf("Daemon: UNKNOWN (%v)", stateErr))
	} else if isRunning {
		ui.PrintOK("Daemon: RUNNING")
	} else {
		ui.PrintWarn("Daemon: STOPPED")
		fmt.Println("  Run 'focusd start' to start tracking")
	}

	if storage.IsPaused() {
		ui.PrintWarn("Tracking: PAUSED")
	} else if daemon.Clock.Incident != nil || daemon.Error != "" || storage.TimeGuarded() {
		ui.PrintWarn("Tracking: WAITING FOR CLOCK DECISION")
	} else if stateErr != nil {
		ui.PrintWarn("Tracking: UNKNOWN (daemon state unavailable)")
	} else if isRunning {
		ui.PrintOK("Tracking: ACTIVE")
	} else {
		ui.PrintInfo("Tracking: INACTIVE (daemon not running)")
	}
	if daemon.Clock.Incident != nil {
		fmt.Printf("  Clock warning: %s\n  Open 'focusd tui' to continue, reset history, or turn off.\n", daemon.Clock.Incident.Reason)
	}

	fmt.Println()

	summary, err := coreapi.GetDailySummary(storage.Today())
	if err != nil {
		ui.PrintWarn(fmt.Sprintf("Could not read today's stats: %v", err))
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
