#!/bin/bash
# Backeddy Server — transforma um Debian recém-instalado num servidor de backup
# para Macs (Time Machine) e PCs (Backeddy for Windows / Kopia).
#
# Uso (como root):
#   ./install.sh                                  # pergunta nome e senha
#   ./install.sh --name "Backup da Casa" --disk /dev/sdb
#   ./install.sh --disk /dev/sdb --disk /dev/sdc      # dois discos espelhados
#
# Opções:
#   --name  "Nome"     Nome do servidor como aparece no Mac (padrão: "Backup da Casa")
#   --disk  /dev/sdX   Disco dedicado aos backups. SERÁ APAGADO (pede confirmação).
#                      Repita para usar vários discos: com 2 ou mais, eles são ESPELHADOS.
#                      Sem --disk, os backups ficam em /srv/backeddy no disco do sistema.
#   --no-mirror        Soma os discos em vez de espelhar (mais espaço, SEM proteção)
#   --tm-percent N     % do espaço reservado ao Time Machine (padrão: 60)
#   --yes              Não pergunta nada (exige BACKEDDY_PASSWORD no ambiente)
#
# Pode ser rodado de novo com segurança: reaplica as configurações.
set -euo pipefail

VERSION="0.2.0"
NAME="Backup da Casa"
DISKS=()
MIRROR=1
TM_PERCENT=60
ASSUME_YES=0
DATA=/srv/backeddy
ETC=/etc/backeddy
SRC="$(cd "$(dirname "$0")" && pwd)"

while [ $# -gt 0 ]; do
  case "$1" in
    --name) NAME="$2"; shift 2 ;;
    --disk) DISKS+=("$2"); shift 2 ;;
    --no-mirror) MIRROR=0; shift ;;
    --tm-percent) TM_PERCENT="$2"; shift 2 ;;
    --yes) ASSUME_YES=1; shift ;;
    -h|--help) sed -n '2,20p' "$0"; exit 0 ;;
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

# ---------- Discos de backup (opcional) ----------
# 1 disco: btrfs simples. 2 ou mais: ESPELHO (RAID1), cada arquivo gravado em 2 discos.
# Os discos não vão para o fstab: quem monta é o backeddy-mount, que usa o modo
# degradado se um disco do espelho morrer (os backups continuam e o status avisa).
human() { numfmt --to=iec --suffix=B "$1" 2>/dev/null || echo "$1 bytes"; }
if [ "${#DISKS[@]}" -gt 0 ]; then
  ROOTDEV=$(lsblk -no PKNAME "$(findmnt -no SOURCE /)" 2>/dev/null | head -1 || true)
  declare -A SEEN=()
  for d in "${DISKS[@]}"; do
    [ -b "$d" ] || die "Disco $d não encontrado."
    [ "/dev/$ROOTDEV" = "$d" ] && die "$d é o disco do sistema. Escolha outro disco."
    [ -z "${SEEN[$d]:-}" ] || die "O disco $d foi informado duas vezes."
    SEEN[$d]=1
  done
  apt-get install -y -qq btrfs-progs >/dev/null

  if mountpoint -q "$DATA" && [ "$(findmnt -no FSTYPE "$DATA")" = btrfs ] \
     && [ "$(blkid -s LABEL -o value "$(findmnt -no SOURCE "$DATA" | sed 's/\[.*//')" 2>/dev/null)" = backeddy ]; then
    ok "Discos de backup já configurados em $DATA (mantidos)"
    echo "    Para trocar ou acrescentar um disco: backeddy replace-disk / backeddy add-disk"
  else
    say "Discos de backup"
    lsblk -o NAME,SIZE,MODEL,FSTYPE,MOUNTPOINT "${DISKS[@]}"
    N=${#DISKS[@]}
    TOTAL=0; MAX=0; MIN=0
    for d in "${DISKS[@]}"; do
      s=$(lsblk -bdno SIZE "$d")
      TOTAL=$((TOTAL + s)); [ "$s" -gt "$MAX" ] && MAX=$s
      { [ "$MIN" -eq 0 ] || [ "$s" -lt "$MIN" ]; } && MIN=$s
    done
    echo
    if [ "$N" -eq 1 ]; then
      DPROF=single; MPROF=dup
      echo "Arranjo: 1 disco · $(human "$TOTAL") para backups (sem cópia em outro disco)"
    elif [ "$MIRROR" = 1 ]; then
      DPROF=raid1; MPROF=raid1
      HALF=$((TOTAL / 2)); REST=$((TOTAL - MAX)); USABLE=$(( HALF < REST ? HALF : REST ))
      echo "Arranjo: $N discos ESPELHADOS · $(human "$USABLE") protegidos"
      echo "         Cada arquivo fica gravado em 2 discos: se um morrer, nada se perde."
      if [ $(( (MAX - MIN) * 100 / MAX )) -ge 1 ]; then
        echo "⚠️  Os discos têm tamanhos diferentes: parte do espaço do maior não entra no espelho."
      fi
    else
      DPROF=single; MPROF=raid1
      echo "Arranjo: $N discos SOMADOS · $(human "$TOTAL")"
      echo "⚠️  SEM proteção: se qualquer disco morrer, o backup inteiro se perde."
    fi
    echo
    echo "⚠️  TUDO em ${DISKS[*]} será apagado para sempre."
    if [ "$ASSUME_YES" != 1 ]; then
      read -rp "Para confirmar, digite APAGAR: " C
      [ "$C" = "APAGAR" ] || die "Cancelado. Nada foi alterado."
    fi
    systemctl stop smbd backeddy-kopia >/dev/null 2>&1 || true
    mountpoint -q "$DATA" && umount "$DATA"
    for d in "${DISKS[@]}"; do wipefs -a "$d" >/dev/null; done
    mkfs.btrfs -f -L backeddy -d "$DPROF" -m "$MPROF" "${DISKS[@]}" >/dev/null
    btrfs device scan >/dev/null 2>&1 || true
    mkdir -p "$DATA"
    # sem os discos montados, nada pode ser gravado por engano no disco do sistema
    chattr +i "$DATA" 2>/dev/null || true
    rm -rf "$ETC/kopia/repository.config" "$ETC/kopia/repository.config.kopia-password"
    ok "Discos formatados (btrfs)"
  fi

  # identidade do conjunto de discos e montagem pelo backeddy-mount (sem fstab)
  install -d -m 0755 "$ETC"
  if mountpoint -q "$DATA"; then FSUUID=$(findmnt -no UUID "$DATA"); else FSUUID=$(blkid -s UUID -o value "${DISKS[0]}"); fi
  echo "UUID=$FSUUID" > "$ETC/disks.conf"
  sed -i "\|[[:space:]]${DATA}[[:space:]]|d" /etc/fstab
  install -m 0755 "$SRC/bin/backeddy-mount" /usr/local/sbin/backeddy-mount
  install -m 0644 "$SRC/config/backeddy-mount.service" /etc/systemd/system/backeddy-mount.service
  for svc in smbd backeddy-kopia; do
    install -d "/etc/systemd/system/$svc.service.d"
    install -m 0644 "$SRC/config/requires-backup-disk.conf" "/etc/systemd/system/$svc.service.d/backeddy-disk.conf"
  done
  systemctl daemon-reload
  systemctl enable backeddy-mount >/dev/null 2>&1
  systemctl restart backeddy-mount
  mountpoint -q "$DATA" || die "Não consegui montar os discos de backup (veja: journalctl -t backeddy)."
  ok "Discos montados em $DATA"
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
