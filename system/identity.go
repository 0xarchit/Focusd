package system

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// AppIdentity separates which install is tracked from how it is labelled. The
// exe base name alone cannot identify an install (firefox.exe is both Firefox
// and Firefox Nightly) and neither can the folder alone (Chrome and Edge both
// live in a folder called "Application").
type AppIdentity struct {
	// AppID is the machine identity used as a database key.
	AppID string
	// ExeName is the base name, for example firefox.exe.
	ExeName string
	// ExePath is the full image path, empty when it cannot be read.
	ExePath string
	// DisplayName is the human label, which may change freely.
	DisplayName string
}

// legacySuffix marks rows migrated from a schema that recorded no path.
const legacySuffix = "legacy"

// systemSuffix marks a live process whose path could not be read, for example
// an elevated or protected process.
const systemSuffix = "system"

// keySeparator divides the exe name from the install hash.
const keySeparator = "|"

// legacyAppID is the key for rows that predate app identity.
func legacyAppID(exeName string) string {
	return normalizeExe(exeName) + keySeparator + legacySuffix
}

// fallbackAppID is the key for a process whose path could not be resolved.
func fallbackAppID(exeName string) string {
	return normalizeExe(exeName) + keySeparator + systemSuffix
}

// normalizeExe lowercases the base name and drops a trailing .exe so the key
// does not depend on how the file is named.
func normalizeExe(exeName string) string {
	name := strings.ToLower(strings.TrimSpace(exeName))
	name = strings.TrimSuffix(name, ".exe")
	if name == "" {
		return "unknown"
	}
	return name
}

// dirHash returns a short, stable hash of an install directory.
func dirHash(dir string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(dir))))
	return hex.EncodeToString(sum[:])[:8]
}

// buildAppID derives the identity key. The directory alone is hashed, never the
// file name, because browsers update their versioned subdirectories in place.
func buildAppID(exeName, exePath string) string {
	base := normalizeExe(exeName)
	if exePath == "" {
		return base + keySeparator + systemSuffix
	}

	dir := parentDir(exePath)
	if dir == "" {
		return base + keySeparator + systemSuffix
	}
	return base + keySeparator + dirHash(dir)
}

// IsLegacyAppID reports whether a key came from a migrated row.
func IsLegacyAppID(appID string) bool {
	_, hash := splitAppKey(appID)
	return hash == legacySuffix
}

// IsFallbackAppID reports whether a key came from a process whose path could
// not be read.
func IsFallbackAppID(appID string) bool {
	_, hash := splitAppKey(appID)
	return hash == systemSuffix
}

// ConfigMatchesApp reports whether a config entry applies to a live app. A
// bare exe name wildcards across every install of that executable, so limits
// written before app identity keep firing; a full app key names one install.
func ConfigMatchesApp(configKey, appID string) bool {
	base, hash := splitAppKey(normalizeConfigKey(configKey))
	if hash != "" {
		return hash == appKeyHash(appID) && base == appKeyBase(appID)
	}
	return base == appKeyBase(appID)
}

// normalizeConfigKey reduces a user-supplied key to a bare exe name, accepting
// "firefox", "firefox.exe", or a full app key.
func normalizeConfigKey(key string) string {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return ""
	}
	if !strings.Contains(trimmed, keySeparator) {
		return normalizeExe(trimmed)
	}
	return trimmed
}

// StoredAppMatches reports whether a stored row's key is the same app as a live
// key. Exact equality only: folding a legacy row into a live install would merge
// Firefox and Firefox Nightly again.
func StoredAppMatches(storedKey, liveKey string) bool {
	return storedKey == liveKey
}

// sameAppKey compares keys for config lookup. A bare exe name acts as a
// wildcard so limits written before this feature keep applying to every install;
// two different hashed keys never match.
func sameAppKey(a, b string) bool {
	if a == b {
		return true
	}

	baseA, hashA := splitAppKey(a)
	baseB, hashB := splitAppKey(b)
	if baseA != baseB {
		return false
	}

	// A bare exe name is a wildcard across every install.
	return hashA == "" || hashB == ""
}

// splitAppKey separates an app key into its exe name and its install hash. The
// exe part is normalized so a bare config entry written as "Firefox.exe" and a
// stored key both reduce to "firefox".
func splitAppKey(appID string) (base, hash string) {
	i := strings.Index(appID, keySeparator)
	if i < 0 {
		return normalizeExe(appID), ""
	}
	return normalizeExe(appID[:i]), appID[i+1:]
}

// appKeyHash returns the install hash from a key, or "" for a bare exe name.
func appKeyHash(appID string) string {
	_, hash := splitAppKey(appID)
	return hash
}

// appKeyBase strips any suffix from a key, leaving the exe name.
func appKeyBase(appID string) string {
	base, _ := splitAppKey(appID)
	return base
}
