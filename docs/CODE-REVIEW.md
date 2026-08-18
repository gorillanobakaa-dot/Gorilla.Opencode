<!-- Version: 1.0.0 · updated 26-08-18-13-36 -->
# Code review, built in

*Dual-track. The first half is the complete explanation in plain language — not
a simplification. The second half is the developer record. Every number was
measured on 18 August 2026 and says which command produced it.*

---

## Part one: in plain language

### What it is

Type `/review` and the program runs about **thirty real code-analysis and
security tools** over your code, then tells you what they found — and, just as
importantly, what they could not check.

These are not AI guesses. They are the standard tools professionals use:
`cppcheck` and `clang-tidy` for C and C++, `gosec` and `golangci-lint` for Go,
`bandit` and `pylint` for Python, `clippy` for Rust, `shellcheck` for shell
scripts, `semgrep` for patterns across all of them, and `gitleaks` for
credentials accidentally committed. They find the mechanical faults: memory
errors, injection holes, leaked secrets, errors nobody checked.

The tools themselves live **inside the program**. Nothing is downloaded when you
run it.

### The part that matters most

**A review that found nothing because nothing was installed looks exactly like a
review that found nothing because your code is fine.**

That is the worst way a review tool can fail, and it is easy to fall for. So this
one is built the other way round. Every answer *starts* with which tools ran,
which are missing, and which failed — before a single finding. And if none of
the tools for your language are installed, it **refuses to run at all** rather
than hand you a comforting blank page.

If a tool is missing, the answer says the code it covers is **UNREVIEWED**, in
those words.

### What to read first

The answer flags every line that **two or more different tools complained about
independently**. Not "the AI thinks this is important" — two separate programs,
written by different people, objecting to the same line. Those are computed, and
they are where to start.

### What it cannot do

It will not find wrong logic. It will not find a broken assumption, an error
quietly swallowed, or code that is technically fine and completely wrong for
what you are building. **No static tool finds those.**

So this is half a review, and it says so every time. The AI still has to read
the code, and is instructed to tell you plainly that it did and what it found.
A review that claims to be complete having only run the tools is lying to you.

### If tools are missing

The answer names them and gives you the command that installs them. The full
toolkit is around 30 programs; you only ever need the ones for the languages you
actually use, and the program works out which those are by looking at your code
rather than installing everything.

---

## Part two: the developer record

### Shape

| | |
|---|---|
| `/review [path]` | routed through the agent, not called directly |
| `review` tool | `path`, `diff`, `deep` |
| Loadout row | `tool.review`, **475 tokens**, on by default |
| Vendored at | `internal/llm/tools/codereview/toolkit/` |
| Payload | 444 KB, `go:embed all:toolkit` |
| Binary growth | **+480 KB** measured |

### Why the command goes through the agent

`/review` does not print findings and stop. It sends the agent an instruction to
run the tool, read the trust block first, start from the corroborated findings,
**and then read the changed code itself** for what static analysis cannot see —
and say so explicitly. A command that dumped analyser output would produce
exactly the "looks complete, is half" failure the tool's own description warns
about.

### Why the toolkit is embedded rather than depended on

The audience is on single-digit KB/s. "Install this other thing first" is a wall,
not a step — the same reasoning that made `lynx` a `Depends:` rather than a
`Recommends:`, and the same reasoning that embedded `pfind`. 444 KB against a
19 MB package is about fifty seconds on an 8 KB/s line.

What is embedded is the **orchestrator**, not the analysers: the part that knows
which tool suits which language, how to normalise thirty different output
formats into one shape, how to verify a reported line actually says what the
tool claims, and how to report what did not run. The analysers themselves are
never embedded and never will be.

Unpacking is content-addressed: `Version()` is a SHA-256 over every embedded
file, and that hash names the unpack directory, so a binary upgrade unpacks
beside the old copy rather than mixing versions. A `.complete` marker is written
**last**, so an extraction killed halfway is redone rather than silently used.

### Output bounding

A real review of this repository's `codereview` package produced **739,476 bytes
of JSON**. The tool returns **7,315 bytes** — a 99% reduction — because every
tool result is re-sent on every later turn. That is the grep lesson in a
different tool: a limit must be expressed in the unit of the resource it
protects.

What is never truncated: the trust block, and the corroborated findings. Both
are small, and both are what stop a reader drawing a false conclusion. The long
tail is capped at 60, sorted most-severe-first, and the real total is always
stated along with how many were left out.

### Tests

- `TestTrustBlockComesBeforeAnyFinding` — ordering is structural, not stylistic.
- `TestNoFindingsIsNotReportedAsClean`
- `TestFindingsAreBoundedAndTruncationIsAnnounced`
- `TestMostSevereFindingsComeFirst`
- `TestCorroboratedFindingsAreNeverTruncated`
- `TestVendoredToolkitMatchesDevCopy` — per-file SHA-256 against the working copy
  in `Scripts.For.Work`, and a check that every Python module there is embedded.
  A new module that is not vendored kills the shipped toolkit at import time.
- `TestEveryModuleTheToolkitImportsIsEmbedded` — closes the import graph without
  needing the development copy at all.

Two were proven non-vacuous by reverting the behaviour they assert.

### One test this change had to fix

`TestCalibrationCoversEveryComponentWithNoLSPClients` asserted that a row's
calibrated token count **differs** from its hand-written estimate. That is a
proxy for "calibration ran", and it fails precisely when the estimate is
correct: `tool.review` measured 475, the estimate was corrected to 475, and the
test then declared the figure a guess.

It now stamps a sentinel value no real schema can produce and asserts
calibration overwrote it — measuring the thing itself rather than a side effect.
Same failure class as a limit counted in the wrong unit.

### Measurements

| What | Value | Source |
|---|---|---|
| Vendored payload | 444 KB | `du -sh` on the embedded tree |
| Binary growth | +480 KB | `stat` before and after |
| Schema cost | 475 tokens | `toolTokens()`, measured |
| First hand-written estimate | 320 tokens — **48% under** | corrected |
| Live run: analysers that ran | 17 | real run, 2026-08-18 |
| Live run: raw JSON | 739,476 bytes | same run |
| Live run: returned summary | 7,315 bytes | same run |
| Live run: wall clock | 42 s over 50 files | same run |

All **stated in input** — measured by the command named. Nothing estimated.

### A note on what the first live run found

`gitleaks-history` flagged credentials in `internal/auth/antigravity_oauth.go`
and `internal/auth/gemini_oauth.go`. These are **installed-app OAuth client
credentials, publicly embedded in the open-source Gemini CLI and Antigravity
CLI** — the source comments have always said so. Per the estate's rule on
secrets, a value that ships inside downloadable software was never confidential.
The remaining hits are deliberate fake keys in test fixtures.

Worth recording because the toolkit behaved correctly: it reported the location
and **withheld every detected value from the report**, which is the rule that
matters more than the finding.
