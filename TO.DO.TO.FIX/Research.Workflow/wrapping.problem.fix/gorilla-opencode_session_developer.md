# Gorilla Opencode fork identity, XDG paths, and inline prompt wrapping

> Session record generated 2026-08-16

---

## Problem Being Solved

You need the fork to stop colliding with upstream OpenCode installations and the inline chat editor must preserve soft-wrapped prompt rows at the first terminal-edge boundary.

## Approach Taken

You route configuration, data, cache, and state through branded XDG helpers. You rename fork-owned runtime literals. You replace the arbitrary half-window footer budget with measured available space. You synchronize nested editor geometry. You then trace the live editor and measure Bubble Tea's rendered textarea at full buffer height after reapplying its value.

## Before

The fork inherits upstream names including `.opencode`, `opencode.db`, and upstream command paths. The footer uses a fixed half-window budget. The editor's live viewport can remain one row while its value grows, so Bubble Tea scrolls earlier text horizontally out of sight. The repository also contains unrelated pre-existing dirty changes.

## After

Fork-owned paths use `gorilla-opencode` and XDG roots. The footer replaces `budget := a.height / 2` with the rows above the status line. The editor probe rebuilds Bubble Tea's wrapped-line cache and restores the live viewport height. `go test ./...` passes and the release build reports `v0.1.85`.

## Known Alternatives

The work does not preserve upstream migration inputs or create compatibility symlinks because the requested test-machine policy explicitly rejects old-name preservation. The work does not replace the terminal UI with a separate readline implementation.

## Before and after evidence

### `internal/config/store.go`

**Before**

```text
// no shared DataBase, CacheBase, or StateBase
// callers derived paths independently
```

**After**

```text
func DataBase() string {
    if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
        return filepath.Join(xdg, gorillaConfigDir)
    }
    return filepath.Join(home, ".local", "share", gorillaConfigDir)
}
```

### `internal/config/config.go`

**Before**

```text
const (
    defaultDataDirectory = ".opencode"
    appName = "opencode"
)
viper.SetEnvPrefix(strings.ToUpper(appName))
```

**After**

```text
const appName = "gorilla-opencode"
// default data.directory resolves through config.DataBase()
viper.SetEnvPrefix("GORILLA_OPENCODE")
```

### `internal/db/connect.go`

**Before**

```text
dbPath := filepath.Join(dataDir, "opencode.db")
```

**After**

```text
dbPath := filepath.Join(dataDir, "gorilla-opencode.db")
```

### `internal/tui/tui.go`

**Before**

```text
budget := a.height / 2
```

**After**

```text
statusRows := lipgloss.Height(status)
if statusRows < 1 { statusRows = 1 }
budget := a.height - statusRows
```

### `internal/tui/components/chat/editor.go`

**Before**

```text
The live textarea kept a stale one-row viewport while the prompt value grew, so earlier text scrolled horizontally out of sight.
```

**After**

```text
A probe textarea is set to `editorBufferLines`, receives the current `Value()`, and its rendered rows determine the live editor height before the probe is discarded.
```

### `internal/tui/layout/container.go`

**Before**

```text
The container rendered content inside a fixed `Height(c.height)`, clipping a child after it wrapped.
```

**After**

```text
The container measures `contentView` first and expands when `contentHeight + VerticalChrome()` exceeds the old allocation.
```

### `internal/tui/components/chat/editor_growth_test.go`

**Before**

```text
No regression test reproduced the exact terminal-edge prompt failure.
```

**After**

```text
`TestExactLongPromptWrapsAtTerminalWidth` asserts that the reported long prompt renders at least two rows.
```

> ⚠ **Correction added 2026-08-16 after re-verification: this test is vacuous with respect
> to the core fix.** Reverting `editor.go:180` to the old `wrappedRows(...)` estimator and
> re-running the test leaves it **passing** — a textarea constructed inside a test has no
> stale one-row viewport to reuse, which is the entire mechanism being fixed. The test
> guards the width/wrap boundary; it does not guard the stale-cache defect. The only
> evidence for `measuredRows()` is the live instrumented run. A non-vacuous test must
> construct the textarea at height 1, drive the long value through the live model, and
> assert `measuredRows() >= 2`. It does not exist yet. Per this project's own standard —
> "a test that passes against the bug is worse than none" — writing it is outstanding work.

### `scripts/build-arch.sh`

**Before**

```text
`bsdtar ... | zstd ... > "$ARCH_PKG"` reported the pipeline result without validating the archive.
```

**After**

```text
The script writes an intermediate tar, compresses it with `zstd -T1`, then runs `zstd -t "$ARCH_PKG"` before success.
```

## Files Changed

| File | Change | What Changed | Why |
|------|--------|--------------|-----|
| `internal/config/store.go` | modified | Adds `ConfigBase`, `DataBase`, `CacheBase`, and `StateBase`; each uses an XDG variable or its Linux fallback and appends `gorilla-opencode`. | The fork must separate config, data, cache, and state from upstream OpenCode. |
| `internal/config/config.go` | modified | Changes `appName` from `opencode` to `gorilla-opencode`, changes the environment prefix to `GORILLA_OPENCODE`, and points default data at `DataBase()`. | Compile-time identity controls generated paths and environment variables. |
| `internal/db/connect.go` | modified | Changes the database filename from `opencode.db` to `gorilla-opencode.db`. | The database must not collide with an upstream installation. |
| `internal/tui/tui.go` | modified | Replaces `budget := a.height / 2` with `budget := a.height - lipgloss.Height(status)` and retains the status-row floor. | A half-window ceiling hides prompt rows even when the terminal has unused space. |
| `internal/tui/page/chat.go` | modified | Propagates the current footer width through the bordered editor container before `FooterView` measures the prompt. | The outer container can have the current width while its textarea child retains stale geometry. |
| `internal/tui/components/chat/editor.go` | modified | Adds `measuredRows()`: it copies the textarea, sets `editorBufferLines`, reapplies `Value()`, strips trailing blank probe rows, counts rendered rows, and restores the live height. | The old Lipgloss estimate reported one row while Bubble Tea's live value exceeded 600 bytes. |
| `internal/tui/layout/container.go` | modified | Measures `contentView` before applying the container height and expands only when the child rendered more rows than the stale allocation. | The first soft-wrapped row was clipped by a one-row parent container. |
| `internal/tui/components/chat/editor_growth_test.go` | modified | Adds `TestExactLongPromptWrapsAtTerminalWidth` with the 171-character reproduction and asserts a second row once the usable width is crossed. | The regression covers the reported terminal-edge boundary instead of only synthetic words. |
| `scripts/build-arch.sh` | modified | Writes an intermediate tar, compresses it with `zstd -T1`, and runs `zstd -t` before reporting success. | The original `bsdtar | zstd` pipeline produced a truncated archive without a reliable artifact check. |
| `internal/tui/startup/workspace.go` | modified | Routes the multiline stale-model notice through the width-constrained renderer and reads the catalogue from `CacheBase()`. | The notice previously expanded narrow startup dialogs beyond the terminal width. |

## Decisions Made

- 📄 **Use branded XDG roots** — The fork must not collide with upstream installation state.
- 📄 **Do not preserve old names** — The test machine does not need migration, routing, or symlink compatibility.
- 📄 **Measure Bubble Tea output** — The trace reports `textarea_height=1` and `measured_rows=1` while `value_bytes` grows above 600.

## Tried and Abandoned

- **Half-window footer budget** — Rejected because long prompts scroll despite unused terminal rows.
- **Width-only synchronization** — Insufficient because the parent container still clips the child at its old height.
- **Full-budget container height** — Rejected because it creates a large blank typing area.

## Upgrade consequences of the clean break

The decision to drop migration is recorded above. Its consequences were not, and they are
user-visible with no on-screen signal. On first launch of v0.1.85 a returning user gets:

| State | Old location (left in place, never read) |
|---|---|
| Sessions and message history | `<project>/.opencode/opencode.db` |
| Custom slash commands | `~/.config/opencode/commands`, `~/.opencode/commands`, `<project>/.opencode/commands` |
| Per-project config | `<project>/.opencode.json` — silently ignored, so a project-pinned cheap model reverts to the global default |
| `/context` loadout toggles | `~/.config/opencode/loadout.json` |
| `OPENCODE_*` environment overrides | Ignored; the prefix is now `GORILLA_OPENCODE_` |

Two consequences appear to be collateral rather than intended, since neither file was
edited in the commit:

- **`internal/config/init.go`** derives the project-init flag from `cfg.Data.Directory`,
  which is now one absolute path, so the flag moved from per-project to machine-global.
  The "initialize this project?" dialog will be offered once on the whole machine.
- **`custom_commands.go`** project commands now resolve under the same global data root,
  so `ProjectCommandPrefix` no longer denotes anything per-project.

Also self-inflicted: `fileutil.go` renamed `.opencode` to `.gorilla-opencode` in
`commonIgnoredDirs`, so the orphaned `.opencode/` directories this release creates are now
**walked** by ripgrep-backed search and `@`-file completion.

## Known defects shipping in v0.1.85

| Defect | Location | Effect |
|---|---|---|
| `install` requests a tarball asset that is never published | `install:15` | The documented one-line install cannot succeed |
| PKGBUILD instructions download 0.1.85, install 0.1.84 | `packaging/PKGBUILD:13,15` | Copy-paste installs the previous release |
| `OPENCODE_*` variables still documented | `README.md:406`, `system-prompts/README.md:301`, `CONTRIBUTING.md:47` | Documented names are ignored by the binary |
| "Folder inside your project" | `settings.go` layman string | False since `data.directory` became global |
| Build machine's home path in generated docs | `docs/SETTINGS.md:220` | `/home/gorilla/...` published as the default |
| `stateBase()` duplicated | `logging/logger.go` vs `config.StateBase()` | Two functions own one directory (import-cycle avoidance, undocumented) |

## Open Items

| Item | Priority | Blocks |
|------|----------|--------|
| Write a non-vacuous test for `measuredRows()` (see correction above) | **high** | honest regression cover for the headline fix |
| Decide whether the global session store and global init flag are intended | **high** | per-project semantics |
| Fix the `install` script and PKGBUILD instruction mismatch | high | the documented install paths |
| Update `OPENCODE_*` references in shipped docs | medium | nothing; correctness |
| Repeat the normal-user install test on a clean machine | medium | nothing currently; this is release confidence work |

## Verification

**Step 1:**
```bash
go test ./...
```
  - **Pass:** Every package test passes.
  - **Fail:** A failing package identifies the regression area.

**Step 2:**
```bash
go build -ldflags '-s -w -X github.com/opencode-ai/opencode/internal/version.Version=v0.1.85' -o gorilla-opencode . && ./gorilla-opencode --version
```
  - **Pass:** The executable exists and reports `v0.1.85`.
  - **Fail:** A missing executable or different version blocks packaging.

**Step 3:**
```bash
Type a prompt longer than the 172-column usable width in the inline editor.
```
  - **Pass:** The first soft-wrapped row remains visible and the editor grows without losing earlier rows.
  - **Fail:** A one-row viewport or missing earlier text indicates a regression.

**Step 4:**
```bash
dpkg-deb -I Compiled.Builds/gorilla-opencode_0.1.85_amd64.deb && zstd -t Compiled.Builds/gorilla-opencode-0.1.85-1-x86_64.pkg.tar.zst
```
  - **Pass:** The Debian metadata reports `0.1.85` and `zstd` reports a valid archive.
  - **Fail:** A version mismatch or decompression error blocks publication.


## Glossary

**XDG** — Linux directory conventions that separate configuration, data, cache, and state.

**soft wrap** — A visual line break that does not add a newline to the prompt value.

**viewport** — The portion of the textarea that the terminal displays.

## Technical Debt

🟠 **MEDIUM** — The source tree contains pre-existing unrelated dirty changes. → Keep unrelated changes out of future release commits and inspect `git status` before staging.
🟡 **LOW** — Several fork-owned branding literals remain duplicated in source comments or generated metadata. → Route future user-facing names through shared config helpers where practical.

## Claim Sources

| Claim | Basis | Evidence |
|-------|-------|----------|
| The live bug is stale Bubble Tea viewport state. | 📄 stated in input | the trace showed `textarea_height=1` while the value grew |
| The final probe fix addresses the stale state by reapplying the value at full height. | 🤖 model inference | *(none — model judgment)* |
| Package publication is complete for `v0.1.85`. | 📄 stated in input | release assets and checksums were verified and uploaded |


---
**How to verify this document:**
`📄 stated in input` — the model's phrasing of something your source text said.
Find the matching line in the original to verify.
`🤖 model inference` — the model's own judgment or synthesis. Treat as opinion,
not measurement. Re-run on the same input and check whether specific numbers
stay consistent between runs.

*Session record. Developer track. Covers work done, not current code state.*