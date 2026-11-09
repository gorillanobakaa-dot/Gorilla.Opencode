#!/usr/bin/env bash
# fetch-toolbox-extras.sh - download 7-Zip, FFmpeg, the image codecs (cwebp, avifenc, oxipng; job P4b) and ripgrep, fd, jq
# (P5 stage 3a) from their official releases and lay out the helpers pinned in
# internal/toolbox/SHA256SUMS, for scripts/build-toolbox.ps1 (-HelpersDir) or scripts/build-toolbox.sh.
#   bash scripts/fetch-toolbox-extras.sh OUTDIR     (needs curl, unzip, tar with xz, sha256sum)
# Writes OUTDIR/windows-amd64/{7za,ffmpeg,ffprobe,cwebp,avifenc,oxipng,rg,fd,jq}.exe and OUTDIR/linux-amd64/ the same without .exe;
# OUTDIR/darwin-amd64/{rg,fd,jq} too (the other macOS helpers were built by hand, see third_party/PINS.md).
# Archive hashes of the codecs: third_party/PINS.md ("Image codecs").
# Each archive is checked against the sha256 below (FFmpeg 7.0.2: sha256 measured on 2026-11-01, neither source
# publishes one; 7-Zip likewise, measured 2026-10-31), then every extracted file against the pin list. Any difference stops.
set -euo pipefail
out=${1:?usage: fetch-toolbox-extras.sh OUTDIR}
here=$(cd "$(dirname "$0")/.." && pwd)
pins="$here/internal/toolbox/SHA256SUMS"
# FFmpeg 7.0.2 on both systems, from archives their makers keep for good (BtbN's rolling autobuilds are deleted: W6).
GY=https://github.com/GyanD/codexffmpeg/releases/download/7.0.2
JV=https://johnvansickle.com/ffmpeg
FFW=ffmpeg-7.0.2-full_build.zip
FFL=ffmpeg-7.0.2-amd64-static.tar.xz
WEBP=https://storage.googleapis.com/downloads.webmproject.org/releases/webp
AVIF=https://github.com/AOMediaCodec/libavif/releases/download/v1.4.2
OXI=https://github.com/oxipng/oxipng/releases/download/v10.2.1
RG=https://github.com/BurntSushi/ripgrep/releases/download/15.2.0
FD=https://github.com/sharkdp/fd/releases/download/v10.5.0
JQ=https://github.com/jqlang/jq/releases/download/jq-1.8.2
declare -A url=(
  [7z2604-extra.7z]=https://www.7-zip.org/a/7z2604-extra.7z
  [7z2604-linux-x64.tar.xz]=https://www.7-zip.org/a/7z2604-linux-x64.tar.xz
  [$FFW]=$GY/$FFW
  [$FFL]=$JV/releases/$FFL
  [libwebp-1.6.0-windows-x64.zip]=$WEBP/libwebp-1.6.0-windows-x64.zip
  [libwebp-1.6.0-linux-x86-64.tar.gz]=$WEBP/libwebp-1.6.0-linux-x86-64.tar.gz
  [avif-windows-artifacts.zip]=$AVIF/windows-artifacts.zip
  [avif-linux-artifacts.zip]=$AVIF/linux-artifacts.zip
  [oxipng-10.2.1-x86_64-pc-windows-msvc.zip]=$OXI/oxipng-10.2.1-x86_64-pc-windows-msvc.zip
  [oxipng-10.2.1-x86_64-unknown-linux-musl.tar.gz]=$OXI/oxipng-10.2.1-x86_64-unknown-linux-musl.tar.gz
  [ripgrep-15.2.0-x86_64-pc-windows-msvc.zip]=$RG/ripgrep-15.2.0-x86_64-pc-windows-msvc.zip
  [ripgrep-15.2.0-x86_64-unknown-linux-musl.tar.gz]=$RG/ripgrep-15.2.0-x86_64-unknown-linux-musl.tar.gz
  [ripgrep-15.2.0-x86_64-apple-darwin.tar.gz]=$RG/ripgrep-15.2.0-x86_64-apple-darwin.tar.gz
  [fd-v10.5.0-x86_64-pc-windows-msvc.zip]=$FD/fd-v10.5.0-x86_64-pc-windows-msvc.zip
  [fd-v10.5.0-x86_64-unknown-linux-musl.tar.gz]=$FD/fd-v10.5.0-x86_64-unknown-linux-musl.tar.gz
  [fd-v10.5.0-x86_64-apple-darwin.tar.gz]=$FD/fd-v10.5.0-x86_64-apple-darwin.tar.gz
  [jq-windows-amd64.exe]=$JQ/jq-windows-amd64.exe
  [jq-linux-amd64]=$JQ/jq-linux-amd64
  [jq-macos-amd64]=$JQ/jq-macos-amd64)
declare -A want=(
  [7z2604-extra.7z]=dc4b11d3399db18b063630137145f5585d8f7ac847bf3639bd1185d7d1f7cee0
  [7z2604-linux-x64.tar.xz]=fc0327ba27e89bd086cf426dff17d77de582953cdbbc10a6576540a06853ffcd
  [$FFW]=0aa623feec95066839db4125679cd04d4a8fc1039ed8877acca9826113e42768
  [$FFL]=abda8d77ce8309141f83ab8edf0596834087c52467f6badf376a6a2a4c87cf67
  [libwebp-1.6.0-windows-x64.zip]=48886f506b21f62e4661f0f4cbfca19800897c385128e8902542d29a950c93f1
  [libwebp-1.6.0-linux-x86-64.tar.gz]=1c5ffab71efecefa0e3c23516c3a3a1dccb45cc310ae1095c6f14ae268e38067
  [avif-windows-artifacts.zip]=cb2d9fea43dcbab1d0707e3b37eb7b08070ad2fb60a2c188c39ec12382c0484a
  [avif-linux-artifacts.zip]=faf58a670ffbfdc0e3559e6d37592cff277c447dd39453f1cd1d7d7f5a20b8ef
  [oxipng-10.2.1-x86_64-pc-windows-msvc.zip]=7e940f83ee46874b73f53031f96a15834cb70b220af27391fb06fe7b4dd798e1
  [oxipng-10.2.1-x86_64-unknown-linux-musl.tar.gz]=1813750ef592c5350ca79c88f98b2c0876d05c826dc7156245256d4255c1ad17
  # ripgrep, fd and jq: the digest GitHub shows on each release asset, re-measured with fieldkit size hash on 2026-11-09
  [ripgrep-15.2.0-x86_64-pc-windows-msvc.zip]=71b2fef860abe467217a538ff31de02f5258807c0129f771846f87bd029aafc5
  [ripgrep-15.2.0-x86_64-unknown-linux-musl.tar.gz]=33e15bcf1624b25cdd2a55813a47a2f95dbe126268203e76aa6a585d1e7b149c
  [ripgrep-15.2.0-x86_64-apple-darwin.tar.gz]=af7825fcc69a2afc7a7aea55fc9af90e26421d8f20fe59df32e233c0b8a231c1
  [fd-v10.5.0-x86_64-pc-windows-msvc.zip]=a227701b8551c35a9931d9f6da75503cf86d88e182d71fb849a70864c5d57cd7
  [fd-v10.5.0-x86_64-unknown-linux-musl.tar.gz]=761c72dc8e120d85b22292063be8a796e2eeb20eb3e4f38b8fa2343ccf3514a7
  [fd-v10.5.0-x86_64-apple-darwin.tar.gz]=7e31028c62c6955877735d0406807aa484c2a5e6f86235a59e26c29c301da590
  [jq-windows-amd64.exe]=a6fc67fedaf9128a3309a1e2ebb8b986aeccf70122ee46d2cb4849e423f0c627
  [jq-linux-amd64]=b1c22172dd303f3be49e935aa56aa48a8b7a46e0bc838b4997d3bb451495870f
  [jq-macos-amd64]=e94b266e3c26690550006abe63152b782280f4e14374accdf04cbde844f00bc0)
d="$out/dl"
mkdir -p "$d" "$out/windows-amd64" "$out/linux-amd64" "$out/darwin-amd64"
for f in "${!url[@]}"; do
  # johnvansickle.com moves an old version from releases/ to old-releases/ when the next one ships
  [ -f "$d/$f" ] || curl -sSfL -o "$d/$f" "${url[$f]}" || curl -sSfL -o "$d/$f" "${url[$f]/\/releases\//\/old-releases\/}"
  got=$(sha256sum "$d/$f" | cut -d' ' -f1)
  [ "$got" = "${want[$f]}" ] || { echo "REFUSED $f: sha256 $got, expected ${want[$f]}" >&2; exit 1; }
done
tar -xJf "$d/7z2604-linux-x64.tar.xz" -C "$d" 7zz
mv "$d/7zz" "$out/linux-amd64/7za"
chmod +x "$out/linux-amd64/7za"
# unpack the .7z with whichever 7-Zip runs here: the Linux one just fetched, or one already installed
for x in "$out/linux-amd64/7za" 7zz 7z 7za; do
  if "$x" i >/dev/null 2>&1; then "$x" e -y -o"$out/windows-amd64" "$d/7z2604-extra.7z" 7za.exe >/dev/null; break; fi
done
[ -f "$out/windows-amd64/7za.exe" ] || { echo "no 7-Zip here to unpack 7z2604-extra.7z" >&2; exit 1; }
unzip -q -j -o "$d/$FFW" '*/bin/ffmpeg.exe' '*/bin/ffprobe.exe' -d "$out/windows-amd64"
tar -xJf "$d/$FFL" -C "$out/linux-amd64" --wildcards '*/ffmpeg' '*/ffprobe' --strip-components=1
unzip -q -j -o "$d/libwebp-1.6.0-windows-x64.zip" '*/bin/cwebp.exe' -d "$out/windows-amd64"
tar -xzf "$d/libwebp-1.6.0-linux-x86-64.tar.gz" -C "$out/linux-amd64" --wildcards '*/bin/cwebp' --strip-components=2
unzip -q -j -o "$d/avif-windows-artifacts.zip" '*avifenc.exe' -d "$out/windows-amd64"
unzip -q -j -o "$d/avif-linux-artifacts.zip" '*avifenc' -d "$out/linux-amd64"
unzip -q -j -o "$d/oxipng-10.2.1-x86_64-pc-windows-msvc.zip" 'oxipng-10.2.1-x86_64-pc-windows-msvc/oxipng.exe' -d "$out/windows-amd64"
tar -xzf "$d/oxipng-10.2.1-x86_64-unknown-linux-musl.tar.gz" -C "$out/linux-amd64" oxipng-10.2.1-x86_64-unknown-linux-musl/oxipng --strip-components=1
RGV=ripgrep-15.2.0; FDV=fd-v10.5.0
unzip -q -j -o "$d/$RGV-x86_64-pc-windows-msvc.zip" "$RGV-x86_64-pc-windows-msvc/rg.exe" -d "$out/windows-amd64"
tar -xzf "$d/$RGV-x86_64-unknown-linux-musl.tar.gz" -C "$out/linux-amd64" "$RGV-x86_64-unknown-linux-musl/rg" --strip-components=1
tar -xzf "$d/$RGV-x86_64-apple-darwin.tar.gz" -C "$out/darwin-amd64" "$RGV-x86_64-apple-darwin/rg" --strip-components=1
unzip -q -j -o "$d/$FDV-x86_64-pc-windows-msvc.zip" "$FDV-x86_64-pc-windows-msvc/fd.exe" -d "$out/windows-amd64"
tar -xzf "$d/$FDV-x86_64-unknown-linux-musl.tar.gz" -C "$out/linux-amd64" "$FDV-x86_64-unknown-linux-musl/fd" --strip-components=1
tar -xzf "$d/$FDV-x86_64-apple-darwin.tar.gz" -C "$out/darwin-amd64" "$FDV-x86_64-apple-darwin/fd" --strip-components=1
cp "$d/jq-windows-amd64.exe" "$out/windows-amd64/jq.exe"
cp "$d/jq-linux-amd64" "$out/linux-amd64/jq"
cp "$d/jq-macos-amd64" "$out/darwin-amd64/jq"
chmod +x "$out"/linux-amd64/* "$out"/darwin-amd64/*
for p in windows-amd64/7za.exe windows-amd64/ffmpeg.exe windows-amd64/ffprobe.exe linux-amd64/7za linux-amd64/ffmpeg linux-amd64/ffprobe \
         windows-amd64/cwebp.exe windows-amd64/avifenc.exe windows-amd64/oxipng.exe linux-amd64/cwebp linux-amd64/avifenc linux-amd64/oxipng \
         windows-amd64/rg.exe windows-amd64/fd.exe windows-amd64/jq.exe linux-amd64/rg linux-amd64/fd linux-amd64/jq \
         darwin-amd64/rg darwin-amd64/fd darwin-amd64/jq; do
  pin=$(awk -v k="$p" '$2==k{print $1}' "$pins")
  got=$(sha256sum "$out/$p" | cut -d' ' -f1)
  [ "$pin" = "$got" ] || { echo "REFUSED $p: sha256 $got, pinned $pin" >&2; exit 1; }
  echo "ok $p $got"
done
