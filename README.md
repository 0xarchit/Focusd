# Focusd

[![License](https://img.shields.io/github/license/0xarchit/focusd?style=flat-square)](LICENSE)
[![Release](https://img.shields.io/github/v/release/0xarchit/focusd?style=flat-square&color=22d3ee)](https://github.com/0xarchit/focusd/releases/latest)
[![Build Status](https://img.shields.io/github/actions/workflow/status/0xarchit/focusd/release.yml?style=flat-square&label=Build%20Status)](https://github.com/0xarchit/focusd/actions/workflows/release.yml)
[![Website](https://img.shields.io/website?url=https%3A%2F%2Fdab.0xarchit.is-a.dev&style=flat-square)](https://dab.0xarchit.is-a.dev/)
[![Dependencies](https://img.shields.io/badge/dependencies-up--to--date-brightgreen?style=flat-square)](#)  
[![Stars](https://img.shields.io/github/stars/0xarchit/focusd?style=flat-square&color=yellow)](https://github.com/0xarchit/focusd/stargazers)
[![Downloads](https://img.shields.io/github/downloads/0xarchit/focusd/total?style=flat-square&color=orange)](https://github.com/0xarchit/focusd/releases)
[![Repo Size](https://img.shields.io/github/repo-size/0xarchit/focusd?style=flat-square&color=blue)](https://github.com/0xarchit/focusd)
[![Issues](https://img.shields.io/github/issues/0xarchit/focusd?style=flat-square&color=red)](https://github.com/0xarchit/focusd/issues)
[![Last Commit](https://img.shields.io/github/last-commit/0xarchit/focusd?style=flat-square&color=green)](https://github.com/0xarchit/focusd/commits/main)
![Platform](https://img.shields.io/badge/platform-Windows_x64-blue?style=flat-square)
<center>
  <pre>
   __                           _ 
  / _|                         | |
 | |_ ___   ___ _   _ ___  ____| |
 |  _/ _ \ / __| | | / __|/ _  | |
 | || (_) | (__| |_| \__ \ (_| |_|
 |_| \___/ \___|\__,_|___/\____(_)
  </pre>
</center>

<h3 align="center">Privacy-First Digital Wellbeing for Windows</h3>

<p align="center">
  <strong>Track your screen time. Own your data. No cloud required.</strong>
</p>

---

## Why Focusd?

Every productivity tracker on the market uploads your data to their servers. **Focusd doesn't.** Your usage data stays on your machine, stored in a local SQLite database that you fully control.

| App | Installer / Disk Size | RAM Usage (Idle) | Tech Stack |
|-----|----------------------|------------------|------------|
| StayFree (Windows) | ~164 MB | 150MB - 400MB | likely Electron / UWP |
| Toggl Track | ~100 MB | 200MB - 500MB | Electron (Chromium bundled) |
| RescueTime (Classic) | ~25 MB | 20 - 50 MB | Native C++ / Qt |
| **Focusd** | **<11 MB** | **~4 - 10 MB** | **Go (Native Syscalls)** |

---

## Quick Start

**PowerShell** (Recommended):
```powershell
iwr "https://github.com/0xarchit/focusd/releases/latest/download/focusd.exe" -OutFile focusd.exe; ./focusd.exe init
```

**Command Prompt**:
```cmd
curl -L -o focusd.exe "https://github.com/0xarchit/focusd/releases/latest/download/focusd.exe" && focusd.exe init
```

> After running `init`, the `focusd` command is available globally from any terminal.

---

## Features

### 📊 Usage Dashboard
View daily and historical screen time with a beautiful terminal UI.
```
focusd stats
```

### ⏱️ Focus Sessions
Built-in Pomodoro timer with completion notifications.
```
focusd focus 25
```

### 🌐 Browser Tracking
Tracks time spent per browser tab (by page title, not URL for privacy).
- View in `focusd stats` → Browser Usage
- Add custom browsers: `focusd browser add <exe_name>`

### 🧪 Smart App Grouping (Experimental)
Automatically groups related browser tabs (e.g., all YouTube videos under "YouTube").
- 80+ supported sites (YouTube, GitHub, Reddit, Discord, LeetCode, etc.)
- Shows parent category with sub-entries
- *Note: This feature is under active development. Some titles may not group correctly.*

### ⏳ App Limits
Set daily time limits for distracting applications.
```
focusd limit
```

### 🔕 Background Daemon
Silent background process with minimal resource usage (~5MB RAM, ~0% CPU).

---

## Commands

| Command | Description |
|---------|-------------|
| `focusd` | Interactive menu |
| `focusd stats` | Open usage dashboard |
| `focusd focus <mins>` | Start focus timer |
| `focusd limit` | Configure app limits |
| `focusd browser` | Add/remove custom browsers |
| `focusd start/stop` | Control background service |
| `focusd update` | Check for updates |
| `focusd uninstall` | Remove all data |

---

## Privacy

- **No telemetry.** Zero network requests except for update checks.
- **No cloud.** All data stored locally in `%APPDATA%\focusd\focusd.db`.
- **Open database.** Standard SQLite—query it yourself with any SQL tool.
- **Open source.** Audit the code anytime.

---

## Building from Source

```powershell
git clone https://github.com/0xarchit/Focusd.git
cd Focusd
go build -ldflags="-s -w" -trimpath -o focusd.exe ./cmd/focusd
```

**Requirements:** Go 1.21+

---

## Contributing

Contributions welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT License. See [LICENSE](LICENSE).

---

<p align="center">
  <sub>Built by <a href="https://github.com/0xarchit">@0xarchit</a></sub>
</p>