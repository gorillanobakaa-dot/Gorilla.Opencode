#!/bin/sh
# Build the portable tarball release asset. Usage: scripts/build-tarball.sh <version>
# Requires: a built ./gorilla-opencode binary in the repo root.
#
# WHY THIS EXISTS (2026-08-16): the repo's `install` script has always fetched
# "gorilla-opencode-linux-x86_64.tar.gz", and that asset was never built or
# published — so the documented one-line install was broken for every release up
# to and including v0.1.85. Publishing the .deb and the Arch package covers
# Debian/Ubuntu/Mint and Arch/CachyOS; this tarball covers everyone else
# (Fedora, openSUSE, Alpine, any distro, and anyone without sudo) by dropping a
# single static binary into ~/.local/bin.
#
# The archive deliberately contains ONE file, the binary, at the archive root:
# the install script extracts it and expects ./gorilla-opencode. Desktop entry
# and icons are not shipped here — `gorilla-opencode install` unpacks those from
# the binary's own embedded assets.
set -eu

VERSION="${1:?usage: scripts/build-tarball.sh <version>}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$ROOT/gorilla-opencode"
[ -x "$BIN" ] || { echo "Build the binary first: go build -o gorilla-opencode ." >&2; exit 1; }

# Verify the artifact, never the exit code: the binary must already carry the
# version being packaged, or the asset name lies about its contents. This is the
# v0.1.46 lesson (a package whose control file said 0.1.46 containing a binary
# that reported v0.1.45), applied before anything is written.
GOT="$("$BIN" --version 2>/dev/null | tr -d 'v' | awk '{print $NF}')" || GOT=""
if [ "$GOT" != "$VERSION" ]; then
    echo "Refusing to package: binary reports '${GOT:-nothing}', asked for '$VERSION'." >&2
    echo "Rebuild it stamped:" >&2
    echo "  go build -ldflags \"-s -w -X github.com/opencode-ai/opencode/internal/version.Version=v$VERSION\" -o gorilla-opencode ." >&2
    exit 1
fi

OUTDIR="$ROOT/Compiled.Builds"
mkdir -p "$OUTDIR"
TARBALL="$OUTDIR/gorilla-opencode-linux-x86_64.tar.gz"

STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT
install -Dm755 "$BIN" "$STAGE/gorilla-opencode"

# Write the tar as a complete intermediate before compressing. A
# `tar -c ... | gzip > file` pipeline reports gzip's status, not tar's, so a
# truncated archive can be reported as a success — the same defect that had to
# be fixed in build-arch.sh.
TAR_FILE="$STAGE/gorilla-opencode-linux-x86_64.tar"
( cd "$STAGE" && tar -cf "$TAR_FILE" gorilla-opencode )
test -s "$TAR_FILE"
gzip -9 -c "$TAR_FILE" > "$TARBALL"

# Prove the artifact before claiming success.
gzip -t "$TARBALL"
CONTENTS="$(tar -tzf "$TARBALL")"
[ "$CONTENTS" = "gorilla-opencode" ] || {
    echo "Unexpected archive contents:" >&2; echo "$CONTENTS" >&2; exit 1; }

VERIFY="$(mktemp -d)"
tar -xzf "$TARBALL" -C "$VERIFY"
ROUNDTRIP="$("$VERIFY/gorilla-opencode" --version 2>/dev/null | tr -d 'v' | awk '{print $NF}')" || ROUNDTRIP=""
rm -rf "$VERIFY"
[ "$ROUNDTRIP" = "$VERSION" ] || {
    echo "Extracted binary reports '${ROUNDTRIP:-nothing}', expected '$VERSION'." >&2; exit 1; }

echo "Built portable tarball: $TARBALL"
echo "  contents:  gorilla-opencode (v$VERSION), $(stat -c%s "$TARBALL") bytes"
