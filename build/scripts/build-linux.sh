#!/usr/bin/env bash
# Production Linux build for VEO3 Manager.
# MUST run on Linux (native, WSL2, Docker, or GitHub Actions ubuntu-* runner).
# Wails uses CGO + WebKitGTK -- cross-compiling from Windows/macOS is not supported.
#
# Required system packages (Debian/Ubuntu):
#   sudo apt-get install -y build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.0-dev
# (Fedora/RHEL: gtk3-devel webkit2gtk4.0-devel; Arch: gtk3 webkit2gtk-4.1)

set -euo pipefail

VERSION="${1:-0.1.0}"
ARCH="${2:-amd64}"   # amd64 or arm64

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
LDFLAGS="-s -w -X main.version=${VERSION} -X main.buildDate=${BUILD_DATE}"

echo "Building VEO3 Manager v${VERSION} (linux/${ARCH})"

# Wails flags (same meaning as on Windows / macOS):
#   -platform linux/<arch>
#   -ldflags "-s -w ..."   strip + inject version
#   -trimpath              remove abs paths
#   -clean                 wipe build/bin first
wails build \
  -platform "linux/${ARCH}" \
  -ldflags "${LDFLAGS}" \
  -trimpath \
  -clean

BIN="build/bin/veo3-manager"
if [[ ! -f "$BIN" ]]; then
  echo "ERROR: $BIN not produced by wails build" >&2
  exit 1
fi

# Optional UPX compression (saves ~60% on the final binary)
if command -v upx >/dev/null 2>&1; then
  echo "UPX: compressing $BIN"
  upx --best --lzma --no-progress "$BIN" || echo "UPX failed, continuing with original binary"
fi

echo
echo "Built: $BIN"
du -sh "$BIN" | awk '{print "Size: " $1}'

# --- Package as AppImage --------------------------------------------------
# AppImage is the portable single-file format that runs on most Linux distros.
# We need: linuxdeploy (orchestrator) + appimagetool. Both are downloaded on
# first run if missing, and cached under build/scripts/.appimage-tools/.

TOOLS_DIR="build/scripts/.appimage-tools"
mkdir -p "$TOOLS_DIR"

LINUXDEPLOY="$TOOLS_DIR/linuxdeploy-${ARCH}.AppImage"
APPIMAGETOOL="$TOOLS_DIR/appimagetool-${ARCH}.AppImage"

if [[ ! -x "$LINUXDEPLOY" ]]; then
  echo "Downloading linuxdeploy..."
  case "$ARCH" in
    amd64) UA="x86_64" ;;
    arm64) UA="aarch64" ;;
    *) echo "Unsupported arch for AppImage: $ARCH"; exit 1 ;;
  esac
  curl -fsSL -o "$LINUXDEPLOY" \
    "https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous/linuxdeploy-${UA}.AppImage"
  chmod +x "$LINUXDEPLOY"
fi

# Stage AppDir
APPDIR="build/bin/veo3-manager.AppDir"
rm -rf "$APPDIR"
mkdir -p "$APPDIR/usr/bin" "$APPDIR/usr/share/applications" "$APPDIR/usr/share/icons/hicolor/512x512/apps"

cp "$BIN" "$APPDIR/usr/bin/veo3-manager"
cp "build/appicon.png" "$APPDIR/usr/share/icons/hicolor/512x512/apps/veo3-manager.png"
cp "build/appicon.png" "$APPDIR/veo3-manager.png"  # also at root for AppImage thumbnail

cat > "$APPDIR/usr/share/applications/veo3-manager.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=VEO3 Manager
Comment=Automated AI video generation orchestrator
Exec=veo3-manager
Icon=veo3-manager
Categories=AudioVideo;Video;Utility;
Terminal=false
StartupWMClass=veo3-manager
EOF
cp "$APPDIR/usr/share/applications/veo3-manager.desktop" "$APPDIR/veo3-manager.desktop"

# AppRun launcher
cat > "$APPDIR/AppRun" <<'EOF'
#!/bin/sh
HERE="$(dirname "$(readlink -f "$0")")"
export PATH="$HERE/usr/bin:$PATH"
exec "$HERE/usr/bin/veo3-manager" "$@"
EOF
chmod +x "$APPDIR/AppRun"

OUT_APPIMAGE="build/bin/VEO3-Manager-${VERSION}-${ARCH}.AppImage"
ARCH_VAR="$ARCH"; [[ "$ARCH" == "amd64" ]] && ARCH_VAR="x86_64"; [[ "$ARCH" == "arm64" ]] && ARCH_VAR="aarch64"

ARCH="$ARCH_VAR" "$LINUXDEPLOY" \
  --appdir "$APPDIR" \
  --output appimage

# linuxdeploy drops the .AppImage in cwd; move it to build/bin/
PRODUCED=$(ls VEO3*.AppImage *VEO3*.AppImage 2>/dev/null | head -n1 || true)
if [[ -n "$PRODUCED" ]]; then mv -f "$PRODUCED" "$OUT_APPIMAGE"; fi

echo
echo "Artifacts:"
ls -lh build/bin/ | awk 'NR>1 {printf "  %-50s %s\n", $9, $5}'
