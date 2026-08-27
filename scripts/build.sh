#!/bin/sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
VERSION="$(tr -d ' \r\n\t' < "$ROOT/VERSION")"
STAGE="$ROOT/build/stage"
DIST="$ROOT/dist"

rm -rf "$ROOT/build"
mkdir -p "$STAGE/CONTROL" "$DIST"

# Reproducible custom logo assets. CONTROL/icon.png is the 90x90 ADM icon.
go run "$ROOT/cmd/mkicon" -size 90 -out "$ROOT/packaging/apkg/CONTROL/icon.png"
go run "$ROOT/cmd/mkicon" -size 256 -out "$ROOT/assets/logo.png"

CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
  -trimpath \
  -ldflags "-s -w -X main.version=$VERSION" \
  -o "$STAGE/cron-manager" "$ROOT/cmd/cron-manager"

# The standalone binary is also published for the in-app self-updater.
cp "$STAGE/cron-manager" "$DIST/cron-manager_linux_arm64"
chmod 755 "$DIST/cron-manager_linux_arm64"

cp -R "$ROOT/packaging/apkg/CONTROL/." "$STAGE/CONTROL/"

OUT="$DIST/CronManager_${VERSION}_arm64.apk"
go run "$ROOT/cmd/mkapkg" -root "$STAGE" -out "$OUT" -version "$VERSION"

checksum() {
  file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    (cd "$DIST" && sha256sum "$(basename "$file")" > "$(basename "$file").sha256")
  elif command -v shasum >/dev/null 2>&1; then
    (cd "$DIST" && shasum -a 256 "$(basename "$file")" > "$(basename "$file").sha256")
  else
    echo "Neither sha256sum nor shasum is available" >&2
    exit 1
  fi
}

checksum "$OUT"
checksum "$DIST/cron-manager_linux_arm64"

echo "Built: $OUT"
echo "Built: $DIST/cron-manager_linux_arm64"
