package agent

// GORILLA OVERRIDE: parallel-role research, so this program can investigate a
// question instead of guessing at it.
//
// WHY THIS EXISTS
//   On 2026-08-13/14 an assistant spent two days and two multi-hour engine
//   rebuilds trying to make WhatsApp video calling work by patching a video
//   encoder. WhatsApp's calling gate never checks the video encoder; it checks
//   SharedArrayBuffer, which was one environment variable away. The failure was
//   not intelligence. It was working alone: no survey of what already existed
//   on the machine, no check for prior art, and — decisively — nobody read the
//   target's own feature detection. A structured multi-role investigation found
//   the answer in 38 minutes using the same model on the same day.
//
//   The `agent` tool could not express that. It takes a bare prompt: no role,
//   no output contract, no model of its own, and its helpers cannot reach the
//   web at all (TaskAgentTools is glob/grep/ls/view). Research needs all four.
//
// WHAT THIS ADDS OVER `agent`
//   1. ROLES with non-overlapping lanes, so helpers cannot all wander to the
//      same obvious place. Overlap is money spent twice; a gap is a wrong
//      answer. The four mandatory lanes are fixed for that reason.
//   2. AN OUTPUT CONTRACT every helper must satisfy, and which is CHECKED here
//      rather than hoped for. Malformed replies are reported as malformed
//      instead of being silently synthesised into a confident answer.
//   3. EVIDENCE TIERS. Every claim carries how it is known. A claim that
//      entered as one forum comment must not leave as a fact.
//   4. ITS OWN AGENT ENTRY (config.AgentResearch) so helpers can run a cheaper
//      model than the coder, and WEB ACCESS (ResearchAgentTools).
//
// PARALLEL, AND WHY THAT IS NOT A CONTRADICTION
//   The `agent` tool's docs say helpers run one at a time, and that is true of
//   the MODEL: it cannot overlap its own tool calls. It is not true inside a
//   single tool. This is one tool call, so the helpers within it run
//   CONCURRENTLY, capped at ResearchMaxInFlight. Six helpers take about as long
//   as the slowest one rather than the sum of six.
//
//   Sequential was the first implementation and it was simply wrong: helpers
//   wait on a provider, not on this CPU, so serialising them bought nothing and
//   multiplied the wall-clock by the agent count.
//
// COST — read before raising the default
//   Parallel is FASTER, not CHEAPER. N helpers is still N LLM sessions and N
//   times the tokens. That is why the 4..10 clamp exists and why roles should
//   be scaled to the question. Tokens are a recurring bill for the people this
//   is built for.

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/lsp"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/permission"
	"github.com/opencode-ai/opencode/internal/session"
)

const (
	ResearchToolName = "research"

	// The owner's bounds. Below 4 the ground is not covered; above 10 the
	// synthesising model skims, and a synthesiser that skims is worse than
	// helpers that never ran.
	ResearchMinAgents     = 4
	ResearchMaxAgents     = 10
	ResearchDefaultAgents = 6

	// How many helpers may be in flight at once.
	//
	// Helpers are network-bound: they wait on a provider, not on this CPU, so
	// running them concurrently costs no extra tokens and turns N sequential
	// round-trips into roughly one. That is the whole reason this tool exists
	// rather than the model calling `agent` N times — the model cannot overlap
	// its own tool calls, but a single tool can overlap what it does inside.
	//
	// GORILLA OVERRIDE 2026-08-14: raised from 4 to 11 at the owner's decision.
	//
	// The old reasoning is preserved because it was not wrong, only outweighed:
	//
	//	"Capped rather than unbounded because providers rate-limit, and ten
	//	 simultaneous streams from one key earns a 429 that looks like a bug.
	//	 Four is enough to hide almost all the latency: the 4 mandatory lanes
	//	 finish in one wave."
	//
	// What that missed is what a cap of 4 does to a run of 10. Six helpers sat
	// invisibly queued — absent from /tasks, absent from the status count, and
	// unkillable, because the registry only knew about helpers that had already
	// taken a slot. A user selected 10, was shown 4, and concluded the feature
	// was broken. He was reading the only evidence available to him.
	//
	// 11 = ResearchMaxAgents + 1, so a full run never queues, plus one spare
	// slot. The 429 risk is real and has not gone away — it is now VISIBLE
	// instead of hidden: a rate-limited helper shows as FAILED with a red
	// marker rather than silently shortening the investigation. A cap that
	// hides failure is worse than a failure you can see.
	//
	// The deciding argument, from the owner: research is MANUALLY TRIGGERED.
	// Nobody is surprised by it, the user is sitting there when it fires and
	// can watch it and kill it, and "there are times when you have to roll the
	// hard six and actually do the research". A throttle that quietly halves an
	// investigation the user deliberately asked for protects the wrong party.
	ResearchMaxInFlight = ResearchMaxAgents + 1
)

type researchTool struct {
	sessions    session.Service
	messages    message.Service
	lspClients  map[string]*lsp.Client
	permissions permission.Service
}

type ResearchParams struct {
	Question string `json:"question"`
	Context  string `json:"context"`
	Agents   int    `json:"agents"`
	Roles    string `json:"roles"`
	Mode     string `json:"mode"`
	// Doctrine selects the discipline: "" / "standard" is the everyday run;
	// "dossier" is the professional product — two-axis grading, a gap round,
	// the assessment format. Gated on the /context dossier row: the schema
	// only offers it when armed, and Run refuses it when not.
	Doctrine string `json:"doctrine"`
}

// researchRole is one investigator's lane. Lane is what it must cover;
// prevents records the failure it exists to stop, and is included in the
// prompt so a weak model understands WHY its lane is narrow.
type researchRole struct {
	ID       string
	Title    string
	Lane     string
	Prevents string
}

// The role library. The first four are MANDATORY and always run, in this
// order. Everything after is added only as the agent budget allows.
var researchRoles = []researchRole{
	{
		ID:    "local",
		Title: "LOCAL — what already exists here",
		Lane: "Search THIS machine and THIS project before anything else. Installed packages, " +
			"existing scripts and tools, project docs, README and CLAUDE.md/AGENTS.md files, memory " +
			"or notes directories, and anything already built that bears on the question. Report what " +
			"exists and its ACTUAL state — distinguish 'built and verified' from 'designed but never " +
			"tested', because that difference usually decides the answer.",
		Prevents: "Building a second copy of something the user already owns. This is the single most expensive failure mode.",
	},
	{
		ID:    "prior_art",
		Title: "PRIOR ART — has someone already solved this",
		Lane: "Find other people's solutions: repositories, packages, forks, distro packages. For each, " +
			"establish whether it ACTUALLY works, from its ISSUE TRACKER rather than its README. A README " +
			"states an intention; an issue thread states what happened. Name versions.",
		Prevents: "Days spent reinventing a fix that shipped upstream last month.",
	},
	{
		ID:    "primary_source",
		Title: "PRIMARY SOURCE — what do the authoritative documents say",
		Lane: "Go to source code, commits, specifications, release notes, changelogs and official docs. " +
			"Quote them with URLs or file:line. Where a blog post and a commit disagree, the commit wins " +
			"and you say so.",
		Prevents: "Believing a summary over the code that contradicts it.",
	},
	{
		ID:    "requirement",
		Title: "REQUIREMENT — what does the target actually demand",
		Lane: "Read what the TARGET itself checks, not what we assume it checks. Its feature detection, " +
			"its API contract, its error paths, its actual shipped code where you can reach it. State the " +
			"precise conditions it tests and in what order.",
		Prevents: "Fixing something the target never looks at. This is the exact error that cost two days and is why this lane is mandatory.",
	},
	{
		ID:    "verifier",
		Title: "VERIFIER — try to refute the others",
		Lane: "You are given the other helpers' findings. Attack them. Default to NOT ESTABLISHED when " +
			"evidence is thin. Grade every load-bearing claim's evidence tier and flag any resting on a " +
			"single README or forum comment. Say which claims apply to a DIFFERENT version, platform or " +
			"distro than the user's, because those are not available to them.",
		Prevents: "A confident wrong answer surviving to the end. This is the expensive failure, not ignorance.",
	},
	{
		ID:    "cost",
		Title: "COST — what would this actually cost the user",
		Lane: "Price every candidate route in the user's real currency: download size in MB, build time in " +
			"hours on THEIR hardware, RAM and CPU requirements, and whether it needs a paid account or a " +
			"credit card. State download size in both bytes and hours on a slow link where that matters.",
		Prevents: "Recommending something technically correct and practically unavailable.",
	},
	{
		ID:    "history",
		Title: "HISTORY — how did it get this way",
		Lane: "Trace deprecations, removed backends, renamed flags, and abandoned approaches. Establish " +
			"which version changed what, and whether the user's installed version is before or after each " +
			"change. A fix in a version they do not have is not a fix they have.",
		Prevents: "Applying advice written for a version that no longer resembles the one installed.",
	},
	{
		ID:    "sidestep",
		Title: "SIDESTEP — is the whole approach avoidable",
		Lane: "Assume the stated approach is wrong. What entirely different route reaches the same goal? " +
			"Different protocol, different tool, different layer, or an existing product that already does " +
			"it. Judge each on whether it actually reaches the goal, not on elegance.",
		Prevents: "Optimising a route that should have been abandoned.",
	},
	{
		ID:    "adversary",
		Title: "ADVERSARY — what breaks, leaks, or is not permitted",
		Lane: "Security, licensing and failure consequences of each candidate route. What data leaves the " +
			"machine, what licence forbids redistribution, what fails at scale or on the user's specific " +
			"hardware.",
		Prevents: "A route that works in testing and is unusable or unshippable in practice.",
	},
	{
		ID:    "completeness",
		Title: "COMPLETENESS CRITIC — what did nobody look at",
		Lane: "You are given the other helpers' findings. Do not summarise them. Name what was NOT covered: " +
			"a source unread, a claim untested, an obvious route nobody considered, a constraint everyone " +
			"ignored. Then name the single cheapest experiment that would most reduce the remaining " +
			"uncertainty.",
		Prevents: "A thorough-looking investigation with a hole in the middle.",
	},
}

// Roles that must be given the other helpers' findings to do their job.
func rolePeeksAtOthers(id string) bool { return id == "verifier" || id == "completeness" }

// SupervisedSessions reports what a supervised run of n helpers ACTUALLY costs,
// in sessions, and how many lanes really get audited.
//
// GORILLA FIX 2026-08-14: the /research dialog computed this as n*2, and the
// tool's own report header as len(roles)*2. Both are wrong for every helper
// count except 4, because supervision iterates firstWave ONLY — the peeking
// roles (verifier, completeness) read the other lanes' work and are never
// themselves audited.
//
// Measured against the real selectRoles: 4→8, 5→9, 6→11, 7→13, 8→15, 9→17,
// 10→18, against a printed 8,10,12,14,16,18,20. At 10 helpers the user was
// billed for two whole sessions that never run, and told 10 of 10 lanes were
// checked when 8 were.
//
// Both numbers are returned together on purpose: the session count and the
// audited-lane count are the same fact, and quoting one without the other is
// how "DOUBLE / every lane checked twice" got printed over a run that does
// neither.
func SupervisedSessions(n int) (sessions, audited int) {
	roles, _ := selectRoles("", n)
	audited = auditableLanes(roles)
	return len(roles) + audited, audited
}

// auditableLanes counts the lanes a supervised run will actually audit: the
// blind ones. This is the same partition the scheduler uses for firstWave, kept
// in one function so the forecast and the run cannot disagree.
func auditableLanes(roles []researchRole) int {
	n := 0
	for _, r := range roles {
		if !rolePeeksAtOthers(r.ID) {
			n++
		}
	}
	return n
}

// RunShape forecasts a run: how many model sessions it creates, how many lanes
// get audited, how many sequential BATCHES it takes, and roughly how long it
// lasts in seconds.
//
// GORILLA FIX 2026-08-14: the /research screen printed a per-minute rate and a
// run total with no DURATION between them, so the one quantity that carries the
// difference between parallel and supervised was invisible. The screen said
// "DOUBLE" next to two identical rates and the user — correctly — read that as
// broken arithmetic. Supervised does not burn faster; it burns for twice as
// long. Without the duration on screen that is unverifiable, and an
// unverifiable number on this screen is worth nothing.
//
// Batches, not helpers, set the wall clock: at most ResearchMaxInFlight run at
// once, so 10 blind lanes are 3 batches, not 1. The peeking lanes (verifier,
// completeness) are a further batch because they must read the others' output
// first, and under supervision the auditors are a whole extra pass over the
// blind lanes.
//
// The seconds are only as good as ResearchSecondsPerStep, which is an
// ASSUMPTION and is labelled as one on screen. The batch COUNT is exact.
func RunShape(mode string, n int) (sessions, audited, batches int, seconds float64) {
	roles, _ := selectRoles("", n)
	audited = auditableLanes(roles)
	peeking := len(roles) - audited

	width := ResearchMaxInFlight
	if mode == ModeSequential {
		width = 1
	}
	ceilDiv := func(a, b int) int {
		if a <= 0 {
			return 0
		}
		return (a + b - 1) / b
	}

	batches = ceilDiv(audited, width)
	sessions = len(roles)
	if mode == ModeSupervised {
		batches += ceilDiv(audited, width) // the auditors, a second full pass
		sessions += audited
	}
	batches += ceilDiv(peeking, width)

	seconds = float64(batches) * float64(config.ResearchStepsPerHelper) * config.ResearchSecondsPerStep
	return sessions, audited, batches, seconds
}

// Execution modes. Sequential is concurrency of 1, so all three share one
// scheduler rather than three code paths that can drift apart.
const (
	ModeSequential = "sequential"
	ModeParallel   = "parallel"
	ModeSupervised = "supervised"
)

// supervisorPrompt asks one checker to audit one lane's report BEFORE the
// orchestrator ever sees it as a finding.
//
// The verifier ROLE and this SUPERVISION LAYER are different things and both
// earn their place: the verifier reads everything and attacks the conclusion;
// a supervisor reads exactly one lane and audits whether that lane's own
// evidence supports its own claims. A weak claim can survive the verifier by
// never being the conclusion it chose to attack.
func supervisorPrompt(role researchRole, question, report string) string {
	return fmt.Sprintf(`You are a SUPERVISOR auditing one researcher's report. You are not
researching. You are deciding whether their work can be trusted.

THE QUESTION UNDER INVESTIGATION:
%s

THE LANE THIS RESEARCHER WAS GIVEN:
%s
%s

THEIR REPORT:
%s

Audit it. For each claim ask: does the evidence cited ACTUALLY support it, is the
tier honest, and did they stay in their lane? A claim tiered primary_source that
cites a forum post is mis-tiered and that is the failure you exist to catch.

You may use your tools to spot-check a claim, but do NOT redo their research.

OUTPUT — use exactly these headings:

## VERDICT
One word: APPROVED | WEAK | REJECTED
  APPROVED - claims are supported and tiers are honest
  WEAK     - usable, but specific claims are over-stated or under-evidenced
  REJECTED - the findings cannot be relied on

## PROBLEMS
One bullet per problem, quoting the claim. Write "none" if there are none.
Say plainly if a claim is mis-tiered, unsupported, out of lane, or contradicted.

## SAFE TO USE
The subset of their findings you would stand behind, restated compactly. If the
verdict is REJECTED, write "nothing from this lane".`,
		strings.TrimSpace(question), role.Title, role.Lane, strings.TrimSpace(report))
}

// sourceAtlas is the curated slice of the 985-source registry
// (docs/source-registry.json) that rides in every helper prompt: the
// strongest FREE, reachable source per research domain, anchor names first.
//
// GORILLA OVERRIDE (2026-08-17): ATP 2-22.9 requires every research plan to
// contain "identification of information sources and HOW those sources will
// be accessed" — until now helpers had working tools but no map, which is how
// a lane ends up googling for what the World Bank serves as a keyless API.
// ~950 tokens per helper, per run (not per turn); the owner chose reach over
// minimalism for research runs, which are manually triggered and budgeted.
// Regenerate from the registry when it changes; TestSourceAtlas guards shape.
//
//go:embed source-atlas.txt
var sourceAtlas string

// researchMethod is the collection cycle every helper works, injected into
// every helper prompt.
//
// GORILLA OVERRIDE (2026-08-17): ported from the owner's OSINT doctrine (the
// intelligence cycle: direction -> collection -> vetting -> analysis ->
// dissemination), fused with the two durable pieces of Anthropic's published
// research prompts (claude-cookbooks patterns/agents/prompts): the
// start-wide-then-narrow search strategy and an explicit stop condition.
// Measured need, 2026-08-07: with working tools, models searched one keyword
// ("deception"), never tried the synonym ("lie"), and padded results with
// invented DOIs. Method failures, not capability failures — so the method is
// imposed here. The audit of Anthropic's own prompts (2026-08-17) found the
// same hole from the other side: vet: 0, credib: 0 occurrences — their
// collector carries no vetting instructions at all.
func researchMethod(steps int) string {
	return fmt.Sprintf(`METHOD — work this cycle, in order:

1. DIRECTION. From your lane, write down the two to four specific questions you
   must answer. Every tool call serves one of them.
2. COLLECTION. Your tools and what each is for:
   - find: THIS machine — code, docs, configs, installed tools. Check here
     before the web; the answer already being on disk is common.
   - web_search: sources scholar / medical / crossref / openaccess / books /
     reference work with no key. source web is the user's private search
     engine; the tool says if it is missing. Start with one or two BROAD
     queries to map the ground, then narrow. Short queries beat long ones.
   - web_fetch: read a page you already have the address of.
   If a term returns nothing, try its synonyms and neighbouring terms before
   concluding absence — an index that misses one word often holds its twin.
3. VETTING. Before a result enters FINDINGS, establish: who wrote it, when, and
   which version or platform it applies to. A claim about a version the user
   does not run is labelled as such or left out. A source that merely repeats
   another source adds nothing to a claim's tier.
4. STOP CONDITION. The user is shown a cost estimate assuming
   about %d tool calls from you. When two consecutive calls add nothing new,
   stop and write up — searching past that point is spend, not diligence.
   Absence after an honest search is a reportable finding.
`, steps)
}

const researchOutputContract = `
OUTPUT — use EXACTLY these five headings, in this order, and nothing else at the
top level. Your reply is parsed. A missing heading is reported as malformed.

## ANSWER
Two to five sentences answering YOUR LANE directly. No preamble.

## FINDINGS
One bullet per finding, each in this exact shape:
- CLAIM: <one sentence> | EVIDENCE: <URL, commit, file:line, version, or command output> | TIER: <tier>

TIER must be one of, strongest first:
  primary_source    a commit, spec, source file, release note, or official doc
  config            a distro build file, package spec, or actual setting on disk
  multiple_reports  several independent reports naming a version
  single_claim      one README or one forum post. The WEAKEST. Label it honestly.

## SOURCES TRIED
One line per source or query consulted, INCLUDING the ones that returned
nothing — "web_search scholar 'X': nothing relevant" is a finding about
coverage. Every EVIDENCE entry above must trace to a line here; an
investigation that lists only its hits cannot be audited.

## CONFIDENCE
Exactly one word: proven | strong | plausible | speculative

## NOT ESTABLISHED
What you could not determine, stated plainly. This section is not optional and
"nothing" is rarely the honest answer.

RULES
- Never present a single_claim as fact.
- Never cite a source you did not open. A DOI or URL you constructed from
  memory is an invention, not evidence — leave the claim in NOT ESTABLISHED
  instead.
- A well-supported NO is a complete answer. If nobody has ever done this, say so
  plainly rather than digging to avoid reporting failure.
- Do not repeat another helper's claim to make it look stronger. A claim's tier
  never improves by being restated.
- Stay in your lane. Another helper is covering the ground you are tempted by.
- You cannot modify anything. Investigate and report.
`

func (r *researchTool) Info() tools.ToolInfo {
	return r.infoWithDoctrine(r.infoBase())
}

// infoBase is the schema WITHOUT the dossier addition.
//
// GORILLA FIX (2026-08-17): calibration measures this, not Info(). Measuring
// Info() while the dossier row was armed counted the dossier's tokens twice —
// once inside tool.research (which had grown by them) and once as the
// tool.dossier row — inflating the /context header total by ~163. Same
// double-count class as the research basis figure fixed on 2026-08-14: two
// rows must measure disjoint things or the total is fiction.
func (r *researchTool) infoBase() tools.ToolInfo {
	info := tools.ToolInfo{
		Name: ResearchToolName,
		Description: "Investigate a question with several helper agents in fixed, non-overlapping roles, " +
			"then report their findings for you to synthesise.\n\n" +
			"USE THIS WHEN the answer is not already known and being wrong is expensive: 'is this approach " +
			"even right', 'has someone solved this', 'why does X not work', 'what should we build'. " +
			"Do NOT use it for something a single grep, file read or command answers — that is pure waste.\n\n" +
			"HOW IT WORKS. Four mandatory lanes always run: LOCAL (what already exists here), PRIOR ART " +
			"(has someone solved it), PRIMARY SOURCE (what the authoritative documents say) and REQUIREMENT " +
			"(what the target actually demands). Extra agents add VERIFIER (tries to refute the others), " +
			"COST, HISTORY, SIDESTEP, ADVERSARY and COMPLETENESS CRITIC. Helpers can search the web, fetch " +
			"pages, and read the filesystem; they cannot modify anything.\n\n" +
			"MODES. 'parallel' (default) runs helpers concurrently — 6 helpers take about as long as the " +
			"slowest one. 'sequential' runs them one at a time. 'supervised' adds a second agent that audits " +
			"each lane before you see it and returns APPROVED/WEAK/REJECTED.\n\n" +
			"SCALE THE RUN TO THE QUESTION before choosing agents: a single factual question with one likely " +
			"answer = 4 (the mandatory lanes); a comparison or 'which approach' question = 5-6, adding verifier " +
			"and cost; an open-ended investigation or one where being wrong is expensive = 7+ and consider " +
			"mode=supervised. Over-spawning is the main way this tool wastes the user's money.\n\n" +
			"Helpers carry the find tool (local search: ranked, with context lines) and web_search " +
			"(seven keyless scholarly sources, plus the user's private SearXNG when configured), and follow a " +
			"collection method: direction, broad-then-narrow collection, source vetting, an explicit stop " +
			"condition, and a SOURCES TRIED log including queries that returned nothing.\n\n" +
			"COST. Parallel is faster, NOT cheaper: 6 helpers is still 6 LLM sessions and 6x the tokens, and " +
			"'supervised' roughly doubles that again. Ask for 4 unless the question genuinely spans more " +
			"ground. Helpers use the 'research' agent model from config, which you can point at a cheaper " +
			"model than the coder.\n\n" +
			"YOU are the orchestrator. This tool returns each helper's findings with their evidence tiers; " +
			"reading them, resolving disagreements and writing the answer is your job. Do not repeat a " +
			"single_claim to the user as fact.",
		Parameters: map[string]any{
			"question": map[string]any{
				"type": "string",
				"description": "The research question, stated fully. Helpers cannot see the conversation, " +
					"so include everything needed to understand it.",
			},
			"context": map[string]any{
				"type": "string",
				"description": "Facts ALREADY established, pasted into every helper so none of them pays to " +
					"re-derive what you know. Include the environment (OS, hardware, versions), constraints " +
					"that disqualify options, and anything already proven or ruled out. Strongly recommended: " +
					"omitting it is the main way a research run wastes money.",
			},
			"agents": map[string]any{
				"type": "integer",
				"description": fmt.Sprintf("How many helpers, %d to %d (default %d). %d runs the mandatory "+
					"lanes only. Each one is a sequential LLM session — scale to the question, not to the maximum.",
					ResearchMinAgents, ResearchMaxAgents, ResearchDefaultAgents, ResearchMinAgents),
			},
			"mode": map[string]any{
				"type": "string",
				"enum": []string{"sequential", "parallel", "supervised"},
				"description": "How helpers run. 'parallel' (default) runs them concurrently, so the wall-clock " +
					"is the slowest helper rather than the sum — same token cost, much less waiting. " +
					"'sequential' runs one at a time: slower, but easier to follow and gentler on a " +
					"rate-limited key. 'supervised' is parallel PLUS a second agent auditing each lane's " +
					"report before you see it, returning APPROVED / WEAK / REJECTED with the problems named — " +
					"roughly DOUBLE the sessions and the tokens, for when being wrong is expensive.",
			},
			"roles": map[string]any{
				"type": "string",
				"description": "Optional comma-separated role IDs to run instead of the default set: " +
					"local, prior_art, primary_source, requirement, verifier, cost, history, sidestep, " +
					"adversary, completeness. The count still clamps to the 4..10 bounds.",
			},
		},
		Required: []string{"question"},
	}
	return info
}

// info wraps the static schema with the dossier addition when — and only
// when — the /context row is armed. The everyday loadout never pays for it.
func (r *researchTool) infoWithDoctrine(base tools.ToolInfo) tools.ToolInfo {
	if !config.LoadoutEnabled(config.DossierComponentID) {
		return base
	}
	return addDoctrine(base)
}

// addDoctrine applies the marginal schema unconditionally. Split out so the
// cost can be measured without consulting — or disturbing — the loadout.
func addDoctrine(base tools.ToolInfo) tools.ToolInfo {
	base.Description += dossierSchemaBlurb
	base.Parameters["doctrine"] = dossierParamSchema()
	return base
}

// selectRoles picks the roles to run. Explicit IDs win; otherwise the library
// order is taken, which puts the four mandatory lanes first by construction.
func selectRoles(spec string, want int) ([]researchRole, string) {
	byID := make(map[string]researchRole, len(researchRoles))
	for _, role := range researchRoles {
		byID[role.ID] = role
	}

	var chosen []researchRole
	var notes []string

	if strings.TrimSpace(spec) != "" {
		seen := make(map[string]bool)
		for _, raw := range strings.Split(spec, ",") {
			id := strings.ToLower(strings.TrimSpace(raw))
			if id == "" || seen[id] {
				continue
			}
			role, ok := byID[id]
			if !ok {
				notes = append(notes, fmt.Sprintf("unknown role %q ignored", id))
				continue
			}
			seen[id] = true
			chosen = append(chosen, role)
		}
		// Top up from the library so the mandatory lanes are never silently lost.
		for _, role := range researchRoles {
			if len(chosen) >= want {
				break
			}
			if !seen[role.ID] {
				seen[role.ID] = true
				chosen = append(chosen, role)
				notes = append(notes, fmt.Sprintf("added %s to reach the %d-agent minimum", role.ID, ResearchMinAgents))
			}
		}
	} else {
		chosen = append(chosen, researchRoles...)
	}

	if len(chosen) > want {
		chosen = chosen[:want]
	}
	return chosen, strings.Join(notes, "; ")
}

// buildPrompt assembles one helper's instructions. Everything the helper knows
// is here: it cannot see the conversation or the other helpers.
func buildPrompt(role researchRole, question, sharedContext, peerFindings string, index, total int, doctrine string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "You are helper %d of %d in a research investigation. Your role is:\n\n%s\n\n",
		index+1, total, role.Title)
	fmt.Fprintf(&b, "YOUR LANE — cover this and only this:\n%s\n\n", role.Lane)
	fmt.Fprintf(&b, "WHY YOUR LANE IS NARROW: %s\n\n", role.Prevents)
	fmt.Fprintf(&b, "THE QUESTION UNDER INVESTIGATION:\n%s\n\n", question)

	if strings.TrimSpace(sharedContext) != "" {
		fmt.Fprintf(&b, "ALREADY ESTABLISHED — treat as given, do NOT re-derive any of this:\n%s\n\n", sharedContext)
	}
	if strings.TrimSpace(peerFindings) != "" {
		fmt.Fprintf(&b, "THE OTHER HELPERS REPORTED THE FOLLOWING. Your job is to test it, not to agree with it:\n%s\n\n", peerFindings)
	}

	b.WriteString(researchMethod(config.ResearchStepsPerHelper))
	if doctrine == "dossier" {
		b.WriteString(dossierMethodAddendum)
	}
	b.WriteString("\n")
	b.WriteString(sourceAtlas)
	if doctrine == "dossier" {
		b.WriteString(dossierOutputContract)
	} else {
		b.WriteString(researchOutputContract)
	}
	return b.String()
}

// dossierSchemaBlurb and dossierParamSchema are the marginal schema the model
// sees ONLY while the /context dossier row is armed — the everyday loadout
// does not pay for a feature that is switched off.
const dossierSchemaBlurb = "\n\nDOSSIER DOCTRINE (armed on this install). Pass doctrine=\"dossier\" ONLY when the user " +
	"chose it through /osint — never volunteer it, the run costs several times an ordinary question. " +
	"It switches helpers to two-axis intelligence grading (source reliability A-F x information " +
	"credibility 1-6), adds a bounded gap round, and your synthesis must follow the dossier product " +
	"format the tool's report will spell out."

func dossierParamSchema() map[string]any {
	return map[string]any{
		"type": "string",
		"enum": []string{"standard", "dossier"},
		"description": "Investigation discipline. \"standard\" (default): the everyday run. \"dossier\": the " +
			"professional product — ONLY when the user explicitly chose it via /osint; it multiplies cost.",
	}
}

// DossierSchemaTokens is the per-turn cost of arming the all-source row.
//
// GORILLA FIX (2026-08-17): measured by DIFFERENCE between the two real
// schemas, not by adding up the strings. Summing the blurb and the parameter
// under-reported by 5 tokens, because the schema is paid as JSON: the
// description gets escaped, and the parameter arrives with a `"doctrine":`
// key around it. A row's figure has to be what the model is actually sent, or
// it is another number nobody can trust — the failure this session already hit
// twice. State-independent by construction: it forces the doctrine on rather
// than asking the loadout, so the figure does not depend on the switch it
// describes.
func DossierSchemaTokens() int {
	r := &researchTool{}
	return infoTokens(addDoctrine(r.infoBase())) - infoTokens(r.infoBase())
}

// dossierMethodAddendum upgrades a helper's vetting to the two-axis discipline.
// Injected AFTER the standard method so the cycle stays; only the grading
// vocabulary deepens.
const dossierMethodAddendum = `
DOSSIER DISCIPLINE — this run produces a professional assessment. Two additions:

GRADING, two axes, every claim. Replace the TIER vocabulary with GRADE: a
letter for the SOURCE and a digit for the INFORMATION, judged separately.
  Source reliability:  A reliable (official/primary, track record) · B usually
  reliable · C fairly reliable · D not usually reliable · E unreliable ·
  F cannot be judged (new or unknown source).
  Information credibility: 1 confirmed by other INDEPENDENT sources · 2 probably
  true · 3 possibly true · 4 doubtful · 5 improbable · 6 cannot be judged.
  A1 = official and independently confirmed. F6 = honest ignorance. B2 is a
  perfectly respectable grade — most solid reporting lives there. Grade
  honestly: an inflated grade is a lie with a letter on it.

CIRCULAR REPORTING. Before counting a second source as confirmation, establish
its ULTIMATE ORIGIN. Ten outlets quoting one press release are ONE source at
one remove. Independence means different original observers, not different URLs.

QUERY HYGIENE. Web queries travel to the sites they reach and into their logs.
Never put the user's personal identifiers — name, email, exact location, or
anything that singles them out — into a query. Generalize the question first:
the medical pattern, not the person; the company class, not the account.

SAY HOW LIKELY, IN THE STANDARD WORDS. When you state a judgement rather than a
fact, express its likelihood with ONE of these seven terms and no others. This
is the UK PHIA Probability Yardstick; the percentages are what the terms mean,
not something to quote at the reader:
    Remote chance        >0% – ~5%
    Highly unlikely     ~10% – ~20%
    Unlikely            ~25% – ~35%
    Realistic possibility ~40% – <50%
    Likely / probable   ~55% – ~75%
    Highly likely       ~80% – ~90%
    Almost certain      ~95% – <100%
The gaps between the bands are deliberate: they are the room between one
judgement and the next, so do not straddle them. Never invent a number of your
own ("about 63% likely") — that is false precision and it is the single easiest
way to sound authoritative while being wrong.

SAY HOW SOUND THE BASIS IS — SEPARATELY. Likelihood and confidence are two
different things: probability is how likely the statement is to be true;
analytical confidence is how solid the foundation under that estimate is. A
well-founded "unlikely" and a shaky "unlikely" are not the same claim. Rate
confidence HIGH, MODERATE or LOW against the three PHIA criteria, and name
which one is dragging it down:
    Information base   — the quantity and quality of what you actually found
    Analytical rigour  — how hard you examined it, and whether you tested it
    Complexity and volatility — how fast-moving or tangled the subject is

LABEL WHAT KIND OF STATEMENT YOU ARE MAKING. Every line in your findings is one
of three things and must be readable as such: a FACT (something a source
states, with that source), an INFERENCE (your reasoning from those facts), or
an ASSUMPTION (something you are taking as given without evidence). Assumptions
are not a weakness — leaving them unlabelled is. State them, and say what would
break each one.

CONSIDER MORE THAN ONE ANSWER. Before settling, write down at least two
distinct, plausible explanations that could account for what you found, and say
what evidence would falsify each. If a single explanation was obvious from the
first search and you never looked for a rival, say so — that is a finding about
your own method, and it belongs in NOT ESTABLISHED.

ANALYTICAL INTEGRITY. Your assessment reports what the evidence supports, not
what the user appears to want, not what is comfortable, and not what would make
the run look worthwhile. If the honest answer is "the evidence does not settle
this", that IS the finding.
`

// dossierOutputContract replaces the standard contract's FINDINGS shape for
// dossier runs: same five headings (the parser stays one parser), GRADE
// replacing TIER.
var dossierOutputContract = strings.NewReplacer(
	"| TIER: <tier>", "| GRADE: <A-F><1-6>",
	`TIER must be one of, strongest first:
  primary_source    a commit, spec, source file, release note, or official doc
  config            a distro build file, package spec, or actual setting on disk
  multiple_reports  several independent reports naming a version
  single_claim      one README or one forum post. The WEAKEST. Label it honestly.`,
	`GRADE is two characters: source reliability A-F, then information
credibility 1-6, as defined in DOSSIER DISCIPLINE above. Grade every claim;
a claim you cannot grade is F6 and belongs in NOT ESTABLISHED.`,
	"- Never present a single_claim as fact.",
	"- Never present anything below grade 2 as fact; C3 and weaker are leads, not findings.",
	"## CONFIDENCE\nExactly one word: proven | strong | plausible | speculative",
	`## CONFIDENCE
Two lines, in this order:
  LIKELIHOOD: one of the seven yardstick terms (remote chance / highly unlikely /
  unlikely / realistic possibility / likely / highly likely / almost certain),
  covering your lane's ANSWER above. Omit this line only if your lane produced
  no judgement at all, only facts.
  CONFIDENCE: HIGH, MODERATE or LOW — and in the same sentence, which of
  information base, analytical rigour, or complexity/volatility set that level.
A high likelihood on a low-confidence basis is a normal and honest result. The
two are separate claims; do not average them into one word.`,
).Replace(researchOutputContract)

// dossierDutiesFooter is appended to the tool's report on dossier runs: the
// gap round (bounded, so the loop cannot run away with the user's money) and
// the product format the synthesis must follow. %s is the dossier directory.
const dossierDutiesFooter = `
## DOSSIER DUTIES — this was a dossier run, three more obligations

1. GAP ROUND (bounded). Collect every NOT ESTABLISHED entry above. If any of
   them is LOAD-BEARING for the answer, you may call the research tool ONCE
   more: doctrine="dossier", agents=4 or fewer, roles chosen to target the
   named gaps, and the gaps pasted into context. ONE follow-up call is the
   ceiling — the user priced this run at the warning screen, not an open loop.
   If the gaps are not load-bearing, say so and skip the round.

2. THE PRODUCT. Assemble the assessment in exactly this shape:
   # All-Source Assessment: <the question>
   ## Key judgements — the direct answer first, three sentences or fewer. EVERY
      judgement carries a yardstick term (remote chance / highly unlikely /
      unlikely / realistic possibility / likely / highly likely / almost
      certain) AND an analytical confidence of HIGH, MODERATE or LOW with the
      reason for that level. Likelihood and confidence are separate claims:
      "highly likely, LOW confidence — one source, no corroboration" is a
      legitimate and useful judgement. Never merge them into one word.
   ## Findings — every claim with its two-axis source GRADE carried through
      unchanged, each marked FACT, INFERENCE or ASSUMPTION.
   ## Alternatives considered — the rival explanations and what would falsify
      each. If you tested none, say that.
   ## Sources tried — including the ones that returned nothing
   ## Not established — the intelligence gaps, stated plainly, never papered
      over. Say which key judgement each gap would move if it were filled.
   ## Recommended action — what the user should do next, one paragraph

   The two grading systems are not rivals and must both appear: the Admiralty
   letter+digit grades the SOURCE and its information; the yardstick and
   confidence rating express YOUR judgement built on them.

3. THE FILE. Write the complete dossier as markdown into a NEW file under
   %s (create the folder if missing), filename
   dossier-<YY-MM-DD-HH-MM>-<three-word-slug>.md, and tell the user the exact
   path. NEVER write it into the working folder: working folders end up in git
   repositories, and a private question must never end up in a public commit.
   In the conversation, give the bottom line and key findings only.
`

// checkContract reports which required headings a reply is missing. A helper
// that ignored the contract is reported as such rather than quietly folded into
// the synthesis — a malformed reply read as a finding is how a guess becomes a
// fact.
func checkContract(reply string) []string {
	var missing []string
	for _, heading := range []string{"## ANSWER", "## FINDINGS", "## SOURCES TRIED", "## CONFIDENCE", "## NOT ESTABLISHED"} {
		if !strings.Contains(strings.ToUpper(reply), heading) {
			missing = append(missing, heading)
		}
	}
	return missing
}

func (r *researchTool) Run(ctx context.Context, call tools.ToolCall) (tools.ToolResponse, error) {
	var params ResearchParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return tools.NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}
	if strings.TrimSpace(params.Question) == "" {
		return tools.NewTextErrorResponse("question is required"), nil
	}

	// GORILLA OVERRIDE: the dossier is user-armed, never model-volunteered.
	// The schema only offers it when the /context row is on, but a model can
	// pass any string — so the gate is enforced here too, with the fix named.
	params.Doctrine = strings.ToLower(strings.TrimSpace(params.Doctrine))
	if params.Doctrine == "standard" {
		params.Doctrine = ""
	}
	if params.Doctrine != "" && params.Doctrine != "dossier" {
		return tools.NewTextErrorResponse(fmt.Sprintf("unknown doctrine %q — use \"standard\" or \"dossier\"", params.Doctrine)), nil
	}
	if params.Doctrine == "dossier" && !config.LoadoutEnabled(config.DossierComponentID) {
		return tools.NewTextErrorResponse("the dossier doctrine is not armed on this install. The USER must switch on \"" +
			config.DossierRowName + "\" in /context (it costs real money, so it ships off). Run standard research instead, or tell the user how to arm it."), nil
	}

	want := params.Agents
	var clampNote string
	switch {
	case want == 0:
		want = ResearchDefaultAgents
	case want < ResearchMinAgents:
		clampNote = fmt.Sprintf("(raised from %d to the %d-agent minimum: fewer than four lanes does not cover the ground)", want, ResearchMinAgents)
		want = ResearchMinAgents
	case want > ResearchMaxAgents:
		clampNote = fmt.Sprintf("(lowered from %d to the %d-agent maximum: past ten the synthesis degrades)", want, ResearchMaxAgents)
		want = ResearchMaxAgents
	}

	sessionID, messageID := tools.GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return tools.ToolResponse{}, fmt.Errorf("session_id and message_id are required")
	}

	// GORILLA OVERRIDE: the same helper-leash the agent tool honours. A research
	// run spawns several helpers, so it is checked ONCE for the whole run rather
	// than per helper — a run that can only half-happen is worse than one that
	// does not start, because a partial investigation still reads like a
	// complete one.
	switch limit := config.MaxSubAgents(); {
	case limit == config.SubAgentsNuclear:
		return tools.NewTextErrorResponse("Sub-agents are DISABLED (Gorilla Nuclear Option). Research helpers cannot run. Investigate with the direct tools, or re-enable helpers in /context."), nil
	case limit != config.SubAgentsUnlimited:
		if ok, used := reserveSubAgentSpawn(sessionID, limit); !ok {
			return tools.NewTextErrorResponse(fmt.Sprintf("Helper-agent limit reached for this turn (%d used of %d allowed). Investigate with the direct tools instead.", used, limit)), nil
		}
		if want > limit {
			clampNote = strings.TrimSpace(clampNote + fmt.Sprintf(" (further limited to %d by the helper-leash in /context)", limit))
			want = limit
		}
		if want < ResearchMinAgents {
			return tools.NewTextErrorResponse(fmt.Sprintf("The helper-leash allows %d helper(s); research needs at least %d to cover the ground. Raise the leash in /context or investigate with the direct tools.", limit, ResearchMinAgents)), nil
		}
	}

	roles, roleNote := selectRoles(params.Roles, want)
	if len(roles) == 0 {
		return tools.NewTextErrorResponse("no usable roles selected"), nil
	}

	mode := strings.ToLower(strings.TrimSpace(params.Mode))
	switch mode {
	case "", ModeParallel, "concurrent":
		mode = ModeParallel
	case ModeSequential, ModeSupervised:
	default:
		return tools.NewTextErrorResponse(fmt.Sprintf(
			"unknown mode %q. Use %q (one at a time, slowest, easiest to follow), %q (default) or %q (every lane audited by a second agent before you see it).",
			params.Mode, ModeSequential, ModeParallel, ModeSupervised)), nil
	}
	// Sequential is simply concurrency of 1 — same scheduler, so the modes
	// cannot drift apart as the code changes.
	inFlight := ResearchMaxInFlight
	if mode == ModeSequential {
		inFlight = 1
	}

	var out strings.Builder
	fmt.Fprintf(&out, "# Research: %s\n\n", strings.TrimSpace(params.Question))
	switch mode {
	case ModeSequential:
		fmt.Fprintf(&out, "%d helpers, one at a time. %s %s\n\n", len(roles), clampNote, roleNote)
	case ModeSupervised:
		// GORILLA FIX: was len(roles)*2 and "EACH audited". Supervision covers
		// the blind lanes only — the verifier and completeness lanes read the
		// others' work and are never audited themselves. Saying "each" over an
		// unaudited verifier is the confident-wrong-answer failure this layer
		// exists to catch.
		audited := auditableLanes(roles)
		fmt.Fprintf(&out, "%d helpers (up to %d at a time), %d of %d lanes audited by their own supervisor before you see it — %d sessions in total. %s %s\n\n",
			len(roles), inFlight, audited, len(roles), len(roles)+audited, clampNote, roleNote)
	default:
		fmt.Fprintf(&out, "%d helpers, up to %d at a time. %s %s\n\n", len(roles), inFlight, clampNote, roleNote)
	}

	// Two waves. Everything that can work blind goes first, concurrently;
	// the roles that must READ the others (verifier, completeness critic) go
	// second, once there is something to read. Splitting on that dependency is
	// the only ordering this actually needs.
	type outcome struct {
		reply string
		err   error
		spend helperSpend
	}
	results := make([]outcome, len(roles))
	var mu sync.Mutex
	var supervisorSpend helperSpend

	// GORILLA FIX 2026-08-14: every helper is REGISTERED BEFORE it queues.
	//
	// Registration used to happen inside runHelper, which a goroutine only
	// reaches after winning a semaphore slot. So a helper waiting its turn was
	// alive, spawned, and completely invisible — missing from /tasks, missing
	// from the status count, and impossible to kill. That is why a run of 10
	// showed 4 and looked broken.
	//
	// It also meant the Nuclear Option did not work: killing the visible
	// helpers released their slots, which let the queued ones start. Now every
	// helper owns a cancellable context from the moment it exists, so killing a
	// QUEUED helper stops it before it ever spends a token.
	runWave := func(idxs []int, peers string) {
		var wg sync.WaitGroup
		sem := make(chan struct{}, inFlight)
		for _, i := range idxs {
			select {
			case <-ctx.Done():
				return
			default:
			}
			wg.Add(1)
			go func(i int) {
				defer wg.Done()

				hctx, hcancel := context.WithCancel(ctx)
				defer hcancel()
				entry := RegisterSubAgentState(
					helperSessionID(call.ID, roles[i]), sessionID, call.ID,
					helperLabel(roles[i]), SubAgentQueued, hcancel)
				defer UnregisterSubAgent(entry.ID)

				// Wait for a slot, but stay killable while waiting.
				select {
				case sem <- struct{}{}:
				case <-hctx.Done():
					SetSubAgentState(entry.ID, SubAgentKilled)
					results[i] = outcome{err: hctx.Err()}
					return
				}
				defer func() { <-sem }()

				SetSubAgentState(entry.ID, SubAgentRunning)
				prompt := buildPrompt(roles[i], params.Question, params.Context, peers, i, len(roles), params.Doctrine)
				reply, spend, err := r.runHelper(hctx, sessionID, call.ID, roles[i], prompt, entry.ID)
				switch {
				case err != nil && hctx.Err() != nil:
					SetSubAgentState(entry.ID, SubAgentKilled)
				case err != nil:
					SetSubAgentState(entry.ID, SubAgentFailed)
				default:
					SetSubAgentState(entry.ID, SubAgentDone)
				}
				results[i] = outcome{reply: reply, err: err, spend: spend}
			}(i)
		}
		wg.Wait()
	}

	var firstWave, secondWave []int
	for i, role := range roles {
		if rolePeeksAtOthers(role.ID) {
			secondWave = append(secondWave, i)
		} else {
			firstWave = append(firstWave, i)
		}
	}

	runWave(firstWave, "")

	// SUPERVISION. One auditor per lane, in parallel, each reading only that
	// lane. Runs before the peeking roles so the verifier reads audited work.
	audits := make([]string, len(roles))
	if mode == ModeSupervised {
		var wg sync.WaitGroup
		sem := make(chan struct{}, inFlight)
		for _, i := range firstWave {
			if results[i].err != nil || strings.TrimSpace(results[i].reply) == "" {
				continue // nothing to audit; the lane is already reported as failed
			}
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				sup := researchRole{ID: "supervisor:" + roles[i].ID, Title: "SUPERVISOR"}

				// Supervisors are helpers too: visible while queued, killable
				// before they start, and shown with their own state.
				hctx, hcancel := context.WithCancel(ctx)
				defer hcancel()
				entry := RegisterSubAgentState(
					helperSessionID(call.ID, sup), sessionID, call.ID,
					helperLabel(sup)+" · "+roles[i].ID, SubAgentQueued, hcancel)
				defer UnregisterSubAgent(entry.ID)

				select {
				case sem <- struct{}{}:
				case <-hctx.Done():
					SetSubAgentState(entry.ID, SubAgentKilled)
					audits[i] = "**SUPERVISION CANCELLED — this lane is UNAUDITED**"
					return
				}
				defer func() { <-sem }()

				SetSubAgentState(entry.ID, SubAgentRunning)
				reply, spend, err := r.runHelper(hctx, sessionID, call.ID, sup,
					supervisorPrompt(roles[i], params.Question, results[i].reply), entry.ID)
				if err != nil {
					if hctx.Err() != nil {
						SetSubAgentState(entry.ID, SubAgentKilled)
					} else {
						SetSubAgentState(entry.ID, SubAgentFailed)
					}
					// An audit that did not happen must not read as approval.
					audits[i] = "**SUPERVISION FAILED — this lane is UNAUDITED: " + err.Error() + "**"
					return
				}
				SetSubAgentState(entry.ID, SubAgentDone)
				audits[i] = strings.TrimSpace(reply)
				mu.Lock()
				supervisorSpend.add(spend)
				mu.Unlock()
			}(i)
		}
		wg.Wait()
	}

	if len(secondWave) > 0 {
		var peers strings.Builder
		for _, i := range firstWave {
			if results[i].err == nil {
				fmt.Fprintf(&peers, "### %s\n%s\n\n", roles[i].Title, strings.TrimSpace(results[i].reply))
			}
		}
		runWave(secondWave, peers.String())
	}

	// One write for the whole run: N goroutines doing read-modify-write on the
	// parent session would lose costs, and under-reporting spend is the wrong
	// direction to be wrong in.
	//
	// GORILLA FIX (2026-08-17): TOKENS are rolled up too, and the write no
	// longer depends on cost being non-zero.
	//
	// MEASURED on a real run: eight helpers burned 248,122 input and 32,622
	// output tokens — 280,744 — while the status bar read "13.3K in / 630 out,
	// spent $0.00" and never moved. Two failures stacked. Helper tokens were
	// never aggregated at all, only cost; and on a free or flat-rate tier every
	// helper's cost is legitimately 0.00, so `if total > 0` skipped the write
	// entirely and the one number that could have warned the user was
	// structurally incapable of changing. The owner noticed by doing the
	// arithmetic in his head and was within 7% of the truth — which is the
	// wrong way for anyone to learn what they just spent.
	//
	// This project's whole argument is that tokens are a recurring bill for
	// people who cannot afford surprises. Under-reporting a run by a factor of
	// twenty is the worst bug available to it.
	total := supervisorSpend
	for _, o := range results {
		total.add(o.spend)
	}
	if total.cost > 0 || total.inTokens > 0 || total.outTokens > 0 {
		if parent, err := r.sessions.Get(ctx, sessionID); err == nil {
			parent.Cost += total.cost
			parent.PromptTokens += total.inTokens
			parent.CompletionTokens += total.outTokens
			_, _ = r.sessions.Save(ctx, parent)
		}
	}

	// Report in role order, not completion order, so the output is the same
	// every run and diffable.
	completed, failed := 0, 0
	for i, role := range roles {
		o := results[i]
		fmt.Fprintf(&out, "## %s\n\n", role.Title)
		switch {
		case o.err != nil:
			failed++
			fmt.Fprintf(&out, "**HELPER FAILED — no findings from this lane.** %s\n\n", o.err)
			out.WriteString("Treat this lane as UNCOVERED when you synthesise; do not assume the others compensate.\n\n---\n\n")
			continue
		case strings.TrimSpace(o.reply) == "":
			failed++
			out.WriteString("**HELPER RETURNED NOTHING — lane UNCOVERED.**\n\n---\n\n")
			continue
		}
		completed++
		if missing := checkContract(o.reply); len(missing) > 0 {
			fmt.Fprintf(&out, "**Contract not followed — missing %s. Read this reply with suspicion; unstructured findings carry no evidence tiers.**\n\n",
				strings.Join(missing, ", "))
		}
		out.WriteString(strings.TrimSpace(o.reply))
		if a := strings.TrimSpace(audits[i]); a != "" {
			out.WriteString("\n\n### Supervisor audit of this lane\n\n")
			out.WriteString(a)
		}
		out.WriteString("\n\n---\n\n")
	}

	fmt.Fprintf(&out, "## Synthesis is YOUR job\n\n")
	fmt.Fprintf(&out, "%d of %d helpers reported", completed, len(roles))
	if failed > 0 {
		fmt.Fprintf(&out, "; %d failed and those lanes are UNCOVERED", failed)
	}
	out.WriteString(".\n\nBefore answering the user:\n" +
		"1. Check at least one load-bearing claim yourself. Helpers are confidently wrong at a rate that matters.\n" +
		"2. Carry evidence tiers through. A `single_claim` stays a single claim when you repeat it.\n" +
		"3. Say what nobody established. A gap reported is cheaper than a gap discovered later.\n" +
		"4. If the honest answer is no, give it plainly rather than offering hope.\n")

	// GORILLA OVERRIDE (2026-08-18): save the graded findings to disk NOW, from
	// Go, before the model is asked to assemble anything. A run on 2026-08-17
	// burned ~850,000 tokens, produced verified findings, announced "writing the
	// dossier now" and died with the orchestrator's context at 145% — nothing
	// was written. See research_salvage.go for the full account. The work must
	// survive the write-up step failing, because on a slow link that failure is
	// routine rather than exceptional.
	replies := make([]string, len(results))
	for i, o := range results {
		replies[i] = o.reply
	}
	if saved := writeRawFindings(params.Question, roles, replies, audits, params.Doctrine); saved != "" {
		fmt.Fprintf(&out, "\n## Findings already saved\n\n"+
			"Every lane's graded report is on disk at:\n\n    %s\n\n"+
			"That happened automatically, before you were asked to do anything, so this run "+
			"cannot be lost by a failure further down. If you run out of context while "+
			"assembling the assessment, say so plainly and tell the user to run "+
			"`/osint --recover` — the findings are safe and the write-up can be redone on a "+
			"model with a larger window. Do NOT silently produce a shortened dossier instead.\n",
			saved)
	}

	if params.Doctrine == "dossier" {
		fmt.Fprintf(&out, dossierDutiesFooter, config.DossierDir())
	}

	return tools.NewTextResponse(out.String()), nil
}

// runHelper spawns one helper in its own session, registered so the user can
// see it in /tasks and kill it.
func (r *researchTool) runHelper(ctx context.Context, parentSessionID, callID string, role researchRole, prompt string, registryID string) (string, helperSpend, error) {
	helper, err := NewAgent(researchAgentName(), r.sessions, r.messages, ResearchAgentTools(r.lspClients, r.permissions))
	if err != nil {
		return "", helperSpend{}, fmt.Errorf("could not create helper: %w", err)
	}

	// GORILLA OVERRIDE: the session id MUST be unique per helper.
	//
	// CreateTaskSession stores its toolCallID argument as the session's PRIMARY
	// KEY (internal/session/session.go:56). The `agent` tool spawns exactly one
	// helper per tool call, so passing call.ID there is safe. This tool spawns
	// up to ten per call: passing the same call.ID meant the first INSERT won
	// and the other nine failed on a UNIQUE constraint.
	//
	// Measured 2026-08-14 on a supervised run with agents=10: 9 of 10 helpers
	// died, the survivor returned unsupported speculation, and the whole
	// investigation was worthless while looking like it had run. Suffixing with
	// the role keeps the id traceable back to the tool call and unique, because
	// selectRoles deduplicates roles and supervisors carry a "supervisor:"
	// prefix.
	helperSession, err := r.sessions.CreateTaskSession(ctx, helperSessionID(callID, role), parentSessionID, "Research: "+role.ID)
	if err != nil {
		return "", helperSpend{}, fmt.Errorf("could not create helper session: %w", err)
	}
	// GORILLA FIX (2026-08-17): tell the permission service this helper belongs
	// to the user's conversation, so one "allow for session" covers the run
	// instead of every lane asking the same question again.
	r.permissions.RegisterChildSession(helperSession.ID, parentSessionID)

	// GORILLA FIX 2026-08-14: registration moved OUT to the caller, which
	// registers before queueing for a slot. Registering here meant a helper
	// waiting its turn did not exist as far as /tasks or the kill switch were
	// concerned. registryID is the handle the caller already created; it is
	// carried in so the label and the session stay traceable to one another.
	//
	// The LABEL, not the prompt, is what gets registered: /tasks truncates that
	// string, and every research prompt opens with "You are helper N of M in a
	// research investigation", so all lanes used to render identically and the
	// user could not tell which one to kill.
	_ = registryID

	done, err := helper.Run(ctx, helperSession.ID, prompt)
	if err != nil {
		return "", helperSpend{}, fmt.Errorf("could not start helper: %w", err)
	}
	result := <-done
	if result.Error != nil {
		return "", helperSpend{}, fmt.Errorf("helper failed: %w", result.Error)
	}
	if result.Message.Role != message.Assistant {
		return "", helperSpend{}, fmt.Errorf("helper returned no assistant message")
	}

	// Report the spend; the CALLER adds it up once. Doing the read-modify-write
	// on the parent session here would race with every other helper and lose
	// costs — /usage would under-report exactly when a run was most expensive.
	//
	// Tokens travel with the cost: on a free tier the cost is 0.00 however much
	// was burned, so tokens are the only honest signal there is.
	var spent helperSpend
	if updated, err := r.sessions.Get(ctx, helperSession.ID); err == nil {
		spent = helperSpend{
			cost:      updated.Cost,
			inTokens:  updated.PromptTokens,
			outTokens: updated.CompletionTokens,
		}
	}

	reply := result.Message.Content().String()
	if strings.TrimSpace(reply) == "" {
		return "", spent, fmt.Errorf("helper returned an empty reply")
	}
	return reply, spent, nil
}

// helperSpend is what one helper consumed. Cost alone is not enough: on a free
// or flat-rate tier it is always 0.00, and a user watching only that number
// would see a 280,000-token run report nothing at all.
type helperSpend struct {
	cost      float64
	inTokens  int64
	outTokens int64
}

func (h *helperSpend) add(o helperSpend) {
	h.cost += o.cost
	h.inTokens += o.inTokens
	h.outTokens += o.outTokens
}

// helperSessionID derives a UNIQUE session id per helper. See the note at the
// CreateTaskSession call: the id is a primary key, and sharing it across
// helpers is what killed 9 of 10 on 2026-08-14.
func helperSessionID(callID string, role researchRole) string {
	return callID + "-" + role.ID
}

// helperLabel is what /tasks shows. Not the prompt: every research prompt opens
// identically, so the list rendered as ten copies of the same line.
func helperLabel(role researchRole) string {
	if role.Title != "" {
		return "research · " + role.Title
	}
	return "research · " + role.ID
}

// researchAgentName picks which configured agent the helpers run as.
//
// GORILLA OVERRIDE: config.AgentResearch is new, so every config written before
// it exists has no "research" entry — and createAgentProvider returns
// "agent research not found" for a missing one. Without this fallback the tool
// fails EVERY lane on EVERY pre-existing install, which is the worst possible
// shape of bug: the feature looks implemented, ships, and does nothing but
// print six failures. Found by checking a real config before testing, not by
// reasoning about it.
//
// Falls back to AgentTask because that is the existing cheap read-only helper
// model and is present in every config. A user who wants research on a
// different model adds an "research" entry and it is picked up automatically.
func researchAgentName() config.AgentName {
	if cfg := config.Get(); cfg != nil {
		if _, ok := cfg.Agents[config.AgentResearch]; ok {
			return config.AgentResearch
		}
	}
	return config.AgentTask
}

func NewResearchTool(
	Sessions session.Service,
	Messages message.Service,
	LspClients map[string]*lsp.Client,
	Permissions permission.Service,
) tools.BaseTool {
	return &researchTool{
		sessions:    Sessions,
		messages:    Messages,
		lspClients:  LspClients,
		permissions: Permissions,
	}
}
