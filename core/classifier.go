package core

import (
	"regexp"
	"strings"
)

var browserSuffixes = []string{
	"google chrome", "mozilla firefox", "microsoft edge", "brave",
	"opera", "vivaldi", "thorium", "librewolf", "chromium",
	"zen browser", "arc", "internet explorer", "personal", "work",
	"profile ",
}

func isBrowserSuffix(suffix string) bool {
	for _, b := range browserSuffixes {
		if strings.Contains(suffix, b) {
			return true
		}
	}
	return false
}

var notificationRegex = regexp.MustCompile(`^[\[({]\d{1,9}[\])}]\s*|\s*[\[({]\d{1,9}[\])}]$`)

func stripNotificationCount(title string) string {
	return strings.TrimSpace(notificationRegex.ReplaceAllString(title, ""))
}

func cleanWindowTitle(rawTitle, exeName string) string {
	if rawTitle == "" {
		return "Unknown Tab"
	}

	baseName := strings.TrimSuffix(strings.ToLower(exeName), ".exe")
	clean := rawTitle

	clean = stripNotificationCount(clean)

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
	clean = strings.Trim(clean, "-–—| ")
	clean = strings.TrimSpace(clean)

	if clean == "" {
		return "New Tab / Other"
	}

	return clean
}
