<!-- Version: 1.2.0 · updated 26-08-17-19-16 -->
# OSINT All-Source Intelligence Analysis — `/osint` explained for people who don't read code

**One sentence:** `/osint <question>` runs a real intelligence cycle — the
method a professional analyst shop uses — against hundreds of free public
sources, and hands you a graded, sourced dossier instead of a chat answer.

"OSINT" means **open-source intelligence**: answering a question using only
material anyone can legally access — public databases, official filings,
scientific papers, news wires. No hacking, no secrets. The method comes from
declassified US Army intelligence doctrine (FM 2-0 and its companions),
translated for civilian use: the military jargon is stripped out, the
procedures — the part that actually took decades to get right — are kept.

This is a **separate command from `/research`**. `/research` is the everyday
tool: one pass, modest cost. `/osint` is the heavy machine you bring out when
the answer genuinely matters and you are willing to pay for it.

---

## Read this before anything else: it costs real money

An `/osint` run spawns **4 to 10 helper agents**. A helper agent is a complete,
separate AI session — each one has its own conversation with the model, and
every message in every one of those conversations is billed to **your**
account, in tokens. (A token is the unit AI companies charge by — roughly
three-quarters of a word. Tokens are the meter that runs while AI works.)

Because of that, three protections are built in:

1. **It ships OFF.** The command does not exist until you arm it yourself in
   the `/context` menu (the screen where you switch program features on and
   off and see what each one costs). Nobody trips over this by accident.
2. **A warning screen before every run** shows the computed burn rate in
   dollars per minute for your current model and settings, plus your choice
   of helper count and mode. The figure is computed live for YOUR model at
   that moment — there is no generic number to quote here, which is the point.
3. **You choose the shape of the run**: how many helpers, and whether they run
   in **parallel** (fastest, most expensive per minute), **sequential** (one
   at a time, slower, cheaper per minute), or **supervised** (an extra agent
   double-checks the others' work — the most thorough and the most expensive).

After the warning, the tool respects your decision. It will not nag, and it
will not secretly throttle. It is your wallet and your call — the tool's job
is to make sure it was an *informed* call, then get out of the way.

If you are on a free-tier model or a small local one, `/research` is probably
the right tool. `/osint` exists for the questions where being wrong costs more
than the run does.

---

## Why this exists: confident and wrong is the default failure

Ask a chat AI a hard factual question and you get a fluent, confident answer —
frequently a wrong one. The model writes from memory, its memory has a cutoff
date and gaps, and nothing in a chat answer tells you *which parts* to trust.
The confidence is uniform; the accuracy is not.

The fix is not a smarter model. The fix is the same one journalism, science,
and intelligence analysis all converged on independently: **go to sources,
say which source said what, and grade how much each source can be trusted.**
Then a reader can check any claim without taking anyone's word for it —
including the AI's.

That is the entire design. Everything below is machinery for doing that
honestly at scale.

---

## What a run actually does

A chat answer is one step: generate text. An `/osint` run is a loop with five
stations, and the loop is the point:

**Plan → Collect → Vet → Assess gaps → (one bounded gap round) → Report**

1. **Plan.** Before touching a single source, the orchestrator (the agent in
   charge) breaks your question into 3–7 ranked sub-questions, each specific
   enough to be answered by one short sourced statement. For competing
   possible answers it writes down **indicators** in advance — the concrete
   evidence that would confirm or kill each candidate. Searching without this
   step is how you get an answer shaped by whatever turned up first.
2. **Collect.** Helper agents each take assigned sub-questions and work
   through real sources (next section) — cheap, broad searches first, and only
   then the expensive, narrow digging on whatever the broad pass surfaced.
3. **Vet.** Every claim found is graded on two axes (section after next),
   checked for circular reporting, and tagged with where it ultimately came
   from. **A source that was not actually opened and read is never cited.**
   No decorative footnotes.
4. **Report.** Findings are assembled into the dossier format described below.
5. **Assess gaps.** Whatever the first pass could not establish is collected,
   and if any of it is load-bearing for the answer, **one** follow-up round —
   smaller, targeted at the named gaps, with a changed approach — is allowed.
   One, by design: you priced this run at the warning screen, and a loop that
   decides for itself how long to keep spending would make that warning a lie.
   A timely answer that meets the need beats a perfect one that arrives late —
   that rule is doctrine, not laziness.

---

## The levels of research: lanes, helper counts, and modes

The tool does not send ten identical agents to google the same thing ten ways.
Each helper works one **lane** — a fixed, non-overlapping angle of attack — and
the lanes exist because each one guards against a specific, named failure.
There are ten:

| # | Lane | The question it owns |
|---|---|---|
| 1 | **LOCAL** | What already exists on this machine? (The answer being on disk already is common.) |
| 2 | **PRIOR ART** | Has someone, somewhere, already solved this? |
| 3 | **PRIMARY SOURCE** | What do the authoritative documents actually say? |
| 4 | **REQUIREMENT** | What does the target actually demand — as opposed to what everyone assumes? |
| 5 | **VERIFIER** | Try to refute the other helpers. Attack, don't agree. |
| 6 | **COST** | What would this actually cost the user? |
| 7 | **HISTORY** | How did it get this way? (Decisions have reasons; some still apply.) |
| 8 | **SIDESTEP** | Is the whole approach avoidable? |
| 9 | **ADVERSARY** | What breaks, leaks, or is not permitted? |
| 10 | **COMPLETENESS CRITIC** | What did nobody look at? |

**Helper count is the depth dial.** Four helpers runs the mandatory first four
lanes — the cheapest real investigation. Each helper you add switches on the
next lane in the list. Ten runs them all.

**Mode is the shape dial**, chosen on the warning screen:

- **parallel** (the default) — all helpers at once. Same total cost as
  sequential, a fraction of the waiting.
- **sequential** — one at a time. Slowest; for hard-rate-limited providers.
- **supervised** — parallel, plus an auditor agent that reviews each blind
  lane's report before you see it and returns APPROVED / WEAK / REJECTED with
  the problems named. The most rigorous and the most expensive: roughly
  double the sessions.

And the two commands are two levels of the same discipline: **`/research`**
runs the lanes once with evidence tiers — the everyday level. **`/osint`**
runs them under the full dossier doctrine: two-axis grading, circular-report
tracing, one targeted gap round, and the formal dossier product.

---

## Where it looks: the source atlas

The tool ships with a built-in source atlas backed by a registry of **985
sources**, classified by how they can be reached for free. The real counts,
from the registry file itself:

| Count | What it means |
|---|---|
| **985** | total sources in the registry |
| **866** | free to use |
| **370** | keyless APIs — machine-readable services needing no account, no key, no card. (An API is a door a program can knock on directly instead of scraping a web page.) |
| **118** | paid-only — kept in the registry **on purpose**, so the tool can tell you "the definitive source exists behind a subscription" instead of pretending it does not |

What lives in there, in plain terms: scholarly paper indexes (OpenAlex,
Crossref, PubMed), World Bank statistics, full-text search of SEC corporate
filings (the documents US companies must file by law), humanitarian data
(HDX, UNHCR), GDELT — a global news event database updated every 15 minutes —
plus sanctions lists, patents, technical standards, and climate data.

The free, no-card paths are the default, not the fallback — same as every
other part of this project. A source that fails or returns nothing during a
run is reported by name in the dossier's SOURCES TRIED section, not silently
skipped.

---

## How claims are graded: two axes, like a real intelligence shop

Every claim in the dossier carries a two-part grade, the same scheme
(sometimes called the Admiralty system) intelligence services use:

- **Source reliability, A–F** — how trustworthy is the *outlet*? A = a
  reliable official or primary source; F = no basis to judge.
- **Information credibility, 1–6** — how solid is *this particular claim*?
  1 = confirmed by other independent sources; 5 = improbable; 6 = cannot be
  judged.

So **A1** means "official source, independently confirmed" and **F6** means
"cannot judge the source, cannot judge the claim" — honest ignorance, recorded
as such. The two axes are
graded separately because they fail separately: a normally reliable outlet
can carry a wrong claim, and an unknown blogger can carry a claim that checks
out. One combined "trust score" hides exactly that distinction.

Two rules worth knowing:

- **A first-time, unknown source enters at F — never at A.** Trust is earned
  by track record, not by a professional-looking website.
- The grade travels with the claim into the final report, so you can see at a
  glance which parts of the answer are load-bearing and which are thin.

---

## How it says how sure it is — the part that fights nonsense

The characteristic failure of an AI answer is not that it is wrong. It is that
everything sounds **equally certain**: the solid parts and the guesses arrive in
the same steady voice, and you cannot tell them apart. That is what makes a
confident wrong answer dangerous.

So this follows the UK government's own standard for intelligence assessment,
which keeps two things apart that ordinary writing smears together:

**How likely is it?** Stated in one of seven fixed terms, never a number the AI
invented for itself:

| Term | What it means |
|---|---|
| Remote chance | above 0% up to about 5% |
| Highly unlikely | about 10–20% |
| Unlikely | about 25–35% |
| Realistic possibility | about 40% to just under 50% |
| Likely / probable | about 55–75% |
| Highly likely | about 80–90% |
| Almost certain | about 95% to just under 100% |

The gaps between the bands are deliberate — they are the space between one
judgement and the next, so nothing gets squeezed into false precision. If you
ever see "roughly 63% likely" in a report, someone made that number up.

**How solid is the basis?** Separately rated **HIGH**, **MODERATE** or **LOW**,
naming which of three things set the level: how much was actually found, how
hard it was tested, or how fast-moving and tangled the subject is.

Why keeping them apart matters: *"highly likely, LOW confidence — one source,
uncorroborated"* is an honest and genuinely useful sentence. It tells you the
best guess AND that you should not bet the house on it. Merged into one word,
that information is gone.

On top of that, every line is marked as a **fact** (a source said it), an
**inference** (the reasoning that followed), or an **assumption** (taken as
given, with no evidence) — and rival explanations must be written down and
tested, not just the first one that fit.

---

## Circular reporting: ten echoes are one voice

The web's defining disease: one press release goes out, ten outlets rewrite
it, and a naive search sees "ten sources agree." That is not corroboration —
it is one source, amplified.

The tool records each claim's **ultimate origin**. Two articles tracing back
to the same press release count as ONE source, and a claim is only promoted
to "confirmed" when genuinely **independent** sources — different origins,
different source families — say the same thing.

---

## What the dossier looks like

The report is built so the most important thing comes first and nothing is
papered over:

1. **BLUF — bottom line up front.** The direct answer to your question, first,
   in plain language, with its overall confidence. No throat-clearing.
2. **Findings per sub-question**, each claim carrying its two-axis grade and a
   "so what" — why it matters for the decision you are trying to make.
3. **SOURCES TRIED.** Every source consulted — *including the ones that
   returned nothing or failed*, with the reason where known. A search that
   found nothing across multiple engines is a finding with a method behind
   it, not a shrug.
4. **NOT ESTABLISHED.** What the run could **not** find out, stated plainly,
   with the searches that proved the absence. This section is the one most
   research products quietly omit. An honest "unknown" is worth more than a
   confident guess, and here it is a first-class part of the product.

---

## What it will not do

- It will not cite a source it did not actually open.
- It will not present one echoed press release as consensus.
- It will not manufacture certainty. Research reduces uncertainty; it does
  not eliminate it. The leftover risk is stated and left with you.
- Its helpers are under standing orders to keep your private details out of
  search queries. Queries travel to the sites they reach, so the discipline is
  to generalize first: the medical pattern, not the person; the company class,
  not your account.
- It will not run, or cost you anything, unless you armed it and accepted the
  warning screen.

---

## `/research` vs `/osint` at a glance

| | `/research` | `/osint` |
|---|---|---|
| Cost | modest, everyday | **real money — warned in $/minute up front** |
| Passes | one cycle | full cycle plus one bounded, targeted gap round |
| Grading | evidence tiers | two-axis A–F × 1–6 on every claim |
| Ships | on | **off — armed manually in `/context`** |
| For | ordinary questions | questions where being wrong costs more than the run |

---

## The doctrine behind it: three field manuals

The method is not invented here and not improvised by an AI. It is ported from
three **public, declassified US Army intelligence documents** — the procedures,
not the military content:

- **FM 2-0, Intelligence (2023)** — the intelligence cycle this tool runs
  (plan → collect → produce → disseminate → assess) and the idea that the
  architecture of sources is declared up front, not discovered mid-run.
- **ADP 2-0, Intelligence (2019)** — the doctrinal foundation: intelligence
  reduces uncertainty for a decision; it does not manufacture certainty.
- **ATP 2-22.9, Open-Source Intelligence (2012)** — the OSINT tradecraft:
  source vetting, circular-reporting detection, and the discipline of
  recording what was searched and found nothing.

Why military doctrine for a civilian tool: these procedures are decades of
institutional learning about **being wrong expensively**, and the failure
modes they guard against — single-source confidence, echo chambers mistaken
for consensus, gaps quietly papered over — are precisely the failure modes of
AI research. The full technical mapping of doctrine to code, including what
was deliberately left behind and the research bibliography behind the
multi-agent design, is in [OSINT-DOCTRINE.md](OSINT-DOCTRINE.md). The complete
source listing is in [OSINT-SOURCE-CATALOG.md](OSINT-SOURCE-CATALOG.md).

---

## Who this is for

Academic research is written for peer reviewers. Intelligence assessments are
written for commanders. Nobody writes serious, sourced assessments for a
scared fifteen-year-old on an 8 KB/s connection with an honest question he
cannot ask anyone else.

That is the person this format is built for. When that question comes in, the
answer arrives the way an analyst briefs a principal: the direct answer
first, every claim graded, what could not be established stated plainly — and
with the dignity the format itself enforces. Not a lecture, not a brush-off,
and never a confident guess dressed up as knowledge.
