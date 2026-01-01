package cli

import (
	"fmt"
	"focusd/system"
	"os"
)

func PrintHelp() {
	fmt.Println()
	fmt.Println("focusd - Privacy-first digital wellbeing tracker")
	fmt.Printf("Version %s\n", system.Version)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  focusd start              Start tracking (background)")
	fmt.Println("  focusd stop               Stop tracking")
	fmt.Println()
	fmt.Println("Setup:")
	fmt.Println("  focusd init      (i)      Initialize and grant consent")
	fmt.Println("  focusd update             Check for updates")
	fmt.Println("  focusd uninstall          Remove focusd completely")
	fmt.Println()
	fmt.Println("Viewing Data:")
	fmt.Println("  focusd status    (s)      Show tracking status")
	fmt.Println("  focusd stats     (st)     Detailed usage breakdown")
	fmt.Println("  focusd export    (e)      Export data to CSV")
	fmt.Println()
	fmt.Println("Tracking Control:")
	fmt.Println("  focusd pause     (p)      Pause tracking")
	fmt.Println("  focusd resume    (r)      Resume tracking")
	fmt.Println("  focusd focus [min]        Start Pomodoro timer")
	fmt.Println("  focusd limit [app] [min]  Set daily app limit")
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Println("  focusd retention (ret)    Show/set retention days")
	fmt.Println("  focusd autostart (auto)   Manage auto-start")
	fmt.Println("  focusd path               Manage PATH integration")
	fmt.Println("  focusd browser            Manage custom browsers")
	fmt.Println()
	fmt.Println("Other:")
	fmt.Println("  focusd help      (h)      Show this help message")
	fmt.Println("  focusd version   (-v)     Show version")
	fmt.Println()
}

func PrintVersion() {
	fmt.Printf("focusd version %s\n", system.Version)
}

func Run(args []string) {
	if len(args) < 2 {
		RunInteractiveMenu()
		return
	}

	command := args[1]

	switch command {
	case "init", "i":
		RunInit()
	case "update":
		RunUpdate()
	case "focus":
		RunFocus(args)
	case "stop-timer":
		RunStopTimer()
	case "limit":
		RunLimits(args)
	case "start":
		RunStart()
	case "stop":
		RunStop()
	case "--daemon":
		RunDaemon()
	case "status", "s":
		RunStatus()
	case "stats", "st":
		RunStats()
	case "pause", "p":
		RunPause()
	case "resume", "r":
		RunResume()
	case "retention", "ret":
		handleRetention(args)
	case "autostart", "auto":
		handleAutostart(args)
	case "path":
		handlePath(args)
	case "browser":
		HandleBrowsersCommand(args)
	case "export", "e":
		RunExport()
	case "uninstall":
		RunUninstall()
	case "help", "-h", "--help", "h":
		PrintHelp()
	case "version", "-v", "--version":
		PrintVersion()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Run 'focusd help' for usage information.")
		os.Exit(1)
	}
}

func handleRetention(args []string) {
	if len(args) < 3 {
		RunRetentionStatus()
		return
	}
	switch args[2] {
	case "status":
		RunRetentionStatus()
	case "set":
		if len(args) < 4 {
			fmt.Println("Usage: focusd retention set <days>")
			os.Exit(1)
		}
		RunRetentionSet(args[3])
	case "reset":
		RunRetentionReset()
	default:
		fmt.Printf("Unknown retention command: %s\n", args[2])
		os.Exit(1)
	}
}

func handleAutostart(args []string) {
	if len(args) < 3 {
		RunAutostartStatus()
		return
	}
	switch args[2] {
	case "enable":
		RunAutostartEnable()
	case "disable":
		RunAutostartDisable()
	case "status":
		RunAutostartStatus()
	default:
		fmt.Printf("Unknown autostart command: %s\n", args[2])
		os.Exit(1)
	}
}

func handlePath(args []string) {
	if len(args) < 3 {
		RunPathStatus()
		return
	}
	switch args[2] {
	case "enable":
		RunPathEnable()
	case "disable":
		RunPathDisable()
	case "status":
		RunPathStatus()
	default:
		fmt.Printf("Unknown path command: %s\n", args[2])
		os.Exit(1)
	}
}
