#!/usr/bin/env sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
DIST_DIR="$ROOT_DIR/dist"
VERSION=${VERSION:-$(git -C "$ROOT_DIR" describe --tags --always --dirty 2>/dev/null || printf "dev")}
CHECKSUMS="$DIST_DIR/checksums.txt"

mkdir -p "$DIST_DIR"
: > "$CHECKSUMS"

checksum() {
  file="$1"
  name=$(basename "$file")
  if command -v sha256sum >/dev/null 2>&1; then
    (cd "$DIST_DIR" && sha256sum "$name" >> "$CHECKSUMS")
    return
  fi
  (cd "$DIST_DIR" && shasum -a 256 "$name" >> "$CHECKSUMS")
}

package() {
  goos="$1"
  goarch="$2"
  ext="$3"
  package_name="staterelay-$VERSION-$goos-$goarch"
  package_dir="$DIST_DIR/$package_name"
  binary="$package_dir/relay$ext"

  mkdir -p "$package_dir"
  echo "Building $binary"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath -ldflags "-s -w" -o "$binary" ./cmd/relay

  cp "$ROOT_DIR/README.md" "$package_dir/README.md"
  cp "$ROOT_DIR/LICENSE" "$package_dir/LICENSE"
  printf "%s\n" "$VERSION" > "$package_dir/VERSION.txt"

  if [ "$goos" = "windows" ]; then
    cp "$ROOT_DIR/scripts/install.ps1" "$package_dir/install.ps1"
    archive="$DIST_DIR/$package_name.zip"
    (cd "$DIST_DIR" && zip -qr "$archive" "$package_name")
  else
    cp "$ROOT_DIR/scripts/install.sh" "$package_dir/install.sh"
    chmod +x "$package_dir/install.sh"
    archive="$DIST_DIR/$package_name.tar.gz"
    (cd "$DIST_DIR" && tar -czf "$archive" "$package_name")
  fi

  checksum "$archive"
}

cd "$ROOT_DIR"

package darwin amd64 ""
package darwin arm64 ""
package linux amd64 ""
package linux arm64 ""
package windows amd64 ".exe"
package windows arm64 ".exe"

echo "Wrote release artifacts to $DIST_DIR"
echo "Wrote checksums to $CHECKSUMS"
