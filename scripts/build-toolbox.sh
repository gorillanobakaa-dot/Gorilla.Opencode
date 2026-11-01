#!/usr/bin/env bash
# build-toolbox.sh - Linux twin of build-toolbox.ps1: gorilla-opencode with the linux-amd64 helpers embedded.
#   bash scripts/build-toolbox.sh HELPERS_DIR [OUTPUT]     HELPERS_DIR holds linux-amd64/ (scripts/fetch-toolbox-extras.sh)
#   PLATFORM=darwin-amd64 bash scripts/build-toolbox.sh HELPERS_DIR OUTPUT   the Intel Mac build (use go1.24, see build-mac-zip.sh)
# Copies every helper of PLATFORM (default linux-amd64) pinned in internal/toolbox/SHA256SUMS, gzip-compressed, stops if ANY sha256 differs
# from its pin, then go build -tags toolbox. Heavy: run under `fieldkit guard --` on a laptop.
set -euo pipefail
helpers=${1:?usage: build-toolbox.sh HELPERS_DIR [OUTPUT]}
output=${2:-gorilla-opencode-toolbox}
repo=$(cd "$(dirname "$0")/.." && pwd)
platform=${PLATFORM:-linux-amd64}
dest="$repo/internal/toolbox/bin/$platform"
rm -rf "$dest"; mkdir -p "$dest"
n=0
while read -r sum name; do
  case "$sum" in ''|'#'*) continue ;; esac
  case "$name" in "$platform"/*) ;; *) continue ;; esac
  f=${name#"$platform"/}
  got=$(sha256sum "$helpers/$platform/$f" | cut -d' ' -f1)
  [ "$got" = "$sum" ] || { echo "$f has sha256 $got but the pin list has $sum; not embedding it" >&2; exit 1; }
  gzip -9 -c "$helpers/$platform/$f" > "$dest/$f.gz"
  echo "  embedded $f sha256 $got"; n=$((n+1))
done < "$repo/internal/toolbox/SHA256SUMS"
[ "$n" -gt 0 ] || { echo "no $platform line in the pin list" >&2; exit 1; }
cd "$repo"
GOOS=${platform%-*} GOARCH=${platform#*-} CGO_ENABLED=0 go build -tags toolbox -trimpath -ldflags "-s -w" -o "$output" .
ls -l "$output"
