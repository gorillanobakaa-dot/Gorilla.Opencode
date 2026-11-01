# Gorilla OpenCode v0.1.160: the same tools on Windows, Linux and the old Mac

**Date:** 2026-11-01

---

## Main goal

The owner's rule is that Gorilla OpenCode has the same abilities on Windows, Linux and the old Intel Mac (macOS 10.13), and that a new version reaches both of his computers without him doing anything. This release finishes the first half for the carried tools.

## Everyday problems

1. **Linux had fewer carried tools than Windows.** Seven tools (the code searcher, the file tools, the file comparer, the speed tester, the build tool, the text replacer and the file finder) came inside the program only on Windows. They now come inside it on Linux too.
2. **The old Intel Mac had none of them.** All 13 tools are now pinned for the Mac as well, each checked against a fixed fingerprint (SHA-256), with 10.13 as the lowest macOS they may need.
3. **Checking Linux.** The 16 everyday jobs that need the carried tools were run on the Linux laptop with the ChatGPT model `gpt-5.6-luna`: 16 passed, 0 failed.

## Ideas

- **A test now stops any system from getting a tool another lacks.** It covers all three systems, and the list of exceptions is empty.
- **The project's automatic checks on a change test only the parts it touches,** so a change is checked faster; the full run still happens on the main branch and on releases.

## What you get, in numbers

- 13 of 13 carried tools on each of Windows, Linux and the Intel Mac; the exception list went from 7 to 0.
- Linux everyday jobs with the carried tools: 16 of 16 (one run per job).

## What was NOT checked

- **The Mac:** the tools are pinned and hashed, but the everyday jobs were not run on a Mac.
- **One run per job is a sample,** not a rate.
- **The Windows program is still unsigned.**
