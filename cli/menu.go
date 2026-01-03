package cli

import (
	"bufio"
	"fmt"
	"focusd/core"
	"focusd/storage"
	"focusd/system"
	"focusd/ui"
	"os"
	"strconv"
	"strings"
)

func RunInteractiveMenu() {
	if err := storage.Init(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to initialize: %v", err))
		fmt.Println("\nPress Enter to exit...")
		bufio.NewReader(os.Stdin).ReadBytes('\n')
		os.Exit(1)
	}
	defer storage.Close()

	reader := bufio.NewReader(os.Stdin)

	if system.IsPasswordEnabled() {
		ui.ClearScreen()
		fmt.Println()
		fmt.Println("╔══════════════════════════════════════════════════════════╗")
		fmt.Println("║                    Password Required                     ║")
		fmt.Println("╚══════════════════════════════════════════════════════════╝")
		fmt.Println()
		fmt.Print("Enter password: ")
		pwd, _ := reader.ReadString('\n')
		pwd = strings.TrimSpace(pwd)
		if !system.CheckPassword(pwd) {
			ui.PrintError("Incorrect password!")
			fmt.Println("\nPress Enter to exit...")
			reader.ReadString('\n')
			return
		}
	}

	core.CheckPomodoroAndNotify()

	for {
		ui.ClearScreen()
		printMenuHeader()
		printCurrentStatus()
		printMenuOptions()

		fmt.Print("\nEnter choice (0-11): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "1":
			handleMenuStart(reader)
		case "2":
			handleMenuStop(reader)
		case "3":
			handleMenuStatus(reader)
		case "4":
			handleMenuStats(reader)
		case "5":
			handleMenuPause(reader)
		case "6":
			handleMenuResume(reader)
		case "7":
			handleFocusTools(reader)
		case "8":
			handleMenuExport(reader)
		case "9":
			handleMenuCustomize(reader)
		case "10":
			handleMenuSettings(reader)
		case "11":
			handleMenuUninstall(reader)
		case "12":
			handleMenuUpdate(reader)
		case "0":
			fmt.Println("\nGoodbye!")
			return
		default:
			fmt.Println("\n[ERROR] Invalid choice. Press Enter to continue...")
			reader.ReadString('\n')
		}
	}
}

func printMenuHeader() {
	ui.PrintMenuHeader()
	fmt.Printf("   Version: %s%s%s\n", ui.Cyan, system.Version, ui.Reset)
}

func printCurrentStatus() {
	ui.PrintSectionHeader("Status")

	if !storage.IsConsentGranted() {
		ui.PrintWarn("Not initialized - Go to Settings > Initialize")
		fmt.Println()
		return
	}

	isRunning := false
	if system.GetProcessCount(system.DaemonProcessName) > 1 {
		isRunning = true
	}

	if isRunning {
		ui.PrintStatus("Daemon", "RUNNING", true)
	} else {
		ui.PrintStatus("Daemon", "STOPPED", false)
	}

	if storage.IsPaused() {
		ui.PrintStatus("Tracking", "PAUSED", false)
	} else if isRunning {
		ui.PrintStatus("Tracking", "ACTIVE", true)
	} else {
		ui.PrintStatus("Tracking", "INACTIVE", false)
	}

	ui.PrintStatus("Retention", fmt.Sprintf("%d days", storage.GetRetentionDays()), true)

	enabled, _, _ := system.GetAutoStartEnabled()
	if enabled {
		ui.PrintStatus("Auto-start", "ENABLED", true)
	} else {
		ui.PrintStatus("Auto-start", "DISABLED", false)
	}

	fmt.Println()
}

func printMenuOptions() {
	ui.PrintSectionHeader("Menu")

	fmt.Printf("   %sTracking%s                     %sData%s\n", ui.Yellow, ui.Reset, ui.Yellow, ui.Reset)
	fmt.Printf("     %s1.%s Start                    %s3.%s View Status\n", ui.Cyan, ui.Reset, ui.Cyan, ui.Reset)
	fmt.Printf("     %s2.%s Stop                     %s4.%s View Stats\n", ui.Cyan, ui.Reset, ui.Cyan, ui.Reset)
	fmt.Println()
	fmt.Printf("   %sControl%s                      %sTools%s\n", ui.Yellow, ui.Reset, ui.Yellow, ui.Reset)
	fmt.Printf("     %s5.%s Pause                    %s8.%s Export\n", ui.Cyan, ui.Reset, ui.Cyan, ui.Reset)
	fmt.Printf("     %s6.%s Resume                   %s9.%s Customize\n", ui.Cyan, ui.Reset, ui.Cyan, ui.Reset)
	fmt.Printf("     %s7.%s Focus Tools             %s10.%s Settings\n", ui.Cyan, ui.Reset, ui.Cyan, ui.Reset)
	fmt.Println()
	fmt.Printf("   %sSystem%s\n", ui.Yellow, ui.Reset)
	fmt.Printf("     %s11.%s Uninstall               %s12.%s Check Updates\n", ui.Red, ui.Reset, ui.Cyan, ui.Reset)
	fmt.Printf("     %s 0.%s Exit\n", ui.Dim, ui.Reset)
	fmt.Println()
}

func handleMenuStart(reader *bufio.Reader) {
	ui.ClearScreen()
	ui.PrintSectionHeader("Start Tracking")
	if !storage.IsConsentGranted() {
		ui.PrintError("Not initialized. Go to Settings > Initialize first.")
		waitForEnterWithReader(reader)
		return
	}

	if system.GetProcessCount(system.DaemonProcessName) > 1 {
		ui.PrintInfo("Tracker is already running!")
	} else {
		StartDaemonProcess()
	}
	waitForEnterWithReader(reader)
}

func handleMenuStop(reader *bufio.Reader) {
	fmt.Println()
	RunStop()
	waitForEnterWithReader(reader)
}

func handleMenuStatus(reader *bufio.Reader) {
	ui.ClearScreen()
	fmt.Println()
	showStatusInMenu()
	waitForEnterWithReader(reader)
}

func handleMenuStats(reader *bufio.Reader) {
	HandleStatsMenu(reader)
}

func handleMenuPause(reader *bufio.Reader) {
	fmt.Println()
	if !storage.IsConsentGranted() {
		ui.PrintError("Not initialized. Go to Settings > Initialize first.")
		waitForEnterWithReader(reader)
		return
	}
	RunPause()
	waitForEnterWithReader(reader)
}

func handleMenuResume(reader *bufio.Reader) {
	fmt.Println()
	if !storage.IsConsentGranted() {
		ui.PrintError("Not initialized. Go to Settings > Initialize first.")
		waitForEnterWithReader(reader)
		return
	}
	RunResume()
	waitForEnterWithReader(reader)
}

func handleMenuExport(reader *bufio.Reader) {
	fmt.Println()
	if !storage.IsConsentGranted() {
		ui.PrintError("Not initialized. Go to Settings > Initialize first.")
		waitForEnterWithReader(reader)
		return
	}
	RunExport()
	waitForEnterWithReader(reader)
}

func handleMenuSettings(reader *bufio.Reader) {
	for {
		ui.ClearScreen()
		fmt.Println()
		fmt.Println("╔══════════════════════════════════════════════════════════╗")
		fmt.Println("║                       Settings                           ║")
		fmt.Println("╚══════════════════════════════════════════════════════════╝")
		fmt.Println()

		if !storage.IsConsentGranted() {
			fmt.Println("  [!] Not initialized")
			fmt.Println()
			fmt.Println("  1. Initialize focusd")
			fmt.Println()
			fmt.Println("  0. Back")
			fmt.Println()
			fmt.Print("Enter choice: ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input == "1" {
				fmt.Println()

				if err := InitLogic(true, false, false); err != nil {
					ui.PrintError(err.Error())
				} else {

				}
				waitForEnterWithReader(reader)
			}
			if input == "0" || input == "" {
				return
			}
			continue
		}

		fmt.Println("  Current Settings:")
		fmt.Printf("    Retention: %d days\n", storage.GetRetentionDays())
		enabled, _, _ := system.GetAutoStartEnabled()
		if enabled {
			fmt.Println("    Auto-start: ENABLED")
		} else {
			fmt.Println("    Auto-start: DISABLED")
		}
		pathEnabled, _ := system.GetPathEnabled()
		if pathEnabled {
			fmt.Println("    PATH: ENABLED")
		} else {
			fmt.Println("    PATH: DISABLED")
		}
		fmt.Println()
		fmt.Println("  1. Set retention days")
		fmt.Println("  2. Toggle auto-start")
		fmt.Println("  3. Toggle PATH")
		fmt.Println("  4. Clear Data")
		fmt.Println()
		fmt.Println("  0. Back")
		fmt.Println()
		fmt.Print("Enter choice: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "1":
			fmt.Print("Enter retention days (1-30): ")
			daysStr, _ := reader.ReadString('\n')
			daysStr = strings.TrimSpace(daysStr)
			if days, err := strconv.Atoi(daysStr); err == nil {
				if err := SetRetentionLogic(days); err != nil {
					ui.PrintError(err.Error())
				}
			} else {
				ui.PrintError("Invalid number.")
			}
			waitForEnterWithReader(reader)
		case "2":
			if enabled {
				if err := DisableAutostartLogic(); err != nil {
					ui.PrintError(err.Error())
				}
			} else {
				if err := EnableAutostartLogic(); err != nil {
					ui.PrintError(err.Error())
				}
			}
			waitForEnterWithReader(reader)
		case "3":
			if pathEnabled {
				if err := DisablePathLogic(); err != nil {
					ui.PrintError(err.Error())
				}
			} else {
				if err := EnablePathLogic(); err != nil {
					ui.PrintError(err.Error())
				}
			}
			waitForEnterWithReader(reader)
		case "4":
			handleClearData(reader)
		case "0", "":
			return
		}
	}
}

func handleClearData(reader *bufio.Reader) {
	for {
		ui.ClearScreen()
		fmt.Println()
		fmt.Println("─────────────────── Clear Data ───────────────────")
		fmt.Println()
		fmt.Println("  [!] Warning: Deleted data cannot be recovered!")
		fmt.Println()
		fmt.Println("  1. Clear last hour")
		fmt.Println("  2. Clear last 24 hours")
		fmt.Println("  3. Clear today's data")
		fmt.Println("  4. Clear ALL data")
		fmt.Println("  5. Clear custom hours")
		fmt.Println()
		fmt.Println("  0. Back (cancel)")
		fmt.Println()
		fmt.Print("Enter choice: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "1":
			if err := storage.ClearLastHourData(); err != nil {
				ui.PrintError(fmt.Sprintf("Failed: %v", err))
			} else {
				ui.PrintOK("Cleared last hour of data")
			}
			waitForEnterWithReader(reader)
		case "2":
			if err := storage.ClearLast24HoursData(); err != nil {
				ui.PrintError(fmt.Sprintf("Failed: %v", err))
			} else {
				ui.PrintOK("Cleared last 24 hours of data")
			}
			waitForEnterWithReader(reader)
		case "3":
			if err := storage.ClearTodayData(); err != nil {
				ui.PrintError(fmt.Sprintf("Failed: %v", err))
			} else {
				ui.PrintOK("Cleared today's data")
			}
			waitForEnterWithReader(reader)
		case "4":
			fmt.Print("Type 'yes' to confirm clearing ALL data: ")
			confirm, _ := reader.ReadString('\n')
			confirm = strings.TrimSpace(confirm)
			if strings.ToLower(confirm) == "yes" {
				if err := storage.ClearAllTrackingData(); err != nil {
					ui.PrintError(fmt.Sprintf("Failed: %v", err))
				} else {
					ui.PrintOK("Cleared all tracking data")
				}
			} else {
				ui.PrintInfo("Cancelled")
			}
			waitForEnterWithReader(reader)
		case "5":
			fmt.Print("Enter number of hours to clear: ")
			hoursStr, _ := reader.ReadString('\n')
			hoursStr = strings.TrimSpace(hoursStr)
			hours, err := strconv.Atoi(hoursStr)
			if err != nil || hours < 1 {
				ui.PrintError("Invalid number. Enter a positive number.")
			} else {
				if err := storage.ClearLastNHoursData(hours); err != nil {
					ui.PrintError(fmt.Sprintf("Failed: %v", err))
				} else {
					ui.PrintOK(fmt.Sprintf("Cleared last %d hour(s) of data", hours))
				}
			}
			waitForEnterWithReader(reader)
		case "0", "":
			return
		}
	}
}

func handleMenuCustomize(reader *bufio.Reader) {
	for {
		ui.ClearScreen()
		fmt.Println()
		fmt.Println("╔══════════════════════════════════════════════════════════╗")
		fmt.Println("║                      Customize                           ║")
		fmt.Println("╚══════════════════════════════════════════════════════════╝")
		fmt.Println()

		fmt.Println("  1. View/Add Whitelist (apps to ignore)")
		fmt.Println("  2. Manage Browsers")
		fmt.Println()
		fmt.Println("  0. Back")
		fmt.Println()
		fmt.Print("Enter choice: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "1":
			handleWhitelist(reader)
		case "2":
			handleManageBrowsers(reader)
		case "0", "":
			return
		}
	}
}

func handleManageBrowsers(reader *bufio.Reader) {
	for {
		ui.ClearScreen()
		fmt.Println()
		fmt.Println("─────────────── Manage Browsers ───────────────")
		fmt.Println()
		fmt.Println("  Add custom browsers here if they aren't detected automatically.")
		fmt.Println("  (Standard browsers like Chrome, Edge, Firefox are already supported)")
		fmt.Println()

		customs := storage.GetCustomBrowsersList()
		if len(customs) > 0 {
			fmt.Println("  User-Defined Browsers:")
			for _, b := range customs {
				fmt.Printf("    • %s\n", b)
			}
		} else {
			fmt.Println("  No user-defined browsers added.")
		}

		fmt.Println()
		fmt.Println("  1. Add custom browser")
		if len(customs) > 0 {
			fmt.Println("  2. Remove custom browser")
		}
		fmt.Println()
		fmt.Println("  0. Back")
		fmt.Println()
		fmt.Print("Enter choice: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "1":
			fmt.Print("Enter browser exe name (e.g., mybrowser.exe): ")
			name, _ := reader.ReadString('\n')
			name = strings.TrimSpace(name)
			if name == "" {
				ui.PrintError("No name entered")
				waitForEnterWithReader(reader)
				continue
			}
			if err := storage.AddCustomBrowser(name); err != nil {
				ui.PrintError(err.Error())
			} else {
				ui.PrintOK(fmt.Sprintf("Added %s", name))
			}
			waitForEnterWithReader(reader)
		case "2":
			fmt.Println("\n  Select browser to remove:")
			for i, b := range customs {
				fmt.Printf("    %d. %s\n", i+1, b)
			}
			fmt.Print("\n  Enter number (or name): ")
			name, _ := reader.ReadString('\n')
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}

			if num, err := strconv.Atoi(name); err == nil && num >= 1 && num <= len(customs) {
				toRemove := customs[num-1]
				if err := storage.RemoveCustomBrowser(toRemove); err != nil {
					ui.PrintError(err.Error())
				} else {
					ui.PrintOK(fmt.Sprintf("Removed %s", toRemove))
				}
			} else {

				if err := storage.RemoveCustomBrowser(name); err != nil {
					ui.PrintError(err.Error())
				} else {
					ui.PrintOK(fmt.Sprintf("Removed %s", name))
				}
			}
			waitForEnterWithReader(reader)
		case "0", "":
			return
		}
	}
}

func handleWhitelist(reader *bufio.Reader) {
	for {
		ui.ClearScreen()
		fmt.Println()
		fmt.Println("─────────────── Whitelist (Ignored Apps) ───────────────")
		fmt.Println()
		fmt.Println("  Apps in whitelist will NOT be tracked.")
		fmt.Println()

		whitelist := system.GetWhitelistApps()
		if len(whitelist) > 0 {
			fmt.Println("  Currently whitelisted:")
			for _, a := range whitelist {
				fmt.Printf("    • %s\n", a)
			}
		} else {
			fmt.Println("  No apps whitelisted yet.")
		}

		fmt.Println()
		fmt.Println("  1. Add app to whitelist")
		if len(whitelist) > 0 {
			fmt.Println("  2. Remove app from whitelist")
		}
		fmt.Println()
		fmt.Println("  0. Back")
		fmt.Println()
		fmt.Print("Enter choice: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "1":
			fmt.Print("Enter app exe name to ignore (e.g., discord or discord.exe): ")
			name, _ := reader.ReadString('\n')
			name = strings.TrimSpace(name)
			if name == "" {
				ui.PrintError("No name entered")
				waitForEnterWithReader(reader)
				continue
			}
			system.AddWhitelistApp(name)
			ui.PrintOK(fmt.Sprintf("Added %s to whitelist - will not be tracked", name))
			waitForEnterWithReader(reader)
		case "2":
			if len(whitelist) == 0 {
				ui.PrintError("No apps in whitelist to remove")
				waitForEnterWithReader(reader)
				continue
			}
			fmt.Println("\n  Select app to remove:")
			for i, a := range whitelist {
				fmt.Printf("    %d. %s\n", i+1, a)
			}
			fmt.Print("\n  Enter number (or name): ")
			name, _ := reader.ReadString('\n')
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if num, err := strconv.Atoi(name); err == nil && num >= 1 && num <= len(whitelist) {
				removed := whitelist[num-1]
				system.RemoveWhitelistApp(removed)
				ui.PrintOK(fmt.Sprintf("Removed %s from whitelist", removed))
			} else {
				found := false
				for _, a := range whitelist {
					if strings.EqualFold(a, name) || strings.EqualFold(a, name+".exe") {
						system.RemoveWhitelistApp(a)
						ui.PrintOK(fmt.Sprintf("Removed %s from whitelist", a))
						found = true
						break
					}
				}
				if !found {
					ui.PrintError(fmt.Sprintf("'%s' not found in whitelist", name))
				}
			}
			waitForEnterWithReader(reader)
		case "0", "":
			return
		}
	}
}

func handleFocusTools(reader *bufio.Reader) {
	for {
		ui.ClearScreen()
		ui.PrintLogo()
		ui.PrintSectionHeader("Focus Tools")

		pomodoroActive, remaining, _ := core.GetPomodoroStatus()
		if pomodoroActive {
			if remaining > 0 {
				ui.PrintStatus("Pomodoro", fmt.Sprintf("%d min remaining", int(remaining.Minutes())+1), true)
			} else {
				ui.PrintStatus("Pomodoro", "COMPLETE!", true)
			}
		}

		if system.GetBreakReminderEnabled() {
			ui.PrintStatus("Break Reminder", fmt.Sprintf("Every %d min", system.GetBreakReminderMinutes()), true)
		}

		limits := system.GetAppTimeLimits()
		if len(limits) > 0 {
			ui.PrintStatus("App Limits", fmt.Sprintf("%d apps", len(limits)), true)
		}

		ui.PrintStatus("Snooze Duration", fmt.Sprintf("%d min", system.GetSnoozeDurationMinutes()), false)

		fmt.Println()
		fmt.Printf("     %s1.%s Pomodoro Timer\n", ui.Cyan, ui.Reset)
		fmt.Printf("     %s2.%s Break Reminder\n", ui.Cyan, ui.Reset)
		fmt.Printf("     %s3.%s App Time Limits\n", ui.Cyan, ui.Reset)
		fmt.Printf("     %s4.%s Password Settings\n", ui.Cyan, ui.Reset)
		fmt.Printf("     %s5.%s Snooze Duration\n", ui.Cyan, ui.Reset)
		fmt.Println()
		fmt.Printf("     %s0.%s Back\n", ui.Dim, ui.Reset)
		fmt.Println()
		fmt.Print("   Enter choice: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "1":
			handlePomodoro(reader)
		case "2":
			handleBreakReminder(reader)
		case "3":
			handleAppTimeLimits(reader)
		case "4":
			handlePasswordSettings(reader)
		case "5":
			handleSnoozeDuration(reader)
		case "0", "":
			return
		}
	}
}

func handlePomodoro(reader *bufio.Reader) {
	for {
		ui.ClearScreen()
		fmt.Println()
		fmt.Println("─────────────────── Pomodoro Timer ───────────────────")
		fmt.Println()

		active, remaining, total := core.GetPomodoroStatus()
		if active {
			if remaining > 0 {
				fmt.Printf("  Status: RUNNING (%d of %d min remaining)\n", int(remaining.Minutes())+1, total)
			} else {
				fmt.Println("  Status: COMPLETE! Take a break!")
				core.ShowNotification("Pomodoro Complete!", "Great work! Take a break.")
				core.StopPomodoro()
				ui.PrintOK("Pomodoro completed and reset!")
				waitForEnterWithReader(reader)
				continue
			}
		} else {
			fmt.Printf("  Status: Not running (default %d min)\n", system.GetPomodoroMinutes())
		}

		fmt.Println()
		fmt.Println("  1. Start Pomodoro")
		fmt.Println("  2. Stop Pomodoro")
		fmt.Println("  3. Set Duration")
		fmt.Println()
		fmt.Println("  0. Back")
		fmt.Println()
		fmt.Print("Enter choice: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "1":
			core.StartPomodoro(system.GetPomodoroMinutes())
			ui.PrintOK("Pomodoro started!")
			waitForEnterWithReader(reader)
		case "2":
			core.StopPomodoro()
			ui.PrintOK("Pomodoro stopped")
			waitForEnterWithReader(reader)
		case "3":
			fmt.Print("Enter duration in minutes: ")
			mins, _ := reader.ReadString('\n')
			mins = strings.TrimSpace(mins)
			if m, err := strconv.Atoi(mins); err == nil && m > 0 {
				system.SetPomodoroMinutes(m)
				ui.PrintOK(fmt.Sprintf("Pomodoro duration set to %d min", m))
			}
			waitForEnterWithReader(reader)
		case "0", "":
			return
		}
	}
}

func handleBreakReminder(reader *bufio.Reader) {
	for {
		ui.ClearScreen()
		fmt.Println()
		fmt.Println("─────────────────── Break Reminder ───────────────────")
		fmt.Println()

		if system.GetBreakReminderEnabled() {
			fmt.Printf("  Status: ENABLED (every %d min)\n", system.GetBreakReminderMinutes())
		} else {
			fmt.Println("  Status: DISABLED")
		}

		fmt.Println()
		fmt.Println("  1. Enable break reminder")
		fmt.Println("  2. Disable break reminder")
		fmt.Println("  3. Set reminder interval")
		fmt.Println()
		fmt.Println("  0. Back")
		fmt.Println()
		fmt.Print("Enter choice: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "1":
			system.SetBreakReminder(true, system.GetBreakReminderMinutes())
			ui.PrintOK("Break reminder enabled")
			waitForEnterWithReader(reader)
		case "2":
			system.SetBreakReminder(false, 0)
			ui.PrintOK("Break reminder disabled")
			waitForEnterWithReader(reader)
		case "3":
			fmt.Print("Remind after how many minutes: ")
			mins, _ := reader.ReadString('\n')
			mins = strings.TrimSpace(mins)
			if m, err := strconv.Atoi(mins); err == nil && m > 0 {
				system.SetBreakReminder(true, m)
				ui.PrintOK(fmt.Sprintf("Will remind after %d min of continuous use", m))
			}
			waitForEnterWithReader(reader)
		case "0", "":
			return
		}
	}
}

func handleAppTimeLimits(reader *bufio.Reader) {
	for {
		ui.ClearScreen()
		fmt.Println()
		fmt.Println("─────────────────── App Time Limits ───────────────────")
		fmt.Println()
		fmt.Println("  Set daily time limits per app. Notification shows once per day.")
		fmt.Println()

		limits := system.GetAppTimeLimits()
		if len(limits) > 0 {
			fmt.Println("  Current limits:")
			for app, mins := range limits {
				fmt.Printf("    • %s: %d min/day\n", app, mins)
			}
		} else {
			fmt.Println("  No limits set.")
		}

		fmt.Println()
		fmt.Println("  1. Add app limit")
		fmt.Println("  2. Remove app limit")
		fmt.Println()
		fmt.Println("  0. Back")
		fmt.Println()
		fmt.Print("Enter choice: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "1":
			fmt.Print("Enter app exe name (e.g., discord or discord.exe): ")
			name, _ := reader.ReadString('\n')
			name = strings.TrimSpace(name)
			if name == "" {
				ui.PrintError("No name entered")
				waitForEnterWithReader(reader)
				continue
			}
			fmt.Print("Daily limit in minutes: ")
			mins, _ := reader.ReadString('\n')
			mins = strings.TrimSpace(mins)
			if m, err := strconv.Atoi(mins); err == nil && m > 0 {
				system.SetAppTimeLimit(name, m)
				ui.PrintOK(fmt.Sprintf("Limit set: %s max %d min/day", name, m))
			} else {
				ui.PrintError("Invalid minutes. Enter a positive number.")
			}
			waitForEnterWithReader(reader)
		case "2":
			if len(limits) == 0 {
				ui.PrintError("No app limits to remove")
				waitForEnterWithReader(reader)
				continue
			}
			fmt.Println("\n  Select app limit to remove:")
			apps := make([]string, 0, len(limits))
			i := 1
			for app, mins := range limits {
				fmt.Printf("    %d. %s (%d min/day)\n", i, app, mins)
				apps = append(apps, app)
				i++
			}
			fmt.Print("\n  Enter number (or name): ")
			name, _ := reader.ReadString('\n')
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if num, err := strconv.Atoi(name); err == nil && num >= 1 && num <= len(apps) {
				removed := apps[num-1]
				system.RemoveAppTimeLimit(removed)
				ui.PrintOK(fmt.Sprintf("Removed limit for %s", removed))
			} else {
				found := false
				normalizedName := strings.ToLower(name)
				if !strings.HasSuffix(normalizedName, ".exe") {
					normalizedName += ".exe"
				}
				if _, ok := limits[normalizedName]; ok {
					system.RemoveAppTimeLimit(normalizedName)
					ui.PrintOK(fmt.Sprintf("Removed limit for %s", normalizedName))
					found = true
				}
				if !found {
					ui.PrintError(fmt.Sprintf("'%s' not found in app limits", name))
				}
			}
			waitForEnterWithReader(reader)
		case "0", "":
			return
		}
	}
}

func handlePasswordSettings(reader *bufio.Reader) {
	for {
		ui.ClearScreen()
		fmt.Println()
		fmt.Println("─────────────────── Password Settings ───────────────────")
		fmt.Println()

		if system.IsPasswordEnabled() {
			fmt.Println("  Status: PASSWORD SET")
		} else {
			fmt.Println("  Status: No password")
		}

		fmt.Println()
		fmt.Println("  1. Set password")
		fmt.Println("  2. Remove password")
		fmt.Println()
		fmt.Println("  0. Back")
		fmt.Println()
		fmt.Print("Enter choice: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "1":
			fmt.Print("Enter new password: ")
			pwd, _ := reader.ReadString('\n')
			pwd = strings.TrimSpace(pwd)
			if pwd != "" {
				system.SetPassword(pwd)
				ui.PrintOK("Password set! Menu will now require password.")
			}
			waitForEnterWithReader(reader)
		case "2":
			if system.IsPasswordEnabled() {
				fmt.Print("Enter current password to remove: ")
				pwd, _ := reader.ReadString('\n')
				pwd = strings.TrimSpace(pwd)
				if system.CheckPassword(pwd) {
					system.SetPassword("")
					ui.PrintOK("Password removed")
				} else {
					ui.PrintError("Incorrect password")
				}
			} else {
				ui.PrintInfo("No password set")
			}
			waitForEnterWithReader(reader)
		case "0", "":
			return
		}
	}
}

func handleMenuUninstall(reader *bufio.Reader) {
	fmt.Println()
	RunUninstall()
	waitForEnterWithReader(reader)
}

func waitForEnterWithReader(reader *bufio.Reader) {
	fmt.Print("\nPress Enter to continue...")
	reader.ReadString('\n')
}

func showStatusInMenu() {
	ui.PrintHeader()

	isRunning := false
	if system.GetProcessCount(system.DaemonProcessName) > 1 {
		isRunning = true
	}

	if isRunning {
		ui.PrintOK("Daemon: RUNNING")
	} else {
		ui.PrintWarn("Daemon: STOPPED")
	}

	if storage.IsPaused() {
		ui.PrintWarn("Tracking: PAUSED")
	} else if isRunning {
		ui.PrintOK("Tracking: ACTIVE")
	} else {
		ui.PrintInfo("Tracking: INACTIVE")
	}

	fmt.Println()
	fmt.Printf("Retention: %d days\n", storage.GetRetentionDays())

	enabled, _, _ := system.GetAutoStartEnabled()
	if enabled {
		fmt.Println("Auto-start: ENABLED")
	} else {
		fmt.Println("Auto-start: DISABLED")
	}
}

func showStatsInMenu() {
	ui.PrintHeader()

	today := storage.Today()
	apps, _ := storage.GetAppStatsForDate(today)

	if len(apps) == 0 {
		ui.PrintInfo("No data recorded yet. Start tracking first.")
		return
	}

	ui.PrintSectionHeader("Apps (Today)")
	columns := []ui.TableColumn{
		{Header: "App", Width: 53},
		{Header: "Time", Width: 10},
		{Header: "Opens", Width: 6},
	}
	var rows [][]string
	for _, app := range apps {
		rows = append(rows, []string{
			ui.TruncateString(app.AppName, 53),
			ui.FormatDurationShort(app.TotalDurationSecs),
			fmt.Sprintf("%d", app.OpenCount),
		})
	}
	ui.PrintTable(columns, rows)
}

func handleMenuUpdate(reader *bufio.Reader) {
	ui.ClearScreen()
	ui.PrintSectionHeader("Update Check")
	RunUpdate()
	waitForEnterWithReader(reader)
}

func handleSnoozeDuration(reader *bufio.Reader) {
	ui.ClearScreen()
	fmt.Println()
	fmt.Println("─────────────────── Snooze Duration ───────────────────")
	fmt.Println()
	fmt.Printf("  Current: %d minutes\n", system.GetSnoozeDurationMinutes())
	fmt.Println()
	fmt.Println("  When you click OK on a notification, it will be")
	fmt.Println("  snoozed for this duration before showing again.")
	fmt.Println()
	fmt.Print("  Enter new duration in minutes (or press Enter to cancel): ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return
	}
	if mins, err := strconv.Atoi(input); err == nil && mins > 0 {
		system.SetSnoozeDurationMinutes(mins)
		ui.PrintOK(fmt.Sprintf("Snooze duration set to %d minutes", mins))
	} else {
		ui.PrintError("Invalid number. Enter a positive number.")
	}
	waitForEnterWithReader(reader)
}
