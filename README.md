<p align="center"><img src="https://raw.githubusercontent.com/BackEddy/mac/main/docs/icon.png" width="112" alt="Backeddy"></p>

<h1 align="center">Backeddy Server</h1>

<p align="center"><b>Turn any old PC into a backup server for your Macs (Time Machine) and Windows PCs.</b></p>

> **Status: community preview (0.8).** Setup, the server screen, alerts and the web panel speak **English, Português and Español**. New to all this? The [Backeddy for Mac page](https://github.com/BackEddy/mac#readme) explains Time Machine, backup servers and virtual machines in plain words.

## Get it

- **A spare PC or laptop (it gets erased):** write the USB drive with the [Backeddy for Mac](https://github.com/BackEddy/mac/releases/latest) app (or balenaEtcher with `backeddy-server-*.img.xz` from [Releases](https://github.com/BackEddy/server/releases/latest)), boot the PC from it and follow the graphical installer. UEFI PCs (about 2012 onward); cable or Wi-Fi.
- **A Windows PC that's always on (nothing gets erased):** download and run **[Instalar-Backeddy.exe](https://github.com/BackEddy/server/releases/latest/download/Instalar-Backeddy.exe)**. It creates a small virtual machine (Hyper-V, or VirtualBox on Windows Home) that boots the same installer.
- **A fresh Debian 12/13:** the install script below.

> ⚠️ **"Windows protected your PC":** `Instalar-Backeddy.exe` isn't digitally signed yet (we're working on it), so Windows SmartScreen warns about it. Click **More info › Run anyway**. It's built by GitHub Actions straight from the open-source code in [`windows/`](windows/). · *PT: o instalador ainda não tem assinatura digital; clique em **Mais informações › Executar assim mesmo**. · ES: el instalador aún no tiene firma digital; haz clic en **Más información › Ejecutar de todas formas**.*

## What it sets up

- **Time Machine over SMB**, tuned for macOS — the server shows up by itself in Time Machine settings.
- **Kopia Repository Server** for Backeddy for Windows (and any Kopia client).
- One or more **dedicated backup disks** (btrfs: checksums, compression). **Two or more are mirrored** by default; if one dies, backups keep running and `backeddy replace-disk` rebuilds the mirror. The installer also offers *More space* (disks summed) and *Everything together*, or picking each disk's role by hand — each choice shows its space and its risk. `backeddy mirror` / `unmirror` switch later without reinstalling.
- **Recover**: the configuration is kept on the backup disks, so if the system disk dies, the USB stick reinstalls the system and the Macs and PCs carry on where they left off.
- The installer checks **disk health (SMART)** and leaves failing disks out.
- **Alerts**: when something is wrong (dead mirror disk, failing disk, backups stopped, disk full, a Mac without backups for 7 days) the server **beeps every minute** until someone acknowledges it — in the web panel (`http://name.local`), a key on the server's keyboard, or `backeddy ack`; quiet hours at night.
- **Space is shared automatically**: PCs use what they need; Time Machine gets the rest (minus headroom), recalculated hourly.
- **Already have a Windows server/NAS?** `Instalar-Backeddy.exe` creates a ready Backeddy VM (Hyper-V, or VirtualBox on Windows Home) — nothing gets formatted — and a tray watcher that beeps on the Windows side.
- **Wi-Fi when there's no cable** (USB stick): the installer lists the networks and asks for the password; cable always wins when plugged in. Change networks later on the server itself with `sudo backeddy wifi`.
- **Old laptop as a server**: lid closed is fine, the battery works as a UPS, the server beeps when the power goes out and shuts down safely at 10%.
- **Never sleeps**, **automatic security updates**, **SMART disk monitoring**.
- The server's own screen shows its health and how to connect (Mac, Windows, SSH), always up to date.
- `backeddy status` / `wifi` / `add-pc` / `passwd` / `replace-disk` / `add-disk` / `mirror` helper (status, alerts and wifi in the server's language; the other admin commands are still in Portuguese).

## For app developers

The server's agent listens on port 80 (local network). Endpoints used by the Backeddy apps:

| Endpoint | Auth | What it does |
| --- | --- | --- |
| `GET /api/hello` | none | `{name, slug, version}` |
| `GET /api/beep` | none | `{beep, alerts}`: should a watcher beep now |
| `POST /api/pair` `{"password","name"}` | server password in the body | Creates (or renews) a Kopia server user for a PC and returns `{pc, username, hostname, password, url, urlIP, fingerprint, name}` |
| `GET /api/status` | Basic `backeddy:PASSWORD` or session | `{name, alerts[{ID,Text}], status, quietStart, quietEnd}` |
| `GET /api/pcs` | same | `{pcs: [...]}` |
| `POST /api/ack` `{"days"}` · `POST /api/quiet` | same | Acknowledge alerts · set quiet hours |

Wrong passwords are rate-limited (10 per 10 minutes per address). Kopia clients connect to `url` (or `urlIP`) and must pin `fingerprint`.

## Install (Debian 12 or 13, as root)

```bash
wget -qO- https://github.com/BackEddy/server/archive/refs/heads/main.tar.gz | tar xz
cd server-main
bash install.sh --name "Backup da Casa" --disk /dev/sdb                  # one disk (ERASED)
bash install.sh --name "Backup da Casa" --disk /dev/sdb --disk /dev/sdc  # two disks, mirrored
bash install.sh --lang en --name "Home Backup" --disk /dev/sdb           # server screen and alerts in English
```

Test guides (Hyper-V, in Portuguese): [USB installer image](docs/TESTING-pendrive.pt-BR.md) · [install script](docs/TESTING.pt-BR.md) · [disks: mirror, failure, replacement](docs/TESTING-discos.pt-BR.md)

The USB image is built by GitHub Actions (`image/build.sh`, workflow *Imagem do pendrive*) from Debian 13 with mmdebstrap; the installer is a small Go program (`installer/`) that draws its own graphical screens on the framebuffer (no X/Wayland), with a text-mode fallback. Its screens can be previewed with `go run ./installer --shots DIR`. Bundled: Inter font (SIL OFL), golang.org/x/image and x/text (BSD), vendored.

## Part of Backeddy

| | |
| --- | --- |
| 🍎 [Backeddy for Mac](https://github.com/BackEddy/mac) | Follow and verify Time Machine backups, write the server USB drive, set up Time Machine on the server |
| 🪟 Backeddy for Windows | Coming soon |
| 🖥️ **Backeddy Server** | This repository |

Free and open source (MIT) · Made by [Eddy Antonini](https://eddyantonini.com.br) · Support: PIX `pix@xwa.com.br` · [GitHub Sponsors](https://github.com/sponsors/lordironballs)
