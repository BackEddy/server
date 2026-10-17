#!/bin/bash
# Ajustes no sistema do servidor (chamado pelo mmdebstrap com o diretório do sistema em $1).
set -euo pipefail
R="$1"

# Backeddy Server (script testado + comandos + configurações)
mkdir -p "$R/opt/backeddy-server"
cp -a "$REPO/install.sh" "$REPO/bin" "$REPO/config" "$REPO/LICENSE" "$REPO/VERSION" "$R/opt/backeddy-server/"
install -m 0644 "$REPO/config/backeddy-firstboot.service" "$R/etc/systemd/system/backeddy-firstboot.service"
install -m 0755 "$REPO/bin/backeddy" "$R/usr/local/bin/backeddy"

# Rede por cabo: DHCP em qualquer placa
mkdir -p "$R/etc/systemd/network"
cat > "$R/etc/systemd/network/20-cabo.network" <<'EOF'
[Match]
Name=en* eth*

[Network]
DHCP=yes
MulticastDNS=no
EOF

# o anúncio na rede (mDNS) é do Avahi: o resolved não pode disputar a porta 5353
mkdir -p "$R/etc/systemd/resolved.conf.d"
printf '[Resolve]\nMulticastDNS=no\nLLMNR=no\n' > "$R/etc/systemd/resolved.conf.d/backeddy.conf"

chroot "$R" systemctl enable systemd-networkd systemd-resolved ssh backeddy-firstboot >/dev/null 2>&1

# Nome provisório, fuso e idioma (o install.sh troca o nome no primeiro boot)
echo backeddy > "$R/etc/hostname"
printf '127.0.0.1\tlocalhost\n127.0.1.1\tbackeddy\n::1\tlocalhost ip6-localhost ip6-loopback\n' > "$R/etc/hosts"
ln -sf /usr/share/zoneinfo/America/Sao_Paulo "$R/etc/localtime"
echo America/Sao_Paulo > "$R/etc/timezone"
sed -i -e 's/^# *pt_BR.UTF-8/pt_BR.UTF-8/' -e 's/^# *en_US.UTF-8/en_US.UTF-8/' "$R/etc/locale.gen"
chroot "$R" locale-gen >/dev/null
echo 'LANG=pt_BR.UTF-8' > "$R/etc/default/locale"

# Console com a fonte padrão (pequena), com acentos e os símbolos usados na tela do servidor
cat > "$R/etc/default/console-setup" <<'EOF'
ACTIVE_CONSOLES="/dev/tty[1-6]"
CHARMAP="UTF-8"
CODESET="Uni2"
FONTFACE="Fixed"
FONTSIZE="8x16"
EOF
printf '\n  Backeddy Server\n\n  Iniciando...\n\n' > "$R/etc/issue"

# A tela de login só aparece depois do primeiro boot e da atualização da tela de situação
mkdir -p "$R/etc/systemd/system/getty@tty1.service.d"
install -m 0644 "$REPO/config/getty-backeddy.conf" "$R/etc/systemd/system/getty@tty1.service.d/backeddy.conf"

# GRUB: o instalador roda grub-install/update-grub; aqui só os padrões e os ganchos de kernel.
# loglevel=3 e systemd.ssh_auto=no: boot com a tela limpa (sem avisos técnicos, como os de VSOCK no Hyper-V)
cat > "$R/etc/default/grub" <<'EOF'
GRUB_DEFAULT=0
GRUB_TIMEOUT=2
GRUB_DISTRIBUTOR="Backeddy Server"
GRUB_CMDLINE_LINUX_DEFAULT="quiet loglevel=3 systemd.ssh_auto=no"
GRUB_CMDLINE_LINUX=""
GRUB_DISABLE_OS_PROBER=true
EOF
for d in postinst.d postrm.d; do
  mkdir -p "$R/etc/kernel/$d"
  if [ ! -e "$R/etc/kernel/$d/zz-update-grub" ]; then
    printf '#!/bin/sh\n# Backeddy: mantém o menu de boot em dia quando o kernel é atualizado\nif command -v update-grub >/dev/null && [ -e /boot/grub/grub.cfg ]; then exec update-grub; fi\n' \
      > "$R/etc/kernel/$d/zz-update-grub"
    chmod 0755 "$R/etc/kernel/$d/zz-update-grub"
  fi
done

# Atualizações automáticas de segurança
printf 'APT::Periodic::Update-Package-Lists "1";\nAPT::Periodic::Unattended-Upgrade "1";\n' > "$R/etc/apt/apt.conf.d/20auto-upgrades"

# Identidade: cada instalação gera a sua (o instalador recria chaves SSH e machine-id)
rm -f "$R"/etc/ssh/ssh_host_*
: > "$R/etc/machine-id"
rm -f "$R/var/lib/dbus/machine-id"

# Limpeza
rm -rf "$R"/var/cache/apt/archives/*.deb "$R"/var/lib/apt/lists/*_Packages
