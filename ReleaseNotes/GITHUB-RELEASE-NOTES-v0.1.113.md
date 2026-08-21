# Gorilla Opencode v0.1.113 — the menu explains itself

Everything is on this page, in full. Nothing here says "see the docs".

Small release, entirely about the first screen anybody sees. It exists because
the owner looked at his own provider menu and said: *"if there is a logic in the
way they are displayed I can't see it."*

---

## The menu had a rule nobody could see

There **was** an order — free sign-ins first, then your own machine, then the
ones needing a key. But Google's three routes sat at positions 1, 3 and 9 with
other companies in between, so from the outside it looked like no order at all.

An order nobody can work out is not an order.

[![The provider menu in the new order: Google Antigravity, Google Code Assist and Google Gemini together at the top, then the ChatGPT sign-in, then NVIDIA NIM, then Ollama, Cloudflare, Groq and Cerebras, then OpenRouter, Anthropic, OpenAI, xAI and DeepSeek — with the Gemini row selected showing that a key is free, needs no card, and comes from aistudio.google.com](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.113/docs/screenshots/gallery/v0113-picker-order-gemini-row.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.113/docs/screenshots/gallery/v0113-picker-order-gemini-row.png)

Easiest way in first, companies kept together:

1. **Google Antigravity** — Gmail sign-in, no key, no card
2. **Google Code Assist** — Gmail sign-in, no key, no card
3. **Google Gemini** — a free API key
4. **ChatGPT sign-in** — works on the free plan, no key
5. **NVIDIA NIM** — free key, about a hundred models
6. Ollama, Cloudflare, Groq, Cerebras — everything else that costs nothing
7. OpenRouter, Anthropic, OpenAI, xAI, DeepSeek — the ones needing a card

The three Google rows now say "Google" in their names. Grouping by position only
helps somebody who already knows Antigravity is a Google product, and most people
do not.

The order is held by a test: the Google block must be contiguous and first,
ChatGPT and NVIDIA must follow it in that order, and **no paid provider may sort
above a free one**. An order that lives only in the order of a list in the source
is one careless edit from being scrambled with nothing failing — which is exactly
what had happened.

## The Gemini row now tells you how to get a key

It used to name the key and assume you already had one. That is backwards: a
person **without** a key is exactly who that row is for.

It now says free, no card needed, made at `aistudio.google.com/apikey` with any
Google account, and to leave billing switched off. Verified against the owner's
own console the same day — Billing Tier reads "Free tier", billing never set up.

The warning under it was rewritten too. It used to say free keys are "heavily
rate-limited" and push you at the sign-in rows, which reads as *do not bother*.
What actually happens is that one busy turn uses up a per-minute allowance and
the failure arrives as `HTTP 429` or a bare "unknown error" — so it looks like a
broken key and people give up on a key that is fine. The row names that now, and
says the sign-in rows spend a **separate** allowance. One dead end becomes two
pools you can alternate between.

Every row got the same treatment where it needed it. The ChatGPT row, for
example, states plainly that it costs nothing, that a free account is enough,
and that GPT-5.4 Mini stops working on 31 August 2026:

[![The ChatGPT sign-in row selected, explaining that it signs in with a ChatGPT account and uses OpenAI models through the Codex backend, that no API key or credit card is needed, that usage counts against the plan's limits so a free plan hits a cooldown rather than a bill, and that GPT-5.4 Mini is retired by OpenAI on 31 August 2026](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.113/docs/screenshots/gallery/v0113-picker-chatgpt-row.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.113/docs/screenshots/gallery/v0113-picker-chatgpt-row.png)

And the paid ones say so in one line, without apology:

[![The DeepSeek row selected at the bottom of the menu, reading simply that it is a paid API priced well below the US providers and requires a DEEPSEEK_API_KEY](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.113/docs/screenshots/gallery/v0113-picker-deepseek-row.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.113/docs/screenshots/gallery/v0113-picker-deepseek-row.png)

## `/update` says what it threw away

When the program asks a company for its model list, it drops the entries that
are not chat models — speech, image, embedding and safety models. Picking one of
those returns an error that looks like a broken key.

That filter is a list of words, so it can be too greedy, and until now that was
invisible: `OpenAI 5 usable` reads as a small catalogue, not as seventy-three
models thrown away by mistake. Both numbers now appear:

```
OpenAI 5 usable, 73 skipped
```

Nothing is printed when nothing was skipped. This exists because nobody here has
a paid key for Anthropic, OpenAI or DeepSeek, so those three cannot be tested
against the real thing. The honest answer to "I cannot check this" is to make the
failure obvious to whoever can, rather than to claim it works.

## The startup warning was wrong, unreachable and looked broken

This is in the release because a screenshot caught it, and it is worth showing
what was wrong rather than quietly fixing it:

[![The folder picker showing the stale-model warning before the fix: it claims the model list has NEVER BEEN UPDATED although the lists had been refreshed an hour earlier, it instructs the reader to quit and run a command-line subcommand, and its wrapped lines lose their indent and start at column zero, giving a ragged left edge that reads as broken output](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.113/docs/screenshots/gallery/v0113-stale-notice-before-fix.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.113/docs/screenshots/gallery/v0113-stale-notice-before-fix.png)

Three faults in one small box:

- **It read only OpenRouter's cache.** Seven other providers keep their own file
  now, so somebody who had just refreshed everything was told their list had
  never been updated. A warning that fires after you have done the thing it asks
  for teaches people to ignore warnings.
- **It told you to quit.** `gorilla-opencode models refresh` is the command
  `/update` replaced, for exactly the reason recorded when `/update` was added:
  having to quit the session meant in practice nobody ran it.
- **It lost its indent when it wrapped**, so the tail of each paragraph landed at
  column 0. Ragged left edge, reads as broken output.

All three fixed. The notice now wraps itself and indents afterwards, measured in
display columns rather than bytes — it contains a gorilla and an em dash, which
`len()` counts as 4 and 3. The layout is tested at five widths; at 40 columns the
test immediately caught the headline, 45 columns wide and never wrapped at all.

## Housekeeping

The agent instruction file, the roadmap and the internal bug notes are no longer
published here. They are working notes for whoever is at the keyboard, not
documentation, and links that pointed at them are plain text now so nothing leads
to a missing page.

## What was NOT verified

- The `skipped` count has never been seen against a paid provider — for the same
  reason it exists: no Anthropic, OpenAI or DeepSeek key on this machine.
- No photograph of the **fixed** startup warning. It only appears when a model
  list is over a week old, and this machine's lists are current, so forcing it
  would mean staging a screenshot. The layout is proven by test instead, and the
  picture above is the real fault before the fix.

## Install

```sh
sudo apt install ./gorilla-opencode_0.1.113_amd64.deb
```

`apt`, not `dpkg -i` — the package depends on `lynx`, which is what makes web
search work with no setup, and `dpkg` resolves nothing.

Full detail, both tracks, in `Changelogs/v0.1.113-release-notes.md` inside the
package and the repository.
