package system

import (
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"
)

var (
	versionInfoMu    sync.Mutex
	versionInfoCache = map[string]string{}
)

var versionInfoBlocklist = []string{
	"microsoft® windows® operating system",
	"microsoft windows",
	"windows",
	"",
}

// ResolveIdentity builds the identity and label for a process. Version info is
// cached per path because it reads from disk on the poll loop.
func ResolveIdentity(exeName, exePath string) AppIdentity {
	id := AppIdentity{
		ExeName: exeName,
		ExePath: exePath,
		AppID:   buildAppID(exeName, exePath),
	}

	if label := readVersionLabel(exePath); label != "" {
		id.DisplayName = label
		return id
	}
	id.DisplayName = TitleCaseExe(exeName)
	return id
}

// readVersionLabel returns a cached FileDescription or ProductName.
func readVersionLabel(exePath string) string {
	if exePath == "" {
		return ""
	}

	versionInfoMu.Lock()
	if cached, ok := versionInfoCache[exePath]; ok {
		versionInfoMu.Unlock()
		return cached
	}
	versionInfoMu.Unlock()

	label := queryVersionLabel(exePath)

	versionInfoMu.Lock()
	versionInfoCache[exePath] = label
	versionInfoMu.Unlock()

	return label
}

func isBlocklistedLabel(label string) bool {
	normalized := strings.ToLower(strings.TrimSpace(label))
	for _, bad := range versionInfoBlocklist {
		if normalized == bad {
			return true
		}
	}
	return false
}

// parentDir returns the directory part of a path, or empty when there is none.
func parentDir(path string) string {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return ""
	}
	return dir
}

// TitleCaseExe renders an exe base name as a readable label. It is the last
// fallback after overrides, version info, and the built-in map.
func TitleCaseExe(exeName string) string {
	name := strings.TrimSuffix(strings.TrimSpace(exeName), ".exe")
	name = strings.TrimSuffix(name, ".EXE")
	if name == "" {
		return exeName
	}
	// Case the first rune, not the first byte: byte slicing splits a
	// multi-byte character and produces mojibake.
	r, size := utf8.DecodeRuneInString(name)
	return strings.ToUpper(string(r)) + name[size:]
}
