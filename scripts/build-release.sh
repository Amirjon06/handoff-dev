#!/usr/bin/env sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
DIST_DIR="$ROOT_DIR/dist"

mkdir -p "$DIST_DIR"

build() {
  goos="$1"
  goarch="$2"
  ext="$3"
  output="$DIST_DIR/relay-$goos-$goarch$ext"

  echo "Building $output"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath -ldflags "-s -w" -o "$output" ./cmd/relay
}

cd "$ROOT_DIR"

build darwin amd64 ""
build darwin arm64 ""
build linux amd64 ""
build linux arm64 ""
build windows amd64 ".exe"
build windows arm64 ".exe"
