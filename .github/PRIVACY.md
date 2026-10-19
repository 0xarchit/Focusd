# Privacy Policy

**Focusd** — a privacy-first screen time tracker for Windows.

_Last updated: 16 October 2026_

## Summary

Focusd does not collect, transmit, sell, or share any data. There is no account,
no server, and no analytics. Everything Focusd records stays on your own
computer, inside a single SQLite file that you can open, read, edit, or delete
at any time.

Focusd makes network requests for exactly one purpose: the update check you
initiate yourself.

---

## 1. What we collect

**Nothing.**

Focusd has no telemetry, no analytics, no crash reporting, no usage statistics,
no advertising identifiers, and no user accounts. There is no code in the
application that sends your activity data anywhere.

## 2. What Focusd records, and where it lives

Because a usage tracker is useless without recording something, Focusd does
write data to your local disk. This data never leaves your machine unless you
explicitly move it.

All of it is stored in a single folder:

```
%APPDATA%\focusd\
```

On a typical Windows install that resolves to
`C:\Users\<you>\AppData\Roaming\focusd\`. It contains:

| File | Contents |
|---|---|
| `focusd.db` | A SQLite database holding your usage history — the app you used, how long you used it, and the window title it had — plus the retention period |
| `config.json` | Your settings: daily limits, whitelisted apps, display and site name overrides, and the title-redaction switch |
| `ipc.key` | A random key used to authenticate commands between the Focusd command line and the Focusd daemon, both running on your machine |

### About window titles

Window titles are the most sensitive thing Focusd records. A title can reveal a
banking page, an email subject, a document name, or a search term.

Focusd takes two specific steps to reduce this:

- **Private and incognito windows are detected and their titles are discarded**
  before anything is written to the database. The time spent is still counted;
  only the title text is dropped.
- **Title redaction can be switched on** (set `"redact_titles": true` in
  `config.json`; off by default), in which case no window text is stored at
  all — only which app was used and for how long.

Both are documented in the source. Neither replaces your own judgement: if a
title is sensitive to you, use redaction.

### Removing your data

- **Retention** — history older than the retention period is deleted
  automatically. The default is **7 days**; the range is 1 to 365 days.
  Change it with `focusd retention set <days>`.
- **Uninstalling leaves your data in place** rather than deleting it, so your
  history is not lost by accident. Delete the `%APPDATA%\focusd` folder
  yourself if you want it gone.
- Deleting `focusd.db` removes all recorded history immediately. There is no
  backup held anywhere else, because there is nowhere else it is held.

## 3. Network requests

Focusd contacts the network for **one reason only: checking for and downloading
updates.**

These requests happen **only when you personally run `focusd update`**. Focusd
does not phone home in the background, on a schedule, or at launch. There is no
automatic update check.

When you do run it, Focusd contacts GitHub, which hosts the project:

| Purpose | Endpoint |
|---|---|
| Look up the newest published version | `api.github.com/repos/0xarchit/focusd/releases/latest` |
| Download the published checksum list | `github.com/0xarchit/focusd/releases/download/...` |
| Download the new installer | `github.com/0xarchit/focusd/releases/download/...` |

These are ordinary outbound HTTPS requests to GitHub. **No information about
you or your computer is sent with them** — no API key, no identifier, no usage
data. There is nothing in the request to identify you beyond what any web
request reveals about an IP address, which is inherent to using the internet.

The downloaded installer is verified against the published SHA-256 checksum
before it is applied.

### Third parties

GitHub, Inc. receives these requests as the content host and network provider.
Focusd's own project tooling used during its build process — not shipped to you,
and not run by the application — contacts an AI model provider to help draft
release notes from the commit history.

There are no other third parties involved.

## 4. Permissions

Focusd runs entirely on your machine and needs no network-facing services. It
does not open ports, run a server, or request administrator privileges.

- **Storage** — all data is kept under your own user profile.
- **IPC** — the command line talks to the daemon over a loopback port on your own
  machine. A per-install key file is required, so other local programs cannot
  issue commands to it.
- **Installation** — Focusd installs per-user and does not require elevation.
- **Registry** — auto-start and PATH integration use your own user registry
  hive, not system-wide entries.

## 5. Children

Focusd is a general-purpose utility suitable for all ages. Because it collects
nothing and transmits nothing, it is safe for anyone to use, including children.

## 6. Your control

Everything remains yours:

- Turn tracking off entirely with `focusd stop`, and pause it with
  `focusd pause`.
- Delete all recorded history by removing `focusd.db`.
- Inspect the database directly — it is an ordinary SQLite file.
- Uninstall at any time; your data directory is left untouched for you to keep
  or remove.

## 7. Changes to this policy

If this policy changes, the updated version will be committed to the public
repository, and the "Last updated" date above will change. Because Focusd
collects nothing, there is no list of affected users to notify.

## 8. Source code

Focusd is open source under the MIT License. The complete source is public, so
every statement in this policy can be verified against the code rather than
taken on trust.

## 9. Contact

Questions about this policy, or a privacy concern:

- Repository: https://github.com/0xarchit/Focusd
- Open an issue on that repository

---

**In short: your activity data is written to one file on your computer and
never leaves it. Focusd's only network activity is the update check you ask
for.**