package system

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	envKeyPath = `Environment`
	appName    = "focusd"
)

func GetAutoStartEnabled() (bool, string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err == nil {
		defer key.Close()
		if val, _, err := key.GetStringValue(appName); err == nil {
			return true, val, nil
		}
	}

	return false, "", nil
}

func EnableAutoStart() error {
	if err := InstallExes(); err != nil {
		return fmt.Errorf("failed to install: %w", err)
	}

	daemonPath := GetInstalledDaemonPath()
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
	return DisableRegistryAutoStart()
}

func DisableRegistryAutoStart() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return nil
	}
	defer key.Close()
	err = key.DeleteValue(appName)
	if err == registry.ErrNotExist {
		return nil
	}
	return err
}

func GetPathEnabled() (bool, error) {
	exeDir := GetInstallDir()
	if exeDir == "" {
		return false, nil
	}

	key, err := registry.OpenKey(registry.CURRENT_USER, envKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false, nil
	}
	defer key.Close()

	val, _, err := key.GetStringValue("Path")
	if err != nil {
		return false, nil
	}

	paths := strings.Split(val, ";")
	for _, p := range paths {
		if strings.EqualFold(strings.TrimSpace(p), exeDir) {
			return true, nil
		}
	}
	return false, nil
}

func EnablePath() error {

	if err := InstallExes(); err != nil {
		return fmt.Errorf("failed to install/update binary: %w", err)
	}

	exeDir := GetInstallDir()
	if exeDir == "" {
		return fmt.Errorf("install directory not available")
	}

	key, _, err := registry.CreateKey(registry.CURRENT_USER, envKeyPath, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %w", err)
	}
	defer key.Close()

	val, _, err := key.GetStringValue("Path")
	if err != nil && err != registry.ErrNotExist {
		return err
	}

	paths := strings.Split(val, ";")
	for _, p := range paths {
		if strings.EqualFold(strings.TrimSpace(p), exeDir) {
			return nil
		}
	}

	if val != "" && !strings.HasSuffix(val, ";") {
		val += ";"
	}
	val += exeDir

	return key.SetStringValue("Path", val)
}

func DisablePath() error {
	exeDir := GetInstallDir()
	if exeDir == "" {
		return fmt.Errorf("install directory not available")
	}

	key, err := registry.OpenKey(registry.CURRENT_USER, envKeyPath, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return nil
	}
	defer key.Close()

	originalPath, _, err := key.GetStringValue("Path")
	if err != nil {
		return nil
	}

	if originalPath == "" {
		return nil
	}

	paths := strings.Split(originalPath, ";")
	var newPaths []string
	found := false

	for _, p := range paths {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		if strings.EqualFold(trimmed, exeDir) {
			found = true
			continue
		}
		newPaths = append(newPaths, p)
	}

	if !found {
		return nil
	}

	newPath := strings.Join(newPaths, ";")

	if len(newPath) < len(originalPath)/2 && len(originalPath) > 100 {
		return fmt.Errorf("safety check failed: new PATH is suspiciously shorter than original")
	}

	return key.SetStringValue("Path", newPath)
}
