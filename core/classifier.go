package core

import (
	"focusd/storage"
	"strings"
)

var browserSuffixes = []string{
	"google chrome", "mozilla firefox", "microsoft edge", "brave",
	"opera", "vivaldi", "thorium", "liberwolf", "chromium",
	"zen browser", "arc", "internet explorer", "personal", "work",
}

func IsBrowser(exeName string) bool {
	return storage.IsBrowser(exeName)
}

func isBrowserSuffix(suffix string) bool {
	suffix = strings.ToLower(suffix)
	for _, b := range browserSuffixes {
		if strings.Contains(suffix, b) {
			return true
		}
	}
	if strings.HasPrefix(suffix, "profile ") {
		return true
	}
	return false
}

func CleanWindowTitle(rawTitle, exeName string) string {
	if rawTitle == "" {
		return "Unknown Tab"
	}

	baseName := strings.TrimSuffix(strings.ToLower(exeName), ".exe")
	clean := rawTitle

	separators := []string{" - ", " — ", " | "}
	for _, sep := range separators {
		if idx := strings.LastIndex(clean, sep); idx != -1 {
			suffix := strings.ToLower(clean[idx+len(sep):])
			if strings.Contains(suffix, baseName) || isBrowserSuffix(suffix) {
				clean = clean[:idx]
			}
		}
	}

	clean = strings.TrimSpace(clean)

	if clean == "" {
		return "New Tab / Other"
	}

	return clean
}
