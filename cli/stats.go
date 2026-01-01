package cli

import (
	"bufio"
	"fmt"
	"focusd/core"
	"focusd/storage"
	"focusd/ui"
	"os"
	"strings"
)

func RunStats() {
	if err := storage.Init(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to initialize: %v", err))
		os.Exit(1)
	}
	defer storage.Close()

	if !storage.IsConsentGranted() {
		ui.PrintError("focusd is not initialized. Run 'focusd init' first.")
		os.Exit(1)
	}

	today := storage.Today()
	summary, err := core.GetDailySummary(today)
	if err != nil || summary.AppCount == 0 {
		ui.PrintInfo("No data recorded yet. Start tracking with 'focusd' command.")
		return
	}

	DisplayStats(summary)
}

func HandleStatsMenu(reader *bufio.Reader) {
	for {
		ui.ClearScreen()
		ui.PrintSectionHeader("View Statistics")

		fmt.Printf("     %s1.%s Today\n", ui.Cyan, ui.Reset)
		fmt.Printf("     %s2.%s Last 7 Days\n", ui.Cyan, ui.Reset)
		fmt.Printf("     %s3.%s All Time (30 Days)\n", ui.Cyan, ui.Reset)
		fmt.Println()
		fmt.Printf("     %s0.%s Back\n", ui.Dim, ui.Reset)
		fmt.Println()
		fmt.Print("   Enter choice: ")

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		var days int
		switch input {
		case "1":
			days = 1
		case "2":
			days = 7
		case "3":
			days = 30
		case "0", "":
			return
		default:
			continue
		}

		ui.ClearScreen()

		var summary *core.DailySummary
		var err error

		if days == 1 {
			summary, err = core.GetDailySummary(storage.Today())
		} else {
			summary, err = core.GetPeriodSummary(days)
		}

		if err != nil {
			ui.PrintError(fmt.Sprintf("Failed to fetch statistics: %v", err))
		} else {
			DisplayStats(summary)
		}

		waitForEnterWithReader(reader)
	}
}

func DisplayStats(summary *core.DailySummary) {
	header := fmt.Sprintf("Stats: %s", summary.Date)
	ui.PrintSectionHeader(header)

	if summary.RangeMessage != "" {
		fmt.Printf("  %s%s%s\n\n", ui.Dim, summary.RangeMessage, ui.Reset)
	}

	if summary.AppCount == 0 {
		fmt.Println("  No data recorded for this period.")
		fmt.Println()
		return
	}

	ui.PrintStatus("Total Screen Time", ui.FormatDuration(summary.TotalAppTime), false)
	ui.PrintStatus("Apps Used", fmt.Sprintf("%d", summary.AppCount), false)
	fmt.Println()

	ui.PrintSectionHeader("Top Apps")
	if len(summary.TopApps) == 0 {
		fmt.Println("  No app data.")
	} else {
		columns := []ui.TableColumn{
			{Header: "App", Width: 53},
			{Header: "Time", Width: 12},
			{Header: "Opens", Width: 8},
		}
		var rows [][]string
		for _, app := range summary.TopApps {
			rows = append(rows, []string{
				ui.TruncateString(app.AppName, 53),
				ui.FormatDurationShort(app.TotalDurationSecs),
				fmt.Sprintf("%d", app.OpenCount),
			})
		}
		ui.PrintTable(columns, rows)
	}
	fmt.Println()

	if len(summary.TopSites) > 0 {
		ui.PrintSectionHeader("Top Browsing")
		columns := []ui.TableColumn{
			{Header: "Site / Title", Width: 53},
			{Header: "Time", Width: 12},
			{Header: "Visits", Width: 8},
		}
		var rows [][]string
		for _, site := range summary.TopSites {
			rows = append(rows, []string{
				ui.TruncateString(site.AppName, 53),
				ui.FormatDurationShort(site.TotalDurationSecs),
				fmt.Sprintf("%d", site.OpenCount),
			})
		}
		ui.PrintTable(columns, rows)
		fmt.Println()
	}
}
