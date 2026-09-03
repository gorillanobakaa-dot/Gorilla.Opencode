#!/bin/sh
# Build the Arch Linux / CachyOS package (.pkg.tar.zst). Usage: scripts/build-arch.sh <version>
# Requires: a built ./gorilla-opencode binary in the repo root, bsdtar, zstd.
# Build it stamped: go build -ldflags "-s -w -X github.com/opencode-ai/opencode/internal/version.Version=v<version>" -o gorilla-opencode .
set -eu

VERSION="${1:?usage: scripts/build-arch.sh <version>}"
PKGREL="1"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$ROOT/gorilla-opencode"
[ -x "$BIN" ] || { echo "Build the binary first: go build -o gorilla-opencode ." >&2; exit 1; }

# GORILLA OVERRIDE (2026-08-17): refuse a binary whose stamp does not match the
# requested version — the same guard build-deb.sh gained the same day, after it
# silently wrapped a stale ~test8 binary as ~test9 AND ~test10. dpkg reported the
# new version while the installed binary reported the old one. An artifact's name
# is not its contents.
STAMPED="$("$BIN" --version 2>/dev/null || true)"
[ "$STAMPED" = "v$VERSION" ] || {
	echo "Binary reports '$STAMPED' but you asked to package 'v$VERSION'." >&2
	echo "Rebuild it stamped: go build -ldflags \"-s -w -X github.com/opencode-ai/opencode/internal/version.Version=v$VERSION\" -o gorilla-opencode ." >&2
	exit 1
}

STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT
PKG_DIR="$STAGE/pkg"
mkdir -p "$PKG_DIR"

# Copy binary
install -Dm755 "$BIN" "$PKG_DIR/usr/bin/gorilla-opencode"

# Icons
for s in 128 256 512 1024; do
  install -Dm644 "$ROOT/internal/assets/icons/gorilla-opencode-$s.png" \
    "$PKG_DIR/usr/share/icons/hicolor/${s}x${s}/apps/gorilla-opencode.png"
done
install -Dm644 "$ROOT/internal/assets/icons/gorilla-opencode.svg" \
  "$PKG_DIR/usr/share/icons/hicolor/scalable/apps/gorilla-opencode.svg"

# Desktop entry
install -Dm644 "$ROOT/packaging/gorilla-opencode.desktop" \
  "$PKG_DIR/usr/share/applications/gorilla-opencode.desktop"

# SearXNG setup helper
install -Dm755 "$ROOT/packaging/setup-searxng.sh" \
  "$PKG_DIR/usr/share/gorilla-opencode/setup-searxng.sh"

# GORILLA OVERRIDE (2026-08-20): ship pfind here too. build-deb.sh gained this
# block on 2026-08-17 when the search engine was vendored, and this script - a
# separately hand-written sibling, not generated from it - never got it. Same
# per-format-copy drift that shipped a launcher with no plain-mode action in
# v0.1.43, and found the same way: by diffing the two BUILT packages instead of
# reading the two scripts.
#
# The same bytes are embedded in the binary (find.go go:embed), so this copy is
# not what makes the tool work. It exists so users get a `pfind` CLI of their
# own and so the engine is inspectable as a file - and the CachyOS tester who
# files the bug reports against this package is exactly that user.
install -Dm644 "$ROOT/internal/llm/tools/pfind.py" \
  "$PKG_DIR/usr/share/gorilla-opencode/pfind.py"
install -d "$PKG_DIR/usr/bin"
cat > "$PKG_DIR/usr/bin/pfind" <<'WRAP'
#!/bin/sh
exec python3 /usr/share/gorilla-opencode/pfind.py "$@"
WRAP
chmod 755 "$PKG_DIR/usr/bin/pfind"

# Documentation & License
install -Dm644 "$ROOT/README.md" "$PKG_DIR/usr/share/doc/gorilla-opencode/README.md"
install -Dm644 "$ROOT/Changelogs/DOCUMENTATION.dual-track.md" "$PKG_DIR/usr/share/doc/gorilla-opencode/DOCUMENTATION.dual-track.md"
install -Dm644 "$ROOT/PHILOSOPHY.md" "$PKG_DIR/usr/share/doc/gorilla-opencode/PHILOSOPHY.md"
install -Dm644 "$ROOT/LICENSE" "$PKG_DIR/usr/share/licenses/gorilla-opencode/LICENSE"
install -Dm644 "$ROOT/docs/CONTROL-AND-COST.md" "$PKG_DIR/usr/share/doc/gorilla-opencode/CONTROL-AND-COST.md"
# GORILLA OVERRIDE (2026-08-17): the OSINT/research documentation set travels
# with the Arch package too. The tester who reports against this package is on
# CachyOS; docs that only reach .deb users do not reach him.
for d in COMMANDS FOOTPRINT SATELLITE SHELL-SAFETY SECURITY-AUDIT-2026-08-18 CODE-REVIEW SESSIONS-AND-STORAGE OSINT-RESEARCH OSINT-DOCTRINE OSINT-SOURCE-CATALOG FIND-TOOL-METRICS; do
	[ -f "$ROOT/docs/$d.md" ] && install -Dm644 "$ROOT/docs/$d.md" \
		"$PKG_DIR/usr/share/doc/gorilla-opencode/$d.md"
done

for cl in "$ROOT"/Changelogs/CHANGELOG*.md; do
  [ -f "$cl" ] && install -Dm644 "$cl" "$PKG_DIR/usr/share/doc/gorilla-opencode/$(basename "$cl")"
done
# GORILLA OVERRIDE (2026-08-20): *release-notes*.md, not *release-notes.md —
# the same bug fixed in build-deb.sh the same day. The dual-track docs split
# into v<ver>-release-notes.layman.md and .developer.md, and the anchored glob
# matched neither, so every package built after the split carried all the OLD
# releases' notes and none for the release it actually was. A full-looking
# directory is how the omission stayed invisible.
for rn in "$ROOT"/Changelogs/*release-notes*.md; do
  [ -f "$rn" ] && install -Dm644 "$rn" "$PKG_DIR/usr/share/doc/gorilla-opencode/$(basename "$rn")"
done
[ -f "$ROOT/docs/SETTINGS.md" ] && install -Dm644 "$ROOT/docs/SETTINGS.md" \
  "$PKG_DIR/usr/share/doc/gorilla-opencode/SETTINGS.md"

# Generate .PKGINFO
SIZE_BYTES=$(du -sb "$PKG_DIR" | cut -f1)
BUILD_DATE=$(date -u +%s)

# GORILLA OVERRIDE (2026-08-20): three dependencies, not one. This declared only
# lynx while the .deb declared lynx, python3 and ripgrep - so pacman reported a
# clean install and the find tool then REFUSED on first use.
#
#   python   - HARD. find.go returns "the find tool needs python3 (its search
#              engine is a Python program embedded in this binary)" and refuses.
#              find replaced ls, grep and glob, so without it the agent has no
#              search at all. NOTE THE NAME: Arch ships `python`, NOT `python3`.
#              Copying the Debian string across verbatim yields an unsatisfiable
#              dependency - a silent gap turned into a package that will not
#              install.
#   ripgrep  - pfind.py accelerates through it when present (HAVE_RG) and works
#              without it. Declared anyway, to match the .deb: per the 2026-08-09
#              lynx decision, a promise that only holds for people who install
#              the expert way is not a promise.
cat > "$PKG_DIR/.PKGINFO" <<EOF
pkgname = gorilla-opencode
pkgbase = gorilla-opencode
pkgver = ${VERSION}-${PKGREL}
pkgdesc = Terminal AI coding agent (revived original OpenCode) — bring your own API keys
url = https://github.com/gorillanobakaa-dot/Gorilla.Opencode
builddate = ${BUILD_DATE}
packager = gorillanobakaa <gorillanobakaa@gmail.com>
size = ${SIZE_BYTES}
arch = x86_64
license = MIT
depend = lynx
depend = python
depend = ripgrep
depend = xclip
EOF

# Generate .MTREE using bsdtar
(
  cd "$PKG_DIR"
  LANG=C bsdtar -czf .MTREE --format=mtree \
    --options='!all,use-set,type,uid,gid,mode,time,size,md5,sha256' \
    .PKGINFO usr
)

# Build the tar.zst archive. Write the tar as a complete intermediate artifact
# before compression; the previous `bsdtar | zstd > package` pipeline could leave
# a truncated archive while still returning the final compressor's status.
OUTDIR="$ROOT/Compiled.Builds"
mkdir -p "$OUTDIR"
ARCH_PKG="$OUTDIR/gorilla-opencode-${VERSION}-${PKGREL}-x86_64.pkg.tar.zst"
TAR_FILE="$STAGE/gorilla-opencode-${VERSION}-${PKGREL}-x86_64.pkg.tar"

(
  cd "$PKG_DIR"
  LANG=C bsdtar -cf "$TAR_FILE" .PKGINFO .MTREE usr
)
test -s "$TAR_FILE"
zstd -q -z -19 -T1 -f -o "$ARCH_PKG" "$TAR_FILE"
zstd -t "$ARCH_PKG"

echo "Built Arch/CachyOS package: $ARCH_PKG"
