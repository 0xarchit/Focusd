package core

import (
	"strings"

	"focusd/coreapi"
	"focusd/system"
)

// ClassifiedObservation carries only metadata permitted by the current privacy policy.
type ClassifiedObservation struct {
	StoredTitle string
	SiteName    string
	Browser     bool
	Private     bool
}

// ClassifyObservation separates a stored title from browser metadata before redaction.
func ClassifyObservation(exeName, exePath, rawTitle string) ClassifiedObservation {
	tor := isTorBrowserIdentity(exeName, exePath)
	return classifyObservation(exeName, rawTitle, system.GetPrivacyMode(), system.IsBrowser(exeName) || tor, tor)
}

func classifyObservation(exeName, rawTitle, mode string, browser, tor bool) ClassifiedObservation {
	result := ClassifiedObservation{Browser: browser, Private: browser && (tor || IsPrivateWindowTitle(rawTitle))}
	title := strings.TrimSpace(rawTitle)
	if result.Private {
		result.StoredTitle = "[redacted]"
		return result
	}
	if title == "" {
		return result
	}
	switch mode {
	case "strict":
		result.StoredTitle = "[redacted]"
	case "categories":
		result.StoredTitle = "[redacted]"
		if browser {
			result.SiteName = coreapi.CanonicalSiteNameFor(cleanWindowTitle(title, exeName))
			if result.SiteName == "" {
				result.SiteName = "Unknown Site"
			}
		}
	case "titles":
		result.StoredTitle = title
		if browser {
			result.SiteName = cleanWindowTitle(title, exeName)
		}
	default:
		result.StoredTitle = "[redacted]"
	}
	return result
}

func isTorBrowserIdentity(exeName, exePath string) bool {
	base := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(exeName)), ".exe")
	if base == "tor" || base == "torbrowser" {
		return true
	}
	if base != "firefox" {
		return false
	}
	parts := strings.Split(strings.ToLower(strings.ReplaceAll(exePath, "/", `\`)), `\`)
	n := len(parts)
	return n >= 3 && parts[n-1] == "firefox.exe" && parts[n-2] == "browser" && parts[n-3] == "tor browser"
}
