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

# Entra direto no instalador no tty1 (sem login), com fonte grande
mkdir -p "$R/etc/systemd/system/getty@tty1.service.d"
cat > "$R/etc/systemd/system/getty@tty1.service.d/autologin.conf" <<'EOF'
[Service]
ExecStart=
ExecStart=-/sbin/agetty --autologin root --noclear %I $TERM
EOF
cat > "$R/root/.bash_profile" <<'EOF'
if [ "$(tty)" = /dev/tty1 ]; then
  setfont /usr/share/consolefonts/Uni3-TerminusBold32x16.psf.gz 2>/dev/null
  export LANG=C.UTF-8
  /usr/local/bin/backeddy-installer
  echo
  echo "Instalador encerrado. Para abrir de novo: backeddy-installer"
fi
EOF
# root sem senha só no instalador (o sistema instalado tem o usuário admin com a senha escolhida)
chroot "$R" passwd -d root >/dev/null

rm -f "$R"/etc/ssh/ssh_host_* 2>/dev/null || true
rm -rf "$R"/var/cache/apt/archives/*.deb "$R"/var/lib/apt/lists/*_Packages
