package core

import (
	"focusd/storage"
	"regexp"
	"strings"
	"sync"
)

var (
	browserSuffixRegex = regexp.MustCompile(`(?i)(\s*[-—|]\s*(Google Chrome|Mozilla Firefox|Microsoft Edge|Brave|Opera|Vivaldi|Thorium|LibreWolf|Chromium|Zen Browser|Arc|Internet Explorer|Personal|Work|Profile \d+).*)$`)
	regexCache         = make(map[string]*regexp.Regexp)
	regexMu            sync.RWMutex
)

func IsBrowser(exeName string) bool {
	return storage.IsBrowser(exeName)
}

func getOrCompileRegex(pattern string) *regexp.Regexp {
	regexMu.RLock()
	if re, ok := regexCache[pattern]; ok {
		regexMu.RUnlock()
		return re
	}
	regexMu.RUnlock()

	regexMu.Lock()
	defer regexMu.Unlock()

	if re, ok := regexCache[pattern]; ok {
		return re
	}

	re := regexp.MustCompile(pattern)
	regexCache[pattern] = re
	return re
}

func CleanWindowTitle(rawTitle, exeName string) string {
	if rawTitle == "" {
		return "Unknown Tab"
	}

	clean := browserSuffixRegex.ReplaceAllString(rawTitle, "")

	baseName := strings.TrimSuffix(strings.ToLower(exeName), ".exe")

	patternStr := `(?i)(\s*[-—|]\s*` + regexp.QuoteMeta(baseName) + `.*)$`
	genericPattern := getOrCompileRegex(patternStr)
	clean = genericPattern.ReplaceAllString(clean, "")

	if strings.Contains(strings.ToLower(rawTitle), "browser") {
		browserPatternStr := `(?i)(\s*[-—|]\s*` + regexp.QuoteMeta(baseName) + `\s+Browser.*)$`
		genericBrowserPattern := getOrCompileRegex(browserPatternStr)
		clean = genericBrowserPattern.ReplaceAllString(clean, "")
	}

	clean = strings.TrimSpace(clean)

	if clean == "" {
		return "New Tab / Other"
	}

	return clean
}
