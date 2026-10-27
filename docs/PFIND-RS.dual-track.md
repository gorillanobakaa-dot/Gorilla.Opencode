# pfind.exe: the search engine as one native program, dual-track documentation

**The one command that checks it**, from the repository root, after building it (`cargo build --release --offline
--locked` in `tools/pfind-rs`):

```
python -I tools/pfind-rs/tests/run_parity.py --both
```

It runs every ported pfind test against BOTH pfind.py and pfind.exe. It exits 0 only if both pass every test and
their outputs match. The last line says `RESULT: PASS` or `RESULT: FAIL`.

This document follows the [Gorilla Open Source Philosophy](../PHILOSOPHY.md): the work is explained twice, once in
plain language and once with technical precision. Both tracks cover the same facts. Neither is a summary of the
other.

It covers job 03 of the Windows capabilities goal (2026-10-27): pfind 4.0.0, a Rust port of pfind.py 3.3.0, in
`tools/pfind-rs/`.

---

# Track one: plain language

## What happened

The `find` tool is how the AI searches your files. Behind it is a program called pfind. Until now pfind was a Python
script, so your computer needed Python installed, and for full speed also a second program, ripgrep.

pfind is now also one ordinary program, `pfind.exe`. It is written in Rust, and it contains ripgrep's own search
parts. Nothing else has to be installed: no Python, no ripgrep.

It gives the same answers as the Python pfind, word for word. That was checked with 84 tests, each run on both.

## How much faster

Measured on this project's own folder (1,567 files that a search looks at), each search run 5 times, middle value kept:

| search | Python pfind + ripgrep | pfind.exe | faster by |
|---|---:|---:|---:|
| a word, with 2 lines around each match (what the AI asks most) | 509 ms | 175 ms | 2.9 times |
| a word, Go files only | 451 ms | 160 ms | 2.8 times |
| a pattern (regular expression) | 448 ms | 202 ms | 2.2 times |
| which files hold a word | 511 ms | 179 ms | 2.9 times |
| list the files in a folder | 239 ms | 43 ms | 5.6 times |
| the AI's machine-readable result | 458 ms | 181 ms | 2.5 times |
| count the lines of code per language | 7,257 ms | 467 ms | 15.5 times |

## Honest state of play

Done and checked on this computer (Windows 11):
- pfind.exe builds without the internet, from the copies of its parts stored in this project.
- All 84 tests pass on both versions, and the two outputs match. To prove the tests can fail, 8 rules in the new code were broken on purpose one at a time (case, the match cap, binary files, the zip-bomb budget, ranking, filters); the tests caught all 8.

Not done yet: the main program does not use pfind.exe yet. Job 04 builds it into `gorilla-opencode.exe` and makes
the `find` tool use it.

Built but not tried: the Linux version. It was built on this Windows computer, but not run, because there is no
Linux here.

## What is different, on purpose

- **Searching inside .zip and .tar files** works without extra programs. Before, on Windows it needed programs that
  Windows does not have.
- **The "second brain" (`--brain`)** semantic recall needs a Python library. pfind.exe still searches your brain
  folder by words, and tells you that the meaning-based recall needs the Python pfind.
- **`--pcre2`**, an extra pattern language, is not built in. pfind.exe says so and uses its normal one.
- **Lists come out in a fixed order.** Before, they came in whatever order the search threads happened to finish.

## Worst case if something is wrong

A search could miss something that the Python pfind finds. The 84 tests cover every kind of search the AI's `find`
tool makes, and every option the tests touch. Options no test covers could still differ: the full list is in track
two. pfind.exe never connects to the internet, and it has no update check and no telemetry. A wrong answer stays a
local wrong answer, and the Python pfind is still there to compare against.

---

# Track two: technical

## Build and check

| what | command (in `tools/pfind-rs`) | result on 2026-10-27 |
|---|---|---|
| Windows build, offline | `cargo build --release --offline --locked` | `target/release/pfind.exe`, 3.3 MB |
| Linux static build from Windows | `cargo build --release --offline --locked --target x86_64-unknown-linux-musl` | ELF, 3.98 MB, linked with `rust-lld` (`.cargo/config.toml`); **not run** |
| unit tests | `cargo test --release --offline --locked` | 11 passed (difflib ratio, RRF, re.escape, smart case, float repr, fnmatch, strftime, JSON) |
| parity oracle | `python -I tools/pfind-rs/tests/run_parity.py --both` (repository root) | 84 cases, 0 assertion failures, 0 output mismatches |
| mutation test | `fieldkit mutate --plan tests/mutations-pfind-rs.yaml --cwd . -- python -I tools/pfind-rs/tests/mutate_check.py` (repository root) | 8 rules broken on purpose, 8 caught (each mutant rebuilt, then cargo test + `run_parity --both`) |
| fresh clone, offline | `git clone --branch feat/pfind-rs` of the worktree, then the Windows build above | builds: the vendored bytes survive git (cargo checks every crate file's sha256) |
| speed table | `python -I tools/pfind-rs/tests/bench.py --markdown` | below |

`.cargo/config.toml` points crates.io at `vendor/` and sets `net.offline = true`. A build from a fresh clone never
touches the network.

## Architecture

| module | what it holds | built on |
|---|---|---|
| `args.rs` | pfind.py's argparse surface: every flag, short bundles, unique-prefix long flags, `--`, nargs `+`/`?`, choices, exit 2 on errors | — |
| `walk.rs` | the `rg --files` walk pfind.py ran: .gitignore/.ignore/.rgignore, hidden, `-g`/`--iglob` overrides in ripgrep's order (globs, then iglobs case-insensitive), depth, size, one-file-system, parallel; and pfind.py's `os.walk` listing for `--only-dirs` | `ignore`, `globset` |
| `search.rs` | the `rg --json` content search: ripgrep's case precedence (-s > -i > -S), -F/-w/-x/-U/--multiline-dotall/--crlf/--null-data/--no-unicode, context, passthru, max-count, binary detection (quit for walked files, convert for explicit ones), encodings, mmap, `--pre`; a `Sink` that records what the JSON printer emitted (one record per match/context event, line, text, absolute offset, submatches) | `grep-regex`, `grep-searcher`, `grep-matcher` |
| `archive.rs`, `zipread.rs` | `-z` in-process: gz/tgz, bz2, xz/lzma, zst, .zip members (stored/deflate, zip64), .tar members; pfind.py's shared decompression budget (256 MB, `PFIND_MAX_DECOMPRESSED_MB`, loud when it bites), xz/lzma capped at the writer | `flate2` (miniz_oxide), `bzip2` (libbz2-rs-sys), `lzma-rs`, `ruzstd`, `tar` |
| `rank.rs` | name matching (exact 5.0, substring 3.0, inside-root path 1.5, fuzzy 0.5+ratio with difflib's SequenceMatcher ported including autojunk), weighted RRF (name 2.0, semantic 1.5, content 1.0, git 0.8, recency 0.5; k=60), coverage ranking (distinct terms, then volume, then path), eza sort vocabulary | `regex` |
| `meta.rs`, `timefmt.rs` | os.lstat as pfind.py saw it (Windows: mode from attributes, 0o111 for .exe/.bat/.cmd/.com, uid/gid 0, ctime = creation; inode/links via `GetFileInformationByHandle`), sizes, permissions, time styles, git status/repo summary/check-ignore by running `git` (resolved on PATH, never the current folder), LOC lexer | `windows-sys`, `libc` |
| `render.rs` | every output: ranked heading/no-heading, context blocks with `--` dividers and `--max` capping, json (`indent=1`, `ensure_ascii=False`, Python float repr and round), long table, grid (down/across), oneline, tree, folder overview, `--code`, vimgrep, counts, files-with(out)-matches, OSC 8 links, `--colors` | — |

## Parity

The oracle is pfind.py 3.3.0 with ripgrep 15 on PATH: the configuration the Go `find` tool runs today. The cases
are the pfind assertions of `internal/llm/tools/find_test.go` and `find_names_test.go`, ported to pfind's command
line with find.go's exact flags (`--no-color --count --limit 15 --max 3 --max-columns 500 --max-columns-preview`).
To those were added cases for the pfind.py 3.3.0 changelog items and the flag surface. Each case asserts on each
implementation separately, then compares the two outputs. Normalization: Python's CRLF stdout on Windows becomes
LF on the pfind.py side only, and listings whose pfind.py order came from rg's threads are compared as sorted
lines. The `.zip`/`.tar.gz` case uses pfind.py's built-in engine (`PFIND_NO_RG=1`) as the oracle, because
`rg -z` does not open zip or tar members.

Differences by design (documented in `src/main.rs`'s 4.0.0 changelog):

| area | pfind.py 3.3.0 | pfind.exe 4.0.0 |
|---|---|---|
| `-z` | rg -z: needs gzip/xz/zstd programs on PATH (absent on Windows); no .zip; a .tar.gz is searched as raw tar bytes | in-process; .zip and .tar members, read in archive order |
| `--brain` semantic seam | Chroma via Python (telemetry cut out) | lexical search of the brain root; stderr says semantic recall needs Python + chromadb (pfind.py); `--doctor` says the same |
| `--pcre2`, `--engine pcre2` | passed to rg | not built in (PCRE2 is C); default engine, stderr says so |
| `--doctor` | offers to install ripgrep | nothing to install; JSON keeps `install_ripgrep: []` |
| listing order (`--files`, unsorted views) | rg's thread completion order | root order, then path components |
| git/recency boost tie order | Python set order (changes per run) | path order |
| `--timeout` hit | the whole content result is dropped | partial result kept, stderr says PARTIAL |
| line endings | CRLF on Windows (Python text mode) | LF everywhere |
| `--replace` with `$name` (no braces) | literal text (only `$1` and `${name}` were translated) | a group reference (Rust regex syntax) |
| regex dialect for `--regex` name matching | Python `re` | Rust `regex` (the dialect rg already used for content) |
| `--json` `engine` | `"ripgrep"` or `"python"` | `"ripgrep"` (it is ripgrep's engine) |

Not covered by a parity case (implemented, not compared): `--pre`/`--pre-glob`, `--encoding`, `--null-data`,
`--crlf`, `--stop-on-nonmatch`, `--one-file-system`, `--ignore-file`, `--extended`, `--flags`, `--mounts`,
`--git-repos`, `--total-size`, `--color-scale`, `--time-style full-iso`/`+FORMAT`, `--hostname-bin`, `--stdin`,
`--treat-dirs-as-files`, `--type-add`/`--type-clear`, `--stats` (timings differ by nature).

## Speed

`python -I tools/pfind-rs/tests/bench.py --markdown`, on `C:\Users\gorilla1\Documents\Gorilla.Opencode` (the live
clone, while job 01 worked in it), Windows 11, ThinkPad. Each query is find.go's flags plus the mode flags. One
warm-up run per side is not counted, the sides alternate, and there are 5 runs each (median). The machine was not
idle: other sessions' python.exe processes held cores at 80-100%, and the CPU was at 70-76 C. Both sides ran under
the same load.

| query (on C:\Users\gorilla1\Documents\Gorilla.Opencode) | pfind.py + rg, median of 5 | pfind.exe, median of 5 | speed-up |
|---|---:|---:|---:|
| content + name search, -C 2 (find query=) | 509 ms | 175 ms | 2.9x |
| content, type filter (-t go) | 451 ms | 160 ms | 2.8x |
| regex content search | 448 ms | 202 ms | 2.2x |
| files only | 511 ms | 179 ms | 2.9x |
| listing (find with no query) | 239 ms | 43 ms | 5.6x |
| --json ranked results | 458 ms | 181 ms | 2.5x |
| --code language summary | 7257 ms | 467 ms | 15.5x |

Where the time went before: the Python start-up, two `rg` processes per search (one `--files` walk for name
matching, one `--json` search), parsing rg's JSON line by line, and for `--code` the LOC lexer in pure Python.
pfind.exe walks once per signal in-process and lexes in Rust. Both still run `git status` once per search for the
badges (one `git status -uall` on this clone took 70 ms when timed alone); the listing row runs no git.

## Downloads and pins

`cargo fetch` + `cargo vendor` ran once on 2026-10-27, from crates.io only. 52 packages are in `Cargo.lock`
(sha256 `fe9b847a4bb0e8a0fb951d3fecf80607568991b24e270c9824cae9160adedd18`), each version listed in
`third_party/PINS.md`. The crates are committed unpacked in `tools/pfind-rs/vendor/`: 41.8 MiB, 2,168 files,
about 7.4 MiB compressed (the repository pack was 43.6 MiB). Measured, then decided: plain source over a tarball.
It is greppable and diffable, the telemetry scan reads it, and `cargo build --offline --locked` works on a fresh
clone with no unpack step. `windows-sys` is 17.4 MiB of that, and the `ignore` crate needs it. `.gitattributes`
marks `vendor/**` `-text`, because cargo checks every vendored file's sha256 and LF normalization would break crates
that ship CRLF. `.gitignore` re-includes `vendor/**`, because `*.py`, `*.sh` and `*.gz` there would drop crate
files. Trimmed before vendoring: `chrono` (iana-time-zone, wasm-bindgen, js-sys, windows-core), `serde_json` and
`zip` (arbitrary, proc-macros, zopfli) were replaced by `timefmt.rs`, `json.rs` and `zipread.rs`: 92 packages down
to 52.

Who can act through this: crates.io served the sources once, and the checksums in `Cargo.lock` and each
`.cargo-checksum.json` pin them. Nothing is fetched at build or run time. The binary opens no network sockets. It
runs `git` (from PATH, never from the current folder), and `--pre`/`--hostname-bin` commands only when the user names
them.

## Licenses

`tools/pfind-rs/LICENSES.md`: AGPL-3.0-or-later for pfind; ripgrep's crates Unlicense OR MIT; everything else
permissive (MIT/Apache-2.0/BSD/Zlib/bzip2/Unicode-3.0). On eza (EUPL-1.2): EUPL-1.2 Article 5 and its Appendix
make AGPL-3.0 a compatible license, but no eza code was copied. pfind.py's own AGPL reimplementation of the eza
views was ported instead, which is what output parity with pfind.py required.

## Not verified

- The musl/Linux binary was built, not run (no Linux machine in this job).
- macOS was not built (no Apple target or SDK here).
- `find.go` was not switched to the exe and the exe is not embedded yet: job 04's work. The Go tests still exercise
  pfind.py.
- The options in "Not covered by a parity case" above.
- Very large trees (a whole home folder, millions of files) were not benchmarked. The table is on one repository of 1,567
  searchable files.
