# pfind.exe (pfind 4.0.0, Rust)

The native port of `internal/llm/tools/pfind.py` 3.3.0: the same flags, the same output, the same `--json` schema,
built on ripgrep's own crates (`ignore`, `globset`, `grep-regex`, `grep-searcher`, `grep-matcher`). No Python and no
ripgrep install needed. AGPL-3.0-or-later; see `LICENSES.md`. Full write-up: `docs/PFIND-RS.dual-track.md`.

## Check it (both implementations, one command)

From the repository root, after building:

```
python -I tools/pfind-rs/tests/run_parity.py --both
```

This runs the ported pfind tests (from `internal/llm/tools/find_test.go`, `find_names_test.go` and the pfind.py
changelog) against pfind.py and pfind.exe. It exits 0 only if both pass and their outputs match.
`--py` / `--exe-only` run one side, `-k TEXT` picks cases, `-v` prints full diffs.

## Build (offline, from `vendor/`)

```
cd tools/pfind-rs
cargo build --release --offline --locked                                       # Windows: target/release/pfind.exe
cargo build --release --offline --locked --target x86_64-unknown-linux-musl   # static Linux binary (rust-lld)
cargo test  --release --offline --locked                                       # unit tests
```

`.cargo/config.toml` replaces crates.io with `vendor/` and sets `net.offline = true`. Each crate version is in
`Cargo.lock` and `third_party/PINS.md`. Use `fieldkit guard --` for heavy runs on the owner's laptop.

## Speed

`python -I tools/pfind-rs/tests/bench.py --markdown` (5 runs each, median; read-only on the Gorilla.Opencode clone).
Latest table: `docs/PFIND-RS.dual-track.md`.

## Layout

`src/args.rs` (flags) · `walk.rs` (file walk) · `search.rs` (content search) · `archive.rs` + `zipread.rs` (`-z`) ·
`rank.rs` (names, fuzzy, RRF, sort) · `meta.rs` + `timefmt.rs` (metadata, git, LOC) · `render.rs` (every output) ·
`json.rs` (presets.json) · `main.rs` (header, changelog, flow).
