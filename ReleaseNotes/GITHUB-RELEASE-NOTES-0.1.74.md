Three fixes, all from watching it fail in front of a user.

**Plain-language version:** One web page quietly ate most of the assistant's
memory and nobody mentioned it. The quota display vanished as soon as you looked
away, and looking again cost you quota. And when we asked the assistant how it
had done something, it made up a convincing answer instead of checking.

### A price tag on large pages

A fetched page joins the conversation, and the model **re-reads the whole
conversation on every reply** — so a large page is a subscription, not a one-off
charge. One fetch took a session to **88% of context in a single call** (84,772
tokens) with no indication of the cost.

The existing 400 KB cap protects memory, which is ~100,000 tokens: the wrong
unit for the resource that runs out. But a tighter cap was worse. Measured, a
converted arXiv abstract page is ~10,700 tokens and an entire novel ~42,400 — any
cap tight enough to guarantee cheapness cuts a paper in half, and half a paper is
worse than none.

| size | behaviour |
|---|---|
| under 15,000 tokens | silent |
| 15,000–40,000 | a notice: token count, that it re-bills every turn, and the ways out |
| over 40,000 | cut, loudly, "the rest was NOT read" |

`summarise: true` condenses locally for free and declares its compression ratio.
*Romeo and Juliet* fits under the ceiling.

### `/usage` writes into the scrollback

```
  16:42:07  quota · Gemini 16% · refreshes in 71h 9m
```

The footer answers "what is it now". The history answers "what was it twenty
minutes ago" — which the footer cannot do at any price, because re-asking spends
a request against the quota being measured. Timestamped, because a quota figure
without a time is not a measurement.

### The model no longer invents its own methodology

Asked "walk me through your procedure", GPT-OSS 20B described a date filter its
tool does not have, JSON output it does not produce, "the top 25 hits" when it
had requested 10, and link verification it never performed. Asked why it missed a
paper, it blamed an indexing lag — for a paper that is the **#1 hit for its own
title** and has been indexed eleven months. The real reason: it searched
"deception" and never tried "lie".

Every structural claim was false, and the account was **more convincing than the
answer it explained**, because it was organised and self-critical. `# honesty`
now covers self-description: *read the trace, do not narrate a plausible
procedure.*

Caught mechanically by `procedure_confabulation` in
<https://github.com/gorillanobakaa-dot/model-eval>.

### Not verified

- ~~Nobody has seen the `/usage` line in the history.~~ **Confirmed working**:
  two readings nine seconds apart both stayed in the scrollback, timestamped,
  the second not replacing the first. Automated tests still cover only the
  plumbing, format and severity — `tea.Println` cannot be driven from an agent
  shell.
- The prompt line ships on reasoning, not measurement.
- Token thresholds use bytes ÷ 4, good to ~±15%.
- The pre-registered prompt experiment remains unrun.

Full dual-track detail: [Changelogs/v0.1.74-release-notes.md](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.1.74/Changelogs/v0.1.74-release-notes.md)
