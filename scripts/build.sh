#!/usr/bin/env bash
# build.sh — build centmem for the current platform and emit SHA256SUMS.
#
# centmem depends on cgo (sqlite-vec + mattn/go-sqlite3), so binaries are built
# natively per platform; the release workflow runs this on each OS/arch runner.
#
# Output:
#   dist/centmem-<goos>-<goarch>
#   dist/SHA256SUMS  (when building the current platform only)
#
# Usage:
#   scripts/build.sh
set -euo pipefail

cd "$(dirname "$0")/.."

GOOS_NATIVE="$(go env GOOS)"
GOARCH_NATIVE="$(go env GOARCH)"
OUT_DIR="dist"
mkdir -p "$OUT_DIR"

BIN_NAME="centmem-${GOOS_NATIVE}-${GOARCH_NATIVE}"
echo "==> building $GOOS_NATIVE/$GOARCH_NATIVE (native)"
go build \
  -tags fts5 \
  -o "$OUT_DIR/$BIN_NAME" ./cmd/centmem
echo "    -> $OUT_DIR/$BIN_NAME"

echo "==> generating SHA256SUMS"
(cd "$OUT_DIR" && shasum -a 256 "$BIN_NAME" > SHA256SUMS)
echo "==> done. artifact in $OUT_DIR"
