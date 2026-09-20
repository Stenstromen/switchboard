# Switchboard

![Switchboard](./logo.webp)

macOS SSH tunnel manager built around OpenSSH.

Create and run local, remote, and dynamic port forwards from a native app — with menu bar control, Keychain passwords, and the OpenSSH options you already know.

- [Switchboard](#switchboard)
  - [Features](#features)
  - [Installation](#installation)
    - [Homebrew](#homebrew)
    - [Download](#download)
    - [Build from source](#build-from-source)
  - [Build and Run](#build-and-run)
  - [Configuration](#configuration)
  - [Gatekeeper note](#gatekeeper-note)

## Features

- **Local, remote & dynamic forwards** — classic `-L` / `-R` / `-D`, plus reverse dynamic forwarding
- **Multiple tunnels** — manage many profiles; connect one or connect all
- **ProxyJump** — reach hosts through one or more jump hosts
- **Auto-connect & reconnect** — bring tunnels up at launch; recover when the link drops
- **Sleep/wake recovery** — after Mac sleep or unlock, probe live sessions and bounce half-dead ssh processes
- **OpenSSH advanced options** — full `-o` coverage with Quick Help from `ssh_config(5)`
- **macOS Keychain** — store passwords securely; never write secrets into config files
- **Import / Export** — move tunnel configs between machines without leaking credentials
- **Menu bar & Dock** — show as menu bar icon, Dock icon, or both; tray brightens when tunnels are live
- **Open at login** — start Switchboard with your session
- **Pins & tags** — keep favorites on top; filter the sidebar by tag; tray menu groups tunnels by tag with Connect/Disconnect All
- **Status notifications** — optional alerts when a tunnel connects, drops, or reconnects
- **Identity & certificates** — IdentityFile, CertificateFile, agent forwarding, compression, and more

## Installation

### Homebrew

```bash
brew tap stenstromen/tap
brew install --cask switchboard
```

### Download

Download the latest macOS DMG from the [Releases page](https://github.com/Stenstromen/switchboard/releases/latest/).

### Build from source

Requires macOS 12+, Go, Node.js, and [Wails v3](https://v3.wails.io/).

```bash
git clone https://github.com/Stenstromen/switchboard.git
cd switchboard
make app
```

## Build and Run

```bash
make run          # build Switchboard.app and launch it
make demo         # fake tunnels + statuses for README screenshots
make dmg          # styled .dmg installer (current architecture)
make dmg-universal  # universal arm64 + amd64 .dmg
```

`make demo` loads `dev/demo-tunnels.json` into an isolated config (`dev/runtime/`) with `SWITCHBOARD_DEMO=1`, so the UI shows connected/retrying states without real SSH. Your normal `~/Library/Application Support/Switchboard/` data is left alone.

## Configuration

Tunnel profiles and preferences live under:

```text
~/Library/Application Support/Switchboard/
```

Passwords are stored in the macOS Keychain under `com.stenstromen.switchboard`, not on disk. Export writes tunnels and preferences only — secrets stay in Keychain.

## Gatekeeper note

Release builds are ad-hoc signed until Developer ID notarization is enabled. On first open, macOS may ask you to allow the app under **System Settings → Privacy & Security**. You can also right-click the app and choose **Open**.
