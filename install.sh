#!/bin/bash
# Backeddy Server — transforma um Debian recém-instalado num servidor de backup
# para Macs (Time Machine) e PCs (Backeddy for Windows / Kopia).
#
# Uso (como root):
#   ./install.sh                                  # pergunta nome e senha
#   ./install.sh --name "Backup da Casa" --disk /dev/sdb
#
# Opções:
#   --name  "Nome"     Nome do servidor como aparece no Mac (padrão: "Backup da Casa")
#   --disk  /dev/sdX   Disco dedicado aos backups. SERÁ APAGADO (pede confirmação).
#                      Sem --disk, os backups ficam em /srv/backeddy no disco do sistema.
#   --tm-percent N     % do espaço reservado ao Time Machine (padrão: 60)
#   --yes              Não pergunta nada (exige BACKEDDY_PASSWORD no ambiente)
#
# Pode ser rodado de novo com segurança: reaplica as configurações.
set -euo pipefail

VERSION="0.1.0"
NAME="Backup da Casa"
DISK=""
TM_PERCENT=60
ASSUME_YES=0
DATA=/srv/backeddy
ETC=/etc/backeddy
SRC="$(cd "$(dirname "$0")" && pwd)"

while [ $# -gt 0 ]; do
  case "$1" in
    --name) NAME="$2"; shift 2 ;;
    --disk) DISK="$2"; shift 2 ;;
    --tm-percent) TM_PERCENT="$2"; shift 2 ;;
    --yes) ASSUME_YES=1; shift ;;
    -h|--help) sed -n '2,17p' "$0"; exit 0 ;;
    *) echo "Opção desconhecida: $1"; exit 1 ;;
  esac
done

say()  { printf '\n\033[1;36m▸ %s\033[0m\n' "$*"; }
ok()   { printf '  \033[32m✓\033[0m %s\n' "$*"; }
die()  { printf '\n\033[1;31m✗ %s\033[0m\n' "$*" >&2; exit 1; }
# escapa um texto para usar como substituição no sed (delimitador |)
sedesc() { printf '%s' "$1" | sed -e 's/[\\&]/\\&/g' -e 's/|/-/g'; }

[ "$(id -u)" = 0 ] || die "Rode como root:  sudo ./install.sh"
# lê o os-release num subshell: ele define NAME e VERSION, que sobrescreveriam as nossas variáveis
OS_ID=$(. /etc/os-release; echo "${ID:-}")
OS_VER=$(. /etc/os-release; echo "${VERSION_ID:-}")
OS_PRETTY=$(. /etc/os-release; echo "${PRETTY_NAME:-desconhecido}")
[ "$OS_ID" = debian ] || die "Este script é para Debian (encontrado: $OS_PRETTY)."
case "$OS_VER" in 12|13) ;; *) echo "Aviso: testado no Debian 12 e 13 (este é ${OS_VER:-?})." ;; esac

printf '\n\033[1m🌀 Backeddy Server %s\033[0m\n' "$VERSION"

# ---------- Senha ----------
PASSWORD="${BACKEDDY_PASSWORD:-}"
if [ -z "$PASSWORD" ]; then
  [ "$ASSUME_YES" = 1 ] && die "Com --yes, defina BACKEDDY_PASSWORD."
  while :; do
    read -rsp "Senha do servidor (usada pelos Macs e PCs): " P1; echo
    read -rsp "Repita a senha: " P2; echo
    [ -n "$P1" ] && [ "$P1" = "$P2" ] && { PASSWORD="$P1"; break; }
    echo "As senhas não conferem ou estão vazias. Tente de novo."
  done
fi

# ---------- Disco dedicado (opcional) ----------
if [ -n "$DISK" ]; then
  [ -b "$DISK" ] || die "Disco $DISK não encontrado."
  ROOTDEV=$(lsblk -no PKNAME "$(findmnt -no SOURCE /)" 2>/dev/null || true)
  [ "/dev/$ROOTDEV" = "$DISK" ] && die "$DISK é o disco do sistema. Escolha outro disco."
  if findmnt -rno TARGET "$DATA" >/dev/null 2>&1 && [ "$(findmnt -no SOURCE "$DATA")" = "$DISK" ]; then
    ok "$DISK já está montado em $DATA (mantido)"
  else
    say "Disco de backup"
    lsblk -o NAME,SIZE,MODEL,FSTYPE,MOUNTPOINT "$DISK"
    echo
    echo "⚠️  TUDO em $DISK será apagado para sempre."
    if [ "$ASSUME_YES" != 1 ]; then
      read -rp "Para confirmar, digite APAGAR: " C
      [ "$C" = "APAGAR" ] || die "Cancelado. Nada foi alterado."
    fi
    apt-get install -y -qq btrfs-progs >/dev/null
    wipefs -a "$DISK" >/dev/null
    mkfs.btrfs -f -L backeddy "$DISK" >/dev/null
    mkdir -p "$DATA"
    UUID=$(blkid -s UUID -o value "$DISK")
    sed -i "\|[[:space:]]${DATA}[[:space:]]|d" /etc/fstab
    echo "UUID=$UUID $DATA btrfs defaults,noatime,compress=zstd:1 0 0" >> /etc/fstab
    systemctl daemon-reload
    mount "$DATA"
    ok "$DISK formatado (btrfs) e montado em $DATA"
  fi
fi

# ---------- Pacotes ----------
say "Instalando pacotes"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq samba avahi-daemon smartmontools unattended-upgrades btrfs-progs \
  curl ca-certificates gnupg openssl >/dev/null
ok "Samba, Avahi, SMART e atualizações automáticas"

if ! command -v kopia >/dev/null; then
  install -d -m 0755 /etc/apt/keyrings
  curl -fsSL https://kopia.io/signing-key | gpg --dearmor --yes -o /etc/apt/keyrings/kopia-keyring.gpg
  echo "deb [signed-by=/etc/apt/keyrings/kopia-keyring.gpg] http://packages.kopia.io/apt/ stable main" \
    > /etc/apt/sources.list.d/kopia.list
  apt-get update -qq && apt-get install -y -qq kopia >/dev/null
fi
ok "Kopia $(kopia --version 2>/dev/null | awk '{print $1}')"

# ---------- Nome e identidade ----------
say "Configurando o servidor \"$NAME\""
SLUG=$(echo "$NAME" | iconv -f utf-8 -t ascii//TRANSLIT 2>/dev/null | tr '[:upper:]' '[:lower:]' | tr -cs 'a-z0-9' '-' | sed 's/^-//;s/-$//')
[ -n "$SLUG" ] || SLUG=backeddy
hostnamectl set-hostname "$SLUG"
grep -q "127.0.1.1" /etc/hosts && sed -i "s/^127.0.1.1.*/127.0.1.1\t$SLUG/" /etc/hosts || echo -e "127.0.1.1\t$SLUG" >> /etc/hosts
install -d -m 0755 "$ETC"
printf 'NAME="%s"\nSLUG="%s"\nVERSION="%s"\n' "$NAME" "$SLUG" "$VERSION" > "$ETC/server.conf"
ok "Endereço na rede: $SLUG.local"

# ---------- Usuário e pastas ----------
id backeddy >/dev/null 2>&1 || useradd --system --home-dir "$DATA" --shell /usr/sbin/nologin backeddy
install -d -m 0755 "$DATA"
install -d -m 0700 -o backeddy -g backeddy "$DATA/timemachine"
install -d -m 0700 -o root -g root "$DATA/kopia"
printf '%s\n%s\n' "$PASSWORD" "$PASSWORD" | smbpasswd -s -a backeddy >/dev/null
smbpasswd -e backeddy >/dev/null
ok "Usuário \"backeddy\" e pastas em $DATA"

# ---------- Samba (Time Machine) ----------
TOTAL_KB=$(df -Pk "$DATA" | awk 'NR==2{print $2}')
TM_MAX_GB=$(( TOTAL_KB * TM_PERCENT / 100 / 1024 / 1024 ))
[ "$TM_MAX_GB" -lt 1 ] && TM_MAX_GB=1
[ -f /etc/samba/smb.conf ] && [ ! -f /etc/samba/smb.conf.original ] && cp /etc/samba/smb.conf /etc/samba/smb.conf.original
sed -e "s|@NAME@|$(sedesc "$NAME")|g" -e "s|@DATA@|$DATA|g" -e "s|@TM_MAX@|${TM_MAX_GB}G|g" \
  "$SRC/config/smb.conf" > /etc/samba/smb.conf
testparm -s /etc/samba/smb.conf >/dev/null 2>&1 || die "Configuração do Samba inválida (rode: testparm)."
systemctl enable --now smbd >/dev/null 2>&1
systemctl restart smbd
systemctl disable --now nmbd >/dev/null 2>&1 || true
ok "Compartilhamento Time Machine (limite ${TM_MAX_GB} GB = ${TM_PERCENT}% do disco)"

# ---------- Avahi (aparecer sozinho no Mac) ----------
XNAME=$(printf '%s' "$NAME" | sed -e 's/&/\&amp;/g' -e 's/</\&lt;/g' -e 's/>/\&gt;/g')
sed -e "s|@NAME@|$(sedesc "$XNAME")|g" "$SRC/config/backeddy.service" > /etc/avahi/services/backeddy.service
systemctl enable --now avahi-daemon >/dev/null 2>&1
systemctl restart avahi-daemon
ok "Anúncio na rede: aparece na lista do Time Machine"

# ---------- Kopia Repository Server (PCs Windows) ----------
KCONF="$ETC/kopia"
install -d -m 0700 "$KCONF"
[ -f "$KCONF/repo-password" ] || { openssl rand -hex 32 > "$KCONF/repo-password"; chmod 600 "$KCONF/repo-password"; }
[ -f "$KCONF/control-password" ] || { openssl rand -hex 24 > "$KCONF/control-password"; chmod 600 "$KCONF/control-password"; }
umask 077
printf 'KOPIA_PASSWORD=%s\nKOPIA_SERVER_CONTROL_PASSWORD=%s\n' \
  "$(cat "$KCONF/repo-password")" "$(cat "$KCONF/control-password")" > "$KCONF/server.env"
umask 022
export KOPIA_PASSWORD; KOPIA_PASSWORD=$(cat "$KCONF/repo-password")
export KOPIA_CONFIG_PATH="$KCONF/repository.config" KOPIA_CACHE_DIRECTORY=/var/cache/backeddy-kopia KOPIA_LOG_DIR=/var/log/backeddy-kopia
if [ ! -f "$KCONF/repository.config" ]; then
  if [ -f "$DATA/kopia/kopia.repository.f" ]; then
    kopia repository connect filesystem --path "$DATA/kopia" >/dev/null
  else
    kopia repository create filesystem --path "$DATA/kopia" >/dev/null
  fi
fi
# a manutenção automática do Kopia pertence a usuario@maquina; acompanha o nome atual
kopia maintenance set --owner=me >/dev/null 2>&1 || true
# certificado novo se ainda não existe ou se o nome do servidor mudou
if [ ! -f "$KCONF/tls.cert" ] || ! openssl x509 -in "$KCONF/tls.cert" -noout -subject 2>/dev/null | grep -q "CN *= *$SLUG.local"; then
  openssl req -x509 -newkey rsa:3072 -nodes -days 3650 -subj "/CN=$SLUG.local" \
    -addext "subjectAltName=DNS:$SLUG.local,DNS:$SLUG" \
    -keyout "$KCONF/tls.key" -out "$KCONF/tls.cert" >/dev/null 2>&1
  chmod 600 "$KCONF/tls.key"
fi
install -m 0644 "$SRC/config/backeddy-kopia.service" /etc/systemd/system/backeddy-kopia.service
systemctl daemon-reload
systemctl enable --now backeddy-kopia >/dev/null 2>&1
systemctl restart backeddy-kopia
FP=$(openssl x509 -in "$KCONF/tls.cert" -noout -fingerprint -sha256 | cut -d= -f2 | tr -d : | tr '[:upper:]' '[:lower:]')
echo "$FP" > "$KCONF/fingerprint"
ok "Kopia Server na porta 51515 (para o Backeddy for Windows)"

# ---------- Sempre ligado, sempre atualizado ----------
systemctl mask sleep.target suspend.target hibernate.target hybrid-sleep.target >/dev/null 2>&1
install -d /etc/systemd/logind.conf.d
printf '[Login]\nHandleLidSwitch=ignore\nHandleLidSwitchExternalPower=ignore\nHandleLidSwitchDocked=ignore\n' \
  > /etc/systemd/logind.conf.d/backeddy.conf
printf 'APT::Periodic::Update-Package-Lists "1";\nAPT::Periodic::Unattended-Upgrade "1";\n' \
  > /etc/apt/apt.conf.d/20auto-upgrades
systemctl enable --now smartd >/dev/null 2>&1 || true
ok "Nunca dorme (nem com a tampa fechada) e se atualiza sozinho"

install -m 0755 "$SRC/bin/backeddy" /usr/local/bin/backeddy

IP=$(hostname -I | awk '{print $1}')
cat <<MSG

──────────────────────────────────────────────
 ✅  "$NAME" está pronto!
──────────────────────────────────────────────
 🍎 Mac: Ajustes do Sistema › Geral › Time Machine › Adicionar disco
         Escolha "$NAME" · usuário: backeddy · senha: a que você definiu
         (se não aparecer: Finder › Ir › Conectar ao Servidor › smb://$SLUG.local)

 🪟 PCs: Kopia Server em https://$SLUG.local:51515  (ou https://$IP:51515)
         Adicione cada PC com:  sudo backeddy add-pc NOME-DO-PC

 ℹ️  Situação a qualquer momento:  sudo backeddy status
──────────────────────────────────────────────
MSG
