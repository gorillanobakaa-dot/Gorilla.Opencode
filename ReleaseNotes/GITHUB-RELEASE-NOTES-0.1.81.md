## The model picker answers questions now — and the gorilla speaks up mid-session

Two fronts in this release.

**The picker.** The catalogue is past 270 models on OpenRouter alone; scrolling
it was a reading assignment. Now:

- **`/` searches everything at once** — type `free coding` or `advanced
  reasoning` and the list narrows to models whose name *or description*
  matches, across every provider you have connected. Esc puts you back where
  you were.
- **`tab` opens a full page per model** — the complete description (not a
  stub), exact prices including cached reads, context window, capabilities,
  and **which of your keys pays for it**: "billed to your openrouter key
  sk-or-…#8f46 (73 chars)". The fingerprint tells rotated keys apart and can
  never reveal the key.
- **Full descriptions.** OpenRouter's own API truncates every description at
  ~215 characters (354 of their 406 models). The release build fetches each
  model's public page instead: 264 of 279 models now carry their whole text.
  The 15 that OpenRouter publishes nothing more for say so plainly: *"sorry
  lads — not our fault: that is ALL OpenRouter provides as a description for
  this model."*

**The bananas.** The ladder grew to nine rungs with escalating gorilla
bulletins below 20%, and quota is re-checked after each response (throttled to
one check per 30s): cross a rung mid-session and it is announced right then,
instead of burning half a week invisibly between `/usage` calls. The weekly
reset stays silent.

Also: the picker no longer overflows narrow terminals, and the catalogue was
regenerated on 2026-08-12 — 279 tool-capable models, six new, one retired,
prices refreshed. If you had run `models refresh` before, run it once more:
the saved list's format changed (schema 7) and the old cache is deliberately
discarded.

Full dual-track notes (what was verified live, what is inference):
[Changelogs/v0.1.81-release-notes.md](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.1.81/Changelogs/v0.1.81-release-notes.md)
