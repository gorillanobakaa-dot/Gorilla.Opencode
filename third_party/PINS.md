# Third-party pins

Every third-party source this repository builds from or copies, with the exact version and where it came from.
A new entry needs the tag AND the commit sha (a tag can be moved; a sha cannot), and its license file.
One section per consumer.

## Copied Go code (third_party/<name>/LICENSE)

| Name | Upstream | Tag | Commit | License | What was taken, where it lives |
|---|---|---|---|---|---|
| hcsshim | https://github.com/microsoft/hcsshim | v0.15.0 | 900183417d29765de2947de22a6e72c22c195550 | MIT (third_party/hcsshim/LICENSE) | Job Object wrapper from internal/jobobject + internal/winapi -> internal/procjob/job_windows.go; pseudoconsole wrapper from internal/conpty -> internal/llm/tools/shell/conpty_windows.go. Copied, not imported. No logging, OpenTelemetry or ETW code was taken. |

## tools/pfind-rs (pfind.exe 4.0.0), crates.io, fetched once on 2026-10-27

Source: crates.io only (`cargo fetch` + `cargo vendor --locked`), committed unpacked in `tools/pfind-rs/vendor/`.
The versions and checksums below are pinned by `tools/pfind-rs/Cargo.lock` (sha256
fe9b847a4bb0e8a0fb951d3fecf80607568991b24e270c9824cae9160adedd18) and each crate's `.cargo-checksum.json`. Licenses
and their reasoning: `tools/pfind-rs/LICENSES.md`.

| crate | version | license |
|---|---|---|
| adler2 | 2.0.1 | 0BSD OR MIT OR Apache-2.0 |
| aho-corasick | 1.1.5 | Unlicense OR MIT |
| bstr | 1.13.1 | MIT OR Apache-2.0 |
| byteorder | 1.5.0 | Unlicense OR MIT |
| bzip2 | 0.6.1 | MIT OR Apache-2.0 |
| cfg-if | 1.0.5 | MIT OR Apache-2.0 |
| core_detect | 1.0.0 | MIT/Apache-2.0 |
| crc-catalog | 2.5.0 | MIT OR Apache-2.0 |
| crc32fast | 1.5.3 | MIT OR Apache-2.0 |
| crc | 3.4.0 | MIT OR Apache-2.0 |
| crossbeam-deque | 0.8.8 | MIT OR Apache-2.0 |
| crossbeam-epoch | 0.9.21 | MIT OR Apache-2.0 |
| crossbeam-utils | 0.8.23 | MIT OR Apache-2.0 |
| encoding_rs_io | 0.1.8 | MIT OR Apache-2.0 |
| encoding_rs | 0.8.42 | (Apache-2.0 OR MIT) AND BSD-3-Clause |
| filetime | 0.2.29 | MIT/Apache-2.0 |
| flate2 | 1.1.10 | MIT OR Apache-2.0 |
| globset | 0.4.20 | Unlicense OR MIT |
| grep-matcher | 0.1.9 | Unlicense OR MIT |
| grep-regex | 0.1.14 | Unlicense OR MIT |
| grep-searcher | 0.1.17 | Unlicense OR MIT |
| ignore | 0.4.33 | Unlicense OR MIT |
| libbz2-rs-sys | 0.2.5 | bzip2-1.0.6 |
| libc | 0.2.191 | MIT OR Apache-2.0 |
| log | 0.4.34 | MIT OR Apache-2.0 |
| lzma-rs | 0.3.0 | MIT |
| memchr | 2.8.3 | Unlicense OR MIT |
| memmap2 | 0.9.12 | MIT OR Apache-2.0 |
| miniz_oxide | 0.9.1 | MIT OR Zlib OR Apache-2.0 |
| multiversion_no_op | 1.0.0 | Apache-2.0 OR MIT |
| proc-macro2 | 1.0.107 | MIT OR Apache-2.0 |
| quote | 1.0.47 | MIT OR Apache-2.0 |
| regex-automata | 0.4.18 | MIT OR Apache-2.0 |
| regex-syntax | 0.8.11 | MIT OR Apache-2.0 |
| regex | 1.13.1 | MIT OR Apache-2.0 |
| rustversion | 1.0.23 | MIT OR Apache-2.0 |
| ruzstd | 0.8.3 | MIT |
| same-file | 1.0.6 | Unlicense/MIT |
| scopeguard | 1.2.0 | MIT OR Apache-2.0 |
| serde_core | 1.0.229 | MIT OR Apache-2.0 |
| serde_derive | 1.0.229 | MIT OR Apache-2.0 |
| simd-adler32 | 0.3.10 | MIT |
| simdutf8 | 0.1.5 | MIT OR Apache-2.0 |
| syn | 3.0.9 | MIT OR Apache-2.0 |
| tar | 0.4.46 | MIT OR Apache-2.0 |
| twox-hash | 2.1.5 | MIT |
| unicode-ident | 1.0.27 | (MIT OR Apache-2.0) AND Unicode-3.0 |
| walkdir | 2.5.0 | Unlicense/MIT |
| winapi-util | 0.1.11 | Unlicense OR MIT |
| windows-link | 0.2.1 | MIT OR Apache-2.0 |
| windows-sys | 0.61.2 | MIT OR Apache-2.0 |

proc-macro2, quote, syn, unicode-ident, serde_core and serde_derive are in the lock file for other crates' optional
features. They are vendored because `--locked` checks the whole lock file, and none of them is compiled into
pfind.exe (`cargo tree --target all` shows none of them).

## Embedded toolbox (internal/toolbox, `-tags toolbox` builds), fetched once and built from source on 2026-10-27

Every helper below is embedded in gorilla-opencode.exe by `scripts/build-toolbox.ps1` and run only if its bytes
match `internal/toolbox/SHA256SUMS`. Rule for the tag: the newest release tag dated on or before 2026-10-13 (at least
14 days old). Each tool's own license file is in `third_party/<tool>/`, fetched from the pinned commit with
`fieldkit evidence fetch` (sha256 below).

| tool | upstream | tag | tag date | commit | license (file, sha256) | shipped file (windows-amd64) |
|---|---|---|---|---|---|---|
| pfind (ours) | tools/pfind-rs | 4.0.0 | 2026-10-27 | merge cdc01cc | AGPL-3.0-or-later (tools/pfind-rs/LICENSES.md) | pfind.exe, 3,272,704 B |
| difftastic | https://github.com/Wilfred/difftastic | 0.71.0 | 2026-09-18 | b7d119e90ac9f972f03f69508da765dacd302c0c | MIT (difftastic/LICENSE, 76f1045e65caa521762a76bbddcc24b6e6ec2251a097d0b3975ca35de47f11e2) | difft.exe, 116,142,080 B |
| sd | https://github.com/chmln/sd | v1.1.0 | 2026-02-25 | 4a7b216552d64134c0fa17a59b9d557d89019f0f | MIT (sd/LICENSE, 531a4a325368974524d09db46e9f48c37c6c7f10357176cb2a727540c80f313e) | sd.exe, 2,180,096 B |
| hyperfine | https://github.com/sharkdp/hyperfine | v2.0.0 | 2026-10-07 | b33755fe1fca8bcc1f182c773ca3ab040bfe517d | MIT OR Apache-2.0 (hyperfine/LICENSE-MIT 1dfee18c2ff07ce551de4d6a1d2db158c0380746b488a7f0d08c8e0d3568b7c3, hyperfine/LICENSE-APACHE c71d239df91726fc519c6eb72d318ec65820627232b2f796219e87dcf35d0ab4) | hyperfine.exe, 1,355,264 B |
| ast-grep | https://github.com/ast-grep/ast-grep | 0.50.0 | 2026-10-07 | 4118186ecca8f4935ae8d85ee6d43c3211f3b763 | MIT (ast-grep/LICENSE, 81471889c77b2161a3e4dcdb1b2e6ca382e485766132d92d5fe1d7497e7dd2d9) | ast-grep.exe, 50,155,008 B |
| ninja | https://github.com/ninja-build/ninja | v1.13.2 | 2025-11-20 | 3441b633c2fe2c494e958780ba0f4227b1327634 | Apache-2.0 (ninja/COPYING, eb7e9ab9690124c5c9f42bdc81383d886a3dede26345b6ed15bbad7caf81f7ea) | ninja.exe, 1,341,440 B |
| uutils coreutils | https://github.com/uutils/coreutils | 0.12.0 | 2026-09-17 | dc1efd89948a9ca4c78c3a4b9a6ac891019a8c69 | MIT (coreutils/LICENSE, 29953405eaa3dcc41c37d1621d55b6a47eee93e05613e439e73295029740b10c) | coreutils.exe (multicall, `--features windows`), 12,182,016 B |

**Linux twins (2026-11-01).** The same six tools are pinned for linux-amd64 (`linux-amd64/<name>` in `internal/toolbox/SHA256SUMS`), built on the VAIO from the same sources, target `x86_64-unknown-linux-musl` (task T-20261027-012841-ee4400); ninja came out as a static glibc build. pfind for Linux is `cargo build --release --offline --locked --target x86_64-unknown-linux-musl` (rust-lld) of tools/pfind-rs on the Windows laptop. Run on Linux: not by this job.

Not shipped: `sg.exe` (upstream's deprecated alias of ast-grep, a second 50 MB copy on Windows). Upstream quirk:
`sd --version` prints 1.0.0 at tag v1.1.0 (upstream did not bump Cargo.toml). hyperfine is the rule pick v2.0.0.

**Who built what.** The six third-party tools were built from upstream git at the commits above on the owner's
second machine (the VAIO: Debian 13 trixie, x86_64), offline after one `cargo vendor`, target
`x86_64-pc-windows-gnu` (mingw-w64 GCC 14-win32, statically linked, stripped, RUSTFLAGS empty):
rustc 1.95.0 (59807616e 2026-04-14) / cargo 1.95.0 for difftastic, sd, ast-grep and coreutils (difftastic and
ast-grep pin 1.85.0 in rust-toolchain.toml; built with RUSTUP_TOOLCHAIN=stable, newer than every declared
rust-version, no upstream code patched); rustc 1.99.1 (58cf8f9c3 2026-10-13) for hyperfine v2.0.0 (rust-version
1.97.0); ninja with CMake 3.31.6 Release + mingw `-static`. pfind.exe was built on the Windows laptop
(`cargo build --release --offline --locked`, x86_64-pc-windows-msvc). Each exe ran `--version` or a real invocation
on real Windows (exit 0) before it was pinned.

**Where the vendored sources live (measured, decided).** The six `cargo vendor` trees, packed as
`vendor-<tool>.tar.zst`, are 161,981,237 bytes together (ast-grep 34,608,128; coreutils 56,479,601; difftastic
32,054,520; hyperfine v2.0.0 21,816,387; sd 17,022,601; ninja has none). They are NOT committed: that would put
162 MB into every clone of a repository whose own source is a few MB, for a rebuild path only a release maker
uses. They stay beside the from-source build output (on the VAIO and its copy on the Windows laptop), pinned here by
sha256, and go up as release assets when a release ships the toolbox. Rebuild: check out the tool at its commit,
`tar -I zstd -xf vendor-<tool>.tar.zst -C <checkout>`, `cargo build --release --offline --locked`.

| vendor tarball | bytes | sha256 |
|---|---|---|
| vendor-ast-grep.tar.zst | 34,608,128 | b7dbf2110c8285cd64f051b25a40c02af7f6c1be5f507619d6ca8cd9c8ab4640 |
| vendor-coreutils.tar.zst | 56,479,601 | 8262681404ad8526491295c58802673d630c90d5a6982476eb213ee0d90a7261 |
| vendor-difftastic.tar.zst | 32,054,520 | 02e5c7d9dee9be939dd864da56feeeb93aac5563108f792b1605f3a2fdc6659a |
| vendor-hyperfine.tar.zst (v2.0.0) | 21,816,387 | 23403bd9313747f2d84608aac237703341ae539f8dfe6b33424b16f9622810ea |
| vendor-sd.tar.zst | 17,022,601 | d44ac85b368c359da0311d432491849475725ed71f96fdbf2221cdb0dfffda4a |

## Image codecs (internal/toolbox, job P4b, 2026-10-31)

The official prebuilt release of each, downloaded once, archive hashed, the one program taken out and hashed (that
hash is the line in `internal/toolbox/SHA256SUMS`). Newest release at least 14 days old on 2026-10-31. Both systems
get the same three names (parity_test.go). Windows `--version` runs exit 0; the Linux files were NOT run here.

| tool | release | date | archive (sha256) | program |
|---|---|---|---|---|
| cwebp (libwebp, BSD-3-Clause) | 1.6.0, storage.googleapis.com/downloads.webmproject.org/releases/webp/ | 2025-07 | libwebp-1.6.0-windows-x64.zip 48886f506b21f62e4661f0f4cbfca19800897c385128e8902542d29a950c93f1; libwebp-1.6.0-linux-x86-64.tar.gz 1c5ffab71efecefa0e3c23516c3a3a1dccb45cc310ae1095c6f14ae268e38067 | bin/cwebp(.exe) |
| avifenc (libavif, BSD-2-Clause; aom and dav1d inside) | v1.4.2, github.com/AOMediaCodec/libavif | 2026-05-26 | windows-artifacts.zip cb2d9fea43dcbab1d0707e3b37eb7b08070ad2fb60a2c188c39ec12382c0484a; linux-artifacts.zip faf58a670ffbfdc0e3559e6d37592cff277c447dd39453f1cd1d7d7f5a20b8ef | avifenc(.exe) |
| oxipng (MIT) | v10.2.1, github.com/oxipng/oxipng | 2026-09-02 | oxipng-10.2.1-x86_64-pc-windows-msvc.zip 7e940f83ee46874b73f53031f96a15834cb70b220af27391fb06fe7b4dd798e1; oxipng-10.2.1-x86_64-unknown-linux-musl.tar.gz 1813750ef592c5350ca79c88f98b2c0876d05c826dc7156245256d4255c1ad17 | oxipng(.exe) |

**Not in the toolbox, on purpose (parity: neither system gets a copy).** mozjpeg `cjpeg`: its releases are source
only (v4.1.1 has no asset; the v4.0.3 zip is Windows-only and from 2021). Tesseract: upstream ships no binary at all
(the Windows setup program is a third party's). ImageMagick: Windows is a multi-file portable folder and Linux an
AppImage, not one program each. The `image` how-to card says how to get these (`scoop install` / `apt install`);
`fieldkit image` uses them when present and falls back to `magick`.

Not done yet: a script that rebuilds the six on Windows (`x86_64-pc-windows-msvc`) from these tarballs (the
reproducibility path; until it is proven the VAIO binaries are the shipped ones), 7-Zip 7za (MSVC build), the
per-crate license notices of the helpers' own dependencies, and Linux/macOS embeds.

### P5b additions (2026-10-31): 7-Zip and FFmpeg, official binaries on both systems

| tool | upstream | version | date | license | shipped files |
|---|---|---|---|---|---|
| 7-Zip | https://www.7-zip.org/a/ | 26.04 | 2026-10-05 (newest at least 14 days old; 26.05 is 2026-10-21) | LGPL + unRAR restriction (third_party/7zip/License.txt, sha256 39d2187b942cc88818dd959e53c6b7dddd152097a42061467c4d56050168d646) | windows-amd64/7za.exe from 7z2604-extra.7z (sha256 dc4b11d3399db18b063630137145f5585d8f7ac847bf3639bd1185d7d1f7cee0); linux-amd64/7za = 7zz from 7z2604-linux-x64.tar.xz (sha256 fc0327ba27e89bd086cf426dff17d77de582953cdbbc10a6576540a06853ffcd) |
| FFmpeg | Windows: https://github.com/GyanD/codexffmpeg (tagged releases, linked from ffmpeg.org/download.html); Linux: https://johnvansickle.com/ffmpeg (static builds, linked from ffmpeg.org/download.html; old versions move to /old-releases/) | **7.0.2** on both systems | Windows 7.0.2 tag; Linux 7.0.2 (2024). Moved off BtbN autobuild-2026-10-18-13-01 (n9.0.2) on 2026-11-01 because BtbN's rolling builds are deleted upstream (that release gave 404 and stopped the Gorilla OpenCode build); both new sources keep every version. 7.0.2 is the one version both keep as a permanent, versioned download (johnvansickle's newest static is 7.0.2; Gyan has 9.0.2) | GPL (third_party/ffmpeg/LICENSE.txt, GPLv3, sha256 8ceb4b9ee5adedde47b31e975c1d90c73ad27b6b165a1dcd80c7c545eb65b903; equal to the LICENSE in the Windows zip and GPLv3.txt in the Linux tarball) | ffmpeg, ffprobe from ffmpeg-7.0.2-full_build.zip (0aa623feec95066839db4125679cd04d4a8fc1039ed8877acca9826113e42768) and ffmpeg-7.0.2-amd64-static.tar.xz (abda8d77ce8309141f83ab8edf0596834087c52467f6badf376a6a2a4c87cf67); neither source publishes a digest, hashes measured 2026-11-01 |

Not built from source (unlike the table above): the upstream binaries are pinned by sha256 in `internal/toolbox/SHA256SUMS` and re-checked by `scripts/fetch-toolbox-extras.sh`. 7-Zip publishes no checksum or signature; its hashes are trust-on-first-use over HTTPS, measured 2026-10-31. The Linux executables ran only on the build machine's Windows side as files (not executed: no Linux run was made here).

## macOS twins (internal/toolbox, darwin-amd64 = Intel, goal parity-everywhere, 2026-11-01)

The same 13 names as Windows and Linux, pinned as `darwin-amd64/<name>` in `internal/toolbox/SHA256SUMS`; macOS 10.13
(High Sierra) is the floor. Made on the Windows laptop. Every Mach-O was parsed (load commands read by script): one x86_64
slice, minimum OS and libraries listed below. **None was run on a Mac** (the MacBook Air did not answer: `fieldkit mac find
macair` "NOT FOUND" at 16:4x on 2026-11-01); only the headers were checked. Inbox item carries the smoke command.

| helper | where it came from | min macOS (Mach-O) | libraries |
|---|---|---|---|
| ast-grep 0.50.0, difft 0.71.0, sd v1.1.0, hyperfine v2.0.0, oxipng v10.2.1 | official `x86_64-apple-darwin` release archives | 10.12 | libSystem, libiconv |
| coreutils 0.12.0 | official `coreutils-0.12.0-x86_64-apple-darwin.tar.gz` | 10.12 | libSystem, libiconv, libobjc, Foundation |
| ninja v1.13.2 | built (official `ninja-mac.zip` is 10.15+) | 10.13 | libSystem only (libc++ static) |
| 7za = 7-Zip 26.04 `7zz` | built from `7z2604-src.tar.xz` (official mac build says macOS 26) | 10.13 | libSystem only |
| cwebp = libwebp 1.6.0 | built (official mac build says macOS 15); PNG and JPEG input compiled in | 10.13 | libSystem only |
| avifenc = libavif v1.4.2 + libaom 3.14.1 | built (official macOS artifact is arm64 only); PNG and JPEG input, AV1 encode and decode | 10.13 | libSystem only |
| pfind (ours) | `tools/pfind-rs`, `cargo build --release --offline --locked --target x86_64-apple-darwin` | 10.13 | libSystem, libiconv |
| ffmpeg, ffprobe | osxexperts.net static FFmpeg **8.0** Intel (7.0.2 is on Windows and Linux) | **11.0** | system frameworks, libc++, libz, libbz2 |

**Build recipe (built ones).** zig 0.16.0 as the C/C++ compiler and linker: `zig cc -target x86_64-macos.10.13`, cmake 4.4 +
ninja with a toolchain file (`CMAKE_SYSTEM_NAME Darwin`, `CMAKE_AR` = `zig ar`, `-I<prefix>/include` first so zig's bundled
SDK `zlib.h` does not beat our zlib). Order: zlib 1.3.1, libpng 1.6.50, libjpeg-turbo 3.1.2 (`WITH_SIMD=0`, no nasm), then
libwebp (`WEBP_BUILD_CWEBP=ON` only), libaom (`AOM_TARGET_CPU=generic`, `ENABLE_NASM=OFF`, no tools or tests), libavif
(`AVIF_CODEC_AOM=SYSTEM`, `AVIF_LIBYUV=OFF`, `AVIF_BUILD_APPS=ON`). libavif's cmake downloaded libargparse at configure time
(its own FetchContent; not pinned here). ninja: `browse` mode switched off (its script step does not run on Windows). 7-Zip:
`make -f ../../cmpl_mac_x64.mak CC="zig cc -target x86_64-macos.10.13" CXX="zig c++ ..." MY_ARCH=` in
`CPP/7zip/Bundles/Alone2`, one patch: `CPP/Windows/TimeUtils.cpp` line 263 also excludes `__APPLE__` so it uses
`clock_gettime` (`timespec_get` is 10.15+ and stops the build at target 10.13). pfind: `CARGO_TARGET_X86_64_APPLE_DARWIN_LINKER`
= a `zig cc -target x86_64-macos.10.13` wrapper, `MACOSX_DEPLOYMENT_TARGET=10.13`, `-L` to a stub `libiconv.tbd`.
Source archives were fetched with `fieldkit evidence fetch`; its store keeps the decompressed tar, so those sha256 are of
that tar, not of upstream's .tar.gz: ninja 146d5abb2cb693fb..., 7z2604-src 9691944c0fe0d01b..., libwebp 7f158e50f55ff7bf...,
libavif 6330f741b93bc3914..., zlib 044f6f2ec86d0436..., libpng d17b16c34544187d..., libjpeg-turbo 722f2f6e4c68ae021..., libaom
32a95a492c7a2db4d.... The zig builds were not compared for byte-for-byte repeatability.

**ffmpeg and ffprobe caveat.** evermeet.cx and ffmpeg.martin-riedl.de (the 9.0.2 mac builds) disallow crawlers in robots.txt,
so they were not fetched. osxexperts.net has Intel 8.0 only, and its Mach-O says macOS 11. macOS 10.13's loader ignores that
load command, but a missing system symbol would stop it: only a run on the Mac settles it.
