# Gorilla OpenCode — screenshots & proof

Real screenshots from a Debian 13 / GNOME 48 machine running the revived
OpenCode on an NVIDIA NIM key. **New here? Read the plain-English
[GUIDE.md](GUIDE.md)** — it explains every part of the screen, the
menus, and the not-obvious ← → arrow trick to switch to the Google
models.

All screenshots are full-resolution (1600×899) so the terminal text is
readable — the complete set is in [`screenshots/gallery/`](screenshots/gallery/).

## The model picker, full width (v0.1.16)

118 models, each with a capability description, sorted best-for-coding
first, with a position counter. Up/down moves through models; **left/
right switches provider**.

![Model picker](screenshots/gallery/10-09-02-16.png)

## Reaching the Google (Gemini) models — press the → arrow

Your models are grouped by provider. Press **→** in the picker to page
from NVIDIA to **"Select Gemini Model"** — the `1/4 →` at the bottom
shows you're on the Gemini page. Bottom-left shows the context down to
**6.9K** (it used to be ~15K).

![Gemini model page](screenshots/gallery/15-09-12-23.png)

## The context loadout — every token accounted for

`/context` shows exactly what's sent to the model every turn and lets you
switch any of it off. See the [gallery](screenshots/gallery/) for the
menu, and the [GUIDE](GUIDE.md#the-context-loadout-context--your-token-budget)
for how to use it.

---

*The design draws on published research — sources cited in
[system-prompts/RESEARCH-SOURCES.md](../system-prompts/RESEARCH-SOURCES.md).*
