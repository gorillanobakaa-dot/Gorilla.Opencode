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
	return tools.ToolInfo{
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
func buildPrompt(role researchRole, question, sharedContext, peerFindings string, index, total int) string {
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
	b.WriteString(researchOutputContract)
	return b.String()
}

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
		cost  float64
	}
	results := make([]outcome, len(roles))
	var mu sync.Mutex
	var supervisorCost float64

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
				prompt := buildPrompt(roles[i], params.Question, params.Context, peers, i, len(roles))
				reply, cost, err := r.runHelper(hctx, sessionID, call.ID, roles[i], prompt, entry.ID)
				switch {
				case err != nil && hctx.Err() != nil:
					SetSubAgentState(entry.ID, SubAgentKilled)
				case err != nil:
					SetSubAgentState(entry.ID, SubAgentFailed)
				default:
					SetSubAgentState(entry.ID, SubAgentDone)
				}
				results[i] = outcome{reply: reply, err: err, cost: cost}
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
				reply, cost, err := r.runHelper(hctx, sessionID, call.ID, sup,
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
				supervisorCost += cost
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
	total := supervisorCost
	for _, o := range results {
		total += o.cost
	}
	if total > 0 {
		if parent, err := r.sessions.Get(ctx, sessionID); err == nil {
			parent.Cost += total
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

	return tools.NewTextResponse(out.String()), nil
}

// runHelper spawns one helper in its own session, registered so the user can
// see it in /tasks and kill it.
func (r *researchTool) runHelper(ctx context.Context, parentSessionID, callID string, role researchRole, prompt string, registryID string) (string, float64, error) {
	helper, err := NewAgent(researchAgentName(), r.sessions, r.messages, ResearchAgentTools(r.lspClients, r.permissions))
	if err != nil {
		return "", 0, fmt.Errorf("could not create helper: %w", err)
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
		return "", 0, fmt.Errorf("could not create helper session: %w", err)
	}

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
		return "", 0, fmt.Errorf("could not start helper: %w", err)
	}
	result := <-done
	if result.Error != nil {
		return "", 0, fmt.Errorf("helper failed: %w", result.Error)
	}
	if result.Message.Role != message.Assistant {
		return "", 0, fmt.Errorf("helper returned no assistant message")
	}

	// Report the spend; the CALLER adds it up once. Doing the read-modify-write
	// on the parent session here would race with every other helper and lose
	// costs — /usage would under-report exactly when a run was most expensive.
	var spent float64
	if updated, err := r.sessions.Get(ctx, helperSession.ID); err == nil {
		spent = updated.Cost
	}

	reply := result.Message.Content().String()
	if strings.TrimSpace(reply) == "" {
		return "", spent, fmt.Errorf("helper returned an empty reply")
	}
	return reply, spent, nil
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
