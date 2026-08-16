#!/bin/sh
# Gorilla OpenCode installer — for people who prefer one command.
#
# What this script does, in order, and nothing else:
#   1. Downloads the latest release tarball from GitHub.
#   2. Verifies its sha256 checksum against the published checksum file.
#   3. Runs `gorilla-opencode install`, which copies the binary onto your
#      PATH (~/.local/bin as a normal user), unpacks its embedded icons,
#      and creates a desktop entry. It prints every file it creates.
#   4. Deletes its own temporary download.
# Remove everything later with: gorilla-opencode uninstall
#
# GORILLA OVERRIDE 2026-08-16: this script asked for two assets that no release
# has ever published — a bare `gorilla-opencode-linux-amd64` binary and
# `checksums.sha256`. Every release publishes a .deb, an Arch package and (since
# v0.1.86) a tarball, so the advertised one-line install in README.md has failed
# for every user who has ever tried it. It now asks for the assets that exist.
#
# Both names below must stay VERSION-FREE. `releases/latest/download/<name>`
# only resolves for a fixed filename, so a versioned asset like
# SHA256SUMS-v0.1.86.txt can never be fetched through /latest/ — that is why a
# stable-named checksums.sha256 is published alongside the versioned one.
set -eu

REPO="gorillanobakaa-dot/Gorilla.Opencode"
ASSET="gorilla-opencode-linux-x86_64.tar.gz"
SUMS="checksums.sha256"
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
if ! fetch "$BASE/$ASSET" "$TMP/$ASSET"; then
  echo "Could not download $ASSET from the latest release." >&2
  echo "See https://github.com/$REPO/releases for what is published." >&2
  exit 1
fi
if ! fetch "$BASE/$SUMS" "$TMP/$SUMS"; then
  echo "Could not download $SUMS — refusing to install an unverified binary." >&2
  exit 1
fi

echo "Verifying checksum ..."
# grep must find the line, or sha256sum -c would be handed an empty list and
# report success without checking anything.
( cd "$TMP" \
  && grep " $ASSET\$" "$SUMS" > "$SUMS.one" \
  && test -s "$SUMS.one" \
  && sha256sum -c "$SUMS.one" )

echo "Unpacking ..."
tar -xzf "$TMP/$ASSET" -C "$TMP"
test -f "$TMP/gorilla-opencode" || { echo "Archive did not contain the binary." >&2; exit 1; }

chmod +x "$TMP/gorilla-opencode"
"$TMP/gorilla-opencode" install
