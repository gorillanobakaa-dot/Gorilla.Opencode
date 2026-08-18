<!-- Version: 1.2.0 · updated 26-08-18-18-21 -->
# What happens when your connection is bad

*Plain language. No jargon, and where a technical word is unavoidable it is
explained on the spot. This is the companion to `FOOTPRINT.md`, which is the same
story written for someone who wants the code. Neither is a simplification of the
other — they are the same facts in two languages.*

*Everything here was measured on 18 August 2026. Where a number is a measurement
it says so. Where something is a judgement, it says that too.*

---

## Why anyone bothered to test this

This program is built for people whose internet is a satellite phone link, a
weak mobile signal, or a shared connection in a building where twenty other
people are also on it. Slow, expensive, and **unreliable** — it drops.

Everything about the program had already been measured against "slow": how much
it downloads, how much memory it needs, how many bytes each message costs.

Nobody had ever tested **"broken"**.

That is a real gap, because slow and broken cost you different things. Slow costs
you patience. Broken can cost you money you did not agree to spend, and time you
never get told about.

So the question was one sentence long:

> **When the connection goes wrong, does this program fail cheaply and out loud,
> or expensively and in silence?**

Cheaply and out loud means: it stops, it tells you what happened, and it does not
spend any more of your data.

Expensively and in silence means: it keeps re-sending your conversation, or it
sits there looking like it is thinking, and either way you are never told.

## Why that is not a matter of polish

Here is the part that makes this a money question rather than a tidiness
question.

Every time you send a message, the program has to send **the whole conversation
again**. Not just your new sentence — all of it. That is how these systems work;
the model has no memory between messages, so the entire history goes up the wire
every single time.

Measured: **83,576 bytes** for one short question in a modest conversation.

Now suppose the connection drops and the program tries again. That is not a small
retry. That is the whole conversation, uploaded a second time. And a third.

On a metered connection — where you pay per megabyte, or where you have a monthly
allowance and no card to top it up — a program that retries fourteen times has
spent a megabyte of your allowance and produced **nothing**.

And if it never tells you it is doing that, you cannot even decide to stop it.

## How you test a broken connection on purpose

You cannot sit around waiting for a satellite to drop out. So you fake it.

I put a small program in the middle, between this software and the internet.
Everything had to pass through it. It relayed all the traffic faithfully — so the
program could not tell it was there — but it kept a log of every connection and
every byte.

Think of a tollbooth on the only road out of town. It lets every car through, and
it counts them.

Then I made the tollbooth misbehave, in the two ways a connection can fail.

**One: the cable gets yanked.** Let eight seconds through, then cut it hard. The
program *knows* something broke.

**Two: it goes quiet.** Let the connection open, then simply stop. Do not close
it, do not refuse, do not send an error. The connection still looks perfectly
alive. Nothing ever comes back.

Those two are not variations of one thing. They are genuinely different, and
software usually copes with the first and hangs forever on the second — because
the second one *looks fine*. There is nothing to react to. It is still waiting
for an answer that is never coming.

The second one is also what a real satellite does. When the dish loses the
satellite behind a cloud or a building, the link does not politely say goodbye.
It just stops carrying anything.

---

## What the tollbooth counted

### When the cable was yanked

The program tried again. And again. **Fourteen times.** Each attempt uploaded the
entire conversation from scratch.

- **1.01 megabytes uploaded**
- **for one question that was never answered**
- **and it never said a word about it**

It was still going when I stopped it after two minutes.

On a connection running at 8 kilobytes a second — which is a realistic bad day —
that is over two minutes of uploading, for nothing, silently, out of an allowance
somebody is paying for.

### When it went quiet

It waited. No error, no message, nothing on the screen. I gave up after ninety
seconds. It would have waited all night.

In the normal screen-and-keyboard version that is *survivable*, and it is even
deliberate: you can see the spinner, and you can press Escape. The program was
built not to give up on a slow answer, which is the right instinct on a bad link.

But you can also run this from a script, with nobody watching. There is no
Escape key in a script. So a scheduled job on a bad connection would hang
forever, silently, and you would find out the next morning.

---

## What was wrong underneath

### There were three things retrying, and none of them knew about the others

The code says, in plain sight, "try at most 5 times". So fourteen attempts should
have been impossible.

It turned out three separate pieces of software were each retrying the same
request, and each was doing its own counting:

1. **This program's own retry loop** — the one with the 5 written in it.
2. **The programming language's networking layer**, which quietly re-sends a
   request if the connection dies before any reply arrives. It thinks it is being
   helpful. It is invisible from above.
3. **The vendor's toolkit** we use to talk to the AI provider, which retries
   twice by default. Nobody had ever told it not to.

The catch is that these **multiply**. They do not add up. Five tries, each of
which is secretly three tries, is fifteen.

Each layer is individually sensible. That is exactly why nobody noticed: there is
no single place you can stand and see all three.

**The general lesson, and it is the most useful thing found all day: a limit is
only a real limit if exactly one thing is counting.** Any time you write down a
maximum, it is worth going and looking at what is underneath it.

For the curious: the way this was caught was arithmetic, not cleverness. I
predicted a test would stop at about 41 seconds. It stopped at 123. That is not
"roughly wrong", it is *exactly three times* wrong — and a clean small multiple
like that is a hidden layer announcing itself. Doing the division found in one
minute what reading code had not found in an hour.

### There was no time limit, and the reason why was wrong

The program deliberately had no limit on how long to wait for the AI to start
answering. There was even a note in the code explaining it: a big model on a slow
link can take a long time to say its first word, so do not cut it off.

That is a good instinct. It is also, in this case, factually wrong — and the
proof was in the programming language's own manual.

The timer in question does not start until *your upload has finished*. So a slow
connection genuinely cannot trip it. Sending 100 kilobytes at 2 KB/s takes fifty
seconds of your life and **zero seconds** of that timer.

The note had never been checked against the manual. It sounded right, so it
stood, and it was quietly protecting the bug the whole time.

I left the wrong note in the file, next to the correction. Being wrong where
people can see it is more useful than being fixed quietly.

---

## The bit that was not planned

Half of this was a test I designed. The other half was an accident, and it is the
half most likely to affect you personally.

Before testing the broken cases, you run a **control** — the boring one, where
everything works, to prove the tollbooth itself is not causing problems. That is
basic. If your instrument is broken, every measurement it gives you is fiction.

The control hung.

So I ran it again with the tollbooth removed entirely. **It hung the same way.**
Which cleared the tollbooth of blame and pointed at something else.

Two direct requests to the AI provider — same company, same password, seconds
apart — split it open:

| what was asked | what came back |
|---|---|
| "list your models" | **worked, in 0.08 seconds** |
| "answer this question" (the model in use) | **nothing at all, forever** |

The provider was up. The account was fine. That *one model* was accepting
requests and never answering.

So I tried eight of the models from the provider's own published list:

| what happened | how many |
|---|---|
| Politely said "no such model" straight away | **4** |
| Worked properly | **1** |
| **Took the request and returned nothing, ever** | **2** |

**Only one model in eight actually worked.** And one of the silent ones was the
model this installation was set to use by default.

The honest refusals are fine — the program already handles those and tells you.
The silent ones are the problem, because from the outside "thinking very hard"
and "dead" look identical. You wait. And on a metered connection, waiting after
already uploading your whole conversation is money already spent.

**Correction, made the same afternoon.** The paragraph above originally said
these models were "not actually being served". Forty minutes later the same
silent model answered in **12 seconds**, and a second one in 19. A fuller sweep
of all 102 listed models found **32 working, 60 refusing politely, and 7 silent**
— so the silence is a **cold start**, not a lie. These endpoints go to sleep when
nobody is using them, and waking one up can take minutes.

That matters, because "this model is broken, use another" is bad advice if the
model would have worked on the next try. The wording the program shows you was
changed to say *not ready yet* rather than *not served*. The original claim is
left here rather than deleted, because a correction is only useful if you can see
what it corrected.

**What still holds: a provider's list of models is advertising, not a promise.**
The only way to know a model is ready is to ask it something. Roughly a third of
the list worked on the day this was written.

---

## What was changed

Three guards. All of them are limits with an explanation attached, and all of
them can be turned off if you disagree.

**1. A budget for how much one question may upload.** Counted in bytes, not in
attempts, because bytes are what you actually pay for. When it runs out the
program stops and tells you exactly what it spent. Default 4 megabytes, which is
dozens of honest retries — it is a backstop against a runaway, not a diet.

**2. A limit on waiting for the answer to start.** If the provider takes the
request and says nothing at all, the program gives up and says so, rather than
sitting there. Default two minutes, which is roughly 330 times longer than the
one model that actually worked took to reply.

**3. A limit on the answer going quiet halfway through.** This is a different
problem and needs a different guard: the answer began, and then the link died
mid-sentence. The first guard cannot catch this, because the answer *did* start.

That third one is the one worth explaining, because getting it wrong would be
worse than the original bug.

It is a **stall timer, not a stopwatch.** It does not care how long the answer
takes in total. It only cares whether anything arrived *recently*. Every fragment
of text that comes in resets it.

So an answer trickling in slowly on a terrible connection is **never** cut off,
no matter how long it takes overall. Only an answer that has delivered nothing at
all for a full minute and a half is.

That distinction is deliberate, and it is the whole design. Cutting off a working
answer would destroy something you had already paid for twice — once in money and
once in the time you spent uploading it. Better to wait too long than to throw
that away.

---

## Before and after

Same fake tollbooth, same two breakages, and for the third one the same genuinely
broken model. All measured.

| The cable is yanked | before | after |
|---|---|---|
| Attempts | 14, still going | **7** |
| Uploaded | 1.01 MB | **252 KB** |
| What you were told | nothing | **what happened and what it cost** |
| Time to give up | never | **50 seconds** |

| The connection goes quiet | before | after |
|---|---|---|
| In a script | hung indefinitely | **stops after 22 seconds** |
| Reported as failed | no | **yes** |

| The model never answers | before | after |
|---|---|---|
| Time to give up | still going at 110 seconds | **43 seconds** |
| What you were told | nothing | **that the model is probably not really being served, and to try another** |

And the row that mattered most to check, because a fix that breaks the normal
case is not a fix:

| A model that works | before | after |
|---|---|---|
| Time to answer | 4 seconds | **4 seconds** |
| Result | correct | **correct, unchanged** |

---

## So: how does it behave on a satellite link?

Honestly, and with what is still unknown stated as unknown.

**What it does well.**

- **It costs nothing when you are not using it.** Every connection closes. No
  background chatter, no check-ins, no telemetry. An idle session on a metered
  plan is genuinely free. *(Measured: zero bytes over 90 seconds idle.)*
- **It does not give up on a slow answer.** There is still no overall time limit
  on a reply. A large model crawling in over a bad link is left alone, which is
  the correct behaviour and was a deliberate choice from the start.
- **It now fails out loud.** Every one of the three failures above used to be
  silent. All three now stop and say what happened, in a sentence written to be
  read by a person rather than a programmer.
- **It fails cheaply.** The worst case found today went from a megabyte to a
  quarter of a megabyte, and from unbounded time to under a minute.

**What is genuinely expensive, and cannot be fixed.**

- Every message re-sends the whole conversation. **About 84 KB for a short
  question.** That is roughly ten seconds of uploading at 8 KB/s, every single
  time you press enter, and it grows as the conversation does. This is how the
  technology works, not a flaw in this program — but it is the single biggest
  cost on a slow link, and it is why the `/context` screen exists. Switching off
  parts you do not use is not decoration; it is measurably fewer bytes per
  message, forever.
- **85% of the traffic is upload.** That is backwards from ordinary internet use,
  and it matters because satellite and mobile connections are almost always much
  weaker going up than coming down. The direction that hurts is the direction
  this program uses most.

**What is not known yet, stated plainly rather than assumed.**

- **A connection that works but crawls — now tested, and it passes.** See the
  section below. This was the gap this document used to admit to; it is closed.
- **Recovery has not been tested.** If the link dies and comes back, does the
  program carry on, or does it need restarting? Unknown.
- The three fixes were verified from a script. The screen-and-keyboard version
  shares the same code underneath, so it should behave identically — but "should"
  is doing real work in that sentence and it has not been watched by eye.

**The overall verdict.** It is now safe to use on a connection that fails, in the
specific sense that mattered: it will not quietly spend your data allowance, and
it will not leave you staring at a screen unable to tell waiting from broken. It
was not safe in either of those ways this morning.

Whether it is *pleasant* on a slow-but-working link is a different question, and
that one is still open.

---

## The ordinary case: a link that works but crawls

Everything up to here broke the connection — cut it, silenced it, black-holed it.
But the everyday satellite experience is not a dramatic break. It is everything
being **slow**. A link that works fine, just at the speed of a trickle.

That is the case most likely to trip a false alarm, because all the new safety
limits are watching for silence — and a slow link produces long silences that are
not failures. If a guard cannot tell "crawling" from "dead", it will kill a
working answer, which is worse than the problem it was added for.

So it was tested directly, at two real satellite speeds, with a model that
answers instantly so the *only* thing being measured is the link.

| link speed | outcome | time | longest gap with nothing arriving |
|---|---|---|---|
| 4 KB/s | **finished, full answer** | 70 s | 18 s |
| 2 KB/s (a bad day) | **finished, full answer** | 130 s | 33 s |

Both finished cleanly. No false alarm, no cut-off answer, no error.

The number that matters is the last column. The guard that watches for a dead
stream gives up after **90 seconds** of nothing arriving. On the worst link
tested, the longest real gap was **33 seconds** — so there is roughly a
three-to-one safety margin before a slow link could ever be mistaken for a broken
one. It holds even on the bad-day 2 KB/s floor.

One honest observation from the same test, because it is the real cost on a slow
link and it is not small: a single throwaway question moved about **140 KB up the
wire** — the whole conversation and every tool description, re-sent for each step
of the answer. At 2 KB/s that is over a minute of uploading, and it is where
almost all of the 130 seconds went. The link was not the bottleneck the safety
limits worry about; the sheer volume of upload was. That is the same finding as
the rest of this document, arriving from a different direction: on a slow link,
**what you send costs far more than what you receive**, which is exactly why
turning off parts of the loadout you do not use is real time saved.

---

## A side question: why did one model answer twice?

While testing, a run printed the answer **twice** — not the same words copied,
two differently worded sentences:

> A mutex is a mutual-exclusion lock that **ensures only one thread can access** a
> shared resource or critical section **at a time**.
>
> A mutex is a mutual-exclusion lock that **lets only one thread access** a shared
> resource or critical section **at any given time**.

Different wording rules out the boring explanation. A program that accidentally
prints something twice prints the *identical* text. Two paraphrases can only come
from the model.

It was worth chasing anyway, because if the program were mixing the model's
private working-out into its answer, you would be paying for that text twice —
once to receive it, and again every following message, since the whole
conversation is re-sent each time.

**It is not.** Checked three ways:

1. **The raw data from the provider keeps them separate.** In one reply: 202
   fragments of private reasoning, 24 fragments of answer. Cleanly divided.
2. **The program stores them separately too** — the saved record has a
   "reasoning" part and a "text" part, and only the text part is printed.
3. **The duplicated text was inside the answer itself**, before the program
   touched it.

Then eleven models were each asked the same question six times:

| model | answered twice |
|---|---|
| `nvidia/nemotron-3-nano-30b-a3b` | **2 of 6** |
| the other ten | **0 of 6** |

One model out of eleven, intermittently. It is a quirk of that model — it writes
a draft, then a tidier version, and hands over both. Nothing in this program
causes it and nothing here can reliably fix it.

**Why it is worth knowing anyway.** If you see it, it is not a bug you should
report and not something to work around — switch model. It also costs you real
money on a slow link: a doubled answer is roughly double the download for that
message, and it stays in the conversation, so it is re-uploaded with everything
you send afterwards.

---

## Where the numbers come from

Every figure here was produced by a command on 18 August 2026 on the reference
machine, and each one is listed with its command in `FOOTPRINT.md`. Nothing here
is estimated or quoted from anyone else.

The two places where this document goes beyond measurement, marked so you can
discount them:

- Converting bytes into seconds at a given connection speed is arithmetic, and it
  assumes the connection achieves its stated speed with nothing re-sent. Real
  satellite links do worse than the arithmetic says.
- "Only one model in eight worked" is a sample of eight taken at one moment on
  one provider. It is not a claim about that provider's general reliability, and
  it may look completely different tomorrow.
