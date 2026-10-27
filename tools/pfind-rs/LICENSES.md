# pfind.exe licenses

pfind.exe (this folder) is **AGPL-3.0-or-later**, the license of pfind.py, which it ports. The header and changelog
are in `src/main.rs`. Copyright (C) 2026 gorillanobakaa.

The program is pfind's own code plus the crates.io crates below, vendored unchanged in `vendor/` with their license
files (`vendor/<crate>/LICENSE*`, `COPYING`, `UNLICENSE`). Each one's license allows it to be part of an
AGPL-3.0-or-later program. The reasoning follows the table.

## What is inside, and under which license

| crate | version | license (as the crate declares it) |
|---|---|---|
| adler2 | 2.0.1 | 0BSD OR MIT OR Apache-2.0 |
| aho-corasick | 1.1.5 | Unlicense OR MIT |
| bstr | 1.13.1 | MIT OR Apache-2.0 |
| byteorder | 1.5.0 | Unlicense OR MIT |
| bzip2 | 0.6.1 | MIT OR Apache-2.0 |
| cfg-if | 1.0.5 | MIT OR Apache-2.0 |
| core_detect | 1.0.0 | MIT/Apache-2.0 |
| crc-catalog | 2.5.0 | MIT OR Apache-2.0 |
| crc | 3.4.0 | MIT OR Apache-2.0 |
| crc32fast | 1.5.3 | MIT OR Apache-2.0 |
| crossbeam-deque | 0.8.8 | MIT OR Apache-2.0 |
| crossbeam-epoch | 0.9.21 | MIT OR Apache-2.0 |
| crossbeam-utils | 0.8.23 | MIT OR Apache-2.0 |
| encoding_rs | 0.8.42 | (Apache-2.0 OR MIT) AND BSD-3-Clause |
| encoding_rs_io | 0.1.8 | MIT OR Apache-2.0 |
| filetime | 0.2.29 | MIT/Apache-2.0 |
| flate2 | 1.1.10 | MIT OR Apache-2.0 |
| globset | 0.4.20 | Unlicense OR MIT (ripgrep) |
| grep-matcher | 0.1.9 | Unlicense OR MIT (ripgrep) |
| grep-regex | 0.1.14 | Unlicense OR MIT (ripgrep) |
| grep-searcher | 0.1.17 | Unlicense OR MIT (ripgrep) |
| ignore | 0.4.33 | Unlicense OR MIT (ripgrep) |
| libbz2-rs-sys | 0.2.5 | bzip2-1.0.6 |
| libc | 0.2.191 | MIT OR Apache-2.0 |
| log | 0.4.34 | MIT OR Apache-2.0 |
| lzma-rs | 0.3.0 | MIT |
| memchr | 2.8.3 | Unlicense OR MIT |
| memmap2 | 0.9.12 | MIT OR Apache-2.0 |
| miniz_oxide | 0.9.1 | MIT OR Zlib OR Apache-2.0 |
| multiversion_no_op | 1.0.0 | Apache-2.0 OR MIT |
| regex | 1.13.1 | MIT OR Apache-2.0 |
| regex-automata | 0.4.18 | MIT OR Apache-2.0 |
| regex-syntax | 0.8.11 | MIT OR Apache-2.0 |
| rustversion | 1.0.23 | MIT OR Apache-2.0 |
| ruzstd | 0.8.3 | MIT |
| same-file | 1.0.6 | Unlicense/MIT |
| scopeguard | 1.2.0 | MIT OR Apache-2.0 |
| simd-adler32 | 0.3.10 | MIT |
| simdutf8 | 0.1.5 | MIT OR Apache-2.0 |
| tar | 0.4.46 | MIT OR Apache-2.0 |
| twox-hash | 2.1.5 | MIT |
| walkdir | 2.5.0 | Unlicense/MIT |
| winapi-util | 0.1.11 | Unlicense OR MIT |
| windows-link | 0.2.1 | MIT OR Apache-2.0 |
| windows-sys | 0.61.2 | MIT OR Apache-2.0 |
| proc-macro2, quote, syn, unicode-ident, serde_core, serde_derive | 1.0.107, 1.0.47, 3.0.9, 1.0.27, 1.0.229, 1.0.229 | MIT OR Apache-2.0 (unicode-ident: AND Unicode-3.0) |

The last row is vendored because `Cargo.lock` lists optional features of other crates. `cargo tree --target all`
shows none of them in pfind's build: they are never compiled into pfind.exe. They stay in `vendor/` because
`cargo build --locked` checks the whole lock file.

## Why every license fits AGPL-3.0-or-later

- **MIT, Unlicense, 0BSD, Zlib, BSD-3-Clause, bzip2-1.0.6, Unicode-3.0** are permissive. They allow the code to be
  part of a program under any license as long as the notice is kept. The notices are kept: each crate's license
  file sits in `vendor/<crate>/`, and this file lists them.
- **Apache-2.0** (only ever offered as an alternative to MIT here) is compatible with GPL-3.0 and AGPL-3.0 (the FSF
  lists Apache-2.0 as compatible with version 3 of the GPL family). pfind takes the MIT option anyway wherever a
  crate offers both.
- **ripgrep's crates** (`ignore`, `globset`, `grep-regex`, `grep-searcher`, `grep-matcher`, and `memchr`,
  `aho-corasick`, `walkdir`, `same-file`, `winapi-util` by the same author) are Unlicense OR MIT. pfind takes MIT.

## eza (EUPL-1.2): what was decided, and why

The brief named "eza's listing, git-status, tree and LOC code" as a source. eza is licensed EUPL-1.2.

**Is EUPL-1.2 compatible with AGPL-3.0?** Yes, in one direction. EUPL-1.2 Article 5 ("Compatibility clause") lets
a licensee distribute a derivative work that combines EUPL code with code under a license in the Appendix's
"Compatible Licences" list under that other license. The Appendix lists the GNU Affero General Public License
(AGPL) v. 3. So a combined work holding eza code could be distributed under AGPL-3.0, with eza's notices kept.

**What was done: no eza code was copied.** pfind.py 3.0.0 had already reimplemented, in its own code, the eza
features pfind uses: the long table, git status badges, the tree view, the comment-aware LOC counter, icons, sizes
and relative times. That code is AGPL-3.0-or-later and owned by this project. pfind.exe ports pfind.py's code to
Rust, function by function (`src/meta.rs`, `src/render.rs`, `src/data.rs`), so that its output matches pfind.py
byte for byte. The parity runner `tests/run_parity.py` checks this. Copying eza's Rust source would have produced
different output from pfind.py, and the job was parity with pfind.py. So the compatibility clause is not used, no
EUPL-licensed file is in this tree, and no eza notice is needed. Should eza code be added later, Article 5 allows
it under AGPL-3.0: keep eza's copyright notice and LICENCE file in `third_party/eza/` and name the files taken.

## Code written instead of vendored

`src/zipread.rs` (a .zip member reader), `src/json.rs` (presets.json) and `src/timefmt.rs` (local time and strftime)
replace the `zip`, `serde_json` and `chrono` crates. Those three pulled in 40 more crates, proc-macros and
wasm/JavaScript bindings among them, for small jobs. These files are pfind's own code (AGPL-3.0-or-later).
