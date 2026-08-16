# Gorilla Opencode naming and prompt wrapping work — Plain Language

> Session record generated 2026-08-16

---

## What happened

You remove the upstream OpenCode identity from the fork's generated files and place runtime state under branded XDG directories. You also fix the chat prompt so earlier text remains visible when a line reaches the terminal edge.

## Honest state of play

The source changes and test suite are complete. You have a verified dirty build. Package creation and GitHub publication remain to be completed.

## Worst case if something is wrong

If packaging or publication fails, the source tree remains available locally and the package artifacts must not be treated as published until their files and checksums verify.

## What changed for you

****
- Before: The fork creates upstream-named files such as `.opencode` and `opencode.db`.
- After:  The build uses `gorilla-opencode` names and XDG configuration, data, cache, and state roots.

****
- Before: The prompt can remain one row while Bubble Tea scrolls earlier text out of view.
- After:  The editor measures the real wrapped textarea output and preserves newly wrapped rows.

****
- Before: The repository contains an upstream project URL.
- After:  The visible project URL points to `https://github.com/gorillanobakaa-dot/Gorilla.Opencode`.

## What you can do now

- Build and run the fork without creating the upstream `.opencode` project directory.
- Use branded `gorilla-opencode` runtime paths below the XDG directories.
- Type a long prompt and keep earlier wrapped rows visible when the line reaches the terminal edge.
- Run `go test ./...` against the current source tree.

## What is still missing

- **Debian package** — You cannot install a verified Debian artifact until packaging runs.
- **Arch package** — You cannot install a verified Arch artifact until packaging runs.
- **GitHub upload** — You cannot download the new source or documents from GitHub until publication runs.

## How to check the completed work

**Step 1:**
```bash
`go test ./...`
```
  - **Pass:** Every package test passes.

**Step 2:**
```bash
`go build -o /tmp/gorilla-opencode-sizing-20260816-v8 .` and `/tmp/gorilla-opencode-sizing-20260816-v8 --version`
```
  - **Pass:** The executable exists and reports `v0.1.84+dirty`.

**Step 3:**
```bash
`git diff --check`
```
  - **Pass:** The command emits no errors.


## Should you be concerned?

The prompt issue affected visibility, not the stored prompt value. The main remaining release risk is procedural: package and upload artifacts require independent verification before you publish them.

## Glossary

**XDG** — The Linux convention that separates configuration, data, cache, and state directories.

**soft wrap** — A visual line break inserted at the terminal edge without adding a newline to the prompt.

**viewport** — The portion of the textarea that Bubble Tea renders on screen.

## Claim Sources

| Claim | Basis | Evidence |
|-------|-------|----------|
| The prompt bug comes from stale viewport sizing. | 📄 stated in input | the whole line has vanished and now I do not see it anymore |
| The final editor fix rebuilds wrapped rows from the textarea renderer. | 🤖 model inference | *(none — model judgment)* |
| Packaging and publication remain open work. | 📄 stated in input | after you did that. create the .deb for debian and the Arch packages and we will upload |


---
**How to verify this document:**
`📄 stated in input` — the model's phrasing of something your source text said.
Find the matching line in the original to verify.
`🤖 model inference` — the model's own judgment or synthesis. Treat as opinion,
not measurement. Re-run on the same input and check whether specific numbers
stay consistent between runs.

*Session record. Plain-language track. Its developer twin covers the same session in technical detail.*