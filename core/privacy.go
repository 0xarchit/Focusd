package core

import (
	"strings"

	"focusd/system"
)

// Browser suffix markers are best effort; strict redaction does not depend on them.
var privateTitleMarkers = []string{
	"inprivate",
	"incognito",
	"inkognito",
	"incógnito",
	"private browsing",
	"private window",
	"private mode",
	"private",
	"guest mode",
	"privates fenster",
	"privater modus",
	"navigation privée",
	"fenêtre privée",
	"ventana privada",
	"navegación privada",
	"modo privado",
	"janela privativa",
	"navegação privativa",
}

// IsPrivateWindowTitle reports whether a title belongs to a private browsing
// session.
func IsPrivateWindowTitle(title string) bool {
	if strings.TrimSpace(title) == "" {
		return false
	}
	candidate := strings.ToLower(sanitizeTitle(title))
	for range 4 {
		idx, sepLen := lastSeparator(candidate)
		segment := candidate
		if idx >= 0 {
			segment = candidate[idx+sepLen:]
		}
		if privateSuffixSegment(segment) {
			return true
		}
		if !isKnownBrowserSuffix(strings.TrimSpace(segment)) || idx < 0 {
			break
		}
		candidate = strings.TrimSpace(candidate[:idx])
	}
	return strings.HasSuffix(candidate, " -private") || strings.HasSuffix(candidate, " -inprivate")
}

func privateSuffixSegment(segment string) bool {
	segment = strings.Trim(strings.TrimSpace(segment), "()[]{} ")
	for _, marker := range privateTitleMarkers {
		if segment == marker {
			return true
		}
		if !strings.HasSuffix(segment, marker) {
			continue
		}
		prefix := strings.TrimSpace(strings.TrimSuffix(segment, marker))
		prefix = strings.Trim(prefix, "()[]{} -")
		if isKnownBrowserSuffix(prefix) {
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
