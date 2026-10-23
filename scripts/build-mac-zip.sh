#!/usr/bin/env bash
# Version: 1.0.0 · updated 26-10-23-13-12
# Build the Mac release asset: one zip a person downloads from the release page.
# Usage: scripts/build-mac-zip.sh <version>      e.g. scripts/build-mac-zip.sh 0.1.150
#
# WHY ONE ZIP (2026-10-23): packaging/macos/Install Gorilla OpenCode.command
# needs the archive and its checksum file next to it, and it has to keep its
# "executable" bit to open on a double-click. Loose release assets lose both:
# GitHub renames a file with spaces ("Install.Gorilla.OpenCode.command") and a
# browser download is never executable. A zip keeps names and permissions, and
# Safari unpacks it into one folder in Downloads with everything in place.
#
# Inside the zip, folder gorilla-opencode-<version>-mac/:
#   Install Gorilla OpenCode.command      double-click to install
#   Undo Gorilla OpenCode.command         double-click to go back
#   gorilla-opencode-<version>-mac-10.13-x86_64.tar.gz
#       gorilla-opencode-<version>-mac/gorilla-opencode-mac   the program
#       gorilla-opencode-<version>-mac/Gorilla OpenCode.app   the app
#   SHA256SUMS-<version>-mac.txt          checked by the installer
#
# Output: Compiled.Builds/gorilla-opencode-<version>-mac-10.13-x86_64.zip
set -euo pipefail

VERSION="${1:?usage: scripts/build-mac-zip.sh <version>}"
VERSION="${VERSION#v}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MAC="$ROOT/packaging/macos"
OUTDIR="$ROOT/Compiled.Builds"
NAME="gorilla-opencode-$VERSION-mac"
TGZ="$NAME-10.13-x86_64.tar.gz"
ZIP="$NAME-10.13-x86_64.zip"

for f in "Gorilla OpenCode.command" gorilla-opencode-launch Info.plist gorilla.icns \
         "Install Gorilla OpenCode.command" "Undo Gorilla OpenCode.command"; do
  [ -f "$MAC/$f" ] || { echo "missing packaging/macos/$f" >&2; exit 1; }
done

STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT
APPDIR="$STAGE/tar/$NAME/Gorilla OpenCode.app/Contents"
mkdir -p "$APPDIR/MacOS" "$APPDIR/Resources" "$STAGE/zip/$NAME" "$OUTDIR"

# The program, built for 10.13 (patched GOROOT, see the script).
VERSION="$VERSION" bash "$ROOT/scripts/build-darwin-legacy.sh" "$STAGE/tar/$NAME/gorilla-opencode-mac"

# This machine cannot run a Mac binary, so check what it can: a Mach-O x86_64
# file that carries the version being packaged (the v0.1.46 lesson: an asset's
# name is not its contents).
file "$STAGE/tar/$NAME/gorilla-opencode-mac" | grep -q "Mach-O 64-bit x86_64" \
  || { echo "not a Mach-O x86_64 binary" >&2; exit 1; }
grep -aq "v$VERSION" "$STAGE/tar/$NAME/gorilla-opencode-mac" \
  || { echo "the binary does not carry v$VERSION" >&2; exit 1; }

# The app.
sed "s/@VERSION@/$VERSION/g" "$MAC/Info.plist" > "$APPDIR/Info.plist"
cp "$MAC/gorilla-opencode-launch" "$APPDIR/MacOS/"
cp "$MAC/Gorilla OpenCode.command" "$MAC/gorilla.icns" "$APPDIR/Resources/"
chmod 755 "$STAGE/tar/$NAME/gorilla-opencode-mac" "$APPDIR/MacOS/gorilla-opencode-launch" \
  "$APPDIR/Resources/Gorilla OpenCode.command"

# The archive the installer reads, and its checksum in shasum -c form.
tar czf "$STAGE/zip/$NAME/$TGZ" -C "$STAGE/tar" "$NAME"
(cd "$STAGE/zip/$NAME" && sha256sum "$TGZ" > "SHA256SUMS-$VERSION-mac.txt" && sha256sum -c --quiet "SHA256SUMS-$VERSION-mac.txt")

cp "$MAC/Install Gorilla OpenCode.command" "$MAC/Undo Gorilla OpenCode.command" "$STAGE/zip/$NAME/"
chmod 755 "$STAGE/zip/$NAME/"*.command

rm -f "$OUTDIR/$ZIP"
(cd "$STAGE/zip" && zip -qr -X "$OUTDIR/$ZIP" "$NAME")

# The artifact, not the exit code: list it and prove the installer is executable inside.
unzip -Z "$OUTDIR/$ZIP" | grep -q -- "-rwxr-xr-x.*Install Gorilla OpenCode.command" \
  || { echo "the installer is not executable inside the zip" >&2; exit 1; }
ls -l "$OUTDIR/$ZIP"
echo "OK: $OUTDIR/$ZIP"
