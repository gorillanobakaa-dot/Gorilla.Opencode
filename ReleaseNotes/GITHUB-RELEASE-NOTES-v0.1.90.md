# Gorilla OpenCode v0.1.90 — OSINT All-Source Intelligence Analysis

**Everything you need to judge this release is on this page.** Not behind a
link, not in a wiki, not "see the docs" — the complete plain-language
explanation and the complete technical one are printed below, in full, because
[the philosophy this project is built on](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.1.90/PHILOSOPHY.md)
holds that publishing something a reader cannot reach is transparency in theory
and a closed door in practice.

> *"Open source gave the world the recipe. It forgot to teach people how to cook."*

---

## Download

| File | For |
|---|---|
| `gorilla-opencode_0.1.90_amd64.deb` | Debian, Ubuntu, Mint — `sudo apt install ./gorilla-opencode_0.1.90_amd64.deb` |
| `gorilla-opencode-0.1.90-1-x86_64.pkg.tar.zst` | Arch, CachyOS, Manjaro — `sudo pacman -U ...` |
| `gorilla-opencode-linux-x86_64.tar.gz` | Any Linux, no installer — unpack and run |
| `SHA256SUMS-v0.1.90.txt` | Check what you downloaded is what we built: `sha256sum -c` |

Use `apt`, not `dpkg -i` — the package depends on `lynx`, and `dpkg` resolves
nothing.

---

## What it looks like

### The capability page — `/osint` with no question typed

[![The /osint capability page: status ARMED, and the five stages of a run explained in plain language](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.90/docs/screenshots/gallery/v0190-osint-page-armed.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.90/docs/screenshots/gallery/v0190-osint-page-armed.png)

### The same page scrolled — the iron rules, where your assessment is saved, and the cost in red

[![Iron rules, the privacy design writing outside the working folder, and the cost warning in red](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.90/docs/screenshots/gallery/v0190-osint-page-cost-and-privacy.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.90/docs/screenshots/gallery/v0190-osint-page-cost-and-privacy.png)

### The model arguing with itself about whether your question is worth the money

Nothing staged. A frivolous question, an explicit demand for the most expensive
run available, and the model's visible reasoning pushing back — *"Running a full
10-agent dossier on this is a waste of money"* — while still honouring an
instruction it was given plainly.

[![The model's reasoning on screen, weighing an expensive ten-agent run against a frivolous question](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.90/docs/screenshots/gallery/v0190-osint-model-weighs-the-spend.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.90/docs/screenshots/gallery/v0190-osint-model-weighs-the-spend.png)

### Eight helpers, each in its own lane, all killable

[![The /tasks monitor: eight helpers running, each labelled with the lane it owns, with kill and nuclear-kill keys](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.90/docs/screenshots/gallery/v0190-osint-eight-helpers-running.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.90/docs/screenshots/gallery/v0190-osint-eight-helpers-running.png)

### It asks before it reaches the internet, and shows the exact query

[![The permission dialog showing the full text of the medical search a helper wants to run](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.90/docs/screenshots/gallery/v0190-osint-permission-medical-search.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.90/docs/screenshots/gallery/v0190-osint-permission-medical-search.png)

*(The two page screenshots were captured on the build immediately before this
release's rename, so they show the previous title. The content is unchanged.)*

---

# For everyone — what it does and what it costs

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

### Read this before anything else: it costs real money

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

### Why this exists: confident and wrong is the default failure

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

### What a run actually does

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

### The levels of research: lanes, helper counts, and modes

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

### Where it looks: the source atlas

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

### How claims are graded: two axes, like a real intelligence shop

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

### How it says how sure it is — the part that fights nonsense

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

### Circular reporting: ten echoes are one voice

The web's defining disease: one press release goes out, ten outlets rewrite
it, and a naive search sees "ten sources agree." That is not corroboration —
it is one source, amplified.

The tool records each claim's **ultimate origin**. Two articles tracing back
to the same press release count as ONE source, and a claim is only promoted
to "confirmed" when genuinely **independent** sources — different origins,
different source families — say the same thing.

---

### What the dossier looks like

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

### What it will not do

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

### `/research` vs `/osint` at a glance

| | `/research` | `/osint` |
|---|---|---|
| Cost | modest, everyday | **real money — warned in $/minute up front** |
| Passes | one cycle | full cycle plus one bounded, targeted gap round |
| Grading | evidence tiers | two-axis A–F × 1–6 on every claim |
| Ships | on | **off — armed manually in `/context`** |
| For | ordinary questions | questions where being wrong costs more than the run |

---

### The doctrine behind it: three field manuals

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
multi-agent design, is in [OSINT-DOCTRINE.md](#for-auditors--where-the-method-comes-from). The complete
source listing is in [OSINT-SOURCE-CATALOG.md](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.1.90/docs/OSINT-SOURCE-CATALOG.md).

---

### Who this is for

Academic research is written for peer reviewers. Intelligence assessments are
written for commanders. Nobody writes serious, sourced assessments for a
scared fifteen-year-old on an 8 KB/s connection with an honest question he
cannot ask anyone else.

That is the person this format is built for. When that question comes in, the
answer arrives the way an analyst briefs a principal: the direct answer
first, every claim graded, what could not be established stated plainly — and
with the dignity the format itself enforces. Not a lecture, not a brush-off,
and never a confident guess dressed up as knowledge.

---

### Sources and credits — where this method comes from

Nothing in this tool's method was invented here. It is assembled from published
doctrine, and the sources deserve naming rather than a footnote.

**UK Government — Professional Head of Intelligence Assessment (PHIA).**
The judgement standard: the Probability Yardstick's seven terms and their
bands, the Analytical Confidence Rating and its three criteria, the separation
of likelihood from confidence, and the competency framework's requirements for
falsifiable hypotheses, challenged assumptions and articulated intelligence
gaps.

- [The Professional Development Framework for All-Source Intelligence Assessment](https://www.gov.uk/government/publications/intelligence-analysis-professional-development-framework/the-professional-development-framework-for-all-source-intelligence-assessment)
- [Explaining Uncertainty in UK Intelligence Assessment](https://www.gov.uk/government/publications/explaining-uncertainty-in-uk-intelligence-assessment/explaining-uncertainty-in-uk-intelligence-assessment)

> Contains public sector information licensed under the
> [Open Government Licence v3.0](https://www.nationalarchives.gov.uk/doc/open-government-licence/version/3/).

That licence permits copying, adapting and commercial use, and requires this
acknowledgement. It also forbids implying official status or endorsement, so to
be explicit: **this project is not endorsed by, affiliated with, or connected to
the UK Government, PHIA, or any government body.** We adapted their published
method; they have no idea we exist.

**US Army — declassified intelligence doctrine**, public domain as US Government
works: FM 2-0 *Intelligence* (2023) for the intelligence cycle and collection
architecture; ADP 2-0 *Intelligence* (2019) for the analytic standards; ATP
2-22.9 *Open-Source Intelligence* (2012) for source vetting, the Admiralty
two-axis grading and circular-reporting detection.

**The 985 sources themselves** are listed with their licences and access terms
in [OSINT-SOURCE-CATALOG.md](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.1.90/docs/OSINT-SOURCE-CATALOG.md). Each is queried under its
own terms of use.

---

# For auditors — where the method comes from

This is the technical companion to [OSINT-RESEARCH.md](#for-everyone--what-it-does-and-what-it-costs), the
plain-language track. That document explains what a run does and what it costs;
this one documents **where the method comes from**: which doctrine documents
were used, which procedures were ported into the tool, which were deliberately
cut, and the research literature the multi-agent architecture draws on. Every
ported concept is named against its source so the translation can be audited
against the original manuals.

---

### 1. Provenance: the three field manuals

The research method is a civilian translation of published, declassified US
Army intelligence doctrine — documents anyone can download. Three were used,
and the local reference copies are kept alongside the design spec
(`doctrine-reference/`): `FM_2-0_Intelligence_2023.pdf`,
`ADP_2-0_Intelligence_2019.pdf`, `ATP_2-22.9_OSINT_2012.pdf`. The military
content — rank, organization, combat application — was deliberately dropped;
the procedures, which are the part that took decades of institutional
experience to get right, were kept. Section 4 names the cut precisely.

**FM 2-0, *Intelligence* (2023)** contributed the process and the structure:
the intelligence cycle as a loop rather than a pipeline (ch. 1 sec. II), the
principle that the requester drives the work and answers are tied to
decisions, direct dissemination of priority answers, the time-critical
interrupt ("combat information" in the original), and the rule that the
collection architecture must be declared and verified before the work starts —
a capability not accounted for in the architecture cannot be counted on
(ch. 4).

**ADP 2-0, *Intelligence* (2019)** is the doctrinal foundation underneath
FM 2-0: the analytic standards (embrace ambiguity — "analysts will never have
all the information"; confidence stated per assessment, not per report;
research reduces uncertainty, it does not eliminate it), the four-factor
source-suitability test (availability, capability, vulnerability, performance
history), continuous assessment between rounds (ch. 5), and the finding that
all-source fusion "takes longer but is more reliable and less susceptible to
deception" than single-source work.

**ATP 2-22.9, *Open-Source Intelligence* (2012)** contributed the
OSINT-specific tradecraft, and is the heart of the vetting upgrade: the
two-axis reliability/credibility grading scheme, the six-factor source
evaluation (identity, authority, motive, access, timeliness, consistency),
circular-reporting detection (ch. 4), the search ladder and multi-engine rule
(ch. 5), the answerable-from-open-sources triage, provenance classes
(primary / secondary / authoritative / nonauthoritative), collection OPSEC
(a query string travels to every site it reaches), and the warning not to
over-restrict an inherently open product.

A fourth input was not doctrine at all: a finished in-house intelligence
product (a 2026 telemetry audit) served as the format exemplar for the dossier
output — answer-first ordering, per-finding "so what", and the habit of
publishing the basis of an estimate rather than only its conclusion. Its
fuller apparatus (a quantified basis-of-estimate header block, measured
per-source coverage tables, a tasking-delivery audit) is *(spec, not
shipped)*.

---

### 2. What was ported, and where it lives in the tool

| Doctrine concept | Source | Where it lives |
|---|---|---|
| Intelligence cycle: direct → collect → produce → disseminate → assess, as a loop | FM 2-0 ch. 1 sec. II; ADP 2-0 ch. 3 | The `/osint` run shape (Plan → Collect → Vet → Assess gaps → one bounded gap round → Report) and the lane method — each helper lane runs a compressed copy of the same cycle |
| PIR → SIR → indicator decomposition | FM 2-0 ch. 5–6, App. B; ATP 2-22.9 ch. 2 | Sub-question planning: 3–7 ranked questions, each tied to a decision, broken into directly-searchable sub-questions with pre-built confirm/kill indicators |
| Source mix, redundancy, cueing | FM 2-0 App. B | Multi-source collection: different source families per question via the source atlas, and cheap broad searches cueing expensive narrow ones (the method block's broad-then-narrow rule). Deliberate double-coverage of top questions is *(spec, not shipped)* |
| Two-axis Admiralty grading (A–F × 1–6) | ATP 2-22.9 ch. 4 | The dossier's evidence grades — every claim carries both axes |
| Circular-reporting detection | ATP 2-22.9 ch. 4 | The ultimate-origin rule: two entries tracing to one origin count as one source |
| BLUF and product format | FM 2-0 ch. 1; ADP 2-0 ch. 6; the exemplar | The dossier output: answer first, so-what per finding, detail layered below |
| Reporting what failed and what is unknown | ATP 2-22.9; ADP 2-0 analytic standards | The SOURCES TRIED and NOT ESTABLISHED sections |
| Write-to-release | replaces the classification system | Dual-track documentation: the layman track is first-class, not an afterthought |

**Shipped vs specified.** The rows above describe what the tool does today.
The design spec (`DOSSIER-DOCTRINE.md`, kept with the project's working notes)
goes further in a few places, each marked *(spec, not shipped)* so the
difference is auditable rather than implied.

The rows that need more than a table cell:

#### 2.1 The cycle is a loop, not a pipeline

The single most important structural fact ported: **assess is not a final
gate, it is the hinge that starts the next round.** Between rounds the
orchestrator runs the doctrinal checklist: retire answered questions ("delete
from the collection plan so collectors stay focused on unanswered and new
requirements" — this is what makes the loop converge); re-task partial ones
with a *changed* approach, never a repeat of the query that already failed;
and admit new questions the findings raised. The stop rule is also doctrine:
a timely answer that meets the requirement beats a perfect one that is late.

**What ships is one bounded round**, not an open loop: the orchestrator may
call the tool exactly once more, targeted at gaps it names as load-bearing.
That ceiling is a deliberate departure from doctrine, and the reason is cost
honesty — the user priced the run on the warning screen, and an agent that
decided for itself how many more rounds to buy would make that price a lie.
Continuous multi-round iteration under a budget governor is *(spec, not
shipped)*.

#### 2.2 Question decomposition

Doctrine's definition of a priority requirement is load-bearing: it exists
"to focus the employment of limited assets against competing demands." So the
question list is short, explicitly ranked, and each question is tied to a
decision — a question with no decision hanging on it is background, not
priority. Sub-questions are written to be answerable "by a simple spot
report": one short sourced statement, not an essay. Lanes receive sub-questions
plus indicators, never a broad topic.

#### 2.3 The two-axis grade

Every evidence entry carries two independently assessed grades, combined as
B2, F3, and so on:

| Source reliability (A–F) | Information credibility (1–6) |
|---|---|
| A reliable · B usually reliable · C fairly reliable · D not usually reliable · E unreliable · F cannot be judged | 1 confirmed by independent sources · 2 probably true · 3 possibly true · 4 doubtful · 5 improbable · 6 cannot be judged |

The axes are graded separately to defeat the halo effect: a reliable source
can carry a wrong claim, and an unknown source can carry a claim that
independently checks out. Two hard rules travel with the scheme: a
first-time, unknown source enters at F, never at A; and credibility grade 1
requires **independent** confirmation — which is where the next rule comes in.

#### 2.4 The ultimate-origin rule

ATP 2-22.9 treats circular reporting as the open web's defining disease: many
outlets echoing one origin is one voice amplified, not corroboration. The
port is mechanical: every evidence entry records the claim's ultimate origin;
two entries tracing to the same origin count as ONE source. Enforced twice —
inside each lane, and again at orchestrator merge time.

#### 2.5 The honesty sections

SOURCES TRIED lists every query, including the ones that returned nothing,
with the failure mechanism where a method structurally cannot work. NOT
ESTABLISHED states what could not be found, citing the searches that proved
the absence — absence is a measured finding with a method behind it, not a
shrug. Both come straight from doctrine's insistence that a gap papered over
is worse than a gap declared: the "not answerable from open sources" triage
bucket seeds NOT ESTABLISHED before collection even starts.

---

### 2.6 Expressing judgement: the UK all-source standard

The three manuals in §1 govern collection and vetting. They do not settle how a
finished judgement should be WORDED, and that gap is where an AI assessment
fails most often: uniform confidence, everything asserted in the same steady
voice. For that layer the tool follows the UK **Professional Development
Framework for All-Source Intelligence Assessment**, published by the
Professional Head of Intelligence Assessment (PHIA) — which is also where the
product takes its name, since fusing several source families and weighing them
IS all-source assessment rather than OSINT collection.

Two instruments are ported verbatim, because approximating them would defeat
the point.

**The PHIA Probability Yardstick.** Likelihood is stated in one of seven fixed
terms, never a number the model invented:

| Term | Range |
|---|---|
| Remote chance | >0% – ~5% |
| Highly unlikely | ~10% – ~20% |
| Unlikely | ~25% – ~35% |
| Realistic possibility | ~40% – <50% |
| Likely / probable | ~55% – ~75% |
| Highly likely | ~80% – ~90% |
| Almost certain | ~95% – <100% |

The gaps between bands are deliberate in the original and are preserved: they
are the space between one judgement and the next.

**The Analytical Confidence Rating.** HIGH, MODERATE or LOW, assessed against
three criteria — *information base* (what was actually found), *analytical
rigour* (how hard it was tested), and *complexity and volatility* (how
fast-moving the subject is). The framework's own distinction is the load-bearing
part: "probability reflects the likelihood that a statement is true, [while]
analytical confidence reflects the soundness and stability of the foundations on
which the assessment of likelihood has been made." So "highly likely, LOW
confidence" is a coherent and useful judgement, and collapsing the two into one
word destroys information.

**Where each grading system applies.** They are not rivals and both appear in a
product: the Admiralty letter+digit (ATP 2-22.9) grades a SOURCE and its
information at collection time; the yardstick and confidence rating express the
ANALYST'S judgement built on top of them.

Also carried from the framework's competencies: hypotheses must be "multiple,
distinct, plausible and falsifiable"; assumptions are identified and
"proactively challenge[d]"; intelligence gaps are articulated rather than
smoothed over; and analytical integrity means the assessment reflects the
evidence "in the face of challenge from your customer" — in this tool's case,
in the face of a user who plainly wants a particular answer.

Source: <https://www.gov.uk/government/publications/intelligence-analysis-professional-development-framework/the-professional-development-framework-for-all-source-intelligence-assessment>
and <https://www.gov.uk/government/publications/explaining-uncertainty-in-uk-intelligence-assessment/explaining-uncertainty-in-uk-intelligence-assessment>

---

### 3. What was deliberately NOT ported

Named so the cut is auditable rather than silent:

- **Rank and organizational structure.** No staff sections, no echelons.
- **Combat-specific content** — targeting, fires, force protection.
- **The classification system.** Replaced by the write-to-release idea only:
  everything produced is written for release, in two audience tracks.
- **Unit battle rhythm as a clock.** Kept only as the loop's hand-off
  sequencing.
- **Human-source operations.** The tool works open sources only; nothing
  requiring human field sources survives.

The concepts, procedures, and standard operating practices cross over; the
vocabulary, rank structure, and combat content do not.

---

### 4. Why military doctrine at all

Because these procedures are the product of decades of institutional learning
about being wrong expensively, and the failure modes they guard against are
exactly the failure modes of LLM research:

- **Single-source confidence.** A model states one retrieved claim as fact;
  doctrine answers with source mix, redundancy, and the rule that all-source
  beats single-source.
- **Circular reporting.** A model counts ten rewrites of one press release as
  consensus; doctrine answers with the ultimate-origin rule.
- **Gaps papered over.** A model fills what it could not find with fluent
  guesses; doctrine answers with NOT ESTABLISHED as a first-class product
  section and "embrace ambiguity" as an analytic standard.
- **Uniform confidence.** A chat answer is equally confident everywhere;
  doctrine grades every claim on two axes and states confidence per
  assessment, not per report.

Journalism and science converged on the same fixes independently. The
intelligence tradition was chosen because it wrote its version down as
checkable procedure — manuals with numbered paragraphs — which is precisely
what a tool needs to implement and what a reader needs to audit.

---

### 5. Design bibliography

The multi-agent architecture (orchestrator, parallel lanes, verification
layer, supervisor) is informed by the following papers, kept in the design
file `research.txt`. Titles and links reproduced verbatim.

#### Hierarchical multi-agent research, task decomposition, debate, verification, and scalable oversight

1. Multi²: Hierarchical Multi Agent Decision Making with LLM Based Agents in
   Interactive Environments — <https://arxiv.org/abs/2606.03698>
2. Teams of LLM Agents can Exploit Zero Day Vulnerabilities —
   <https://arxiv.org/abs/2406.01637>
3. Improving Factuality and Reasoning in Language Models through Multiagent
   Debate — <https://arxiv.org/abs/2305.14325>
4. On Scalable Oversight with Weak LLMs Judging Strong LLMs —
   <https://arxiv.org/abs/2407.04622>
   (PDF: <https://arxiv.org/pdf/2407.04622>)
5. Debating Truth: Debate Driven Claim Verification with Multiple Large
   Language Model Agents — <https://arxiv.org/abs/2507.19090>
6. Towards Detecting LLMs Hallucination via Markov Chain Based Multi Agent
   Debate Framework — <https://arxiv.org/abs/2406.03075>
7. Towards Scalable Oversight with Collaborative Multi Agent Debate in Error
   Detection — <https://arxiv.org/abs/2510.20963>
8. Hierarchical Debate Based Large Language Model (LLM) for Complex Task
   Planning of 6G Network Management — <https://arxiv.org/abs/2506.06519>
9. On the Importance of Task Complexity in Evaluating LLM Based Multi Agent
   Systems — <https://arxiv.org/abs/2510.04311>

#### The three papers to start with

The design file marks entries 1, 2, and 4 above as the starting set:
<https://arxiv.org/abs/2606.03698>, <https://arxiv.org/abs/2406.01637>,
<https://arxiv.org/abs/2407.04622>.

The same file carries the working list of research areas behind the
architecture — hierarchical task decomposition, multi-agent debate,
adversarial verification, critic/verifier/judge agents, scalable oversight,
iterative refinement — and the reference architecture sketch (overseer → task
decomposition → parallel research leads → evidence set → verification layer:
fact check / red team / source check → supervisor QC → synthesis → final
audit) that the lane-and-supervisor design implements.

---

### Sources and credits — where this method comes from

Nothing in this tool's method was invented here. It is assembled from published
doctrine, and the sources deserve naming rather than a footnote.

**UK Government — Professional Head of Intelligence Assessment (PHIA).**
The judgement standard: the Probability Yardstick's seven terms and their
bands, the Analytical Confidence Rating and its three criteria, the separation
of likelihood from confidence, and the competency framework's requirements for
falsifiable hypotheses, challenged assumptions and articulated intelligence
gaps.

- [The Professional Development Framework for All-Source Intelligence Assessment](https://www.gov.uk/government/publications/intelligence-analysis-professional-development-framework/the-professional-development-framework-for-all-source-intelligence-assessment)
- [Explaining Uncertainty in UK Intelligence Assessment](https://www.gov.uk/government/publications/explaining-uncertainty-in-uk-intelligence-assessment/explaining-uncertainty-in-uk-intelligence-assessment)

> Contains public sector information licensed under the
> [Open Government Licence v3.0](https://www.nationalarchives.gov.uk/doc/open-government-licence/version/3/).

That licence permits copying, adapting and commercial use, and requires this
acknowledgement. It also forbids implying official status or endorsement, so to
be explicit: **this project is not endorsed by, affiliated with, or connected to
the UK Government, PHIA, or any government body.** We adapted their published
method; they have no idea we exist.

**US Army — declassified intelligence doctrine**, public domain as US Government
works: FM 2-0 *Intelligence* (2023) for the intelligence cycle and collection
architecture; ADP 2-0 *Intelligence* (2019) for the analytic standards; ATP
2-22.9 *Open-Source Intelligence* (2012) for source vetting, the Admiralty
two-axis grading and circular-reporting detection.

**The 985 sources themselves** are listed with their licences and access terms
in [OSINT-SOURCE-CATALOG.md](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.1.90/docs/OSINT-SOURCE-CATALOG.md). Each is queried under its
own terms of use.

---

## Honest limits of this release

- The download grows to about 52 MB. On a single-digit-KB/s connection that is
  a long wait, once. The reasons are itemised in the metrics document shipped
  in the package.
- A tester on Arch/CachyOS reported leftover lines painted into his scrollback
  on v0.1.87. A bug of the same class was found and fixed in v0.1.89, and it
  may or may not have been his cause. **His report is still open**, and until he
  can confirm it on this build, nobody should say it is fixed.
- The `/osint` cost forecast is a calculation, not a measurement. It rests on
  three assumptions printed on the warning screen beside the number, so you can
  argue with them.
- The 985-source catalogue is too long to print here — it is
  [in the repository](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.1.90/docs/OSINT-SOURCE-CATALOG.md)
  and ships inside the package at
  `/usr/share/doc/gorilla-opencode/OSINT-SOURCE-CATALOG.md`.
- No telemetry, no accounts, no analytics. Nothing about your use of this
  program reaches us, which also means we learn about faults only when someone
  tells us.
