package core

import (
	"focusd/storage"
	"strings"
	"unicode"
)

var browserSuffixes = []string{
	"google chrome", "mozilla firefox", "microsoft edge", "brave",
	"opera", "vivaldi", "thorium", "librewolf", "chromium",
	"zen browser", "arc", "internet explorer", "personal", "work",
}

func IsBrowser(exeName string) bool {
	return storage.IsBrowser(exeName)
}

func isBrowserSuffix(suffix string) bool {
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

func stripNotificationCount(title string) string {
	title = strings.TrimSpace(title)

	if len(title) >= 3 {
		first := title[0]
		if first == '(' || first == '[' || first == '{' {
			var closeChar byte
			switch first {
			case '(':
				closeChar = ')'
			case '[':
				closeChar = ']'
			case '{':
				closeChar = '}'
			}

			for i := 1; i < len(title) && i < 10; i++ {
				if title[i] == closeChar {
					inner := title[1:i]
					allDigits := len(inner) > 0
					for _, r := range inner {
						if !unicode.IsDigit(r) {
							allDigits = false
							break
						}
					}
					if allDigits {
						title = strings.TrimSpace(title[i+1:])
					}
					break
				}
			}
		}
	}

	if len(title) >= 3 {
		last := title[len(title)-1]
		if last == ')' || last == ']' || last == '}' {
			var openChar byte
			switch last {
			case ')':
				openChar = '('
			case ']':
				openChar = '['
			case '}':
				openChar = '{'
			}

			for i := len(title) - 2; i >= 0 && i > len(title)-12; i-- {
				if title[i] == openChar {
					inner := title[i+1 : len(title)-1]
					allDigits := len(inner) > 0
					for _, r := range inner {
						if !unicode.IsDigit(r) {
							allDigits = false
							break
						}
					}
					if allDigits {
						title = strings.TrimSpace(title[:i])
					}
					break
				}
			}
		}
	}

	return title
}

func CleanWindowTitle(rawTitle, exeName string) string {
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

	if clean == "" {
		return "New Tab / Other"
	}

	return clean
}
