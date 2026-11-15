# Gorilla OpenCode 0.1.163: Esc asks, fewer permission dialogs, model families with effort, a doctor checklist

**Pressing Esc during a run now always shows "Continue or change?" on screen.**

## What is this program?

Gorilla OpenCode is a program you talk to by typing. You write what you want in
ordinary words, and an AI does the work: it opens files on your computer, runs
commands, reads what they print, and tells you what it found.

It is free and has no account of its own. You choose which AI it uses: one on
your own computer, or one from a company you have a key or sign-in for.

## Should you download this version?

**Yes, if you use it every day.** It asks fewer questions and the model picker is clearer.

**Know before you update:** the Windows program is still not signed.

## Why this matters to you

Fewer permission dialogs and a clearer choice of model and effort mean less clicking and fewer wrong settings.

## Also in this version

- Searching the web and fetching a public web page no longer ask for permission by default; private addresses, long query text and plan mode still ask.
- Read-only `git -C DIR` and `--no-pager` commands never ask; `-c`, push and branch delete still ask.
- The model picker shows one row per model family with an effort chooser (the `[` and `]` keys); the chosen effort is the model id sent to the backend.
- `/usage` lists every live model, including Argon and Orchestrator.
- `gorilla-opencode doctor features` is a checklist for a new model or tool: stale model references, missing tool surfaces, saved settings that name removed models.
- Messages typed while the agent is busy are queued and delivered at the next tool boundary; Esc stops the running tool.
- `-p` works in the folder it was started in; the output limit is clamped to what the provider allows.
- Gemini 4 models are listed only when the backend serves them.

## Files

| File | Bytes | SHA-256 |
|---|---|---|
| (filled in by `fieldkit release build` after the build) | | |

---
# In plain language: everything in this release

This is the complete explanation, not a summary of one. Nothing below is behind a link.

<!-- plain-language track: in full, on this page -->

### What this is

A set of everyday fixes and a clearer model picker.

### Should you download it?

Yes, if you use Gorilla OpenCode. Nothing you have set up is removed.

### Why this matters

- Pressing Esc during a run now always shows "Continue or change?" on screen.
- Searching the web and fetching a public web page no longer ask for permission by default; private addresses, long query text and plan mode still ask.
- Read-only `git -C DIR` and `--no-pager` commands never ask; `-c`, push and branch delete still ask.
- The model picker shows one row per model family with an effort chooser (the `[` and `]` keys); the chosen effort is the model id sent to the backend.
- `/usage` lists every live model, including Argon and Orchestrator.
- `gorilla-opencode doctor features` is a checklist for a new model or tool: stale model references, missing tool surfaces, saved settings that name removed models.
- Messages typed while the agent is busy are queued and delivered at the next tool boundary; Esc stops the running tool.
- `-p` works in the folder it was started in; the output limit is clamped to what the provider allows.
- Gemini 4 models are listed only when the backend serves them.

### What was NOT checked

- ChatGPT and OpenAI effort is proven by a recorded request in the tests, not by a live account.
- The notice shows only what the backend sends.
- The Windows program is unsigned.

---

# For developers: how it works, and how to check it

Written for someone who will audit, fork or change the code. It covers the same release as the plain-language part above; neither is a summary of the other.

<!-- developer track: in full, on this page -->

### Summary

- picker: one row per model family with an effort chooser; usage: live model list; doctor features (YAML rules)
- permissions.web, git -C / --no-pager reads, read-only diagnosis commands, outside-root places (PRs #74-#82)
- tui: Esc 'Continue or change?'; steer queue; peers notice; status bar working folder
- antigravity: real app list body, UA and ideType; probe hides models the backend does not serve; v0.1.161 and v0.1.162 were never tagged, their changes ship here
- range `v0.1.160..ae03148` (PRs #64-#82)

### Security posture

No new network channel and no telemetry.

### Deployment

```
gorilla-opencode --version
# Expected: v0.1.163
```

### Not verified

- macOS and Windows runs of the new picker and permission rules were not done by hand; CI ran the tests on Linux and Windows.

### Known issues

- The Windows exe is unsigned.
- `doctor features` reports 14 stale doc references that were not fixed.
