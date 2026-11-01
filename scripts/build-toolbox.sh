#!/usr/bin/env bash
# build-toolbox.sh - Linux twin of build-toolbox.ps1: gorilla-opencode with the linux-amd64 helpers embedded.
#   bash scripts/build-toolbox.sh HELPERS_DIR [OUTPUT]     HELPERS_DIR holds linux-amd64/ (scripts/fetch-toolbox-extras.sh)
# Copies every linux-amd64 helper pinned in internal/toolbox/SHA256SUMS, gzip-compressed, stops if ANY sha256 differs
# from its pin, then go build -tags toolbox. Heavy: run under `fieldkit guard --` on a laptop.
set -euo pipefail
helpers=${1:?usage: build-toolbox.sh HELPERS_DIR [OUTPUT]}
output=${2:-gorilla-opencode-toolbox}
repo=$(cd "$(dirname "$0")/.." && pwd)
dest="$repo/internal/toolbox/bin/linux-amd64"
rm -rf "$dest"; mkdir -p "$dest"
n=0
while read -r sum name; do
  case "$sum" in ''|'#'*) continue ;; esac
  case "$name" in linux-amd64/*) ;; *) continue ;; esac
  f=${name#linux-amd64/}
  got=$(sha256sum "$helpers/linux-amd64/$f" | cut -d' ' -f1)
  [ "$got" = "$sum" ] || { echo "$f has sha256 $got but the pin list has $sum; not embedding it" >&2; exit 1; }
  gzip -9 -c "$helpers/linux-amd64/$f" > "$dest/$f.gz"
  echo "  embedded $f sha256 $got"; n=$((n+1))
done < "$repo/internal/toolbox/SHA256SUMS"
[ "$n" -gt 0 ] || { echo "no linux-amd64 line in the pin list" >&2; exit 1; }
cd "$repo"
CGO_ENABLED=0 go build -tags toolbox -trimpath -ldflags "-s -w" -o "$output" .
ls -l "$output"
