package system

import "strings"

// displayNameMap gives friendly labels to executables whose own version info is
// unhelpful or absent. It is consulted after user overrides and version info.
var displayNameMap = map[string]string{
	"code":            "VS Code",
	"devenv":          "Visual Studio",
	"idea64":          "IntelliJ IDEA",
	"pycharm64":       "PyCharm",
	"webstorm64":      "WebStorm",
	"goland64":        "GoLand",
	"rider64":         "Rider",
	"notepad++":       "Notepad++",
	"sublime_text":    "Sublime Text",
	"atom":            "Atom",
	"explorer":        "File Explorer",
	"slack":           "Slack",
	"teams":           "Microsoft Teams",
	"winword":         "Microsoft Word",
	"excel":           "Microsoft Excel",
	"powerpnt":        "PowerPoint",
	"outlook":         "Outlook",
	"terminal":        "Windows Terminal",
	"windowsterminal": "Windows Terminal",
	"cmd":             "Command Prompt",
	"powershell":      "PowerShell",
	"pwsh":            "PowerShell",
	"wt":              "Windows Terminal",
	"spotify":         "Spotify",
	"discord":         "Discord",
	"slack.exe":       "Slack",
}

// normalizeConfigName reduces a user-supplied app name to a bare exe name so
// that "firefox", "Firefox.exe", and "Firefox" all address the same app.
func normalizeConfigName(name string) string {
	return normalizeExe(strings.TrimSpace(name))
}

// NormalizeConfigName is the exported form, used by the UI to compare a config
// entry against a live app.
func NormalizeConfigName(name string) string {
	return normalizeConfigName(name)
}
