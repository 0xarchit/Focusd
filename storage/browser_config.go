package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

var (
	defaultBrowsers = map[string]bool{
		"chrome.exe":    true,
		"firefox.exe":   true,
		"msedge.exe":    true,
		"brave.exe":     true,
		"opera.exe":     true,
		"vivaldi.exe":   true,
		"waterfox.exe":  true,
		"arc.exe":       true,
		"iexplore.exe":  true,
		"safari.exe":    true,
		"whale.exe":     true,
		"yandex.exe":    true,
		"thorium.exe":   true,
		"librewolf.exe": true,
		"chromium.exe":  true,
		"floorp.exe":    true,
		"zen.exe":       true,
	}
	browserCache map[string]bool
	browserMu    sync.RWMutex
)

type BrowserConfig struct {
	CustomBrowsers  []string `json:"custom_browsers"`
	IgnoredBrowsers []string `json:"ignored_default_browsers"`
}

func GetBrowserConfigPath() (string, error) {
	dataDir, err := GetDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "browsers.json"), nil
}

func LoadBrowserConfig() (*BrowserConfig, error) {
	path, err := GetBrowserConfigPath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &BrowserConfig{CustomBrowsers: []string{}}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config BrowserConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func SaveBrowserConfig(config *BrowserConfig) error {
	path, err := GetBrowserConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func GetBrowserList() map[string]bool {
	browserMu.RLock()
	if browserCache != nil {
		defer browserMu.RUnlock()
		return browserCache
	}
	browserMu.RUnlock()

	browserMu.Lock()
	defer browserMu.Unlock()

	if browserCache != nil {
		return browserCache
	}

	combined := make(map[string]bool)

	for k, v := range defaultBrowsers {
		combined[k] = v
	}

	config, _ := LoadBrowserConfig()
	if config != nil {
		for _, b := range config.CustomBrowsers {
			combined[strings.ToLower(b)] = true
		}
	}

	browserCache = combined
	return combined
}

func AddCustomBrowser(exeName string) error {
	exeName = strings.ToLower(strings.TrimSpace(exeName))
	if exeName == "" {
		return fmt.Errorf("invalid browser name")
	}
	if !strings.HasSuffix(exeName, ".exe") {
		exeName += ".exe"
	}

	browserMu.Lock()
	defer browserMu.Unlock()

	config, err := LoadBrowserConfig()
	if err != nil {
		return err
	}

	for _, b := range config.CustomBrowsers {
		if b == exeName {
			return fmt.Errorf("%s is already in custom list", exeName)
		}
	}

	if defaultBrowsers[exeName] {
		return fmt.Errorf("%s is already a default browser", exeName)
	}

	config.CustomBrowsers = append(config.CustomBrowsers, exeName)
	if err := SaveBrowserConfig(config); err != nil {
		return err
	}

	browserCache = nil
	return nil
}

func RemoveCustomBrowser(exeName string) error {
	exeName = strings.ToLower(strings.TrimSpace(exeName))

	browserMu.Lock()
	defer browserMu.Unlock()

	config, err := LoadBrowserConfig()
	if err != nil {
		return err
	}

	found := false
	newList := []string{}
	for _, b := range config.CustomBrowsers {
		if b == exeName {
			found = true
			continue
		}
		newList = append(newList, b)
	}

	if !found {
		return fmt.Errorf("%s not found in custom list (cannot remove default browsers)", exeName)
	}

	config.CustomBrowsers = newList
	if err := SaveBrowserConfig(config); err != nil {
		return err
	}

	browserCache = nil
	return nil
}

func GetCustomBrowsersList() []string {
	config, _ := LoadBrowserConfig()
	if config == nil {
		return []string{}
	}
	sort.Strings(config.CustomBrowsers)
	return config.CustomBrowsers
}

func IsBrowser(exeName string) bool {
	list := GetBrowserList()
	return list[strings.ToLower(exeName)]
}
