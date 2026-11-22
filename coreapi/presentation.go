package coreapi

import (
	"strings"

	"focusd/storage"
	"focusd/system"
)

// ApplyDisplayOverrides changes labels on a copied read model without changing identities.
func ApplyDisplayOverrides(stats []storage.AppDailyStat) []storage.AppDailyStat {
	result := append([]storage.AppDailyStat(nil), stats...)
	overrides := system.GetDisplayOverrides()
	for i := range result {
		if label := strings.TrimSpace(overrides[result[i].AppID]); label != "" {
			result[i].AppName = label
		}
	}
	return result
}

// ApplySiteOverrides renames recorded site labels without rewriting stored history.
func ApplySiteOverrides(stats []storage.AppDailyStat) []storage.AppDailyStat {
	result := append([]storage.AppDailyStat(nil), stats...)
	overrides := normalizedSiteOverrides()
	for i := range result {
		key := strings.ToLower(strings.TrimSpace(result[i].AppName))
		if label := strings.TrimSpace(overrides[key]); label != "" {
			result[i].AppName = label
		}
	}
	return result
}

// DistinctInstallLabels identifies separate installs that share a displayed label.
func DistinctInstallLabels(stats []storage.AppDailyStat) []string {
	labels := make([]string, len(stats))
	identities := make(map[string]map[string]bool)
	for i, stat := range stats {
		name := strings.TrimSpace(stat.AppName)
		if name == "" {
			name = system.TitleCaseExe(stat.ExeName)
		}
		labels[i] = name
		if identities[name] == nil {
			identities[name] = make(map[string]bool)
		}
		identity := stat.AppID
		if identity == "" {
			identity = stat.ExeName
		}
		identities[name][identity] = true
	}
	for i, stat := range stats {
		if len(identities[labels[i]]) > 1 {
			identity := stat.AppID
			if identity == "" {
				identity = stat.ExeName
			}
			if identity == "" {
				identity = "unknown"
			}
			// Put the identity first so truncating a long page name keeps it visible.
			labels[i] = "[" + identity + "] " + labels[i]
		}
	}
	return labels
}
