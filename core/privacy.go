package core

import (
	"strings"

	"focusd/system"
)

// privateTitleMarkers identify a private or incognito window, matched
// case-insensitively. Best effort: it errs toward keeping a title.
var privateTitleMarkers = []string{
	"-inprivate",
	" inprivate",
	"incognito",
	"private browsing",
	"private window",
	"private mode",
	"-private",
	"guest mode",
}

// IsPrivateWindowTitle reports whether a title belongs to a private browsing
// session.
func IsPrivateWindowTitle(title string) bool {
	if strings.TrimSpace(title) == "" {
		return false
	}
	lower := strings.ToLower(title)
	for _, marker := range privateTitleMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// storableTitle returns the title to persist, or an empty string when it must
// not be stored. With redaction on (the redact_titles switch in config.json),
// a title is replaced with a placeholder rather than dropped, so the app and
// duration are still visible.
func storableTitle(title string) string {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return ""
	}
	if IsPrivateWindowTitle(trimmed) {
		return ""
	}
	if system.GetRedactTitles() {
		return "[redacted]"
	}
	return trimmed
}
