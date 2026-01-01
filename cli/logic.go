package cli

import (
	"fmt"
	"focusd/storage"
	"focusd/system"
	"focusd/ui"
	"os"
	"strings"
)

func SetRetentionLogic(days int) error {
	if !storage.IsConsentGranted() {
		return fmt.Errorf("not initialized")
	}

	if currentExe, err := os.Executable(); err == nil {
		installedExe := system.GetInstalledExePath()
		if currentExe != installedExe {
			ui.PrintInfo("Updating installed binary...")
		}
	}

	ui.PrintOK(fmt.Sprintf("Retention set to %d days.", days))
	return nil
}

func EnablePathLogic() error {
	if !storage.IsConsentGranted() {
		return fmt.Errorf("not initialized")
	}

	if currentExe, err := os.Executable(); err == nil {
		installedExe := system.GetInstalledExePath()
		if currentExe != installedExe {
			ui.PrintInfo("Updating installed binary...")
		}
	}

	if err := system.EnablePath(); err != nil {
		return fmt.Errorf("failed to add to PATH: %w", err)
	}

	storage.SetConfig(storage.ConfigKeyPathEnabled, "true")
	ui.PrintOK("Added to user PATH.")
	ui.PrintWarn("Restart your terminal for changes to take effect.")
	return nil
}

func DisablePathLogic() error {
	if err := system.DisablePath(); err != nil {
		return fmt.Errorf("failed to remove from PATH: %w", err)
	}

	storage.SetConfig(storage.ConfigKeyPathEnabled, "false")
	ui.PrintOK("Removed from user PATH.")
	ui.PrintWarn("Restart your terminal for changes to take effect.")
	return nil
}

func EnableAutostartLogic() error {
	if !storage.IsConsentGranted() {
		return fmt.Errorf("not initialized")
	}

	if currentExe, err := os.Executable(); err == nil {
		installedExe := system.GetInstalledExePath()
		if currentExe != installedExe {
			ui.PrintInfo("Updating installed binary...")
		}
	}

	if err := system.EnableAutoStart(); err != nil {
		return fmt.Errorf("failed to enable auto-start: %w", err)
	}

	storage.SetConfig(storage.ConfigKeyAutostart, "true")
	ui.PrintOK("Auto-start enabled.")
	fmt.Println("focusd will start automatically on Windows boot.")
	fmt.Println("Visible in Task Manager → Startup tab.")
	return nil
}

func InitLogic(consent bool, autoStart bool, path bool) error {

	if consent {
		if err := storage.SetConsent(true); err != nil {
			return fmt.Errorf("failed to grant consent: %w", err)
		}

		if err := system.InstallExes(); err != nil {
			return fmt.Errorf("failed to install binaries: %w", err)
		}
		ui.PrintOK("Consent granted.")

		currentExe, _ := os.Executable()
		installedExe := system.GetInstalledExePath()

		if currentExe != "" && installedExe != "" && !strings.EqualFold(currentExe, installedExe) {
			ui.PrintInfo("focusd installed to: " + installedExe)
			ui.PrintWarn(fmt.Sprintf("You can now delete this setup file: %s", currentExe))
		}

	} else {
		if err := storage.SetConsent(false); err != nil {
			return fmt.Errorf("failed to revoke consent: %w", err)
		}
		ui.PrintOK("Consent revoked.")
	}

	if autoStart {
		if err := EnableAutostartLogic(); err != nil {
			return fmt.Errorf("failed to enable auto-start during init: %w", err)
		}
	} else {
		if err := DisableAutostartLogic(); err != nil {
			return fmt.Errorf("failed to disable auto-start during init: %w", err)
		}
	}

	if path {
		if err := EnablePathLogic(); err != nil {
			return fmt.Errorf("failed to enable path during init: %w", err)
		}
	} else {
		if err := DisablePathLogic(); err != nil {
			return fmt.Errorf("failed to disable path during init: %w", err)
		}
	}

	ui.PrintOK("Initialization complete.")
	return nil
}

func DisableAutostartLogic() error {
	if err := system.DisableAutoStart(); err != nil {
		return fmt.Errorf("failed to disable auto-start: %w", err)
	}

	storage.SetConfig(storage.ConfigKeyAutostart, "false")
	ui.PrintOK("Auto-start disabled.")
	return nil
}
