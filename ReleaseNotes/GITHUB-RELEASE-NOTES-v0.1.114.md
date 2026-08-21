# Gorilla Opencode v0.1.114 — the switch reaches the model

Everything is on this page, in full. Nothing here says "see the docs".

## The bug: the menu changed the label, not the model

You pick a provider. The bar at the bottom says the new model immediately. Your
next message goes to the **old** one.

Picking a provider wrote your choice into a settings file. The bottom bar reads
that file, so it updated at once — but the part of the program that actually
sends the request was never told, and kept using whatever it was built with when
the session started.

Nothing errored. Nothing looked wrong. You could work for an hour believing you
were on a free model while every message came off a different account's
allowance.

**It was caught by behaviour, not by a test.** The owner switched to Claude,
typed "hm", and got back an unrequested attempt to fetch a website:

> *"I have a feeling this is not Claude, only Llama does that."*

He was right. The small label under the reply read **Llama 3.3 70B**, under a
bottom bar reading **Claude Sonnet 4.6 (Antigravity free)**.

## Fixed — and it says which model is answering

[![The provider portal after switching to Antigravity, printing the line Now answering as Claude Sonnet 4.6 Thinking Antigravity free, use slash model for a different one from this provider, with the same model shown in the footer](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.114/docs/screenshots/gallery/v0114-now-answering-as.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.114/docs/screenshots/gallery/v0114-now-answering-as.png)

Two paths change the model and only one of them told the agent:

| Path | Behaviour |
|---|---|
| `/model` | builds the new provider, updates the config, **swaps the live provider** |
| `/providers` | updated the config and stopped |

The portal now routes through the **same** atomic path — not a second copy of it
— and reports the result. If the switch fails it says the session is **still** on
the old model and names it, because silence there is what created the bug.

The old wording, *"Provider updated — use /models if you want a different model
from it"*, is gone too: it implied the switch was still pending while the footer
already claimed it was done.

## Verified in the database, not just on screen

| Session | Bottom bar said | `model` column of the saved reply |
|---|---|---|
| Before | Claude Sonnet 4.6 (Antigravity free) | `local.meta/llama-3.3-70b-instruct` |
| After | Claude Sonnet 4.6 (Antigravity free) | `antigravity.claude-sonnet-4-6` |

And the invented tool call is gone. Before, from the word "hm":

```
TOOL CALL: web_fetch -> {"format":"markdown","url":"https://www.debian.org/"}
finish: permission_denied
```

After, a real question and a clean answer — parts are `['text', 'finish']`,
finish reason `end_turn`, no tool calls:

[![Claude Sonnet 4.6 on the free Antigravity tier answering a question about banana counts by explaining it has no tool that can check personal inventory, with the reply labelled Claude Sonnet 4.6 Thinking Antigravity free and a cost of zero dollars](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.114/docs/screenshots/gallery/v0114-claude-answers-clean.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.114/docs/screenshots/gallery/v0114-claude-answers-clean.png)

Tools still reach the Antigravity path — "list all the files in your working
folder" produces a real `Find` call with a tree view and a correct summary:

[![The model asked to list all files in the working folder, running the Find tool with a tree view over the Debian kernel work directory and summarising the contents folder by folder, labelled Claude Sonnet 4.6 Thinking Antigravity free](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.114/docs/screenshots/gallery/v0114-tools-work-find-tree.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.114/docs/screenshots/gallery/v0114-tools-work-find-tree.png)

## Two smaller repairs

**Sign-in could wait forever.** Every call in the auth package used a connection
with **no time limit at all**, from a context with no deadline — five call sites
across Antigravity, Google Code Assist and ChatGPT. One request that never came
back would leave "Setting up your free tier..." on screen indefinitely, which is
indistinguishable from "working, please wait". They now give up after 45 seconds
and say so; the saved sign-in is kept either way and retries on first use.

**"Provider busy" now names the provider.** It used to name nobody, which let an
NVIDIA rate-limit be mistaken for a Google one — including by me, in writing, to
the owner, about an account he had used for months. It now reads:

```
Gorilla.FREE.NVIDIA.NIM busy (rate-limit/5xx) on meta/llama-3.3-70b-instruct, retrying 2/5 in 4.8s
```

For a local endpoint it uses **your** name for it. "local busy" identifies
nothing when several endpoints are configured.

That line was also the evidence: it exists only in the OpenAI-compatible client,
and Antigravity has its own client and cannot emit it — which is how the
throttling was proven to be NVIDIA rather than Google. The code path, not a
hunch.

## What was NOT verified

- The new timeout has not been seen to fire against a real provider. It is proven
  against a stub server that accepts the connection and never answers.
- The renamed "busy" line has not been photographed against a live rate-limit.
- 45 seconds is a judgement, not a measurement: generous for small JSON calls,
  short enough that nobody concludes the program has died.

## Install

```sh
sudo apt install ./gorilla-opencode_0.1.114_amd64.deb
```

`apt`, not `dpkg -i` — the package depends on `lynx`, which is what makes web
search work with no setup, and `dpkg` resolves nothing.

Full detail, both tracks, in `Changelogs/v0.1.114-release-notes.md` inside the
package and the repository.
