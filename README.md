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

```text
   __                           _ 
  / _|                         | |
 | |_ ___   ___ _   _ ___  ____| |
 |  _/ _ \ / __| | | / __|/ _  | |
 | || (_) | (__| |_| \__ \ (_| |_|
 |_| \___/ \___|\__,_|___/\____(_)
```

**Privacy-first, offline digital wellbeing tracker for Windows.**

**Focusd** is an ultra-lightweight CLI daemon that tracks your application usage locally. It gives you deep insights into your digital habits without ever sending a single byte of data to the cloud.

---

## 🚀 Installation

### ⚡ One-Line Install

**PowerShell** (Recommended):
```powershell
iwr "https://github.com/0xarchit/focusd/releases/latest/download/focusd.exe" -OutFile focusd.exe; ./focusd.exe init
```

**Command Prompt** (cmd.exe):
```cmd
curl -L -o focusd.exe "https://github.com/0xarchit/focusd/releases/latest/download/focusd.exe" && focusd.exe init
```

### 📦 Manual Download
1.  Download the latest `focusd.exe` from the [**Releases Page**](https://github.com/0xarchit/focusd/releases/latest).
2.  Place it in a folder of your choice (e.g., `C:\Program Files\Focusd`).
3.  Open a terminal (cmd/PowerShell) in the download folder.
4.  Run `focusd init`.

> [!NOTE] 
> The `focusd` command is only available globally after you run `init` and accept the "Add to PATH" option. Until then, use `./focusd.exe` in the download folder.

---

## 📘 User Guide

### 1. The Main Menu
Simply type `focusd` (without arguments) to open the interactive main menu. From here you can navigate to Stats, Settings, or Focus Mode using arrow keys or numbers.

```powershell
focusd
```

### 2. The Dashboard (`stats`)
Your central hub for insights. Run `focusd stats` (or select it from the menu) to open the interactive terminal UI.
*   **Daily Overview**: See total screen time and unique apps used today.
*   **Top Apps**: A visual bar chart of your most used applications.
*   **History**: view "Last 7 Days" or "All Time" trends to track your productivity over time.
    *(Note: The dashboard uses ANSI color codes for a beautiful, hacker-style aesthetic.)*

### 3. Focus Sessions (Pomodoro)
Need to get work done? Start a focus timer:
```powershell
focusd focus 25
```
*   Starts a **25-minute** deep work session.
*   Focusd will notify you when the session ends.
*   Usage during this time is tracked separately to calculate your "Focus Score".

### 3. App Limits (Digital Detox)
Prevent doom-scrolling by setting daily allowances for distraction apps.
```powershell
focusd limit
```
*   Select an app from your recent usage list.
*   Set a daily cap (e.g., "30 minutes").
*   Focusd will alert you when you exceed this limit (blocking features coming in v1.2).

### 4. Background Monitoring
*   **Zero Impact**: The daemon uses ~3MB of RAM and <0.1% CPU.
*   **Auto-Start**: Can be configured to start with Windows during `init`.
*   **Privacy**: Data is stored in `%APPDATA%\focusd\db.sqlite`. It never leaves your machine.

---

## 📖 Command Reference

| Command | Short | Action |
| :--- | :--- | :--- |
| `focusd start` | | Start the background tracking service |
| `focusd stop` | | Stop the service |
| `focusd restart` | | Restart the service (useful after updates) |
| `focusd stats` | `st` | **Open the Dashbaord** (Main UI) |
| `focusd focus <m>`| `f` | Start a Pomodoro timer for `m` minutes |
| `focusd limit` | `l` | Configure app time limits |
| `focusd status` | `s` | Check daemon status and PID |
| `focusd update` | | Check for and install version updates |
| `focusd version` | `v` | Show version info |
| `focusd init` | `i` | Run the setup wizard/repair |
| `focusd uninstall`| | Remove database, config, and binary |
| `focusd browser` | | Manage browser tracking configuration |

---

## 🔒 Privacy Architecture

Focusd is built on a "Local-First" philosophy:
1.  **No Telemetry**: We don't track you. Period.
2.  **No Cloud Sync**: Your data lives on your SSD.
3.  **Open Database**: Your data is stored in standard SQLite format. You technically own it and can query it yourself using any SQL viewer.

---

## 🏗️ Development

**Requirements**: Go 1.21+

```powershell
# Clone the repo
git clone https://github.com/0xarchit/Focusd.git
cd Focusd

# Build the binary (with linker flags for optimization)
go build -ldflags="-s -w" -trimpath -o focusd.exe ./cmd/focusd

# (Optional) Compress using UPX
upx focusd.exe
```

---

## 🤝 Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on how to submit Pull Requests or report bugs.

## 📜 License

Distributed under the MIT License. See [LICENSE](LICENSE) for more information.

Copyright (c) 2025 0xArchit