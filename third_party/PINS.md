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
