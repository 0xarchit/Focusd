package cli

import (
	"fmt"
	"focusd/system"
	"focusd/ui"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

func handleSettingsCommand(args []string) {
	if err := settingsCommand(args[1:], os.Stdout); err != nil {
		ui.PrintError(err.Error())
		os.Exit(1)
	}
}

func settingsCommand(args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("configuration command is required")
	}
	command := args[0]
	if command == "display" || command == "site" {
		return labelSettingsCommand(args, out)
	}
	if len(args) == 1 || len(args) == 2 && args[1] == "status" {
		switch command {
		case "privacy":
			_, err := fmt.Fprintf(out, "Privacy mode: %s\n", system.GetPrivacyMode())
			return err
		case "idle":
			_, err := fmt.Fprintf(out, "Idle threshold: %d minutes (0 disables the cutoff)\n", system.GetIdleThresholdMinutes())
			return err
		case "track-while-idle":
			_, err := fmt.Fprintf(out, "Track while idle: %t\n", system.GetTrackWhileIdle())
			return err
		}
	}
	var err error
	switch command {
	case "privacy":
		if len(args) != 3 || args[1] != "set" {
			return fmt.Errorf("usage: focusd privacy [status|set <titles|categories|strict>]")
		}
		err = system.SetPrivacyMode(args[2])
	case "idle":
		if len(args) != 3 || args[1] != "set" {
			return fmt.Errorf("usage: focusd idle [status|set <minutes>]")
		}
		minutes, parseErr := strconv.Atoi(args[2])
		if parseErr != nil || minutes < system.MinIdleThresholdMinutes || minutes > system.MaxIdleThresholdMinutes {
			return fmt.Errorf("idle threshold must be %d-%d minutes", system.MinIdleThresholdMinutes, system.MaxIdleThresholdMinutes)
		}
		err = system.SetIdleThreshold(minutes)
	case "track-while-idle":
		if len(args) != 2 || args[1] != "enable" && args[1] != "disable" {
			return fmt.Errorf("usage: focusd track-while-idle [status|enable|disable]")
		}
		err = system.SetTrackWhileIdle(args[1] == "enable")
	default:
		return fmt.Errorf("unknown configuration command: %s", command)
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, "Setting saved.")
	return err
}

func labelSettingsCommand(args []string, out io.Writer) error {
	command := args[0]
	if len(args) == 1 || len(args) == 2 && args[1] == "list" {
		values := system.GetDisplayOverrides()
		if command == "site" {
			values = system.GetSiteOverrides()
		}
		keys := make([]string, 0, len(values))
		for key := range values {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if len(keys) == 0 {
			_, err := fmt.Fprintln(out, "No labels configured.")
			return err
		}
		for _, key := range keys {
			if _, err := fmt.Fprintf(out, "%s: %s\n", key, values[key]); err != nil {
				return err
			}
		}
		return nil
	}
	if len(args) != 3 && len(args) != 4 {
		return fmt.Errorf("usage: focusd %s [list|set <key> <label>|remove <key>]", command)
	}
	key := strings.TrimSpace(args[2])
	if key == "" {
		return fmt.Errorf("label key is required")
	}
	label := ""
	switch args[1] {
	case "set":
		if len(args) != 4 || strings.TrimSpace(args[3]) == "" {
			return fmt.Errorf("usage: focusd %s set <key> <label>", command)
		}
		label = args[3]
	case "remove":
		if len(args) != 3 {
			return fmt.Errorf("usage: focusd %s remove <key>", command)
		}
	default:
		return fmt.Errorf("unknown %s command: %s", command, args[1])
	}
	var err error
	if command == "display" {
		err = system.SetDisplayOverride(key, label)
	} else {
		err = system.SetSiteOverride(key, label)
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, "Label saved.")
	return err
}
