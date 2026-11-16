# Gorilla OpenCode 0.1.164: every quota meter, and peer messages drawn apart

**`/usage` now shows one meter for every limit the backend reports, and a missing reading says unknown instead of 0%.**

## What is this program?

Gorilla OpenCode is a program you talk to by typing. You write what you want in
ordinary words, and an AI does the work: it opens files on your computer, runs
commands, reads what they print, and tells you what it found.

It is free and has no account of its own. You choose which AI it uses: one on
your own computer, or one from a company you have a key or sign-in for.

## Should you download this version?

**Yes, if you use it every day.** The usage meters are complete and messages from other sessions are easier to read.

**Know before you update:** the Windows program is still not signed.

## Why this matters to you

You see how much of each limit is left, and you are never told 0% when the real answer is unknown.

---
# In plain language: everything in this release

This is the complete explanation, not a summary of one. Nothing below is behind a link.

<!-- plain-language track: in full, on this page -->

### What this is

Quota meters that show every limit, and a clearer display of messages from other sessions.

### Should you download it?

Yes, if you use Gorilla OpenCode. Nothing you have set up is removed.

### Why this matters

- `/usage` shows one quota meter for each limit the backend reports, including the Ultra-only one.
- A reading that is missing is shown as unknown, not as 0%; a model with no meter says why.
- Messages from other sessions (peers) are drawn as their own block with a header, apart from what you typed, and are never passed to the AI as your words.
- Every theme and the status bar now reach WCAG AA contrast, with tests.
- Peers work when the state folder path is long (shorter socket path).
- A debug line shows the reasoning effort that was sent.

### What was NOT checked

- The meters were tested against recorded readings, not a live account.
- The Windows program is unsigned.

---

# For developers: how it works, and how to check it

Written for someone who will audit, fork or change the code. It covers the same release as the plain-language part above; neither is a summary of the other.

<!-- developer track: in full, on this page -->

### Summary

- usage: one quota meter per backend bucket (Ultra-only included); missing reading is unknown, not 0%; unmapped models say why (PR #87)
- peers: short socket path for a long state folder; debug line with the effort sent (PR #86)
- tui: peer messages are their own entry, drawn apart; display-only copy of Zero's traffic, never queued for the AI; every theme pair and the status bar reach WCAG AA (PR #88)
- range `v0.1.163..origin/main` (PRs #86-#88)

### Security posture

No new network channel and no telemetry.

### Deployment

```
gorilla-opencode --version
# Expected: v0.1.164
```

### Not verified

- No live-account run of the meters; CI ran the tests on Linux and Windows.

### Known issues

- The Windows exe is unsigned.
