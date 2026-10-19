package system

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	envKeyPath = `Environment`
	appName    = "focusd"

	// pathBackupValue holds the PATH from before the last focusd edit so a user
	// can restore it by hand after a bad write.
	pathBackupValue = "Path.focusd-backup"

	// toastAppIDKeyBase is where Windows looks up toast-capable application
	// identities. Toasts from an AUMID that is neither in this tree nor backed
	// by a Start Menu shortcut are dropped by the shell without any error.
	toastAppIDKeyBase = `Software\Classes\AppUserModelId`
)

const (
	hwndBroadcast    = 0xffff
	wmSettingChange  = 0x001A
	smtoAbortIfHung  = 0x0002
	broadcastTimeout = 3000
)

var (
	user32dll              = windows.NewLazySystemDLL("user32.dll")
	procSendMessageW       = user32dll.NewProc("SendMessageW")
	procSendMessageTimeout = user32dll.NewProc("SendMessageTimeoutW")
)

// broadcastEnvironmentChange tells running shells that the environment changed.
// Without this, shells keep the old PATH until restart. The installer already
// broadcast this; the Go path did not.
func broadcastEnvironmentChange() {
	envMsg, err := windows.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}
	var result uintptr
	procSendMessageTimeout.Call(
		hwndBroadcast,
		wmSettingChange,
		0,
		uintptr(unsafe.Pointer(envMsg)),
		smtoAbortIfHung,
		broadcastTimeout,
		0,
		uintptr(unsafe.Pointer(&result)),
	)
}

const startupShortcutName = "Focus Daemon.lnk"

func getStartupLinkPath() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return ""
	}
	return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup", startupShortcutName)
}

func GetAutoStartEnabled() (bool, string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err == nil {
		defer key.Close()
		if val, _, err := key.GetStringValue(appName); err == nil {
			return true, val, nil
		}
	}

	linkPath := getStartupLinkPath()
	if linkPath != "" {
		if _, err := os.Stat(linkPath); err == nil {
			return true, "Startup Folder: " + linkPath, nil
		}
	}

	return false, "", nil
}

func EnableAutoStart() error {
	if err := installExes(); err != nil {
		return fmt.Errorf("failed to install: %w", err)
	}

	linkPath := getStartupLinkPath()
	if linkPath != "" {
		if err := os.Remove(linkPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove legacy startup shortcut: %w", err)
		}
	}

	daemonPath := getInstalledDaemonPath()
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open registry run key: %w", err)
	}
	defer key.Close()

	val := fmt.Sprintf(`"%s"`, daemonPath)
	if err := key.SetStringValue(appName, val); err != nil {
		return fmt.Errorf("failed to write registry run value: %w", err)
	}

	return nil
}

func DisableAutoStart() error {
	linkPath := getStartupLinkPath()
	if linkPath != "" {
		if err := os.Remove(linkPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove legacy startup shortcut: %w", err)
		}
	}
	return disableRegistryAutoStart()
}

func disableRegistryAutoStart() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return nil
		}
		return err
	}
	defer key.Close()
	err = key.DeleteValue(appName)
	if err == registry.ErrNotExist {
		return nil
	}
	return err
}

// EnsureToastAppID registers an AppUserModelID so the shell displays focusd's
// toasts. Without this, Show() succeeds and the notification platform even
// logs delivery, but no banner or Action Center entry ever appears.
func EnsureToastAppID(appID, displayName, iconPath string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, toastAppIDKeyBase+`\`+appID, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to register toast app id: %w", err)
	}
	defer key.Close()

	if err := key.SetStringValue("DisplayName", displayName); err != nil {
		return fmt.Errorf("failed to write toast display name: %w", err)
	}
	if iconPath != "" {
		if err := key.SetStringValue("IconUri", iconPath); err != nil {
			log.Printf("WARN: failed to write toast icon path: %v", err)
		}
	}
	return nil
}

// readUserPath returns the current user PATH plus its registry value type.
// GetStringValue accepts both REG_SZ and REG_EXPAND_SZ and returns the raw
// unexpanded text, so %VAR% entries are preserved rather than flattened.
func readUserPath(key registry.Key) (string, uint32, error) {
	val, valtype, err := key.GetStringValue("Path")
	if err == registry.ErrNotExist {
		return "", registry.SZ, nil
	}
	return val, valtype, err
}

// setUserPathValue writes PATH back with the value type it already had. A
// REG_EXPAND_SZ PATH rewritten as REG_SZ stops expanding %VAR% entries.
func setUserPathValue(key registry.Key, value string, valtype uint32) error {
	if valtype == registry.EXPAND_SZ {
		return key.SetExpandStringValue("Path", value)
	}
	return key.SetStringValue("Path", value)
}

// writeUserPath stores PATH, backing up the previous value first, and refuses
// any result that would drop a pre-existing entry.
func writeUserPath(key registry.Key, value string, previous string, valtype uint32, removedEntry string) error {
	if !pathWriteAllowed(previous, value, removedEntry) {
		return fmt.Errorf("refusing to write PATH: would drop entries present before")
	}

	if previous != "" {
		if err := key.SetExpandStringValue(pathBackupValue, previous); err != nil {
			log.Printf("WARN: failed to write PATH backup: %v", err)
		}
	}

	if err := setUserPathValue(key, value, valtype); err != nil {
		return err
	}
	broadcastEnvironmentChange()
	return nil
}

func GetPathEnabled() (bool, error) {
	exeDir := getInstallDir()
	if exeDir == "" {
		return false, nil
	}

	key, err := registry.OpenKey(registry.CURRENT_USER, envKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false, nil
	}
	defer key.Close()

	val, _, err := readUserPath(key)
	if err != nil {
		return false, nil
	}

	return pathContains(val, exeDir), nil
}

func EnablePath() error {

	if err := installExes(); err != nil {
		return fmt.Errorf("failed to install/update binary: %w", err)
	}

	exeDir := getInstallDir()
	if exeDir == "" {
		return fmt.Errorf("install directory not available")
	}

	key, _, err := registry.CreateKey(registry.CURRENT_USER, envKeyPath, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %w", err)
	}
	defer key.Close()

	originalPath, valtype, err := readUserPath(key)
	if err != nil {
		return err
	}

	updated := pathAdd(originalPath, exeDir)
	if updated == originalPath {
		return nil
	}

	return writeUserPath(key, updated, originalPath, valtype, "")
}

func DisablePath() error {
	exeDir := getInstallDir()
	if exeDir == "" {
		return fmt.Errorf("install directory not available")
	}

	key, err := registry.OpenKey(registry.CURRENT_USER, envKeyPath, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			// No Environment key means PATH cannot contain focusd.
			return nil
		}
		return fmt.Errorf("failed to open registry key: %w", err)
	}
	defer key.Close()

	originalPath, valtype, err := readUserPath(key)
	if err != nil {
		// readUserPath already treats a missing PATH value as empty, so any
		// error here is real and must surface: the uninstaller checks the exit
		// code before it deletes the binary.
		return err
	}

	if !pathContains(originalPath, exeDir) {
		return nil
	}

	updated := pathRemove(originalPath, exeDir)

	return writeUserPath(key, updated, originalPath, valtype, exeDir)
}
