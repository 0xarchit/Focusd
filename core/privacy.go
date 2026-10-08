package core

import "strings"

// Window titles are stored in plaintext. A title can carry a bank account page,
// a mail subject line, a document name, or a search term, so titles from
// private browsing modes are dropped before they ever reach the database.
//
// This is a best-effort marker list, not a guarantee. It cannot detect every
// private session, and it deliberately errs toward keeping a title rather than
// dropping legitimate history.

// privateTitleMarkers identify a private or incognito window. Matching is
// case-insensitive on the whole title.
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

// redactTitles controls whether titles are stored at all. Disabled means only
// the app and its duration are recorded, with no window text.
var redactTitles = false

// SetRedactTitles turns window-title storage off.
func SetRedactTitles(enabled bool) {
	redactTitles = enabled
}

// RedactTitles reports whether title storage is disabled.
func RedactTitles() bool {
	return redactTitles
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

// storableTitle returns the title to persist, or an empty string when the title
// must not be stored.
//
// A private window yields no title at all. When redaction is enabled the title
// is replaced with a generic placeholder rather than dropped, so the user can
// still see which app was used and for how long.
func storableTitle(title string) string {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return ""
	}
	if IsPrivateWindowTitle(trimmed) {
		return ""
	}
	if redactTitles {
		return "[redacted]"
	}
	return trimmed
}
