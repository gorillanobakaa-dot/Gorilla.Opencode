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

# GORILLA OVERRIDE (2026-08-17): refuse a binary whose stamp does not match the
# requested version. This script packages whatever sits in the repo root; on
# 2026-08-17 it happily wrapped a stale ~test8 binary as ~test9 AND ~test10 —
# dpkg reported the new version while the installed binary reported the old,
# and only running the extracted inner binary caught it. Same failure class the
# release pipeline was cured of on 2026-07-28: an artifact's name is not its
# contents. A version check at the door makes the mismatch impossible to package.
STAMPED="$("$BIN" --version 2>/dev/null || true)"
[ "$STAMPED" = "v$VERSION" ] || {
	echo "Binary reports '$STAMPED' but you asked to package 'v$VERSION'." >&2
	echo "Rebuild it stamped: go build -ldflags \"-s -w -X github.com/opencode-ai/opencode/internal/version.Version=v$VERSION\" -o gorilla-opencode ." >&2
	exit 1
}

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
# GORILLA: install the TRACKED desktop entry rather than inlining a copy here.
#
# This script used to carry its own copy of the entry, which made three in total
# (this one, packaging/gorilla-opencode.desktop, and the string in cmd/install.go).
# When the plain-mode Desktop Action was added in v0.1.44 the first two were
# updated and this one was not, so the .deb shipped a launcher with no action —
# caught only by inspecting the built package. Consuming the tracked file removes
# the copy instead of trying to remember it.
install -Dm644 "$ROOT/packaging/gorilla-opencode.desktop" \
  "$PKG/usr/share/applications/gorilla-opencode.desktop"

# GORILLA: the SearXNG installer the agent offers to run when web search is not
# configured. Shipped from the tracked packaging/ copy for the same reason as
# the desktop file above - a second copy inlined here is how v0.1.43 shipped a
# launcher with no plain-mode action.
install -Dm755 "$ROOT/packaging/setup-searxng.sh" \
  "$PKG/usr/share/gorilla-opencode/setup-searxng.sh"

# GORILLA: the find tool's search engine. The SAME bytes are embedded in the
# binary (internal/llm/tools/find.go go:embed), so this copy is not what makes
# the tool work — it exists so users get a `pfind` CLI of their own, and so the
# engine is inspectable as a file. Shipped from the tracked vendored copy;
# TestVendoredPfindMatchesDevCopy guards it against drifting from upstream.
install -Dm644 "$ROOT/internal/llm/tools/pfind.py" \
  "$PKG/usr/share/gorilla-opencode/pfind.py"
install -d "$PKG/usr/bin"
cat > "$PKG/usr/bin/pfind" <<'WRAP'
#!/bin/sh
exec python3 /usr/share/gorilla-opencode/pfind.py "$@"
WRAP
chmod 755 "$PKG/usr/bin/pfind"

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
# GORILLA OVERRIDE (2026-08-17): the OSINT/research documentation set — the
# layman explanation, the doctrine mapping (field manuals -> code), the full
# source catalog, and the find-tool token-economics lesson. The package must
# carry the whole "why & how", not just the binary that does it.
# GORILLA (2026-08-18): the generated command reference. v0.1.90 shipped five
# new commands documented only inside the program — someone deciding whether
# to download could not see what it does.
install -Dm644 "$ROOT/docs/COMMANDS.md" "$PKG/usr/share/doc/gorilla-opencode/COMMANDS.md"
# GORILLA OVERRIDE (2026-08-18): /sessions and the storage-management story.
# Dual-track, with the measurement that justifies the design — deleting rows in
# SQLite frees no disk at all until the file is rebuilt, and on a 2 GB device
# that is the difference between an erase and a lie.
install -Dm644 "$ROOT/docs/SESSIONS-AND-STORAGE.md" "$PKG/usr/share/doc/gorilla-opencode/SESSIONS-AND-STORAGE.md"
# GORILLA OVERRIDE (2026-08-18): the built-in code review. The doc matters more
# than most: it is where the "an empty findings list is not clean code" rule is
# written down for a reader who is not looking at the tool's description.
install -Dm644 "$ROOT/docs/CODE-REVIEW.md" "$PKG/usr/share/doc/gorilla-opencode/CODE-REVIEW.md"
# GORILLA OVERRIDE (2026-08-18): the measured footprint — RAM floor, idle
# network, bytes per message. Ships because the audience needs to know it fits
# BEFORE spending 42 minutes downloading it.
install -Dm644 "$ROOT/docs/FOOTPRINT.md" "$PKG/usr/share/doc/gorilla-opencode/FOOTPRINT.md"
install -Dm644 "$ROOT/docs/SATELLITE.md" "$PKG/usr/share/doc/gorilla-opencode/SATELLITE.md"
install -Dm644 "$ROOT/docs/SHELL-SAFETY.md" "$PKG/usr/share/doc/gorilla-opencode/SHELL-SAFETY.md"
install -Dm644 "$ROOT/docs/SECURITY-AUDIT-2026-08-18.md" "$PKG/usr/share/doc/gorilla-opencode/SECURITY-AUDIT-2026-08-18.md"
install -Dm644 "$ROOT/docs/OSINT-RESEARCH.md" "$PKG/usr/share/doc/gorilla-opencode/OSINT-RESEARCH.md"
install -Dm644 "$ROOT/docs/OSINT-DOCTRINE.md" "$PKG/usr/share/doc/gorilla-opencode/OSINT-DOCTRINE.md"
install -Dm644 "$ROOT/docs/OSINT-SOURCE-CATALOG.md" "$PKG/usr/share/doc/gorilla-opencode/OSINT-SOURCE-CATALOG.md"
install -Dm644 "$ROOT/docs/FIND-TOOL-METRICS.md" "$PKG/usr/share/doc/gorilla-opencode/FIND-TOOL-METRICS.md"
for cl in "$ROOT"/Changelogs/CHANGELOG*.md; do
  [ -f "$cl" ] && install -Dm644 "$cl" "$PKG/usr/share/doc/gorilla-opencode/$(basename "$cl")"
done
# GORILLA: the dual-track release notes are named v<ver>-release-notes.md, which
# the CHANGELOG* glob above misses — v0.1.39's notes were absent from the .deb
# until this was added. Ship every release-notes file so the package carries the
# same prose as the GitHub release page.
#
# GORILLA OVERRIDE (2026-08-20): the glob is *release-notes*.md, not
# *release-notes.md. The dual-track docs were split into two files per release
# — v<ver>-release-notes.layman.md and .developer.md — and the old anchored
# glob matched NEITHER. Every release from that split onward shipped a .deb
# containing no notes for the release it was, while still carrying all the
# older single-file ones, so the directory looked full and the omission was
# invisible. Caught at v0.1.111 by the checklist step that greps the built
# package. This is the SECOND time this exact line has silently dropped the
# current release's prose; the first was v0.1.39, described above. A filename
# convention changed and a glob anchored to the old one kept passing.
for rn in "$ROOT"/Changelogs/*release-notes*.md; do
  [ -f "$rn" ] && install -Dm644 "$rn" "$PKG/usr/share/doc/gorilla-opencode/$(basename "$rn")"
done
# Generated settings reference (cmd/settings-doc). Ships so `/settings` has an
# offline counterpart in the package.
[ -f "$ROOT/docs/SETTINGS.md" ] && install -Dm644 "$ROOT/docs/SETTINGS.md" \
  "$PKG/usr/share/doc/gorilla-opencode/SETTINGS.md"

install -d "$PKG/DEBIAN"
SIZE_KB=$(du -sk "$PKG" | cut -f1)
# GORILLA (2026-08-09): lynx is a DEPENDS, not a Recommends.
#
# It was a Recommends, which apt honours and nothing else does. gdebi - the
# right-click graphical installer, and the realistic path for a non-technical
# user - never mentions Recommends anywhere in its source; it resolves Depends
# only. dpkg -i resolves neither. So the two friendliest install routes silently
# skipped the package that makes web search work out of the box, and the feature
# would have appeared broken to exactly the people it was built for.
#
# 641 KB, and it buys the advertised behaviour on every install path. A promise
# that only holds for people who install the expert way is not a promise.
cat > "$PKG/DEBIAN/control" <<EOF
Package: gorilla-opencode
Version: $VERSION
Section: devel
Priority: optional
Architecture: amd64
Installed-Size: $SIZE_KB
Depends: lynx, python3, ripgrep, xclip | xsel | wl-clipboard
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

# GORILLA (2026-08-09): built packages go in Compiled.Builds/, never the repo
# root. Dropping one .deb per release into the root looked harmless and was not:
# after ~30 releases the root held 47 of them, and the system prompt's top-level
# listing - capped at 25 entries - was consumed by build output, so the model was
# never shown cmd/, internal/ or go.mod at all. It also left thirteen
# GITHUB-RELEASE-NOTES-0.1.65…77 lines reading as a counter, which a model given
# the input "oi" duly continued by running `git tag -a v0.1.78`.
OUTDIR="$ROOT/Compiled.Builds"
mkdir -p "$OUTDIR"
DEB="$OUTDIR/gorilla-opencode_${VERSION}_amd64.deb"
# GORILLA (2026-10-25, owner's decision): xz -9e, not dpkg-deb's default -6. Measured on the
# v0.1.152 package: 17.6 MB instead of 22.2 MB for the same files. Unpacking needs ~65 MB of
# memory instead of ~9 MB; every download over a slow or metered link is 4.6 MB lighter.
# scripts/build-packages-nfpm.py (the Windows path) uses the same level.
dpkg-deb --build --root-owner-group -Zxz -z9 -Sextreme "$PKG" "$DEB"
echo "Built: $DEB"
