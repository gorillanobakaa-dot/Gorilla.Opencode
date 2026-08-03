# v0.1.66 — the free Claude/GPT tier now actually works when you use it

v0.1.65 shipped free Claude, GPT-OSS and Gemini — and then three bugs made it fall
over the moment you tried to use it for real. All three are fixed here. If v0.1.65's
Antigravity sign-in let you down, **this is the one to have.**

### Fixed

- **Pick Claude, get Claude.** Signing in to Antigravity *looked* like it worked but
  silently ran Gemini instead — the app didn't record the sign-in until a restart, so
  it decided the provider wasn't configured and fell back
  (`agent "coder" model "antigravity.claude-sonnet-4-6" is unusable ... falling back
  to "gemini-flash-latest"`). The provider is now registered the instant you sign in,
  first session included.

- **Claude and GPT-OSS can use tools again — the big one.** A plain chat worked, but
  the moment the model tried to run a command, read a file, or search your code it
  crashed:
  `invalid_request_error: ...tool_use.id: Field required` (Claude), or
  `Expected the 'id' of a(n) 'assistant' 'tool_calls' array element` (GPT-OSS). Since
  a coding assistant uses tools constantly, this made them chat-only. Root cause:
  Claude/GPT need a tool-call `id` that the Gemini format we were sending doesn't
  carry. Fixed — for the Antigravity path only, so Gemini is untouched.

- **`/usage` works when typed** (not just from the command menu) and is now in `/help`.

Each fix ships with a test that fails without it, and the Gemini paths are byte-for-byte
unchanged and tested to confirm it.

*Not a bug:* switching models mid-conversation can make the assistant misidentify
itself (say "Claude", then "actually, Gemini"). That's each model reading the shared
history — the one on Gemini corrected itself. Nothing to fix.

### Install
```sh
sha256sum -c checksums.txt
sudo dpkg -i gorilla-opencode_0.1.66_amd64.deb
gorilla-opencode --version && dpkg -l gorilla-opencode | tail -1
```
Both must say `0.1.66`. Quit any running copy first — an open session runs the old
binary until you relaunch. Your existing Antigravity sign-in carries over.

Full dual-track notes ship inside the package and are in the repo at
[`Changelogs/v0.1.66-release-notes.md`](Changelogs/v0.1.66-release-notes.md).
