# Gorilla Opencode v0.1.112 — the model lists stop being a lie

Everything is on this page, in full. Nothing here says "see the docs".

---

## What happened

Somebody picked a model from the menu and got this back:

```
POST "https://api.groq.com/openai/v1/chat/completions": 400 Bad Request
{"message":"The model `deepseek-r1-distill-llama-70b` has been decommissioned
and is no longer supported."}
```

They picked another one. `qwen-qwq-32b`. Same answer, three minutes later.

The program carried a **hand-typed list** of every model each company offers,
written into the program itself. Companies retire models constantly. Nothing
ever checked. So the list quietly stopped being true — and a retired entry does
not warn you, it sits in the menu looking selectable and fails the moment you
choose it.

Checked against the companies on 21 August 2026, **sixteen entries were dead**:

| Company | Dead | Most recent shutdown |
|---|---|---|
| Groq | **all five** | `llama-3.3-70b-versatile`, 16 August — five days before this was written |
| Anthropic | **all seven** | 15 June |
| xAI | **all four** | the whole grok-3 line |

Those same dead names were also the **defaults**, so a new user pasting a Groq
key had every agent pointed at a model Groq had already switched off.

---

## The program now asks

The moment you paste a key it asks that company what it actually serves,
remembers the answer, and asks again on `/update` — where it **names** whatever
has disappeared. Six companies work this way: Anthropic, OpenAI, Groq, Cerebras,
xAI, DeepSeek.

[![The /update report printed in full in the transcript: OpenRouter 289 usable, Antigravity 20 usable, xAI failed because the key was refused with HTTP 403, Groq 8 usable having fetched them live, Cerebras 2 usable, two configured endpoints re-asked — while the footer below shows the same notice truncated mid-sentence](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.112/docs/screenshots/gallery/v0112-update-live-catalogues.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.112/docs/screenshots/gallery/v0112-update-live-catalogues.png)

Groq fetching **8 real models** where the binary used to carry five dead ones.
The xAI line is the error path working: a key that no longer authenticates is
reported as refused, not as a broken provider.

That screenshot also shows a second fix. The whole notice is printed **in the
transcript**, where it wraps and stays; the blue footer bar below it shows the
same message cut off at `xAI failed: xAI refused the key (HT...`. Long notices
used to exist only as that truncation.

No default anywhere names a model any more — they all ask the list, so a default
can only ever be something the company really serves.

---

## Four companies removed

Microsoft Azure, GitHub Copilot, Amazon Bedrock and Google Cloud are gone from
the menu, along with the code that spoke to them. Every one needs a company
account, a paid subscription or a cloud project with billing switched on. This
program is built for people on old laptops, metered connections and no credit
card. Thirty entries nobody in that position could reach were taking up a menu
whose whole problem is being too long to read.

[![The launch provider menu showing fourteen rows — Antigravity, ChatGPT sign-in, Google Code Assist, NVIDIA NIM, Ollama, Cloudflare, Anthropic, OpenAI, Gemini, Groq, Cerebras, OpenRouter, xAI and DeepSeek — with no Microsoft Azure, no GitHub Copilot, no Amazon Bedrock and no Google Cloud project row](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.112/docs/screenshots/gallery/v0112-provider-portal-cull.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.112/docs/screenshots/gallery/v0112-provider-portal-cull.png)

Nothing was thrown away — the files are in the quarantine folder, as the house
rule requires. DeepSeek gained a row while we were there: it had a provider, a
client and a model list but no way to reach it without editing a config file by
hand.

---

## The list you actually see: 369 → 45

Menu entries built into the download went from **369 to 45**.

Most of that is OpenRouter, which shipped **279 models of which 261 charge
money** — 94% of the largest block in the menu unreachable without a card. The
download now carries the 18 free ones. Anyone with an OpenRouter key runs
`/update` and gets the full live catalogue from the network, so nothing is lost;
it just stops being in everybody's download.

[![After a purge, /update refetches and reports OpenRouter 289 usable with plus-289 added, proving the models were really cleared and really came back from the network rather than from the binary](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.112/docs/screenshots/gallery/v0112-purge-then-update-refetch.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.112/docs/screenshots/gallery/v0112-purge-then-update-refetch.png)

---

## One row per model

The same model is often reachable through several companies. GPT-OSS 120B is
served by NVIDIA, Groq, Cerebras and OpenRouter — four rows that look identical
and are not, because they spend different allowances.

Searching now shows **one row**, on the route with the most usable free
allowance, naming the others.

[![Searching for gpt-oss returns four rows instead of nine: NVIDIA NIM's GPT-OSS 20B marked also on groq and openrouter, NIM's GPT-OSS 120B marked also on cerebras, groq and openrouter, plus Antigravity's separate Medium variant and OpenRouter's safeguard model](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.112/docs/screenshots/gallery/v0112-search-duplicates-collapsed.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.112/docs/screenshots/gallery/v0112-search-duplicates-collapsed.png)

The order comes from a measurement, not a preference:

- **OpenRouter free tier: 20 requests a minute, 50 a DAY** — and 1000 a day only
  after buying credits, which means a card.
  ([their docs](https://openrouter.ai/docs/api-reference/limits))
- **NVIDIA NIM: no monthly credit cap**, a soft ~40 requests a **minute**.

A per-minute limit slows you down. A per-day limit sends you away until
tomorrow. That difference is the entire ranking. It is deliberately coarse: this
project already settled that free-tier limits are undocumented and vary by hour,
so a table of exact numbers would be wrong within weeks and wrong silently.

At a wider search you can see it holding across providers — the ChatGPT sign-in
winning over OpenRouter for GPT-5.5, and your own endpoint named rather than a
bare "local":

[![A search for gpt across every provider at once, 49 of 386 models matching, with the NVIDIA NIM rows naming their alternatives and the ChatGPT sign-in rows marked also on openrouter](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.112/docs/screenshots/gallery/v0112-search-gpt-all-providers.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.112/docs/screenshots/gallery/v0112-search-gpt-all-providers.png)

---

## `/purge` stops overstating

`/purge` clears downloaded model lists. It used to report "purged 284 models"
when 279 of those shipped inside the program and came back the moment you
restarted. The number was true for one session and nothing said so.

[![The /purge report reading cleared 421 models, 23 left, then stating that 23 of them ship with the app and come back when you restart and that 102 of them came from configured endpoints and re-register on the next launch](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.112/docs/screenshots/gallery/v0112-purge-honest-count.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.112/docs/screenshots/gallery/v0112-purge-honest-count.png)

That screenshot is a retake. **The first one said 38**, and it was wrong — the
count asked a variable that a refresh writes into, so models fetched from the
network were counted as shipping in the binary. The fix for an overstated count
had an overstated count. It was caught because this release was installed and
photographed before it was published; every automated test had passed against
it. Fixed, with a regression test that fails if the old lookup returns.

`/purge` had also been leaving about a hundred NVIDIA models untouched. Also
fixed — that is the 102 on the line above.

---

## Is the download smaller?

A little. **50.72 MB → 49.35 MB**, about 1.4 MB, roughly three minutes off every
download at 8 KB/s.

Almost none of that is the model lists — those are tiny. It is the four removed
companies taking their supporting code with them, which also dropped eight
dependencies.

---

## Everything measured

| | Before | After |
|---|---|---|
| Stripped binary | 53,186,852 B (50.72 MB) | 51,762,204 B (49.35 MB) |
| Models compiled in | 369 | 45 |
| OpenRouter compiled in | 279 (18 free, 261 paid) | 18, all free |
| Providers fetching their own list | 0 | 6 |
| Menu rows | 15 | 14, all reachable without a card |
| Go dependencies | — | 8 fewer |

## What was NOT verified

- No provider was listed **with a paid key**. Groq, Cerebras, OpenRouter and
  Antigravity were exercised live on this machine; Anthropic, OpenAI and
  DeepSeek were exercised only against local stubs and against the real
  endpoints without credentials, which correctly answer 401.
- The OpenAI non-chat filter list is reasoned, not measured against a live
  keyed listing.
- Antigravity's *GPT-OSS 120B (Medium)* deliberately stays a separate row: its
  api id differs, and the collapse refuses to merge anything it is not certain
  about. A wrong merge hides a model, which is worse than a duplicate row.

## Install

```sh
sudo apt install ./gorilla-opencode_0.1.112_amd64.deb
```

`apt`, not `dpkg -i` — the package depends on `lynx`, which is what makes web
search work with no setup, and `dpkg` resolves nothing.

Full detail, both tracks, in `Changelogs/v0.1.112-release-notes.md` inside the
package and the repository.
