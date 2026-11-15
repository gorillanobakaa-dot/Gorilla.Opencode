# v0.1.163, in plain language

## What this is

A set of everyday fixes and a clearer model picker.

## Should you download it?

Yes, if you use Gorilla OpenCode. Nothing you have set up is removed.

## Why this matters

- Pressing Esc during a run now always shows "Continue or change?" on screen.
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

## What was NOT checked

- ChatGPT and OpenAI effort is proven by a recorded request in the tests, not by a live account.
- The notice shows only what the backend sends.
- The Windows program is unsigned.
