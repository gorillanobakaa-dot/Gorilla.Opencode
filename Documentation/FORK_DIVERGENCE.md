# How far this fork has diverged from upstream OpenCode

Measured 2026-07-28, against the working tree (including uncommitted work at the
time). Regenerate with the commands at the bottom before quoting these in a
release — they drift with every commit.

## The fork point

`73ee493` — **"docs(readme): update archive note"**, 2025-09-17, by Christian Rocha.

That commit is upstream archiving the project. Every one of the 115 commits since
is this fork's; there is no moving upstream to track. Worth stating plainly in
release notes, because "fork of OpenCode" implies a parent that is still
developing, and it is not.

## Volume

| | Upstream at fork | Now | Change |
|---|---|---|---|
| Go files | 140 | 228 | +88 |
| Go lines | 42,176 | 61,029 | **+44.7%** |
| Go lines excluding generated LSP protocol | 31,645 | 50,546 | **+59.7%** |
| Test files | 4 | 52 | 13× |
| Test lines | 706 | 7,241 | 10.3× |

The generated `internal/lsp/protocol` tree is 10,483 lines of machine-written
code. Quote the figure that excludes it when talking about hand-written work, and
say which one you are using.

## Patches

- **115 commits** since the fork
- **239 files changed**: +31,243 / −2,021
- Go only: **138 files**: +19,510 / −1,249
  - **86 new files** carrying 15,355 lines
  - **51 modified** upstream files: +4,155 / −866
  - 1 deleted
- Docs, changelogs, release notes: 101 files, +11,733
- **554 `GORILLA OVERRIDE` markers across 81 files** — the count of individually
  justified deliberate divergences

## The three percentages, and what each one answers

Pick deliberately; they answer different questions and the honest number depends
on which is being asked.

- **~32% of the current Go tree is code written here** (19,510 added of 61,029).
  Excluding generated code, nearer 39%.
- **~3% of upstream's lines were disturbed** (1,249 deletions of 42,176). 88 of
  140 upstream Go files are untouched byte-for-byte. This is the number that says
  the fork is *additive*.
- **37% of upstream's files were touched at all** (52 of 140), and heavily
  concentrated: `tui.go` (+854) and `config.go` (+692) absorb ~40% of all edits to
  pre-existing code.

## The shape, in one line

New-file lines outnumber insertions into existing files **3.7 : 1**. This is new
subsystems bolted onto a mostly intact core — `/connect`, `/settings`, the loadout
registry, roots and `/add-dir`, the command registry, Gemini Code Assist OAuth,
the startup workspace picker — which is why 63% of upstream still stands
unmodified.

The largest proportional change is not a feature: **90% of all test code in the
tree was written here** (6,535 of 7,241 lines). Tests went from 1.7% of the
codebase to 11.9%; upstream shipped 4 test files, there are now 52.

## Caveats to state, not bury

- The two counting methods disagree by ~600 lines (net growth 18,853 vs. diff net
  18,261) because `wc -l` and git count trailing newlines and added/deleted files
  differently. Treat every figure as ±1–2%.
- "Lines" flatters the fork. `GORILLA OVERRIDE` blocks are deliberately verbose
  prose explaining *why*, and they count as added lines. Some of that 32% is
  explanation, not machinery.

## Regenerating

```sh
BASE=73ee493265acf15fcd8caab2bc8cd3bd375b63cb
git rev-list --count $BASE..HEAD                      # commits
git diff --shortstat $BASE                            # everything
git diff --shortstat $BASE -- '*.go'                  # Go only
git diff --numstat --diff-filter=A $BASE -- '*.go' | awk '{a+=$1} END {print a}'   # new-file lines
git diff --numstat --diff-filter=M $BASE -- '*.go' | awk '{a+=$1; d+=$2} END {print a, d}'
grep -r "GORILLA OVERRIDE" --include=*.go . | wc -l
```
