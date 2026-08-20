# Gorilla OpenCode v0.1.109 — a message that told people their connection was broken when it was fine

Everything about this release is printed in full on this page, including the
pictures. This one is a bug fix for something v0.1.108 introduced.

---

## What went wrong

v0.1.108 made answers arrive **all at once** on the two slowest connection
profiles, because sending a reply word by word costs **27 times more data**.
That part works, the saving is real, and it is unchanged here.

What it broke was a warning.

The program watches for a worrying silence: if nothing has come back after
twelve seconds it says so, because on a shared or free AI service that usually
means the service is cold. That made sense while answers always arrived word by
word — the first word turning up was itself proof something was alive.

With the answer now arriving in one piece, **nothing turns up until it is
finished**. So the program saw silence, concluded the connection was in trouble,
and said so on **every single message** — on a fast connection, with a healthy
service. It was seen firing twice inside a single answer.

The wording was the damaging part:

> *"a quiet endpoint is usually warming up, not stuck"*

That is a diagnosis, and it was the wrong one. It sends somebody hunting a
broken connection that was never broken. This project cares more about being
misleading than about being slow, so that is the bug.

---

## What it says now

There are two messages, chosen by how the answer is being delivered.

**On the fast profiles** (Modest, Broadband, Unconstrained) the original message
stays, because silence there really is a symptom.

**On the slow profiles** (Austere, Constrained) it now says the quiet is
expected, the whole answer is coming in one piece, that this uses about 27 times
less data for the same reply, that nothing is stalled, and that `/connection`
switches back if you would prefer to watch it type.

The bottom line of the screen shows the short form — `Generating... (37s) (one
piece)` — and the full sentence goes into the conversation. That split exists
because the bottom line is exactly one row and a long sentence would wrap into a
second row and corrupt the display.

[![The connection picker with Austere environment selected, showing the 1-9 KB/s band and the explanation that the whole answer arrives at once instead of word by word, which is the setting that triggered the incorrect warning](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.109/docs/screenshots/gallery/v0108-connection-picker-austere-explains-non-streaming.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.109/docs/screenshots/gallery/v0108-connection-picker-austere-explains-non-streaming.png)

The setting responsible, unchanged. What changed is what the program says while
it is waiting.

---

## You can now change tools while the AI is working

A model caught in a loop of tool calls used to leave you unable to change
anything until it finished.

**`/context` now works while the AI is working.** You can switch off the tools it
is looping on; the change is recorded straight away and applied when the current
answer ends. This is safe because the program already knew how to defer that
kind of change until a request finishes.

**The busy message now mentions that escape cancels.** It always did. Nothing
had ever said so.

---

## Not changed, deliberately

**You still cannot switch model mid-answer.** Opening the model list is
harmless, but choosing from it replaces the connection the running request is
using. Press escape first, then switch — which is the safer order anyway, since
you would not want half an answer attributed to the new model.

`/connection` and `/providers` also stay unavailable mid-answer: they hand the
whole terminal over to a full-screen picker, and doing that while an answer is
being written to the same terminal corrupts the display.

---

## What is NOT verified

- **The corrected message has not been seen on screen in a real slow-link
  session.** It is asserted by tests and by reading the code, not by observation.
- **The twelve-second threshold was not re-examined** for the new mode. A whole
  answer legitimately takes longer than a first word, so it may still be eager.
- **One unrelated test fails when tests run in random order** (the language
  server sidebar). It fails the same way on the previous release and is not
  caused by this work.

---

## How to check this yourself

1. Run `gorilla-opencode --version`. It should print **v0.1.109**.
2. Type `/connection` and choose **Austere environment**.
3. Ask something that takes more than twelve seconds. The message should say the
   quiet is expected — **not** that your endpoint is warming up.
4. Look at the elapsed line. It should read `Generating... (37s) (one piece)`.
5. While it is still working, type `/context`. The tool list should open instead
   of refusing.

---

## Install

**Debian / Ubuntu**

```sh
sudo dpkg -i gorilla-opencode_0.1.109_amd64.deb
```

**Arch / CachyOS**

```sh
sudo pacman -U gorilla-opencode-0.1.109-1-x86_64.pkg.tar.zst
```

Verify what you downloaded against `checksums.txt` before installing.
