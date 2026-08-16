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

Fork-owned paths use `gorilla-opencode` and XDG roots. The footer uses the rows above the status line. The inline container preserves a child that grows during rendering. The editor probe rebuilds Bubble Tea's wrapped-line cache and restores the live viewport height. `go test ./...` passes and the verified build reports `v0.1.84+dirty`.

## Known Alternatives

The work does not preserve upstream migration inputs or create compatibility symlinks because the requested test-machine policy explicitly rejects old-name preservation. The work does not replace the terminal UI with a separate readline implementation.

## Files Changed

| File | Change | What Changed | Why |
|------|--------|--------------|-----|
| ``internal/config/store.go`` | Adds branded XDG configuration, data, cache, and state roots. |  |  |
| ``internal/config/config.go`` | Sets the application name and environment prefix and removes legacy naming paths. |  |  |
| ``internal/db/connect.go`` | Uses the branded database filename. |  |  |
| ``internal/tui/tui.go`` | Sizes the inline footer from available rows above the status line. |  |  |
| ``internal/tui/page/chat.go`` | Synchronizes the nested editor width before footer measurement. |  |  |
| ``internal/tui/components/chat/editor.go`` | Probes a full-height textarea after reapplying its value to measure wrapped rows. |  |  |
| ``internal/tui/layout/container.go`` | Preserves a child render that grows beyond a stale one-row allocation. |  |  |
| ``internal/tui/components/chat/editor_growth_test.go`` | Adds the exact long-prompt wrapping regression. |  |  |
| ``internal/tui/startup/workspace.go`` | Constrains the stale-model notice to narrow terminal widths. |  |  |

## Decisions Made

- 🤖 **Use branded XDG roots** — The fork must not collide with upstream installation state.
- 🤖 **Do not preserve old names** — The test machine does not need migration, routing, or symlink compatibility.
- 🤖 **Measure Bubble Tea output** — The Lipgloss-only estimate reports one row while the live textarea viewport has stale state.

## Tried and Abandoned

- **Half-window footer budget** — 
- **Width-only synchronization** — 
- **Full-budget container height** — 

## ⚠ Claimed But Not Verified

*Prior documents claimed these are done. No test evidence found in this diff:*

- Debian package creation
- Arch package creation
- GitHub upload

## Open Items

| Item | Priority | Blocks |
|------|----------|--------|
| Create package artifacts |  |  |
| Verify package metadata and checksums |  |  |
| Commit and push source plus rendered documentation |  |  |

## Verification

**Step :**
```bash
Run `go test ./...`.
```

**Step :**
```bash
Build with `go build -o /tmp/gorilla-opencode-sizing-20260816-v8 .` and run `--version`.
```

**Step :**
```bash
Type a prompt longer than the 172-column usable width in the inline editor.
```

**Step :**
```bash
Build each package and inspect its archive contents and checksum.
```


## Technical Debt

🟡 **LOW** — The source tree contains pre-existing unrelated dirty changes. → 
🟡 **LOW** — Several fork-owned branding literals remain duplicated in source comments or generated metadata. → 

## Claim Sources

| Claim | Basis | Evidence |
|-------|-------|----------|
| The live bug is stale Bubble Tea viewport state. | 📄 stated in input | the trace showed `textarea_height=1` while the value grew |
| The final probe fix addresses the stale state by reapplying the value at full height. | 🤖 model inference | *(none — model judgment)* |
| Package publication is not complete. | 📄 stated in input | after you did that. create the .deb for debian and the Arch packages |


---
**How to verify this document:**
`📄 stated in input` — the model's phrasing of something your source text said.
Find the matching line in the original to verify.
`🤖 model inference` — the model's own judgment or synthesis. Treat as opinion,
not measurement. Re-run on the same input and check whether specific numbers
stay consistent between runs.

*Session record. Developer track. Covers work done, not current code state.*