package system

import "strings"

// PATH is a semicolon-delimited list on Windows. These helpers are pure so the
// entry logic can be tested without touching HKCU\Environment.

func splitPathEntries(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func pathContains(value, entry string) bool {
	for _, p := range splitPathEntries(value) {
		if strings.EqualFold(p, entry) {
			return true
		}
	}
	return false
}

// pathAdd returns value with entry appended, unless already present. Existing
// entries keep their original text so %VAR% references survive untouched.
func pathAdd(value, entry string) string {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return value
	}
	if pathContains(value, entry) {
		return value
	}

	entries := splitPathEntries(value)
	if len(entries) == 0 {
		return entry
	}
	return strings.Join(entries, ";") + ";" + entry
}

// pathRemove returns value without entry. Returns value unchanged when entry is
// absent, so callers never rewrite the registry for a no-op.
func pathRemove(value, entry string) string {
	if !pathContains(value, entry) {
		return value
	}

	entries := splitPathEntries(value)
	out := make([]string, 0, len(entries))
	for _, p := range entries {
		if strings.EqualFold(p, entry) {
			continue
		}
		out = append(out, p)
	}
	return strings.Join(out, ";")
}

// pathRetainsAll reports whether after has every entry from before except the
// one being removed. Guards against a lossy PATH rewrite.
func pathRetainsAll(before, after, removed string) bool {
	want := map[string]bool{}
	for _, p := range splitPathEntries(before) {
		if !strings.EqualFold(p, removed) {
			want[strings.ToLower(p)] = true
		}
	}
	for _, p := range splitPathEntries(after) {
		delete(want, strings.ToLower(p))
	}
	return len(want) == 0
}