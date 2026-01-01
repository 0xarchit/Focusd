package core

import (
	"focusd/storage"
	"regexp"
	"strings"
)

var browserSuffixRegex = regexp.MustCompile(`(?i)(\s*[-—|]\s*(Google Chrome|Mozilla Firefox|Microsoft Edge|Brave|Opera|Vivaldi|Thorium|LibreWolf|Chromium|Zen Browser|Arc|Internet Explorer|Personal|Work|Profile \d+).*)$`)

func IsBrowser(exeName string) bool {

	return storage.IsBrowser(exeName)
}

func CleanWindowTitle(rawTitle, exeName string) string {
	if rawTitle == "" {
		return "Unknown Tab"
	}

	clean := browserSuffixRegex.ReplaceAllString(rawTitle, "")

	baseName := strings.TrimSuffix(strings.ToLower(exeName), ".exe")

	genericPattern := regexp.MustCompile(`(?i)(\s*[-—|]\s*` + regexp.QuoteMeta(baseName) + `.*)$`)
	clean = genericPattern.ReplaceAllString(clean, "")

	if strings.Contains(strings.ToLower(rawTitle), "browser") {

		genericBrowserPattern := regexp.MustCompile(`(?i)(\s*[-—|]\s*` + regexp.QuoteMeta(baseName) + `\s+Browser.*)$`)
		clean = genericBrowserPattern.ReplaceAllString(clean, "")
	}

	clean = strings.TrimSpace(clean)

	if clean == "" {

		return "New Tab / Other"
	}

	return clean
}
