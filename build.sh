#!/usr/bin/env bash
# Build the Windows deliverables into dist/.
#
#   ./build.sh            normal build (stripped symbols, no console window)
#   ./build.sh garble     obfuscated build (needs `garble` installed) — harder to reverse
#
# Requires: Go (cross-compiles from Linux/macOS/Windows; no CGO, no mingw).
set -euo pipefail
cd "$(dirname "$0")"

MODE="${1:-plain}"
OUT=dist
mkdir -p "$OUT"

export GOOS=windows GOARCH=amd64 CGO_ENABLED=0

# -s -w strips the symbol table and DWARF (smaller, less to reverse-engineer).
# -H windowsgui makes miele.exe a GUI app with no console window.
LDFLAGS_GUI="-s -w -H windowsgui"
LDFLAGS_CLI="-s -w"

if [ "$MODE" = "garble" ]; then
  if ! command -v garble >/dev/null; then
    echo "garble not found. Install with: go install mvdan.cc/garble@latest" >&2
    exit 1
  fi
  echo ">> obfuscated build (garble)"
  garble -tiny -literals build -trimpath -ldflags "$LDFLAGS_GUI" -o "$OUT/miele.exe" ./cmd/miele
  garble -tiny -literals build -trimpath -ldflags "$LDFLAGS_CLI" -o "$OUT/keygen.exe" ./cmd/keygen
else
  echo ">> plain build"
  go build -trimpath -ldflags "$LDFLAGS_GUI" -o "$OUT/miele.exe" ./cmd/miele
  go build -trimpath -ldflags "$LDFLAGS_CLI" -o "$OUT/keygen.exe" ./cmd/keygen
fi

# External application manifest (enables modern controls + DPI awareness without a build tool).
cp cmd/miele/app.manifest "$OUT/miele.exe.manifest"

# Handy extras for the operator.
cp examples/kitchen.csv "$OUT/parts-template.csv" 2>/dev/null || true

echo
echo "built:"
ls -lh "$OUT"
echo
if [ -z "${MIELE_PROVISIONED:-}" ] && ! grep -q 'PublicKeyHex = "[0-9a-f]' internal/license/pubkey.go; then
  echo "WARNING: this build is NOT provisioned with a license key."
  echo "         run './keygen.exe init' on your machine first, then rebuild,"
  echo "         otherwise the app will refuse to start. See docs/SECURITY.md."
fi
