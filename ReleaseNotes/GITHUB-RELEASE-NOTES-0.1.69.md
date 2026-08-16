# v0.1.69 — the provider menu finds your setup whatever you named it

The startup menu asked for an NVIDIA NIM API key **that was already saved**. The
NVIDIA row showed as not set up while Groq, Cerebras and xAI showed `(ready)`.
Retyping the key would have looked like a fix — and then it would have asked
again on the next launch, forever.

### Fixed

- **Your connection is recognised whatever you named it.** The app searched for
  an entry called exactly `NVIDIA NIM`. Name yours anything else — say
  `Gorilla.FREE.NVIDIA.NIM` — and the menu found nothing and decided you had
  never set it up. A connection is identified by **where it points**, not by what
  you called it. Both the "is this configured?" check and the save path now match
  by address; the built-in name is only a default for when you have no entry at
  all.

- **Re-picking a provider no longer creates a duplicate.** Choosing NVIDIA also
  wrote the app's fixed name back, so it added a **second** entry beside yours
  aimed at the same address. Two entries on one address fight over which serves
  the models and the loser's list is emptied — which is how a config ended up
  with two NVIDIA connections and zero usable models between them. Selecting a
  provider now updates your existing entry and carries your saved key across, so
  pressing Enter on the row can never wipe a working credential.

Four tests, verified to fail against the original code — including one that
reproduces the duplicate on demand.

### Note

Your key was never lost, and NVIDIA models kept working throughout: model
registration already resolved connections by address and was name-agnostic. Only
the menu disagreed. **If this was already broken for you it repairs itself on the
next launch** — nothing to re-enter.

### Install
```sh
sha256sum -c checksums.txt
sudo dpkg -i gorilla-opencode_0.1.69_amd64.deb
gorilla-opencode --version && dpkg -l gorilla-opencode | tail -1
```
Both must say `0.1.69`. Quit any running copy first.

Full dual-track notes ship inside the package and are in the repo at
[`Changelogs/v0.1.69-release-notes.md`](Changelogs/v0.1.69-release-notes.md).
