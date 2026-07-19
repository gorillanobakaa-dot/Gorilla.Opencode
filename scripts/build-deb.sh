#!/bin/sh
# Build the Debian package. Usage: scripts/build-deb.sh <version>
# Requires: a built ./opencode-dino binary in the repo root, dpkg-deb.
# The package installs system-wide equivalents of what
# `opencode-dino install` does per-user: /usr/bin binary, hicolor
# icons, desktop entry, plus the dual-track documentation.
set -eu

VERSION="${1:?usage: scripts/build-deb.sh <version>}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$ROOT/opencode-dino"
[ -x "$BIN" ] || { echo "Build the binary first: go build -o opencode-dino ." >&2; exit 1; }

STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT
PKG="$STAGE/opencode-dino_${VERSION}_amd64"

install -Dm755 "$BIN" "$PKG/usr/bin/opencode-dino"
for s in 128 256 512 1024; do
  install -Dm644 "$ROOT/internal/assets/icons/opencode-dino-$s.png" \
    "$PKG/usr/share/icons/hicolor/${s}x${s}/apps/opencode-dino.png"
done
install -Dm644 "$ROOT/internal/assets/icons/opencode-dino.svg" \
  "$PKG/usr/share/icons/hicolor/scalable/apps/opencode-dino.svg"

install -d "$PKG/usr/share/applications"
cat > "$PKG/usr/share/applications/opencode-dino.desktop" <<'EOF'
[Desktop Entry]
Type=Application
Name=OpenCode Dino
Comment=Terminal AI coding agent (revived original OpenCode) — bring your own API keys
Exec=opencode-dino
Icon=opencode-dino
Terminal=true
Categories=Development;IDE;
Keywords=ai;coding;agent;terminal;llm;
EOF

install -Dm644 "$ROOT/README.md" "$PKG/usr/share/doc/opencode-dino/README.md"
install -Dm644 "$ROOT/DOCUMENTATION.dual-track.md" "$PKG/usr/share/doc/opencode-dino/DOCUMENTATION.dual-track.md"
install -Dm644 "$ROOT/PHILOSOPHY.md" "$PKG/usr/share/doc/opencode-dino/PHILOSOPHY.md"
install -Dm644 "$ROOT/LICENSE" "$PKG/usr/share/doc/opencode-dino/copyright"

install -d "$PKG/DEBIAN"
SIZE_KB=$(du -sk "$PKG" | cut -f1)
cat > "$PKG/DEBIAN/control" <<EOF
Package: opencode-dino
Version: $VERSION
Section: devel
Priority: optional
Architecture: amd64
Installed-Size: $SIZE_KB
Maintainer: gorillanobakaa <gorillanobakaa@gmail.com>
Homepage: https://github.com/gorillanobakaa-dot/Gorilla.Opencode
Description: Terminal AI coding agent (revived original OpenCode)
 The original MIT-licensed Go OpenCode by Kujtim Hoxha, revived and
 kept working with 2026 AI providers: NVIDIA NIM, Google Gemini 3,
 local models via Ollama, and the originally supported providers.
 No telemetry, no accounts; bring your own API keys.
 .
 Dual-track documentation (plain-language and developer) is installed
 under /usr/share/doc/opencode-dino.
EOF

cat > "$PKG/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e
command -v gtk-update-icon-cache >/dev/null 2>&1 && gtk-update-icon-cache -f -t /usr/share/icons/hicolor || true
command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database /usr/share/applications || true
EOF
cp "$PKG/DEBIAN/postinst" "$PKG/DEBIAN/postrm"
chmod 755 "$PKG/DEBIAN/postinst" "$PKG/DEBIAN/postrm"

dpkg-deb --build --root-owner-group "$PKG" "$ROOT/opencode-dino_${VERSION}_amd64.deb"
echo "Built: $ROOT/opencode-dino_${VERSION}_amd64.deb"
