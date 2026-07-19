#!/bin/sh
# Gorilla OpenCode installer — for people who prefer one command.
#
# What this script does, in order, and nothing else:
#   1. Downloads the latest release binary from GitHub.
#   2. Verifies its sha256 checksum against the published checksum file.
#   3. Runs `gorilla-opencode install`, which copies the binary onto your
#      PATH (~/.local/bin as a normal user), unpacks its embedded icons,
#      and creates a desktop entry. It prints every file it creates.
#   4. Deletes its own temporary download.
# Remove everything later with: gorilla-opencode uninstall
set -eu

REPO="gorillanobakaa-dot/Gorilla.Opencode"
ASSET="gorilla-opencode-linux-amd64"
BASE="https://github.com/$REPO/releases/latest/download"

case "$(uname -s)/$(uname -m)" in
  Linux/x86_64) ;;
  *) echo "Sorry: prebuilt binaries currently exist only for Linux x86_64." >&2
     echo "Build from source instead: go build -o gorilla-opencode ." >&2
     exit 1 ;;
esac

fetch() { # fetch <url> <outfile>
  if command -v curl >/dev/null 2>&1; then curl -fsSL "$1" -o "$2"
  elif command -v wget >/dev/null 2>&1; then wget -qO "$2" "$1"
  else echo "Need curl or wget." >&2; exit 1
  fi
}

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "Downloading $ASSET ..."
fetch "$BASE/$ASSET" "$TMP/$ASSET"
fetch "$BASE/checksums.sha256" "$TMP/checksums.sha256"

echo "Verifying checksum ..."
( cd "$TMP" && grep " $ASSET\$" checksums.sha256 | sha256sum -c - )

chmod +x "$TMP/$ASSET"
"$TMP/$ASSET" install
