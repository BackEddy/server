<p align="center"><img src="https://raw.githubusercontent.com/BackEddy/mac/main/docs/icon.png" width="112" alt="Backeddy"></p>

<h1 align="center">Backeddy Server</h1>

<p align="center"><b>Turn any old PC into a backup server for your Macs (Time Machine) and Windows PCs.</b></p>

> **Status: early preview (0.4).** Two ways to install: the **USB installer image** (plug into an old PC, follow a Mac-style graphical installer, type a password, done — UEFI and wired network only for now) or the **install script** on a fresh Debian. Soon the Backeddy app will create the USB stick for you.

## What it sets up

- **Time Machine over SMB**, tuned for macOS — the server shows up by itself in Time Machine settings.
- **Kopia Repository Server** for Backeddy for Windows (and any Kopia client).
- One or more **dedicated backup disks** (btrfs: checksums, compression). **Two or more are mirrored** automatically; if one dies, backups keep running and `backeddy replace-disk` rebuilds the mirror.
- **Never sleeps** (laptop lid closed is fine), **automatic security updates**, **SMART disk monitoring**.
- The server's own screen shows its health and how to connect (Mac, Windows, SSH), always up to date.
- `backeddy status` / `add-pc` / `passwd` / `replace-disk` / `add-disk` helper.

## Install (Debian 12 or 13, as root)

```bash
wget -qO- https://github.com/BackEddy/server/archive/refs/heads/main.tar.gz | tar xz
cd server-main
bash install.sh --name "Backup da Casa" --disk /dev/sdb                  # one disk (ERASED)
bash install.sh --name "Backup da Casa" --disk /dev/sdb --disk /dev/sdc  # two disks, mirrored
```

Test guides (Hyper-V, in Portuguese): [USB installer image](docs/TESTING-pendrive.pt-BR.md) · [install script](docs/TESTING.pt-BR.md) · [disks: mirror, failure, replacement](docs/TESTING-discos.pt-BR.md)

The USB image is built by GitHub Actions (`image/build.sh`, workflow *Imagem do pendrive*) from Debian 13 with mmdebstrap; the installer is a small Go program (`installer/`) that draws its own graphical screens on the framebuffer (no X/Wayland), with a text-mode fallback. Its screens can be previewed with `go run ./installer --shots DIR`. Bundled: Inter font (SIL OFL), golang.org/x/image and x/text (BSD), vendored.

## Part of Backeddy

| | |
| --- | --- |
| 🍎 [Backeddy for Mac](https://github.com/BackEddy/mac) | Force, follow and verify Time Machine backups |
| 🪟 Backeddy for Windows | Coming soon |
| 🖥️ **Backeddy Server** | This repository |

Free and open source (MIT) · Made by [Eddy Antonini](https://eddyantonini.com.br) · Support: PIX `pix@xwa.com.br` · [GitHub Sponsors](https://github.com/sponsors/lordironballs)
