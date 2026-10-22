#!/usr/bin/env sh
# Builds nulltrace + nulltraced and copies them into $PREFIX/bin (default /usr/local).
# Run it as yourself; it only asks for sudo if $PREFIX/bin isn't writable.
set -eu
PREFIX="${PREFIX:-/usr/local}"
ROOT="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
export CGO_ENABLED=0

command -v go >/dev/null 2>&1 || { echo "go is not installed (need 1.23+)"; exit 1; }

go build -C "$ROOT" -trimpath -ldflags="-s -w" -o "$ROOT/bin/nulltrace" ./cmd/nulltrace
go build -C "$ROOT" -trimpath -ldflags="-s -w" -o "$ROOT/bin/nulltraced" ./cmd/nulltraced

SUDO=""
target="$PREFIX/bin"
[ -d "$target" ] || target="$PREFIX"
if [ ! -w "$target" ] && [ "$(id -u)" -ne 0 ]; then
  echo "$PREFIX/bin needs root, using sudo"
  SUDO="sudo"
fi
$SUDO install -d "$PREFIX/bin"
$SUDO install -m 0755 "$ROOT/bin/nulltrace" "$PREFIX/bin/nulltrace"
$SUDO install -m 0755 "$ROOT/bin/nulltraced" "$PREFIX/bin/nulltraced"
echo "installed to $PREFIX/bin"

# The daemon is optional. It runs as your user, never as root.
if command -v systemctl >/dev/null 2>&1 && [ "$(id -u)" -ne 0 ]; then
  UNIT_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
  mkdir -p "$UNIT_DIR"
  sed "s#/usr/local/bin#$PREFIX/bin#" "$ROOT/scripts/systemd/nulltraced.service" > "$UNIT_DIR/nulltraced.service"
  echo "user unit written to $UNIT_DIR (optional)"
  echo "  to use it: put your vault password in ~/.config/nulltrace/passphrase (chmod 600)"
  echo "  then:      systemctl --user daemon-reload && systemctl --user enable --now nulltraced"
fi

echo
echo "run: nulltrace"
