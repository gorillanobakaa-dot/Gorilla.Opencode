# Gorilla Opencode v0.1.88 — one search tool, and a research command that cites its sources

Two big things and a pile of honesty fixes.

## `find` replaces `ls`, `glob` and `grep`

Three tools that each did part of a search are now one that does the whole job —
matching lines with context, in a single turn. It is backed by pfind, which
travels **inside** the binary, so a bare downloaded build searches properly on a
machine with nothing else installed.

Why it matters where you feel it — in tokens, which are money:

| | Old way | `find` |
|---|---|---|
| Answering "where is X handled?" | 16 + 1,829 tokens, two turns (and a ~10k-token restart when the search failed) | **132 tokens, one turn** |
| Riding every single message | 1,388 tokens of tool descriptions | **1,279** |

Results are capped in **bytes**, not "items", and say so when truncated — a
single grep once returned 2.4 MB and took a conversation from 15.9K tokens to
675K in one turn. That cannot happen now.

## `/osint` — the serious research command

A professional intelligence assessment instead of a chat answer: it plans your
question into sub-questions, collects from **985 catalogued sources** (866 free,
370 needing no account at all), grades every claim on two axes the way real
analysts do — source reliability A–F × information credibility 1–6 — traces
circular reporting to its ultimate origin, hunts its own gaps once, and then
states plainly what it could **not** establish.

**It ships OFF and it burns real money.** Arm it by hand in `/context`
("Serious OSINT dossier — EXPENSIVE"), and every run opens with a warning
showing the burn rate computed live for *your* model, where you choose 4–10
helpers and how they run. After that it is your wallet and your call. Type
`/osint` alone to read the full capability page first.

The finished dossier is written **outside your working folder** (under your own
home directory), because working folders are often git repositories and a
private question must never end up in a public commit.

`/research` is unchanged and still the everyday tool — same discipline, one
pass, far cheaper.

## Everything else

- **`/context` is legible now.** A real user asked: *"Which is off/on, is it
  x'ed or unx'ed and greyed out, regardless the description still shows off"* —
  and he was right, because enabled rows opened their description with the word
  "off". Rows now lead with an **ON**/**OFF** badge that visibly flips when you
  press space, `>` marks where you are, and the list is sorted alphabetically.
- **Web search stops overpromising.** The description now says whether a search
  backend is actually configured instead of claiming a capability it lacks.
- **Four new keyless sources**: GDELT world news (15-minute refresh), World Bank
  documents, humanitarian data (HDX), and SEC corporate filings full-text.
- **Tool-using turns no longer stall the transcript** while results arrive.
- **Number-shaped arguments are accepted** when a model sends `"100"` instead of
  `100`, without accepting actual nonsense.

## Documentation

Shipped inside the package and in `docs/`:
[OSINT-RESEARCH.md](docs/OSINT-RESEARCH.md) (plain language),
[OSINT-DOCTRINE.md](docs/OSINT-DOCTRINE.md) (which field manuals the method comes
from, and what was deliberately left out),
[OSINT-SOURCE-CATALOG.md](docs/OSINT-SOURCE-CATALOG.md) (every one of the 985
sources, so "hundreds of sources" is checkable rather than a slogan), and
[FIND-TOOL-METRICS.md](docs/FIND-TOOL-METRICS.md) (the measurements above, with
the method).

## Honest limits

- Download grows by **252 KiB** (51,867,940 → 52,125,988 bytes). That is a
  one-off cost buying back a recurring per-message one.
- An Arch/CachyOS tester reported leftover `Generating...` lines and footer
  smearing on v0.1.87. **Not reproduced here and not fixed in this release** —
  the leading hypothesis is a font/glyph-width mismatch, and confirming it needs
  the reporter's terminal, font and `$TERM`.
- The `/osint` cost forecast rests on three unmeasured assumptions. They are
  printed on the warning screen next to the number so you can argue with them.
- The SEC source sends a maintainer contact address as its User-Agent because
  SEC policy requires one.

Full dual-track notes: [v0.1.88-release-notes.md](Changelogs/v0.1.88-release-notes.md).

**Install (Debian/Ubuntu):** `sudo apt install ./gorilla-opencode_0.1.88_amd64.deb`
— use `apt`, not `dpkg -i`, so the `lynx` dependency resolves.
**Arch/CachyOS:** `sudo pacman -U gorilla-opencode-0.1.88-1-x86_64.pkg.tar.zst`
