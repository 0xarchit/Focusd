# Focusd

[![Version](https://img.shields.io/github/v/release/0xarchit/focusd?style=for-the-badge&logo=github&logoColor=white&labelColor=000000&color=000000)](https://github.com/0xarchit/focusd/releases)
[![Build Status](https://img.shields.io/github/actions/workflow/status/0xarchit/focusd/release.yml?style=for-the-badge&logo=githubactions&logoColor=white&labelColor=000000&color=000000)](https://github.com/0xarchit/focusd/actions)
[![Downloads](https://img.shields.io/github/downloads/0xarchit/focusd/total?style=for-the-badge&logo=rolldown&logoColor=white&labelColor=000000&color=000000)](https://github.com/0xarchit/focusd/releases)
[![License](https://img.shields.io/badge/License-MIT-000000.svg?style=for-the-badge&logo=apache&logoColor=white&labelColor=000000&color=000000)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.24+-000000.svg?style=for-the-badge&logo=go&logoColor=white&labelColor=000000&color=000000)](https://go.dev)
![Platform](https://img.shields.io/badge/Platform-Windows_x64-000000.svg?style=for-the-badge&logo=nsis&logoColor=white&labelColor=000000&color=000000)
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
iwr "https://github.com/0xarchit/focusd/releases/latest/download/focusd_setup.exe" -OutFile focusd_setup.exe; ./focusd_setup.exe
```

**Command Prompt**:
```cmd
curl -L -o focusd_setup.exe "https://github.com/0xarchit/focusd/releases/latest/download/focusd_setup.exe" && focusd_setup.exe
```

> Run the installer. It adds `focusd` to your PATH automatically. Open a new terminal and type `focusd` to launch.

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
| `focusd` | Interactive TUI menu |
| `focusd start` | Start background tracking daemon |
| `focusd stop` | Stop tracking daemon |
| `focusd status` | Show tracking status |
| `focusd stats` | Detailed usage breakdown |
| `focusd focus [mins]` | Start Pomodoro timer (default 25 min) |
| `focusd stop-timer` | Stop running timer |
| `focusd limit [app] [mins]` | Set daily app time limit |
| `focusd pause` | Pause tracking |
| `focusd resume` | Resume tracking |
| `focusd browser` | Manage custom browsers (add/remove) |
| `focusd export` | Export data to CSV |
| `focusd retention` | Manage data retention (set/reset) |
| `focusd autostart` | Manage auto-start (enable/disable) |
| `focusd path` | Manage PATH (enable/disable) |
| `focusd update` | Check for updates |
| `focusd reset-password` | Reset password protection |

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

**Requirements:** Go 1.24+

---

## Contributing

Contributions welcome. See [CONTRIBUTING.md](.github/CONTRIBUTING.md) for guidelines.

## License

MIT License. See [LICENSE](LICENSE).

---

<p align="center">
  <sub>Built by <a href="https://github.com/0xarchit">@0xarchit</a></sub>
</p>
