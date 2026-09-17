package system

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// AppIdentity separates which install is being tracked from how it is labelled.
//
// Identity has to be stable for the same install and different for distinct
// installs. The exe base name alone fails the second test: firefox.exe is both
// Firefox and Firefox Nightly. The install folder alone fails too, because
// Chrome and Edge both live in a folder called "Application".
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

// legacyAppID is the key used for rows that predate app identity. Live code
// produces this only when no richer identity is available, so a migrated app and
// a newly seen app of the same exe land on the same key.
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

// dirHash returns a short, stable hash of an install directory. Only the
// directory is used, never the file name, because Chrome and Edge update their
// versioned subdirectories.
func dirHash(dir string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(dir))))
	return hex.EncodeToString(sum[:])[:8]
}

// buildAppID derives the identity key for an exe path.
//
// With a path: firefox.exe|a1b2c3d4, distinct per install while surviving
// version bumps. Without one: firefox.exe|system, which merges anything sharing
// a name, and is the honest answer when the path is unreadable.
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

// ConfigMatchesApp reports whether a config entry applies to a live app.
//
// Config keys are always bare exe names, either because that is what the user
// typed or because they predate this feature. They therefore act as wildcards
// across every install of that executable, so nobody's limits or whitelist
// entries silently stop firing after the upgrade.
func ConfigMatchesApp(configKey, appID string) bool {
	baseConfig := appKeyBase(normalizeConfigKey(configKey))
	baseApp := appKeyBase(appID)
	return baseConfig != "" && baseConfig == baseApp
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
// key. It is deliberately strict: exact equality only.
//
// Migrated rows carry the legacy marker because no install directory was ever
// recorded. They are NOT folded into a live install's totals, because a user
// running both Firefox and Firefox Nightly would then have every row merged
// again, which is exactly the problem this identity split exists to solve.
// History recorded before the upgrade stays in its own bucket, and that
// limitation is stated in the release notes.
func StoredAppMatches(storedKey, liveKey string) bool {
	return storedKey == liveKey
}

// sameAppKey reports whether two keys refer to the same app for config lookup
// purposes. It exists so a config entry written before this feature, which is
// just the bare exe name, keeps working after the upgrade.
//
// The rules, in order:
//
//   - identical keys match;
//   - a bare exe name matches any key with the same exe name, so existing
//     limits and whitelist entries act as wildcards across installs;
//   - two different hashed keys never match, which is what keeps Firefox and
//     Firefox Nightly separate.
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

// appKeyBase strips any suffix from a key, leaving the exe name.
func appKeyBase(appID string) string {
	base, _ := splitAppKey(appID)
	return base
}
