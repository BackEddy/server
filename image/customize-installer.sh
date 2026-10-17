#!/bin/bash
# Ajustes no sistema do instalador (roda a partir do pendrive). $1 = diretório do sistema.
set -euo pipefail
R="$1"

echo backeddy-instalador > "$R/etc/hostname"
cat > "$R/etc/fstab" <<'EOF'
PARTLABEL=backeddy-inst  /                  ext4  defaults,noatime              0 1
LABEL=BACKEDDY           /run/backeddy-plan vfat  ro,nofail,x-systemd.device-timeout=5s  0 0
EOF

# Rede por cabo
mkdir -p "$R/etc/systemd/network"
printf '[Match]\nName=en* eth*\n\n[Network]\nDHCP=yes\n' > "$R/etc/systemd/network/20-cabo.network"
chroot "$R" systemctl enable systemd-networkd >/dev/null 2>&1

# Entra direto no instalador no tty1 (sem login).
# O programa roda a partir da memória (/run): assim o pendrive pode ser retirado no fim,
# e o instalador percebe e reinicia o PC sozinho.
mkdir -p "$R/etc/systemd/system/getty@tty1.service.d"
cat > "$R/etc/systemd/system/getty@tty1.service.d/autologin.conf" <<'EOF'
[Service]
ExecStart=
ExecStart=-/sbin/agetty --autologin root --noclear %I $TERM
EOF
cat > "$R/root/.bash_profile" <<'EOF'
if [ "$(tty)" = /dev/tty1 ]; then
  export LANG=C.UTF-8
  udevadm settle -t 15 2>/dev/null   # espera o driver de vídeo e os discos
  # /run não permite executar programas: monta uma área própria na memória para o instalador
  BIN=/usr/local/bin/backeddy-installer
  mkdir -p /run/backeddy-bin
  if { mountpoint -q /run/backeddy-bin || mount -t tmpfs -o size=64m,mode=0755 tmpfs /run/backeddy-bin 2>/dev/null; } &&
     cp "$BIN" /run/backeddy-bin/; then
    BIN=/run/backeddy-bin/backeddy-installer
  fi
  "$BIN"
  echo
  echo "Instalador encerrado. Para abrir de novo: backeddy-installer"
fi
EOF
# root sem senha só no instalador (o sistema instalado tem o usuário admin com a senha escolhida)
chroot "$R" passwd -d root >/dev/null

rm -f "$R"/etc/ssh/ssh_host_* 2>/dev/null || true
rm -rf "$R"/var/cache/apt/archives/*.deb "$R"/var/lib/apt/lists/*_Packages
