# Focusd

[![Version](https://img.shields.io/github/v/release/0xarchit/focusd?style=for-the-badge&logo=github&logoColor=white&labelColor=000000&color=000000)](https://github.com/0xarchit/focusd/releases)
[![Build Status](https://img.shields.io/github/actions/workflow/status/0xarchit/focusd/release.yml?style=for-the-badge&logo=githubactions&logoColor=white&labelColor=000000&color=000000)](https://github.com/0xarchit/focusd/actions)
[![Downloads](https://img.shields.io/github/downloads/0xarchit/focusd/total?style=for-the-badge&logo=rolldown&logoColor=white&labelColor=000000&color=000000)](https://github.com/0xarchit/focusd/releases)  
[![License](https://img.shields.io/badge/License-MIT-000000.svg?style=for-the-badge&logo=apache&logoColor=white&labelColor=000000&color=000000)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26+-000000.svg?style=for-the-badge&logo=go&logoColor=white&labelColor=000000&color=000000)](https://go.dev)
![Platform](https://img.shields.io/badge/Platform-Windows_x64-000000.svg?style=for-the-badge&logo=nsis&logoColor=white&labelColor=000000&color=000000)

<p align="center">
  <img src="website/assets/focusd-icon.svg" width="128" alt="focusd icon">
</p>

<h3 align="center">Privacy-First Digital Wellbeing for Windows</h3>

<p align="center">
  <strong>Track your screen time. Own your data. No cloud required.</strong><br/>
  <a href="https://focusd.0xarchit.is-a.dev">focusd.0xarchit.is-a.dev</a>
</p>

---

## Overview

Focusd is a native Windows **screen time tracker** and **digital wellbeing** tool written in Go. A lightweight background daemon (`focusd_daemon.exe`) polls the foreground window via Win32 syscalls and records per-app and per-browser-tab usage into a local SQLite database. A separate CLI/TUI binary (`focusd.exe`) renders dashboards, manages limits, and controls the daemon over a local TCP IPC channel.

It functions as an **app usage monitor**, **browser time tracker**, **Pomodoro focus timer**, and **productivity dashboard** — all in one under-12 MB binary with no runtime dependencies. If you're looking for a **free alternative to RescueTime, StayFree, or Toggl Track** that keeps your data local, focusd is built for that.

Nothing leaves your machine. The only network traffic is the optional update check against the GitHub Releases API.

| App | Installer / Disk Size | RAM Usage (Idle) | Tech Stack |
|-----|----------------------|------------------|------------|
| StayFree (Windows) | ~164 MB | 150 – 400 MB | likely Electron / UWP |
| Toggl Track | ~100 MB | 200 – 500 MB | Electron (Chromium bundled) |
| RescueTime (Classic) | ~25 MB | 20 – 50 MB | Native C++ / Qt |
| **Focusd** | **< 12 MB** | **~10 – 25 MB** | **Go (Native Win32 syscalls)** |

---

## Why focusd?

| Problem | How focusd solves it |
|---------|---------------------|
| Screen time trackers that send your data to the cloud | All data stays in a local SQLite database — no accounts, no telemetry |
| Heavy Electron-based trackers (100-400 MB RAM) | Under 12 MB installer, 10-25 MB RAM idle, native Win32 syscalls |
| No visibility into browser tab usage | Tracks time per tab via window title, groups 70+ sites automatically |
| No way to limit distracting apps | Set daily per-app time limits with notifications |
| Hard to stay focused | Built-in Pomodoro timer with break reminders |

---

## Quick Start

**PowerShell** (recommended):
```powershell
iwr "https://github.com/0xarchit/focusd/releases/latest/download/focusd_setup.exe" -OutFile focusd_setup.exe; ./focusd_setup.exe
```

**Command Prompt**:
```cmd
curl -L -o focusd_setup.exe "https://github.com/0xarchit/focusd/releases/latest/download/focusd_setup.exe" && focusd_setup.exe
```

The NSIS installer (`installer.nsi`) installs both binaries to `%APPDATA%\focusd`, adds the install directory to your user `PATH`, registers `focusd_daemon.exe` under the `HKCU\...\Run` autostart key, registers an entry in **Add/Remove Programs**, and immediately launches the daemon.

Open a new terminal and type `focusd` to launch the TUI.

---

## Features

### Interactive TUI
Launching `focusd` with no arguments opens a Bubble Tea / Lipgloss terminal UI with five tabs: **Dashboard**, **Stats**, **Focus**, **Limits**, **Settings**. Mouse and alt-screen are enabled; minimum terminal size is 80×24. Press `?` for the full key-binding overlay.

### Background daemon
`focusd_daemon.exe` is built with the `-H=windowsgui` linker flag so it runs with no console window. It polls the active window every **5 seconds** (configurable 1–60s), batches sessions to SQLite every **5 minutes**, persists the active session every 5 minutes (so a crash recovers cleanly), enforces retention every hour, and checks Pomodoro / break / app-limit conditions every 5 seconds.

### Per-app and per-browser tracking
The daemon reads the foreground window's title and process executable using `GetForegroundWindow`, `GetWindowTextW`, and `GetModuleBaseNameW`. App sessions are written to the `sessions` table; daily totals are upserted into `apps_daily`. When the foreground process is a recognised browser, the cleaned window **title** (not URL) is upserted into `browsing_daily`.

### Smart browser-tab grouping
Tab titles are matched against ~74 hard-coded URL/title patterns in `core/app_groups.go` and grouped under a parent category (YouTube, GitHub, LeetCode, ChatGPT, Claude, Notion, Figma, AWS, Coursera, Discord, Gmail, Reddit, etc.). Sub-entries are listed under each category in stats output.

### Default browser detection
`chrome`, `firefox`, `msedge`, `brave`, `opera`, `vivaldi`, `waterfox`, `arc`, `iexplore`, `safari`, `whale`, `yandex`, `thorium`, `librewolf`, `chromium`, `floorp`, `zen`. Add others with `focusd browser add <name>.exe`.

### Pomodoro timer
File-backed timer at `%APPDATA%\focusd\pomodoro.json` so state survives across processes. Default 25 minutes; configurable. Completion is announced through a Win32 `MessageBoxW` notification with a 10-second cooldown.

### Break reminders
When enabled, the daemon notifies after a configurable number of minutes of continuous use (default 60). The reminder can be snoozed for the global snooze duration (default 60 min).

### Daily app time limits
Set per-executable limits (`focusd limit chrome.exe 60`). The daemon notifies once when the day's usage exceeds the limit; the snooze button suppresses repeats for the snooze window. Limits reset at local midnight.

### Whitelist
Whitelisted executables are skipped entirely by the tracker (never recorded).

### Data export
Export tracking data to CSV or JSON from the TUI Settings tab. Files are saved to `%USERPROFILE%\Desktop`. You can also open the SQLite database directly at `%APPDATA%\focusd\focusd.db`.

### Auto-update
`focusd update` queries `api.github.com/repos/0xarchit/focusd/releases/latest`, downloads `focusd_setup.exe` for that tag, verifies its SHA-256 against `checksums.txt`, then invokes the installer silently (`/S`) with a UAC elevation prompt via `ShellExecute("runas")`. The daemon is stopped before install and restarted afterwards.

---

## Commands

```
focusd                       Launch interactive TUI (default)
focusd start                 Start the background daemon
focusd stop                  Stop the daemon (graceful via IPC, falls back to kill)
focusd status        (s)     Daemon + tracking status with today's top 5 apps
focusd stats         (st)    Full daily breakdown: top apps + grouped browsing
focusd pause         (p)     Pause tracking (daemon keeps running, drops samples)
focusd resume        (r)     Resume tracking
focusd focus [min]           Start a Pomodoro timer (default 25 min)
focusd stop-timer            Stop the running Pomodoro
focusd limit <app> <min>     Set a daily time limit (omit args to list)
focusd update                Check for and install updates
focusd help          (h)     Show built-in help
focusd version       (-v)    Print version
```

### Subcommands

```
focusd retention status      Show current retention window
focusd retention set <days>  Set retention (1–30, default 7)
focusd retention reset       Reset retention to default

focusd autostart status      Show autostart state
focusd autostart enable      Add daemon to HKCU\...\Run
focusd autostart disable     Remove daemon from HKCU\...\Run

focusd path status           Show whether install dir is on PATH
focusd path enable           Append install dir to user PATH
focusd path disable          Remove install dir from user PATH

focusd browser list          Show user-defined browsers
focusd browser add <exe>     Add a custom browser executable
focusd browser remove <exe>  Remove a custom browser
```

`focusd --daemon` is the entry the daemon binary uses to enter its tracker loop; you should not invoke it directly.

---

## Architecture

```
┌─────────────────┐   TCP 127.0.0.1:48321   ┌──────────────────────┐
│   focusd.exe    │ ────── "stop" / ──────► │  focusd_daemon.exe   │
│  (CLI + TUI)    │ ◄──── "flush" / "ok" ── │  (windowsgui, no UI) │
└────────┬────────┘                         └──────────┬───────────┘
         │                                             │
         │ reads / writes                              │ reads / writes
         ▼                                             ▼
┌────────────────────────────────────────────────────────────────────┐
│                       %APPDATA%\focusd\                            │
│  focusd.db (SQLite WAL)   config.json   browsers.json              │
│                           pomodoro.json                            │
└────────────────────────────────────────────────────────────────────┘
```

| Package | Purpose |
|---------|---------|
| `cmd/focusd` | Entry point for the CLI/TUI binary. Calls `system.AttachParentConsole`, `system.CleanupOldBinary`, then `cli.Run`. |
| `cmd/focusd_daemon` | Entry point for the silent daemon. Calls `cli.RunDaemon`. |
| `cmd/release_notes` | Build-time helper that generates `release_notes.md` from `git log` via the GitHub Models API. |
| `cli` | Command parsing, interactive menu, update flow, status / stats / export commands. |
| `core` | Tracker loop, IPC server/client, app classification, browser-tab grouping, title cleaner, Pomodoro state, notifications. |
| `storage` | SQLite schema and queries, retention enforcement, browser registry, durable config. |
| `system` | All Windows-specific code: Win32 syscalls (`user32`, `kernel32`, `psapi`), process enumeration, registry (autostart + PATH), user-config file, daemon process spawn. Stubs exist for non-Windows builds so `go build` still passes on other OSes (the daemon refuses to run). |
| `tui` | Bubble Tea / Lipgloss model, splash, tabs, dashboard, stats, focus, limits, settings panels. |
| `ui` | ANSI colours, table renderer, duration formatter for the classic CLI commands. |

### IPC
The daemon listens on **`127.0.0.1:48321`** and accepts two text commands:

| Command | Effect |
|---------|--------|
| `stop` | Acknowledge with `ok`, flush state, exit. |
| `flush` | Close the current session, write batched sessions, persist active-session checkpoint, reply `ok`. |

The CLI calls `flush` before reading stats so reports never miss the in-flight session.

### Tickers inside the tracker loop
| Ticker | Interval | What it does |
|--------|----------|--------------|
| `pollTicker` | 5s (configurable) | Sample the foreground window |
| `batchTicker` | 5 min | Write batched sessions to SQLite |
| `persistTicker` | 5 min | Snapshot the active session for crash recovery |
| `retentionTicker` | 1 hour | Delete rows older than retention window |
| `focusTicker` | 5s | Pomodoro completion, break reminders, app-limit notifications |

---

## Data & Configuration

Everything lives under `%APPDATA%\focusd\`.

| File | Contents |
|------|----------|
| `focusd.db` | SQLite database (WAL mode, incremental autovacuum, NORMAL synchronous) |
| `config.json` | User config: whitelist, app limits, Pomodoro minutes, break reminder, snooze duration, custom browsers |
| `pomodoro.json` | Current Pomodoro state (active flag, start time, duration, notified flag) |

### Database schema

```sql
config           (key, value, updated_at)
sessions         (id, app_name, exe_name, window_title, start_time, end_time, duration_secs, date)
apps_daily       (id, date, app_name, exe_name, total_duration_secs, open_count)   -- UNIQUE(date, exe_name)
browsing_daily   (id, date, domain_or_title, total_duration_secs, open_count)      -- UNIQUE(date, domain_or_title)
active_session   (id=1, app_name, exe_name, window_title, start_time, last_seen, date)
```

Indexes: `idx_sessions_date`, `idx_sessions_start`, `idx_apps_daily_date`.

### Defaults & ranges (from `storage/config.go` and `system/userconfig.go`)

| Setting | Default | Range |
|---------|---------|-------|
| Retention (days) | 7 | 1 – 30 |
| Tracking poll interval (s) | 5 | 1 – 60 |
| Pomodoro (min) | 25 | > 0 |
| Break reminder (min) | 60 | > 0 |
| Snooze (min) | 60 | > 0 |

### Registry keys written

- `HKCU\Software\Microsoft\Windows\CurrentVersion\Run\focusd` — autostart entry (full quoted path to `focusd_daemon.exe`).
- `HKCU\Environment\Path` — append-only modification of user PATH.
- `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\Focusd` — written by the NSIS installer for **Add/Remove Programs**.

---

## Privacy

focusd is built for privacy by design:

- **No telemetry.** The only network call is `GET https://api.github.com/repos/0xarchit/focusd/releases/latest` when you run `focusd update` (and the subsequent download from `github.com` if you confirm). Grep for `http.` if you don't believe it.
- **Local-only storage.** All data lives in `%APPDATA%\focusd\focusd.db`. The SQLite file is standard; query it with any SQL tool.
- **Titles, not URLs.** Browser tracking records the window title only. There is no browser extension, no HTTP interception, no DNS sniffing.
- **Whitelist & pause** give you full control. Whitelisted executables are never recorded.
- **No background privilege.** Both binaries run under your user account. The only elevation request is for the auto-updater (NSIS installer needs admin to write to `%APPDATA%` files held by the running daemon).
- **Open source.** Full source code available. Audit it yourself.

---

## Building from Source

**Requirements:** Go **1.26+**, Windows (the daemon only runs on Windows). NSIS is required to build the installer.

```powershell
git clone https://github.com/0xarchit/focusd.git
cd focusd

# CLI / TUI binary
$env:CGO_ENABLED = "0"
go build -ldflags="-s -w -X 'focusd/system.Version=dev-build'" -trimpath -o focusd.exe ./cmd/focusd

# Silent background daemon
go build -ldflags="-s -w -H=windowsgui -X 'focusd/system.Version=dev-build'" -trimpath -o focusd_daemon.exe ./cmd/focusd_daemon

# NSIS installer (optional)
makensis /DVERSION=dev-build installer.nsi
```

Replace `dev-build` with the release version. The `-X 'focusd/system.Version=...'` ldflag is read at runtime by `focusd version` and the update checker.

### CI

| Workflow | Trigger | Output |
|----------|---------|--------|
| `.github/workflows/release.yml` | Push of a `vX.Y.Z` tag | Builds both binaries, NSIS installer, SHA-256 `checksums.txt`, AI-generated release notes via GitHub Models (`gpt-4o-mini`), publishes a GitHub Release. |
| `.github/workflows/test_build.yml` | Push to `test` branch | Builds an unsigned dev installer, uploads as a 5-day artifact. |
| `.github/workflows/deploy-site.yml` | Push to `main`/`master` touching `website/**` | Deploys the static site under `website/` to GitHub Pages (`focusd.0xarchit.is-a.dev`). |

---

## Uninstalling

Use **Settings → Apps → Installed apps → Focusd → Uninstall**, which invokes the NSIS uninstaller. It stops the daemon, removes the binaries, deletes the autostart registry entry, and removes the install directory from your user `PATH`.

To delete your usage history as well, remove `%APPDATA%\focusd\` manually after uninstalling.

---

## Contributing

Contributions welcome. See [CONTRIBUTING.md](.github/CONTRIBUTING.md) and [CODE_OF_CONDUCT.md](.github/CODE_OF_CONDUCT.md). Security issues: see [SECURITY.md](.github/SECURITY.md).

## License

MIT License — see [LICENSE](LICENSE).

---

<p align="center">
  <sub>Built by <a href="https://github.com/0xarchit">@0xarchit</a></sub>
</p>
