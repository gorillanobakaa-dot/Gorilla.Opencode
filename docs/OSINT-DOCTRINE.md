<!-- Version: 1.1.0 · updated 26-08-17-19-16 -->
# The intelligence doctrine behind `/osint` — technical track

This is the technical companion to [OSINT-RESEARCH.md](OSINT-RESEARCH.md), the
plain-language track. That document explains what a run does and what it costs;
this one documents **where the method comes from**: which doctrine documents
were used, which procedures were ported into the tool, which were deliberately
cut, and the research literature the multi-agent architecture draws on. Every
ported concept is named against its source so the translation can be audited
against the original manuals.

---

## 1. Provenance: the three field manuals

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

## 2. What was ported, and where it lives in the tool

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

### 2.1 The cycle is a loop, not a pipeline

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

### 2.2 Question decomposition

Doctrine's definition of a priority requirement is load-bearing: it exists
"to focus the employment of limited assets against competing demands." So the
question list is short, explicitly ranked, and each question is tied to a
decision — a question with no decision hanging on it is background, not
priority. Sub-questions are written to be answerable "by a simple spot
report": one short sourced statement, not an essay. Lanes receive sub-questions
plus indicators, never a broad topic.

### 2.3 The two-axis grade

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

### 2.4 The ultimate-origin rule

ATP 2-22.9 treats circular reporting as the open web's defining disease: many
outlets echoing one origin is one voice amplified, not corroboration. The
port is mechanical: every evidence entry records the claim's ultimate origin;
two entries tracing to the same origin count as ONE source. Enforced twice —
inside each lane, and again at orchestrator merge time.

### 2.5 The honesty sections

SOURCES TRIED lists every query, including the ones that returned nothing,
with the failure mechanism where a method structurally cannot work. NOT
ESTABLISHED states what could not be found, citing the searches that proved
the absence — absence is a measured finding with a method behind it, not a
shrug. Both come straight from doctrine's insistence that a gap papered over
is worse than a gap declared: the "not answerable from open sources" triage
bucket seeds NOT ESTABLISHED before collection even starts.

---

## 2.6 Expressing judgement: the UK all-source standard

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

## 3. What was deliberately NOT ported

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

## 4. Why military doctrine at all

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

## 5. Design bibliography

The multi-agent architecture (orchestrator, parallel lanes, verification
layer, supervisor) is informed by the following papers, kept in the design
file `research.txt`. Titles and links reproduced verbatim.

### Hierarchical multi-agent research, task decomposition, debate, verification, and scalable oversight

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

### The three papers to start with

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

## Sources and credits — where this method comes from

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
in [OSINT-SOURCE-CATALOG.md](OSINT-SOURCE-CATALOG.md). Each is queried under its
own terms of use.
