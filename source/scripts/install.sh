#!/usr/bin/env sh
# Install nulltrace / nulltraced into PREFIX (default /usr/local).
set -eu
PREFIX="${PREFIX:-/usr/local}"
ROOT="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
export CGO_ENABLED=0

echo "building with CGO_ENABLED=0"
go build -C "$ROOT" -o "$ROOT/bin/nulltrace" ./cmd/nulltrace
go build -C "$ROOT" -o "$ROOT/bin/nulltraced" ./cmd/nulltraced

install -d "$PREFIX/bin"
install -m 0755 "$ROOT/bin/nulltrace" "$PREFIX/bin/nulltrace"
install -m 0755 "$ROOT/bin/nulltraced" "$PREFIX/bin/nulltraced"

if [ -d /etc/systemd/system ]; then
  install -d /etc/systemd/system
  install -m 0644 "$ROOT/scripts/systemd/nulltraced.service" /etc/systemd/system/nulltraced.service
  echo "systemd unit installed; run: systemctl daemon-reload && systemctl enable --now nulltraced"
fi

echo "installed to $PREFIX/bin"
echo "initialize with: nulltrace vault init"
