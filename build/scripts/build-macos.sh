#!/usr/bin/env bash
# Production macOS build for VEO3 Manager.
# MUST run on a real macOS machine (or GitHub Actions macos-* runner).
# Wails uses CGO + WKWebView -- cross-compiling from Windows/Linux is not supported.

set -euo pipefail

VERSION="${1:-0.1.0}"
ARCH="${2:-universal}"   # one of: amd64 (Intel), arm64 (Apple Silicon), universal

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

# Same -ldflags as Windows build:
#   -s strips the symbol table  (smaller binary)
#   -w strips DWARF debug info  (smaller binary)
#   -X injects build-time string vars into main package
LDFLAGS="-s -w -X main.version=${VERSION} -X main.buildDate=${BUILD_DATE}"

echo "Building VEO3 Manager v${VERSION} (darwin/${ARCH})"

# Wails flags:
#   -platform darwin/<arch>      universal => fat binary (amd64 + arm64 lipo'd)
#   -ldflags "..."               forwarded to go build
#   -trimpath                    remove abs paths from binary (smaller, reproducible)
#   -clean                       wipe build/bin first
#   (no -webview2 here -- macOS uses the OS's built-in WKWebView)
wails build \
  -platform "darwin/${ARCH}" \
  -ldflags "${LDFLAGS}" \
  -trimpath \
  -clean

APP="build/bin/veo3-manager.app"
if [[ ! -d "$APP" ]]; then
  echo "ERROR: $APP not produced by wails build" >&2
  exit 1
fi

echo
echo "Built: $APP"
du -sh "$APP" | awk '{print "Size: " $1}'

# --- Optional: package as DMG ---------------------------------------------
# Requires `create-dmg` (brew install create-dmg) or hdiutil (built-in).
# We'll use create-dmg if available (nicer UX), else fall back to hdiutil.
DMG="build/bin/VEO3-Manager-${VERSION}-${ARCH}.dmg"
if command -v create-dmg >/dev/null 2>&1; then
  echo "Creating DMG via create-dmg..."
  rm -f "$DMG"
  create-dmg \
    --volname "VEO3 Manager ${VERSION}" \
    --window-size 540 380 \
    --icon-size 100 \
    --icon "veo3-manager.app" 140 190 \
    --app-drop-link 400 190 \
    --hide-extension "veo3-manager.app" \
    "$DMG" \
    "$APP"
else
  echo "create-dmg not found, using hdiutil..."
  rm -f "$DMG"
  STAGE="$(mktemp -d)"
  cp -R "$APP" "$STAGE/"
  ln -s /Applications "$STAGE/Applications"
  hdiutil create -volname "VEO3 Manager ${VERSION}" \
    -srcfolder "$STAGE" \
    -ov -format UDZO \
    "$DMG"
  rm -rf "$STAGE"
fi

echo
echo "Artifacts:"
ls -lh build/bin/ | awk 'NR>1 {printf "  %-50s %s\n", $9, $5}'

# --- Code signing & notarization (manual, requires Apple Developer ID) ----
# Uncomment and fill in your identity to sign + notarize:
#
#   APPLE_TEAM_ID="ABCDE12345"
#   APPLE_ID="you@example.com"
#   APPLE_APP_PASSWORD="abcd-efgh-ijkl-mnop"   # app-specific password
#   IDENTITY="Developer ID Application: Your Name ($APPLE_TEAM_ID)"
#
#   codesign --deep --force --options runtime --sign "$IDENTITY" "$APP"
#   codesign --verify --deep --strict --verbose=2 "$APP"
#
#   xcrun notarytool submit "$DMG" \
#     --apple-id "$APPLE_ID" \
#     --password "$APPLE_APP_PASSWORD" \
#     --team-id "$APPLE_TEAM_ID" \
#     --wait
#   xcrun stapler staple "$DMG"
#
# Without signing, users will see "VEO3 Manager.app cannot be opened because the
# developer cannot be verified". They can right-click -> Open the first time, or
# you can ship signed builds via GitHub Actions with the secrets above.
