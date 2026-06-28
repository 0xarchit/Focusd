package cli

import (
	"fmt"
	"focusd/storage"
	"focusd/system"
	"focusd/ui"
	"log"
	"os"
)

func SetRetentionLogic(days int) error {
	if currentExe, err := os.Executable(); err == nil {
		installedExe := system.GetInstalledExePath()
		if currentExe != installedExe {
			ui.PrintInfo("Updating installed binary...")
		}
	}

	if err := storage.SetRetentionDays(days); err != nil {
		return fmt.Errorf("failed to save retention days: %w", err)
	}

	ui.PrintOK(fmt.Sprintf("Retention set to %d days.", days))
	return nil
}

func EnablePathLogic() error {
	if currentExe, err := os.Executable(); err == nil {
		installedExe := system.GetInstalledExePath()
		if currentExe != installedExe {
			ui.PrintInfo("Updating installed binary...")
		}
	}

	if err := system.EnablePath(); err != nil {
		return fmt.Errorf("failed to add to PATH: %w", err)
	}

	if err := storage.SetConfig(storage.ConfigKeyPathEnabled, "true"); err != nil {
		log.Printf("WARN: failed to persist PATH-enabled flag: %v", err)
	}
	ui.PrintOK("Added to user PATH.")
	ui.PrintWarn("Restart your terminal for changes to take effect.")
	return nil
}

func DisablePathLogic() error {
	if err := system.DisablePath(); err != nil {
		return fmt.Errorf("failed to remove from PATH: %w", err)
	}

	if err := storage.SetConfig(storage.ConfigKeyPathEnabled, "false"); err != nil {
		log.Printf("WARN: failed to persist PATH-disabled flag: %v", err)
	}
	ui.PrintOK("Removed from user PATH.")
	ui.PrintWarn("Restart your terminal for changes to take effect.")
	return nil
}

func EnableAutostartLogic() error {
	if currentExe, err := os.Executable(); err == nil {
		installedExe := system.GetInstalledExePath()
		if currentExe != installedExe {
			ui.PrintInfo("Updating installed binary...")
		}
	}

	if err := system.EnableAutoStart(); err != nil {
		return fmt.Errorf("failed to enable auto-start: %w", err)
	}

	if err := storage.SetConfig(storage.ConfigKeyAutostart, "true"); err != nil {
		log.Printf("WARN: failed to persist autostart-enabled flag: %v", err)
	}
	ui.PrintOK("Auto-start enabled.")
	fmt.Println("focusd will start automatically on Windows boot.")
	fmt.Println("Visible in Task Manager → Startup tab.")
	return nil
}

func DisableAutostartLogic() error {
	if err := system.DisableAutoStart(); err != nil {
		return fmt.Errorf("failed to disable auto-start: %w", err)
	}

	if err := storage.SetConfig(storage.ConfigKeyAutostart, "false"); err != nil {
		log.Printf("WARN: failed to persist autostart-disabled flag: %v", err)
	}
	ui.PrintOK("Auto-start disabled.")
	return nil
}
