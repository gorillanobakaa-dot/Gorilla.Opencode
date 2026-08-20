# Gorilla OpenCode v0.1.108 — your connection is now a setting

Everything about this release is printed in full on this page, including the
pictures. Nothing here sends you somewhere else to find out what changed.

---

## What changed, in one paragraph

This program is built for people whose internet is a satellite phone link or a
weak mobile signal, where data is bought by the megabyte and often cannot be
topped up with a card. All the machinery for surviving a bad connection already
existed and had been measured. **None of it could be found** — every setting was
an environment variable, so the person on a 2 KB/s Iridium link had to read the
source code to survive on it. This release bundles those settings into five
named profiles you pick from a list.

It also changes one thing you will see immediately: **on the two slowest
profiles the answer arrives all at once instead of typing itself out**, because
watching it type costs 27 times more data for the identical reply.

---

## The five profiles

| profile | your connection | how the answer arrives | waits up to | data per message |
|---|---|---|---|---|
| **Austere environment** | 1-9 KB/s | **whole answer at once** | 15 min | 0.5 MB |
| **Constrained** | 10-60 KB/s | **whole answer at once** | 8 min | 1.5 MB |
| **Modest** *(default)* | 60-250 KB/s | types live | 4 min | 4 MB |
| **Broadband** | 250 KB/s - 5 MB/s | types live | 2 min | 8 MB |
| **Unconstrained** | 5 MB/s and up | types live | 1 min | 16 MB |

Recognise your own connection:

- **Austere** — Iridium Short Burst, Inmarsat C, 2G phone data
- **Constrained** — Iridium Certus 100/200, EDGE, 1xRTT
- **Modest** — Inmarsat BGAN, Certus 700, 3G/UMTS, older satellite broadband
- **Broadband** — HSPA+, EV-DO, early 4G
- **Unconstrained** — modern 4G and 5G, Starlink, current satellite broadband

**A profile changes waiting and data only. It never changes what the AI can
do** — same tools, same abilities, same quality of answer, on every row.

---

## The picker, as it actually appears

You are asked once, on first run. After that it stays quiet unless your measured
speed disagrees with your chosen profile by two rungs or more — one rung is
inside the noise of a single measurement and must never nag you. You can open it
any time with `/connection`, or find it in `/settings` under "Network and pace".

[![The connection picker with Austere environment selected, showing the 1-9 KB/s band, Iridium Short Burst and 2G examples, and the full plain-language explanation of why the typing effect is switched off including the 22,256 against 834 byte measurement](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.108/docs/screenshots/gallery/v0108-connection-picker-austere-explains-non-streaming.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.108/docs/screenshots/gallery/v0108-connection-picker-austere-explains-non-streaming.png)

Every row explains itself in plain language. The Austere row above states the
measurement, what you lose, what you do not lose, and how to overrule it — so
the decision is yours rather than ours.

[![The connection picker with Constrained selected, showing the 10-60 KB/s band with Iridium Certus and EDGE examples, an 8 minute wait limit, 1.5 MB per message and 3 retries](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.108/docs/screenshots/gallery/v0108-connection-picker-constrained-profile.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.108/docs/screenshots/gallery/v0108-connection-picker-constrained-profile.png)

[![The connection picker with Modest selected and marked as current, the shipped default covering 60-250 KB/s with Inmarsat BGAN and UMTS examples, 4 minute wait and 4 MB per message](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.108/docs/screenshots/gallery/v0108-connection-picker-modest-current-default.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.108/docs/screenshots/gallery/v0108-connection-picker-modest-current-default.png)

Modest is the default deliberately. Its numbers sit close to the previous
built-in behaviour, so upgrading changes almost nothing for anyone who never
opens this screen. The default is not the fastest profile, because this program
must not assume a good connection.

[![The connection picker with Unconstrained selected for 5 MB/s and up, showing Starlink and 5G examples, a 60 second wait limit, and text confirming answers still arrive a word at a time on fast links](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.108/docs/screenshots/gallery/v0108-connection-picker-unconstrained-keeps-live-typing.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.108/docs/screenshots/gallery/v0108-connection-picker-unconstrained-keeps-live-typing.png)

On the fast profiles the typing effect stays on, and the screen says why the
slow ones switch it off.

---

## Why watching it type costs 27 times more data

When the AI writes a word at a time, **each single word is sent in its own
package with a full label wrapped around it** — an id, a model name, a position
counter, the same fields repeated every time. The labels are far bigger than the
words.

Measured on the same question, same model, same 60-word answer:

| | |
|---|---|
| Word by word | **22,256 bytes** |
| All at once | **834 bytes** |
| Difference | **27 times** |

**You are charged by the AI company for the same number of words either way.**
We checked both: 106 counted on both routes, identical. This changes only the
data your connection carries — and where data is bought by the megabyte, that is
money out of your pocket.

**What you lose:** the answer no longer appears gradually. The screen stays
quiet, then the whole reply arrives. It also becomes harder to tell a slow
answer from a stuck one, because the dribble of words was itself proof that
something was alive.

**What you do not lose:** anything else. Same answer, same quality, same
abilities, same word count, same bill.

If you would prefer to watch it type, choose a faster profile, or set
`GORILLA_OPENCODE_STREAM=1` to overrule the profile entirely.

---

## Nothing is downloaded to measure your connection

The picker shows an estimate of your speed. **It never downloads anything to
find out.**

That is a rule, not an optimisation. At 2 KB/s a 100 KB test file costs about 50
seconds and real money from a metered allowance, spent to tell you something you
already know. It would not even be correct: a satellite round trip is one to two
seconds, so a small test measures the delay, not the speed, and would report a
usable link as far worse than it is.

Instead the estimate times traffic that was going to happen anyway. On first run
there has been no traffic yet, so the screen says **"Nothing measured yet, so
there is no suggestion below"** and asks you to pick the line that sounds like
your connection. It does not invent a recommendation it cannot support.

---

## Also in this release

**A message that is too big now says so honestly.** Before, when a message
exceeded your data ceiling, you were told the connection kept failing — sending
you to debug a link that was working perfectly. It now says the connection is
fine and the conversation has grown too big for this profile, with the remedy
that actually works.

**Everything still works as before.** The agent, its tools and its abilities are
untouched by all of this.

[![Gorilla OpenCode v0.1.108 running a web search tool call against Llama 3.3 70B, showing the search results block with untrusted content markers and the model composing its follow-up](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.108/docs/screenshots/gallery/v0108-agent-tool-call-working-on-v0108.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.108/docs/screenshots/gallery/v0108-agent-tool-call-working-on-v0108.png)

---

## What is NOT done, stated plainly

- **The retry count on each profile is not yet used.** Each profile declares how
  many times to retry, but the program still uses its old fixed number of five.
  Nothing is broken by this — the number has no effect yet.
- **These profiles have never run on a genuinely slow connection.** They were
  built and tested on a fast one. The numbers come from measurements taken in
  August against a deliberately broken link, which is a reasonable basis but is
  not the same as a satellite phone in the field.
- **Compression findings are partial.** Whether answers are compressed was
  checked on NVIDIA NIM and a local server only. Other providers may differ.

## Should you be concerned?

Moderately, in two specific places.

The likeliest harmful outcome is **a profile too impatient for your real
connection**. Choose Unconstrained on a satellite link and the program gives up
after sixty seconds on answers that were still coming — the exact problem this
feature exists to prevent, caused by the feature. The fix is to pick a slower
profile.

Second, **turning off the typing effect removes the signal that told you
something was alive**. We think that trade is worth 27 times less data, and the
screen explains it so you can disagree, but it is a real loss and not a free win.

What should not worry you: the default profile behaves almost exactly like the
previous version, and a profile cannot change what the AI is able to do.

---

## How to check this yourself

1. Run `gorilla-opencode --version`. It should print **v0.1.108**.
2. Start the program in a folder you have not used before and continue past the
   provider screen. The connection picker should appear.
3. Move to **Austere environment** and read the text under it. It should explain
   the typing effect, the measurement, and how to overrule it.
4. Press Escape to change nothing, then type `/connection`. The same screen
   should open on demand.
5. Choose Austere and ask a question. The screen should stay quiet, then the
   whole answer should appear at once.

---

## Install

**Debian / Ubuntu**

```sh
sudo dpkg -i gorilla-opencode_0.1.108_amd64.deb
```

**Arch / CachyOS**

```sh
sudo pacman -U gorilla-opencode-0.1.108-1-x86_64.pkg.tar.zst
```

Verify what you downloaded against `checksums.txt` before installing.
