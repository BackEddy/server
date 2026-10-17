<p align="center"><img src="https://raw.githubusercontent.com/BackEddy/mac/main/docs/icon.png" width="112" alt="Backeddy"></p>

<h1 align="center">Backeddy Server</h1>

<p align="center"><b>Turn any old PC into a backup server for your Macs (Time Machine) and Windows PCs.</b></p>

> **Status: early preview (0.6).** Two ways to install: the **USB installer image** (plug into an old PC, follow a graphical installer with mouse or keyboard, type a password, done — UEFI and wired network only for now) or the **install script** on a fresh Debian. Soon the Backeddy app will create the USB stick for you.

## What it sets up

- **Time Machine over SMB**, tuned for macOS — the server shows up by itself in Time Machine settings.
- **Kopia Repository Server** for Backeddy for Windows (and any Kopia client).
- One or more **dedicated backup disks** (btrfs: checksums, compression). **Two or more are mirrored** by default; if one dies, backups keep running and `backeddy replace-disk` rebuilds the mirror. The installer also offers *More space* (disks summed) and *Everything together*, or picking each disk's role by hand — each choice shows its space and its risk. `backeddy mirror` / `unmirror` switch later without reinstalling.
- **Recover**: the configuration is kept on the backup disks, so if the system disk dies, the USB stick reinstalls the system and the Macs and PCs carry on where they left off.
- The installer checks **disk health (SMART)** and leaves failing disks out.
- **Alerts**: when something is wrong (dead mirror disk, failing disk, backups stopped, disk full, a Mac without backups for 7 days) the server **beeps every minute** until someone acknowledges it — in the web panel (`http://name.local`), a key on the server's keyboard, or `backeddy ack`; quiet hours at night.
- **Space is shared automatically**: PCs use what they need; Time Machine gets the rest (minus headroom), recalculated hourly.
- **Already have a Windows server/NAS?** `windows/Instalar Backeddy.cmd` creates a ready Backeddy VM (Hyper-V, or VirtualBox on Windows Home) — nothing gets formatted — and a tray watcher that beeps on the Windows side.
- **Wi-Fi when there's no cable** (USB stick): the installer lists the networks and asks for the password; cable always wins when plugged in. Change networks later on the server itself with `sudo backeddy wifi`.
- **Old laptop as a server**: lid closed is fine, the battery works as a UPS, the server beeps when the power goes out and shuts down safely at 10%.
- **Never sleeps**, **automatic security updates**, **SMART disk monitoring**.
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
