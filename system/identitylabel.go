package system

import (
	"path/filepath"
	"strings"
	"sync"
)

// Version info gives a far better label than the exe base name, so Firefox
// Nightly reads as "Firefox Nightly" rather than "Firefox". The read is cached
// per path because it touches the disk and the poll loop runs every few
// seconds.

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

// ResolveIdentity builds the identity and label for a process. exePath may be
// empty, in which case the key falls back to the exe name.
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
	return strings.ToUpper(name[:1]) + name[1:]
}
