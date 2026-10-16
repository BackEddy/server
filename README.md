<p align="center"><img src="https://raw.githubusercontent.com/BackEddy/mac/main/docs/icon.png" width="112" alt="Backeddy"></p>

<h1 align="center">Backeddy Server</h1>

<p align="center"><b>Turn any old PC into a backup server for your Macs (Time Machine) and Windows PCs.</b></p>

> **Status: early preview (0.1).** Today it's an install script for a fresh Debian. The goal is a USB stick you plug into an old PC: confirm, type a password, done — created by the Backeddy app.

## What it sets up

- **Time Machine over SMB**, tuned for macOS — the server shows up by itself in Time Machine settings.
- **Kopia Repository Server** for Backeddy for Windows (and any Kopia client).
- Optional **dedicated backup disk** formatted as btrfs (checksums, compression).
- **Never sleeps** (laptop lid closed is fine), **automatic security updates**, **SMART disk monitoring**.
- `backeddy status` / `backeddy add-pc` / `backeddy passwd` helper.

## Install (Debian 12 or 13, as root)

```bash
wget -qO- https://github.com/BackEddy/server/archive/refs/heads/main.tar.gz | tar xz
cd server-main
./install.sh --name "Backup da Casa" --disk /dev/sdb   # --disk is optional and ERASES that disk
```

Step-by-step test guide (Hyper-V, in Portuguese): [docs/TESTING.pt-BR.md](docs/TESTING.pt-BR.md)

## Part of Backeddy

| | |
| --- | --- |
| 🍎 [Backeddy for Mac](https://github.com/BackEddy/mac) | Force, follow and verify Time Machine backups |
| 🪟 Backeddy for Windows | Coming soon |
| 🖥️ **Backeddy Server** | This repository |

Free and open source (MIT) · Made by [Eddy Antonini](https://eddyantonini.com.br) · Support: PIX `pix@xwa.com.br` · [GitHub Sponsors](https://github.com/sponsors/lordironballs)
