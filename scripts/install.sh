#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
PREFIX=${PREFIX:-/usr/local}
INSTALL_DIR="$PREFIX/bin"
SOURCE="$SCRIPT_DIR/relay"
TARGET="$INSTALL_DIR/relay"

if [ ! -f "$SOURCE" ]; then
  echo "relay binary not found next to install.sh" >&2
  exit 1
fi

mkdir -p "$INSTALL_DIR"
cp "$SOURCE" "$TARGET"
chmod +x "$TARGET"

echo "Installed relay to $TARGET"
echo "Run: relay version"
