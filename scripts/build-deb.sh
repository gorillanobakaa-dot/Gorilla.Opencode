#!/bin/sh
# Build the Debian package. Usage: scripts/build-deb.sh <version>
# Requires: a built ./gorilla-opencode binary in the repo root, dpkg-deb.
# Build it stamped: go build -ldflags "-X github.com/opencode-ai/opencode/internal/version.Version=v<version>" -o gorilla-opencode .
# The package installs system-wide equivalents of what
# `gorilla-opencode install` does per-user: /usr/bin binary, hicolor
# icons, desktop entry, plus the dual-track documentation.
set -eu

VERSION="${1:?usage: scripts/build-deb.sh <version>}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$ROOT/gorilla-opencode"
[ -x "$BIN" ] || { echo "Build the binary first: go build -o gorilla-opencode ." >&2; exit 1; }

STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT
PKG="$STAGE/gorilla-opencode_${VERSION}_amd64"

install -Dm755 "$BIN" "$PKG/usr/bin/gorilla-opencode"
for s in 128 256 512 1024; do
  install -Dm644 "$ROOT/internal/assets/icons/gorilla-opencode-$s.png" \
    "$PKG/usr/share/icons/hicolor/${s}x${s}/apps/gorilla-opencode.png"
done
install -Dm644 "$ROOT/internal/assets/icons/gorilla-opencode.svg" \
  "$PKG/usr/share/icons/hicolor/scalable/apps/gorilla-opencode.svg"

install -d "$PKG/usr/share/applications"
cat > "$PKG/usr/share/applications/gorilla-opencode.desktop" <<'EOF'
[Desktop Entry]
Type=Application
Name=Gorilla OpenCode
Comment=Terminal AI coding agent (revived original OpenCode) — bring your own API keys
Exec=gorilla-opencode launch
Icon=gorilla-opencode
Terminal=true
Categories=Development;IDE;
Keywords=ai;coding;agent;terminal;llm;
EOF

install -Dm644 "$ROOT/README.md" "$PKG/usr/share/doc/gorilla-opencode/README.md"
# GORILLA: changelogs and the dual-track doc now live under Changelogs/ to keep
# the repo root tidy — reference them there.
install -Dm644 "$ROOT/Changelogs/DOCUMENTATION.dual-track.md" "$PKG/usr/share/doc/gorilla-opencode/DOCUMENTATION.dual-track.md"
install -Dm644 "$ROOT/PHILOSOPHY.md" "$PKG/usr/share/doc/gorilla-opencode/PHILOSOPHY.md"
install -Dm644 "$ROOT/LICENSE" "$PKG/usr/share/doc/gorilla-opencode/copyright"
# GORILLA OVERRIDE: ship the standalone teaching lesson (tokens, agents, and the
# cost/pace/leash controls — dual-track, with sources and a recreate-it guide)
# and every dated release changelog, so the .deb carries the "why & how" too.
install -Dm644 "$ROOT/docs/CONTROL-AND-COST.md" "$PKG/usr/share/doc/gorilla-opencode/CONTROL-AND-COST.md"
for cl in "$ROOT"/Changelogs/CHANGELOG*.md; do
  [ -f "$cl" ] && install -Dm644 "$cl" "$PKG/usr/share/doc/gorilla-opencode/$(basename "$cl")"
done

install -d "$PKG/DEBIAN"
SIZE_KB=$(du -sk "$PKG" | cut -f1)
cat > "$PKG/DEBIAN/control" <<EOF
Package: gorilla-opencode
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
 under /usr/share/doc/gorilla-opencode.
EOF

cat > "$PKG/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e
command -v gtk-update-icon-cache >/dev/null 2>&1 && gtk-update-icon-cache -f -t /usr/share/icons/hicolor || true
command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database /usr/share/applications || true
EOF
cp "$PKG/DEBIAN/postinst" "$PKG/DEBIAN/postrm"
chmod 755 "$PKG/DEBIAN/postinst" "$PKG/DEBIAN/postrm"

dpkg-deb --build --root-owner-group "$PKG" "$ROOT/gorilla-opencode_${VERSION}_amd64.deb"
echo "Built: $ROOT/gorilla-opencode_${VERSION}_amd64.deb"
