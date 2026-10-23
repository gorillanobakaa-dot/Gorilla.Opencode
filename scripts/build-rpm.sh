#!/bin/sh
# Version: 1.0.0 · updated 26-10-23-03-25
# Build the RPM package (Fedora, openSUSE, RHEL family). Usage: scripts/build-rpm.sh <version>
# Requires: Compiled.Builds/gorilla-opencode_<version>_amd64.deb (scripts/build-deb.sh, run first), rpmbuild
# (Debian: apt install rpm), dpkg-deb.
#
# GORILLA OVERRIDE (2026-10-23): the RPM is made FROM the .deb's own file tree, not from a second list of files.
# Owner: every release file from one Linux machine, no jumping between three computers. Two hand-kept file lists
# (one per package format) drift: the launcher once existed in three copies and the .deb shipped the stale one
# (CLAUDE.md, v0.1.44). Unpacking the built .deb makes the two packages carry the same bytes by construction.
set -eu
VERSION="${1:?usage: scripts/build-rpm.sh <version>}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUTDIR="$ROOT/Compiled.Builds"
DEB="$OUTDIR/gorilla-opencode_${VERSION}_amd64.deb"
[ -f "$DEB" ] || { echo "No $DEB: run scripts/build-deb.sh $VERSION first (the RPM is made from it)." >&2; exit 1; }
command -v rpmbuild >/dev/null 2>&1 || { echo "rpmbuild is missing (Debian/Ubuntu: sudo apt install rpm)." >&2; exit 1; }

STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT
TREE="$STAGE/tree"
dpkg-deb -x "$DEB" "$TREE"

# the same guard as build-deb.sh: never package a binary whose stamp is not the version asked for
STAMPED="$("$TREE/usr/bin/gorilla-opencode" --version 2>/dev/null || true)"
[ "$STAMPED" = "v$VERSION" ] || { echo "The .deb's binary reports '$STAMPED', not 'v$VERSION'." >&2; exit 1; }

# RPM versions may not contain '-': 0.1.149-test becomes 0.1.149~test (sorts before 0.1.149, as Debian's ~ does)
RPMVER="$(printf '%s' "$VERSION" | tr '-' '~')"
mkdir -p "$STAGE/SPECS" "$STAGE/RPMS" "$STAGE/rpmdb"   # a private rpm database: Debian has no /var/lib/rpm
{
  cat <<EOF
Name:           gorilla-opencode
Version:        $RPMVER
Release:        1
Summary:        Terminal AI coding agent (revived original OpenCode)
License:        MIT
URL:            https://github.com/gorillanobakaa-dot/Gorilla.Opencode
BuildArch:      x86_64
Requires:       python3, lynx, ripgrep
Recommends:     xclip
AutoReqProv:    no

%description
The original MIT-licensed Go OpenCode by Kujtim Hoxha, revived and kept working with 2026 AI providers.
No telemetry, no accounts; bring your own API keys. Dual-track documentation (plain-language and developer)
is installed under /usr/share/doc/gorilla-opencode. Built from the .deb of the same version, same files.

%install
cp -a "$TREE/." "%{buildroot}/"
rm -rf "%{buildroot}/DEBIAN"

%post
command -v gtk-update-icon-cache >/dev/null 2>&1 && gtk-update-icon-cache -f -t /usr/share/icons/hicolor || true
command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database /usr/share/applications || true

%postun
command -v gtk-update-icon-cache >/dev/null 2>&1 && gtk-update-icon-cache -f -t /usr/share/icons/hicolor || true
command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database /usr/share/applications || true

%files
EOF
  # every file from the .deb, by path; directories owned by the system (/usr, /usr/bin ...) are not claimed
  ( cd "$TREE" && find . -path ./DEBIAN -prune -o \( -type f -o -type l \) -print | sed 's|^\.||' | sort ) \
    | sed 's|^|"|; s|$|"|'
} > "$STAGE/SPECS/gorilla-opencode.spec"

rpmbuild -bb --quiet \
  --define "_topdir $STAGE" --define "_rpmdir $STAGE/RPMS" --define "_build_id_links none" \
  --define "_dbpath $STAGE/rpmdb" \
  "$STAGE/SPECS/gorilla-opencode.spec"

RPM="$(find "$STAGE/RPMS" -name '*.rpm' | head -1)"
[ -n "$RPM" ] || { echo "rpmbuild made no package." >&2; exit 1; }
OUT="$OUTDIR/gorilla-opencode-${VERSION}-1.x86_64.rpm"
cp "$RPM" "$OUT"
# verify the artifact, not the exit code: the same file list and the stamped binary inside
DEBN="$(cd "$TREE" && find . -path ./DEBIAN -prune -o \( -type f -o -type l \) -print | wc -l)"
RPMN="$(rpm --dbpath "$STAGE/rpmdb" -qlp "$OUT" 2>/dev/null | wc -l)"
[ "$DEBN" -eq "$RPMN" ] || { echo "file count differs: .deb $DEBN, .rpm $RPMN" >&2; exit 1; }
echo "Built: $OUT ($RPMN files, the same as the .deb)"
