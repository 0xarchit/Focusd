package core

import (
	"focusd/coreapi"
	"regexp"
	"strings"
	"unicode"
)

// Browser window titles vary by browser, version, and locale, so parsing here
// is defensive: aim for a usable name, never a perfect parse.

// invisibleRunes are zero-width and soft-hyphen characters that browsers inject
// into their own suffix. Edge puts a zero-width space inside "Microsoft Edge",
// which makes a literal substring match fail every time.
var invisibleRunes = map[rune]bool{
	'\u200b': true, // zero-width space
	'\u200c': true, // zero-width non-joiner
	'\u200d': true, // zero-width joiner
	'\u00ad': true, // soft hyphen
	'\ufeff': true, // zero-width no-break space
}

// sanitizeTitle removes characters that break matching and normalizes
// whitespace. It runs before any comparison.
func sanitizeTitle(title string) string {
	var b strings.Builder
	b.Grow(len(title))
	for _, r := range title {
		if invisibleRunes[r] {
			continue
		}
		// Control characters would break both matching and display.
		if unicode.IsControl(r) {
			b.WriteRune(' ')
			continue
		}
		b.WriteRune(r)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

var (
	// Counters such as "(3)", "[12]", or "{12}".
	counterAnywhere = regexp.MustCompile(`\s*[\[({]\s*\d{1,9}\s*[\])}]\s*`)
	// Edge's "and N more pages" tab-group suffix.
	morePages = regexp.MustCompile(`\s+and\s+\d+\s+more\s+pages?\s*$`)
	// A trailing profile or person segment such as "- Person 2 -".
	trailingPerson = regexp.MustCompile(`\s*-\s*Person\s+\d+\s*-\s*$`)
)

// stripDecorations removes notification counters and, once the browser suffix
// has been stripped, the tab-group and profile markers browsers append last.
func stripDecorations(title string) string {
	clean := morePages.ReplaceAllString(title, "")
	clean = counterAnywhere.ReplaceAllString(clean, " ")
	clean = trailingPerson.ReplaceAllString(clean, " ")
	return strings.Join(strings.Fields(clean), " ")
}

// browserSuffixRules are matched right-to-left against the end of a title. Each
// entry is the literal text of a browser's own suffix, so matching is exact
// rather than "contains this word somewhere".
var browserSuffixRules = []string{
	"google chrome",
	"chrome",
	"chrome beta",
	"chrome dev",
	"chrome canary",
	"microsoft® edge",
	"microsoft edge",
	"edge",
	"mozilla firefox",
	"firefox",
	"firefox developer edition",
	"firefox nightly",
	"zen browser",
	"zen",
	"brave",
	"brave-browser",
	"vivaldi",
	"thorium",
	"librewolf",
	"chromium",
	"arc",
	"opera gx",
	"opera",
	"yandex browser",
	"waterfox",
	"coccoc",
	"internet explorer",
}

// cleanWindowTitle turns a raw browser window title into a page or site name.
// The browser suffix is stripped first because the remaining decorations anchor
// to the end of the title and would not match while a browser name is attached.
func cleanWindowTitle(rawTitle, exeName string) string {
	if strings.TrimSpace(rawTitle) == "" {
		return "Unknown Tab"
	}

	clean := sanitizeTitle(rawTitle)
	clean = stripBrowserSuffix(clean, exeName)
	clean = stripDecorations(clean)
	clean = strings.Trim(clean, "-–—|· \t")
	clean = strings.TrimSpace(clean)
	clean = coreapi.CollapseURLTitle(clean)

	if clean == "" {
		return "New Tab / Other"
	}
	return clean
}

// stripBrowserSuffix removes trailing separators plus a recognised browser
// suffix, working right to left so a title like "Inbox - Person 2 - Microsoft
// Edge" loses the browser first and then the profile segment.
func stripBrowserSuffix(title, exeName string) string {
	base := baseExeName(exeName)

	// Splits "Page - Profile 3" into its page and trailing segments. Both are
	// captured: checking the page text against the profile vocabulary would
	// strip a page that merely starts with "Work".
	profileSegment := regexp.MustCompile(`^(.*?)\s+-\s+([^-]{1,32})$`)

	for range 4 {
		before := title

		if idx, sepLen := lastSeparator(title); idx >= 0 {
			segment := strings.ToLower(strings.TrimSpace(title[idx+sepLen:]))
			if segment != "" {
				if base != "" && segment == base {
					title = title[:idx]
				} else if isKnownBrowserSuffix(segment) {
					title = title[:idx]
				} else if m := profileSegment.FindStringSubmatch(title); m != nil && looksLikeProfileSegment(m[2]) {
					// Keep the page name, drop only the profile marker.
					title = m[1]
				}
			}
		}

		if title == before {
			break
		}
	}
	return strings.TrimSpace(title)
}

// lastSeparator returns the index and length of the final separator in a title,
// so the caller can slice past the whole separator rather than leaving its last
// character attached to the segment.
func lastSeparator(title string) (index int, length int) {
	seps := []string{" - ", " — ", " – ", " | ", " · ", ": "}
	best := -1
	bestLen := 0
	for _, sep := range seps {
		if idx := strings.LastIndex(title, sep); idx > best {
			best = idx
			bestLen = len(sep)
		}
	}
	return best, bestLen
}

// lastSeparatorIndex finds the final " - ", " — ", " – ", " | ", " · " or
// ": " in a title.
func lastSeparatorIndex(title string) int {
	idx, _ := lastSeparator(title)
	return idx
}

// isKnownBrowserSuffix matches a trailing segment against the browser list.
// The comparison is anchored to the whole segment, which is what stops
// "arc" from matching "search" and "code" from matching "LeetCode".
func isKnownBrowserSuffix(segment string) bool {
	normalized := strings.ToLower(strings.TrimSpace(segment))
	for _, rule := range browserSuffixRules {
		if normalized == rule {
			return true
		}
	}
	return false
}

// profileSegmentWords are the words a browser uses for a profile tab title.
// Only these are dropped, so a page titled "Search - Work" keeps its text.
var profileSegmentWords = map[string]bool{
	"profile":  true,
	"work":     true,
	"personal": true,
	"private":  true,
	"guest":    true,
}

func looksLikeProfileSegment(segment string) bool {
	normalized := strings.ToLower(strings.TrimSpace(segment))
	if normalized == "" {
		return false
	}
	// "Profile 3" or "Work".
	if strings.Contains(normalized, "profile") {
		return true
	}
	fields := strings.Fields(normalized)
	if len(fields) == 1 && profileSegmentWords[fields[0]] {
		return true
	}
	// "Person 2", used by Edge for profile 1.
	if len(fields) == 2 && fields[0] == "person" {
		return true
	}
	return false
}

func baseExeName(exeName string) string {
	name := strings.ToLower(strings.TrimSpace(exeName))
	name = strings.TrimSuffix(name, ".exe")
	return name
}
