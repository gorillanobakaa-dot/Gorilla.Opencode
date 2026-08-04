# v0.1.68 — when something fails, you can finally read why

An error used to read:

```
failed to process events: POST "https://integrate.api.nvidia.com/v1/chat/completions": 404 Not Found
```

True, and completely useless. It looks like the app is broken, or the network is
down, or your key expired. It was none of those — the key was fine and the
provider was simply refusing to run that one model for that account. Working that
out took an evening. It should have taken a moment.

### Fixed

- **Errors are in English now, with the machine's words kept.** The same failure
  reads `Llama 3.3 70B isn't enabled for your account (HTTP 404 — your key is
  fine). Pick another with /models.  ⟨POST "…/chat/completions": 404 Not Found ⟩`
  — explanation first, raw error appended. **Both, never one instead of the
  other**: a translation that throws away the evidence is worse than none when
  the translation turns out to be wrong. 401 and 404 give deliberately different
  advice (`/connect` vs `/models`), because sending you to regenerate a perfectly
  good key is its own kind of waste. Statuses we cannot confidently explain pass
  through untouched.

- **Failures stay in the conversation.** They used to appear only in the one-line
  status bar, which cuts off around 100 characters and is wiped by the next
  message. The full error now lands in the transcript: scrollable, selectable,
  copy-pasteable, still there tomorrow.

- **`failed to process events` is gone.** Internal jargon that pushed the useful
  sentence off the edge of the screen.

- **A tool-using turn is no longer reported as a crash.** When the model runs a
  command instead of talking — most of what a coding assistant does — it was
  labelled *"Finished without output"*, the wording reserved for turns nobody can
  explain.

- **A text-mangling bug in the status bar**, found while fixing the above: it
  shortened messages by counting *bytes* instead of *characters*, slicing any
  dash or accented letter that landed on the cut point into invalid UTF-8.
  Harmless for plain English — but the new error messages contain `—` and `⟨⟩`,
  so the fix above would have started triggering it.

Every fix ships with a test verified to fail against the original code.

### Note

Provider error text is now saved to your local session database, which is what
makes it survive in the transcript. Inspected errors carry the method, URL and
status only — API keys travel in headers, not error bodies — but this has not
been audited across every provider, so it is stated as a limit rather than a
guarantee. Nothing is sent anywhere.

This package also carries the v0.1.67 release notes, backfilled: v0.1.67 shipped
without any.

### Install
```sh
sha256sum -c checksums.txt
sudo dpkg -i gorilla-opencode_0.1.68_amd64.deb
gorilla-opencode --version && dpkg -l gorilla-opencode | tail -1
```
Both must say `0.1.68`. Quit any running copy first — an open session runs the
old binary until you relaunch.

Full dual-track notes ship inside the package and are in the repo at
[`Changelogs/v0.1.68-release-notes.md`](Changelogs/v0.1.68-release-notes.md).
