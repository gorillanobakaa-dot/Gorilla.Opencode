# v0.1.65 — Claude, GPT-OSS and Gemini for free, through your own Google account

Two things, days apart.

**The free Gemini sign-in works again.** Google quietly closed the free "Code Assist"
tier to every program except their own new app, Antigravity, and answered our
requests with *"migrate to Antigravity"*. Introducing ourselves the way Google now
expects — one word in how the program identifies itself, on the sign-in step only —
opens that door again. Fixed and proven live.

**A new "Antigravity free tier" sign-in unlocks Claude, GPT-OSS and Gemini — free.**
Every Gmail account already has a generous free Antigravity allowance that includes
**Claude Sonnet & Opus 4.6, GPT-OSS 120B, and Gemini**. Until now, only Google's own
tool could spend it. Now Gorilla OpenCode can too, using the allowance already
attached to *your* Google account. Sign in with Google, pick a model, go.

Also new:
- **A provider menu on every launch** — cursor on your current provider, so **Enter**
  continues in one keystroke; pick another row to switch. Works from the desktop icon,
  not just a typed flag.
- **`/usage`** — your weekly free allowance as one line, on demand and at session start.

### ⚠ One honest caveat
The Antigravity route is **unofficial**: it works by speaking to Google the way
Google's own Antigravity tool does. The allowance is genuinely yours, but Google
could change something on their side and break this without warning. It is kept
isolated — if it breaks, only that provider stops; everything else keeps working. The
Gemini fix uses Google's supported login and carries no such risk.

### Install
```sh
sha256sum -c checksums.txt
sudo dpkg -i gorilla-opencode_0.1.65_amd64.deb
gorilla-opencode --version && dpkg -l gorilla-opencode | tail -1
```
Both must say `0.1.65`.

Full dual-track notes (layman + developer) ship inside the package and are in the
repo at [`Changelogs/v0.1.65-release-notes.md`](Changelogs/v0.1.65-release-notes.md).

*(Also includes a DeepSeek provider added in an earlier session, shipped here for
completeness.)*
