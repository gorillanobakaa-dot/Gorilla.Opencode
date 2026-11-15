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
- The model picker shows the notice the backend sends for a model (for example that GPT-OSS is removed from Antigravity on November 18, 2026).
- ChatGPT now sends the reasoning effort you chose; before, it always sent medium.
- Tests record the request and check that the effort is in it for OpenAI, ChatGPT and Antigravity.

## Files

| File | Bytes | SHA-256 |
|---|---|---|
| (filled in by `fieldkit release build` after the build) | | |
