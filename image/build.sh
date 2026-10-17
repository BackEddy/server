#!/bin/bash
# Monta a imagem do pendrive do Backeddy Server (roda como root num Debian 13, ex.: GitHub Actions).
#
#   1. sistema do servidor  (mmdebstrap → rootfs.tar.zst): o que vai para o disco do PC
#   2. sistema do instalador (mmdebstrap → diretório): roda do pendrive e abre o instalador
#   3. imagem de disco: ESP (shim + grub assinados) | instalador (ext4) | BACKEDDY (FAT, plano)
#
# Saída em out/: backeddy-server-VERSAO.img.xz (pendrive), .vhdx (Hyper-V) e SHA256SUMS.
# Sem loop devices nem montagens: os sistemas de arquivos são criados direto de diretórios.
set -euo pipefail

REPO=$(cd "$(dirname "$0")/.." && pwd)
export REPO
VER=$(cat "$REPO/VERSION" 2>/dev/null || echo dev)
W="$REPO/work"
OUT="$REPO/out"
SUITE=trixie
NAME="backeddy-server-$VER"

log() { printf '\n\033[1;36m== %s\033[0m\n' "$*"; }

[ "$(id -u)" = 0 ] || { echo "Rode como root."; exit 1; }
rm -rf "$W" "$OUT"
mkdir -p "$W" "$OUT"

MIRRORS=(
  "deb http://deb.debian.org/debian $SUITE main non-free-firmware"
  "deb http://deb.debian.org/debian $SUITE-updates main non-free-firmware"
  "deb http://security.debian.org/debian-security $SUITE-security main non-free-firmware"
)

log "Chave do repositório do Kopia"
KOPIA_KEY=/usr/share/keyrings/kopia-keyring.gpg
curl -fsSL https://kopia.io/signing-key | gpg --dearmor --yes -o "$KOPIA_KEY"
export KOPIA_KEY
KOPIA_SRC="deb [signed-by=$KOPIA_KEY] http://packages.kopia.io/apt/ stable main"

log "Vigia do servidor (Go)"
( cd "$REPO/agent" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -buildvcs=false -trimpath -ldflags "-s -w" -o "$W/backeddy-agent" . )
export W

log "Instalador (Go)"
( cd "$REPO/installer" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -buildvcs=false -trimpath -ldflags "-s -w -X main.version=$VER" -o "$W/backeddy-installer" . )

FIRMWARE="firmware-linux-free,firmware-misc-nonfree,firmware-realtek"
# Wi-Fi (0.7): firmwares das placas mais comuns + iwd (conecta) + iw (lista as redes)
FIRMWARE="$FIRMWARE,firmware-iwlwifi,firmware-atheros,firmware-brcm80211,firmware-mediatek,iwd,iw,wireless-regdb"

log "1/3 Sistema do servidor"
TARGET_PKGS="linux-image-amd64,systemd-sysv,systemd-resolved,udev,kmod,dbus,sudo,openssh-server,\
samba,avahi-daemon,smartmontools,unattended-upgrades,btrfs-progs,curl,ca-certificates,gnupg,openssl,kopia,\
grub-efi-amd64-bin,grub-efi-amd64-signed,shim-signed,grub2-common,efibootmgr,\
locales,console-setup,kbd,iproute2,iputils-ping,less,nano,wget,zstd,hyperv-daemons,$FIRMWARE"
mmdebstrap --variant=important --include="$TARGET_PKGS" \
  --setup-hook='mkdir -p "$1/usr/share/keyrings" && cp "$KOPIA_KEY" "$1/usr/share/keyrings/"' \
  --customize-hook='bash "$REPO/image/customize-target.sh" "$1"' \
  "$SUITE" "$W/rootfs.tar.zst" "${MIRRORS[@]}" "$KOPIA_SRC"
ls -lh "$W/rootfs.tar.zst"

log "2/3 Sistema do instalador"
INST_PKGS="linux-image-amd64,systemd-sysv,udev,kmod,util-linux,gdisk,parted,dosfstools,e2fsprogs,btrfs-progs,\
zstd,tar,kbd,console-setup-linux,iproute2,iputils-ping,procps,less,smartmontools,shim-signed,grub-efi-amd64-signed,$FIRMWARE"
INST="$W/installer-root"
mmdebstrap --variant=important --include="$INST_PKGS" \
  --customize-hook='bash "$REPO/image/customize-installer.sh" "$1"' \
  "$SUITE" "$INST" "${MIRRORS[@]}"
install -D -m 0644 "$W/rootfs.tar.zst" "$INST/opt/backeddy/rootfs.tar.zst"
install -D -m 0755 "$W/backeddy-installer" "$INST/usr/local/bin/backeddy-installer"

log "Inicialização do pendrive (shim + grub assinados)"
KVER=$(basename "$(ls "$INST"/boot/vmlinuz-* | sort -V | tail -1)" | sed 's/^vmlinuz-//')
mkdir -p "$INST/boot/grub"
cat > "$INST/boot/grub/grub.cfg" <<CFG
set timeout=3
set default=0
insmod all_video
menuentry "Backeddy Server — instalar" {
  search --no-floppy --label backeddy-inst --set=root
  linux /boot/vmlinuz-$KVER root=PARTLABEL=backeddy-inst rw quiet loglevel=3 systemd.ssh_auto=no
  initrd /boot/initrd.img-$KVER
}
menuentry "Desligar" { halt }
CFG
ESP="$W/esp"
mkdir -p "$ESP/EFI/BOOT" "$ESP/EFI/debian"
cp "$INST/usr/lib/shim/shimx64.efi.signed" "$ESP/EFI/BOOT/BOOTX64.EFI"
cp "$(ls "$INST"/usr/lib/shim/mmx64.efi* | head -1)" "$ESP/EFI/BOOT/mmx64.efi"
cp "$INST/usr/lib/grub/x86_64-efi-signed/grubx64.efi.signed" "$ESP/EFI/BOOT/grubx64.efi"
# o grub assinado procura a configuração em /EFI/debian; o cfg só aponta para o ext4 do instalador
STUB='search --no-floppy --label backeddy-inst --set=root
set prefix=($root)/boot/grub
configfile ($root)/boot/grub/grub.cfg'
echo "$STUB" > "$ESP/EFI/debian/grub.cfg"
echo "$STUB" > "$ESP/EFI/BOOT/grub.cfg"

log "3/3 Imagem de disco"
ROOT_MB=$(( $(du -sm "$INST" | cut -f1) * 115 / 100 + 300 ))
ESP_MB=128
PLAN_MB=32
mkfs.ext4 -q -F -L backeddy-inst -d "$INST" "$W/root.ext4" "${ROOT_MB}M"
mkfs.vfat -C -F 32 -n EFI "$W/esp.vfat" $((ESP_MB * 1024)) >/dev/null
mcopy -s -i "$W/esp.vfat" "$ESP/EFI" ::/
mkfs.vfat -C -n BACKEDDY "$W/plan.vfat" $((PLAN_MB * 1024)) >/dev/null
printf 'Backeddy Server %s\r\nEsta particao guarda o plano criado pelo app Backeddy (plan.json).\r\n' "$VER" > "$W/LEIA-ME.txt"
mcopy -i "$W/plan.vfat" "$W/LEIA-ME.txt" ::/

IMG="$OUT/$NAME.img"
truncate -s $(( ESP_MB + ROOT_MB + PLAN_MB + 4 ))M "$IMG"
sgdisk -Z \
  -n1:0:+${ESP_MB}M  -t1:ef00 -c1:EFI \
  -n2:0:+${ROOT_MB}M -t2:8300 -c2:backeddy-inst \
  -n3:0:+${PLAN_MB}M -t3:0700 -c3:BACKEDDY \
  "$IMG" >/dev/null
start() { sgdisk -i "$1" "$IMG" | awk '/First sector/ {print $3}'; }
put() { dd if="$1" of="$IMG" bs=4M seek=$(( $2 * 512 )) oflag=seek_bytes conv=notrunc status=none; }
put "$W/esp.vfat"  "$(start 1)"
put "$W/root.ext4" "$(start 2)"
put "$W/plan.vfat" "$(start 3)"

log "Formatos de saída (pendrive)"
qemu-img convert -f raw -O vhdx -o subformat=dynamic "$IMG" "$OUT/$NAME.vhdx"   # pendrive virtual: Hyper-V
qemu-img convert -f raw -O vdi "$IMG" "$OUT/$NAME.vdi"                           # pendrive virtual: VirtualBox
xz -T0 -6 "$IMG"   # .img.xz: o balenaEtcher grava direto

( cd "$OUT" && sha256sum ./* > SHA256SUMS )
ls -lh "$OUT"
