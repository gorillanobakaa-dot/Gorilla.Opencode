# Gorilla OpenCode v0.1.115: the meter, and the list that had stopped being true

Everything about this release is printed in full on this page, including the
pictures.

Two things here were wrong in the same way, and neither of them looked wrong. A
usage meter showed a number belonging to a different account. A model list
showed models that were not the ones on offer. Both failed silently, both looked
healthy, and both were caught the same way: the owner noticed that another
program on the same machine disagreed.

---

## The meter was reading the wrong barrel

Sign in with a ChatGPT account, type `/usage`, and you were shown a weekly
allowance and a Google email address. Neither belonged to the account you were
using. They were the numbers for **Antigravity**, a different free tier
entirely, printed under a ChatGPT session because nothing checked which one you
were actually on.

That is worse than showing nothing. A meter that reads full when your tank is
nearly empty is not broken, it is misleading, and you would find out when the
program stopped answering.

Here is the state that started it. Our panel on the left with nothing to say;
OpenAI's own Codex on the right, same account, reporting a real number:

[![The Gorilla OpenCode usage panel showing no ChatGPT figures at all beside OpenAI's Codex status output reporting twenty percent of a monthly limit remaining on the same Google account](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.115/docs/screenshots/gallery/v0115-before-codex-shows-20-percent.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.115/docs/screenshots/gallery/v0115-before-codex-shows-20-percent.png)

The number plainly existed. We just were not reading it.

## Where it was hiding, and why reading it is free

There is no page to ask. OpenAI publishes no "how much have I used" endpoint for
this sign-in. The figures ride along on the replies to requests you were
**already making**, in the response headers.

That is the right answer for anyone on a metered connection: reading your usage
costs **nothing at all**. No extra request, no polling, no background check.

The catch, stated plainly because it is a real limitation: there is nothing to
show until your first reply comes back. On a fresh session the meter is blank
until you have asked something.

[![The usage panel showing a ChatGPT monthly limit bar reading eighteen percent remaining with the banana emergency label, above a secondary limit bar at one hundred percent](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.115/docs/screenshots/gallery/v0115-usage-chatgpt-meter.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.115/docs/screenshots/gallery/v0115-usage-chatgpt-meter.png)

And the same panel beside Codex, so you are not taking our arithmetic on trust.
Both read 18%. Codex prints "resets 19:01 on 15 Sep"; the value we stored is
`reset_at=1789495295`, which is 2026-09-15 19:01:35 BST, and we render it as
"resets in 23d":

[![The Gorilla OpenCode usage panel reading eighteen percent of a monthly limit next to OpenAI Codex reporting eighteen percent left resetting at 19:01 on 15 September, the two agreeing exactly](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.115/docs/screenshots/gallery/v0115-usage-chatgpt-agrees-with-codex.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.115/docs/screenshots/gallery/v0115-usage-chatgpt-agrees-with-codex.png)

## A rounding error that would have panicked you

A test caught this before release, and it is worth spelling out.

The program stores how much you have **used** and shows how much you have
**left**. That conversion is a subtraction, and the obvious way to write it is
slightly wrong. Ask a computer for `1 - 80/100` and you do not get `0.2`. You
get `0.19999999999999996`.

One part in fifty thousand billion, and it matters at exactly one place: the
colour scale changes at 20%. Somebody with exactly a fifth of their allowance
left would have been shown **"Banana emergency! Scraping the peel"** instead of
the calm amber they had earned. Every warning level (20, 15, 10 and 5 percent)
sat one step too alarming.

Written `(100 - 80) / 100` it lands on exactly `0.2`. The test now pins all four
boundaries to exact values, with `!=` rather than a tolerance, so it cannot
drift back.

## Nothing else lost its meter

The gate removes the wrong reading, not the right one. Antigravity still shows
both model groups, the account line and the OpenRouter balance:

[![The usage panel on Antigravity showing the Gemini models group at ninety one percent and the Claude and GPT models group at one hundred percent, with the account line and OpenRouter balance intact](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.115/docs/screenshots/gallery/v0115-usage-antigravity-unchanged.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.115/docs/screenshots/gallery/v0115-usage-antigravity-unchanged.png)

---

## The model list had quietly stopped being true

The ChatGPT model list was typed out by hand six days earlier, from a reading of
what OpenAI was serving that day.

By this week it was wrong. OpenAI was offering **four** models. We showed
**two**, and one of those two, GPT-5.4-Mini, is switched off by OpenAI on
**31 August 2026**, eight days after this release. That list was about to have a
single working entry on it.

Caught from the outside again: Codex was running **GPT-5.6-Luna** on the owner's
account while this program told him it was not available.

## Why the good models were missing, and why that reason was wrong

This is the part worth reading, because it is a mistake about **how to know
things**, not a mistake in the code.

The two GPT-5.6 models were not missing by accident. They were **deliberately
excluded**, with a confident comment in the source explaining why: those models
carry a `code_mode_only` flag, which supposedly meant they wanted their tools
presented as a code sandbox, so offering them would put two entries in the menu
that "sign in fine and then fail on the first tool call".

Every part of that was reasoned from **the name of the flag**. Nobody had sent a
single request to check. A guess, written in the voice of a finding, and it cost
six days of a better model being unavailable.

OpenAI publishes the source of their own client. It says the flag means
*"Restrict model-visible tools to code mode entrypoints"*: a decision **their
program** makes about what to hand the model. Not a wire format. Not a rule the
server enforces. Nothing stops another client doing it the ordinary way.

So we stopped reasoning and asked the server. One ordinary request each, one
ordinary tool:

```
gpt-5.5        HTTP 200  get_weather({"city":"Bucharest"})
gpt-5.6-luna   HTTP 200  get_weather({"city":"Bucharest"})
gpt-5.6-terra  HTTP 200  get_weather({"city":"Bucharest"})
```

Then in the shipped binary, on the free plan, off one throwaway question. Not
one tool call. **Two, chained:**

[![GPT-5.6-Terra on a free ChatGPT sign-in answering a single question by running two tool calls in sequence, a file view of SETTLED.md followed by a directory find, with the status bar showing zero dollars spent](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.115/docs/screenshots/gallery/v0115-terra-chains-two-tool-calls.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.115/docs/screenshots/gallery/v0115-terra-chains-two-tool-calls.png)

The wrong paragraph has been **left in the source with the correction printed
underneath it**, rather than quietly deleted, so anyone reading later can see
what changed and why.

## So the list is asked for now, not typed

[![The ChatGPT model picker listing four models ranked best first, with GPT-5.6-Terra and GPT-5.6-Luna above GPT-5.5 and GPT-5.4-Mini](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.115/docs/screenshots/gallery/v0115-picker-four-chatgpt-models.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.115/docs/screenshots/gallery/v0115-picker-four-chatgpt-models.png)

What the live list actually says, on a free plan:

| model | OpenAI's rank | shown? |
|---|---|---|
| GPT-5.6-Terra | 2 | yes |
| GPT-5.6-Luna | 3 | yes |
| GPT-5.5 | 7 | yes |
| GPT-5.4-Mini | 23 | yes, until OpenAI retires it |
| codex-auto-review | 43 | no, OpenAI marks it hidden and it is not a chat model |

- The **order is OpenAI's own**, not ours.
- When next week's retirement lands, the model **disappears on its own**.
- It costs **no extra request**: the program already asked that same question to
  check your sign-in worked.

[![The update command reporting ChatGPT four usable alongside OpenRouter, Antigravity, Groq and Cerebras, above a line reading model list refreshed from OpenAI four available](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.115/docs/screenshots/gallery/v0115-update-fetches-chatgpt-list.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.115/docs/screenshots/gallery/v0115-update-fetches-chatgpt-list.png)

Signing in no longer picks your models by name either. It used to hard-code
"coder on GPT-5.5, background jobs on 5.4-Mini", a decision made once that then
governed your whole session and pinned your background jobs to the model that
dies next week. It now asks the list which is best and which is cheapest.

The cheap one still writes your conversation titles, deliberately: on a free
plan what you run out of is **the cooldown, not money**, so the good model
should not be spent naming chats.

---

## Three smaller things

**Your files stop being sent twice.** When the agent reads a file and later
reads the same file again, the first copy used to be re-sent with every
following message for the rest of the session. It is now replaced with a
one-line note saying a newer identical read appears below. Nothing is lost: the
full text stays in the session record on disk. Read-only operations only,
identical arguments only, results over 400 characters only, because collapsing
something small is churn.

**"I could not find it" is now a finished job.** The model's instructions listed
several ways a task could stop and every one was about failing to *do*
something. None covered failing to *know* something. That gap is most expensive
on a kernel or browser build, where an invented cause reads exactly like a
diagnosed one. One line now says an unestablished finding is finished work.

**The literature is cited.** The research behind the context work is listed with
sources rather than paraphrased.

You can see what every turn costs you, per feature, with `/context`:

[![The context loadout screen listing tools with their per turn token costs and a header reading roughly 8,802 tokens sent on every turn at zero dollars per turn on the free tier](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.115/docs/screenshots/gallery/v0115-context-loadout-tools-and-cost.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.115/docs/screenshots/gallery/v0115-context-loadout-tools-and-cost.png)

---

## What was checked, and what was not

Verified: full test suite green; the catalogue tests checked **non-vacuously**
(restoring the old exclusion fails three of them); the meter agrees with an
independent client on the same account; the packaged binary's SHA-256 matches
the built binary and the installed copy.

**Not verified, said plainly:**

- **How much context the supersession actually saves.** The mechanism is tested.
  The saving is not measured on a real session. The claim here is a mechanism,
  not a number.
- **The 872K context ceiling** the 5.6 models advertise. Left alone: a window
  set too high fails in a way indistinguishable from a broken sign-in.
- **Any paid ChatGPT plan.** Everything was checked on a free plan. The weekly
  and 5-hour window labels are ported from OpenAI's client and unit-tested, but
  no paid account has run them.
- **The 31 August retirement date**, taken from OpenAI's announcement. The
  mechanism no longer depends on it: the model disappears when the backend stops
  listing it, whatever the date turns out to be.

---

## Install

```sh
sudo apt install ./gorilla-opencode_0.1.115_amd64.deb
```

`apt` rather than `dpkg -i`, because the package depends on `lynx` and `dpkg`
resolves nothing. Verify the download against `checksums.txt` first.
