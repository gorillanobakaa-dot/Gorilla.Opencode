# Gorilla OpenCode v0.1.102 — it says when it has finished

**Everything you need is on this page**, printed in full.

## Download

| File | For |
|---|---|
| `gorilla-opencode_0.1.102_amd64.deb` | Debian, Ubuntu, Mint — `sudo apt install ./gorilla-opencode_0.1.102_amd64.deb` |
| `gorilla-opencode-0.1.102-1-x86_64.pkg.tar.zst` | Arch, CachyOS, Manjaro — `sudo pacman -U ...` |
| `gorilla-opencode-linux-x86_64.tar.gz` | Any Linux, no installer |
| `SHA256SUMS-v0.1.102.txt` | `sha256sum -c` |

Use `apt`, not `dpkg -i`. Restart the program if it is already running.

---

## Plain-language track

### "It looked like it was still calculating"

Press `p` in `/arsenal` and it asks your package manager what your selection
really costs. That takes a second or two, so it says `measuring with apt...`.

It never stopped saying it.

The answer arrived and went into the header — **97.9 MB to download, 331.0 MB
on disk, about 3.4 hours** — while the line underneath still said it was
working. So the screen was showing you the result and telling you it did not
have it yet, at the same time. Anyone reading that concludes the program has
hung, and they would be right to.

The rule this breaks is one the rest of the program follows: **a thing that has
finished must not still look like a thing that is running.** The message is now
replaced by its outcome:

```
measured: 97.9 MB down / 331.0 MB disk / ~3.4 hours -- press i
```

If the measurement fails it says that too, instead of sitting on "measuring"
for ever. And it is kept short enough to survive a narrow window, because the
figure is the entire point of the line and half a number is no number.

### The page was being cut off, and it was not the page's fault

The same screenshot showed the capability list stopping partway down with no
bottom edge, blank space below it.

The page was fine. What was wrong was the surface it was painted on.

When a pop-up is placed on screen, the program refuses to let it be taller than
the thing underneath it — sensible, since a dialog hanging off the bottom of
its own background is exactly the kind of mess this project keeps fixing. But
in the mode where the conversation is ordinary scrolling terminal text, the
thing underneath is only the conversation and the status bar. Short. So a
**full-screen** page painted onto that short background had its bottom rows and
its bottom border quietly trimmed off.

Measured: a 40-row page on a 2-row background came back as **2 rows**.

That is the exact reverse of the problem the trimming was written to prevent,
and it only happens in the scrolling mode — which is the mode this project
recommends for older machines, so it is the mode most people are in.

The background is now filled out to the full height of your terminal before
anything is placed on top of it. That fixes it for every pop-up, not just this
one.

### Worth saying plainly

Two separate things were true at once: the page was correct, and the test
proving the page is exactly the height of your terminal was correct. The
picture was still wrong. Neither of them was looking at the surface underneath.

Neither bug was visible to the test suite. Both came from someone opening the
program and taking a photograph of it.

---

## Developer track

### The stuck notice

`price()` set `m.notice = "measuring with ..."` and the `arsenalPricedMsg`
handler stored the cost without touching it; `notice` is otherwise only cleared
on the next `tea.KeyMsg`. So it persisted until the user pressed something.

The handler now branches on the result: unmeasurable reports the note,
zero-cost says everything is already present, otherwise the figures. Two tests,
one per outcome; both fail against the previous commit.

### The clipped overlay

`layout.PlaceOverlay` clamps `fgLines` to `bgHeight`. Correct, and load-bearing
— the comment above it records a previous bug where an over-tall overlay was
emitted at full size.

But `appModel.View()` in scrollback mode composes `appView` from the chat page
plus the status bar, which is short, and then places overlays on it. A page
rendering exactly `a.height` rows therefore lost `a.height - lipgloss.Height(appView)`
rows off its bottom, including its border.

Fix is one place, before the first overlay branch:

```go
if a.height > 0 {
    if h := lipgloss.Height(appView); h < a.height {
        appView += strings.Repeat("\n", a.height-h)
    }
}
```

`TestAFullScreenOverlaySurvivesAShortBackground` demonstrates the clip (40 rows
-> 2) and then the fix, asserting the last row of the page survives.

This applies to every dialog. `/osint`'s explainer page has the same shape and
was presumably losing its tail in scrollback mode too.

### Note on the test that "passed"

`TestThePageFillsTheScreenExactly` was correct and stayed green throughout: the
page really is exactly the terminal height. The defect was downstream of it, in
composition. A component-level assertion cannot see what happens to its output
after it returns — worth remembering before trusting one.

### Claim Sources

| Claim | Basis | Evidence |
|---|---|---|
| The notice never cleared | 📄 stated in input | User screenshot; reproduced in a test that fails on the previous commit. |
| Header showed 97.9 MB / 331.0 MB / 3.4 hours | 📄 stated in input | Visible in the screenshot; matches the live apt measurement. |
| A 40-row overlay clipped to 2 rows | 📄 stated in input | `TestAFullScreenOverlaySurvivesAShortBackground`, logged. |
| The fix applies to every dialog | 🤖 model inference | Same composition path; only `/arsenal` was observed failing. |
| `/osint` was also losing its tail | 🤖 model inference | Same shape, not tested. |
