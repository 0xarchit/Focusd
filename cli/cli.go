package cli

import (
	"fmt"
	"focusd/system"
	"os"
)

func printHelp() {
	fmt.Println()
	fmt.Println("focusd - Privacy-first digital wellbeing tracker")
	fmt.Printf("Version %s\n", system.Version)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  focusd start              Start tracking (background)")
	fmt.Println("  focusd stop               Stop tracking")
	fmt.Println()
	fmt.Println("Setup:")
	fmt.Println("  focusd update             Check for updates")
	fmt.Println()
	fmt.Println("Viewing Data:")
	fmt.Println("  focusd status    (s)      Show tracking status")
	fmt.Println("  focusd stats     (st)     Detailed usage breakdown")
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

func printVersion() {
	fmt.Printf("focusd version %s\n", system.Version)
}

func Run(args []string) {
	if len(args) < 2 {
		runInteractiveMenu()
		return
	}

	command := args[1]

	switch command {
	case "update":
		runUpdate()
	case "focus":
		runFocus(args)
	case "stop-timer":
		runStopTimer()
	case "limit":
		runLimits(args)
	case "start":
		runStart()
	case "stop":
		runStop()
	case "--daemon":
		RunDaemon()
	case "status", "s":
		runStatus()
	case "stats", "st":
		runStats()
	case "pause", "p":
		runPause()
	case "resume", "r":
		runResume()
	case "retention", "ret":
		handleRetention(args)
	case "autostart", "auto":
		handleAutostart(args)
	case "path":
		handlePath(args)
	case "browser":
		handleBrowsersCommand(args)
	case "help", "-h", "--help", "h":
		printHelp()
	case "version", "-v", "--version":
		printVersion()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Run 'focusd help' for usage information.")
		os.Exit(1)
	}
}

// Switch-based command dispatching is preferred here over map-based lookup
// to keep argument index parsing simple, static, and extremely transparent
// without needing extra struct wrapping or interface reflection.
func handleRetention(args []string) {
	if len(args) < 3 {
		runRetentionStatus()
		return
	}
	switch args[2] {
	case "status":
		runRetentionStatus()
	case "set":
		if len(args) < 4 {
			fmt.Println("Usage: focusd retention set <days>")
			os.Exit(1)
		}
		runRetentionSet(args[3])
	case "reset":
		runRetentionReset()
	default:
		fmt.Printf("Unknown retention command: %s\n", args[2])
		os.Exit(1)
	}
}

func handleAutostart(args []string) {
	if len(args) < 3 {
		runAutostartStatus()
		return
	}
	switch args[2] {
	case "enable":
		runAutostartEnable()
	case "disable":
		runAutostartDisable()
	case "status":
		runAutostartStatus()
	default:
		fmt.Printf("Unknown autostart command: %s\n", args[2])
		os.Exit(1)
	}
}

func handlePath(args []string) {
	if len(args) < 3 {
		runPathStatus()
		return
	}
	switch args[2] {
	case "enable":
		runPathEnable()
	case "disable":
		runPathDisable()
	case "status":
		runPathStatus()
	default:
		fmt.Printf("Unknown path command: %s\n", args[2])
		os.Exit(1)
	}
}
