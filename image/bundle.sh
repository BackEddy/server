#!/bin/bash
# Pacote da atualização automática (backeddy update): os mesmos arquivos que vão para /opt/backeddy-server
# na imagem, mais o vigia compilado. Gera out/backeddy-server.tar.gz, o .sha256 e out/latest-version.txt.
set -euo pipefail
REPO=$(cd "$(dirname "$0")/.." && pwd)
VER=$(tr -d ' \n' < "$REPO/VERSION")
OUT="$REPO/out"; W=$(mktemp -d)
mkdir -p "$W/backeddy-server" "$OUT"
cp -a "$REPO/install.sh" "$REPO/bin" "$REPO/config" "$REPO/LICENSE" "$REPO/VERSION" "$W/backeddy-server/"
( cd "$REPO/agent" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -buildvcs=false -trimpath -ldflags "-s -w" -o "$W/backeddy-server/bin/backeddy-agent" . )
tar -C "$W" --owner=0 --group=0 -czf "$OUT/backeddy-server.tar.gz" backeddy-server
( cd "$OUT" && sha256sum backeddy-server.tar.gz > backeddy-server.tar.gz.sha256 )
echo "$VER" > "$OUT/latest-version.txt"
ls -l "$OUT"/backeddy-server.tar.gz* "$OUT/latest-version.txt"
