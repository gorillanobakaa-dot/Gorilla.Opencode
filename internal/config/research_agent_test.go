package config

import (
	"github.com/opencode-ai/opencode/internal/llm/models"
	"os"
	"strings"
	"testing"
)

// Research helpers must not be silently pointed at an unrelated provider.
//
// THE BUG: AgentResearch was added to the loop that defaults agents to the
// first LOCAL endpoint model. A user signed in to Antigravity, chatting to
// Claude Sonnet 4.6, was told "helpers run on Llama 3.3 70B" — and it was
// TRUE, because that loop had pointed the research agent at a local server on
// a different provider entirely. Reported 2026-08-14. His reaction, correctly:
// "what the fuck is this moron talking about... this will lead to extreme
// distrust towards my app."
//
// With no research entry, researchAgentName() falls back to AgentTask, so
// helpers run on the same provider as everything else.
func TestResearchIsNotDefaultedToALocalEndpoint(t *testing.T) {
	src, err := os.ReadFile("config.go")
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(string(src), "\n") {
		if !strings.Contains(line, "range []AgentName{AgentCoder") {
			continue
		}
		if strings.Contains(line, "AgentResearch") {
			t.Errorf("config.go:%d puts AgentResearch in the local-endpoint default list; "+
				"helpers will be sent to a local model on another provider", i+1)
		}
	}
}

// Non-vacuous guard: the line this test inspects must still exist. If the loop
// is renamed or moved, the test above silently stops checking anything.
func TestTheLocalEndpointDefaultLoopStillExists(t *testing.T) {
	src, err := os.ReadFile("config.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "range []AgentName{AgentCoder") {
		t.Fatal("the local-endpoint default loop was renamed or removed; " +
			"TestResearchIsNotDefaultedToALocalEndpoint is now inspecting nothing")
	}
}

// The quota multiple must be DERIVED from the token model, not be the helper
// count. "4 helpers = 4x a question" asserts one helper equals one question,
// which nothing supports.
//
// GORILLA OVERRIDE (2026-08-23): ROADMAP item 6. This test used to assert
// `got == helpers * ResearchStepsPerHelper`, directly under a comment saying the
// figure must be derived from the TOKEN model. Those two statements contradict
// each other: helpers times steps is a step count, and no token appears in it.
// The test pinned the very thing its own comment called wrong, so it passed for
// as long as the bug existed and would have failed the fix.
//
// Same shape as the /tasks visibility test in this session: a test whose stated
// intent and whose assertion disagree is worse than no test, because the comment
// makes the next reader believe the property is covered.
//
// It now asserts the PROPERTY, not a formula. A specific number would just be
// the new formula written twice.
func TestQuotaMultipleIsDerivedNotTheHelperCount(t *testing.T) {
	for _, helpers := range []int{4, 10, 20} {
		got := ResearchQuotaMultiple(helpers)
		if got == helpers {
			t.Errorf("%d helpers -> %dx: that is the helper count, not a token-derived figure", helpers, got)
		}
		if got < 1 {
			t.Errorf("%d helpers -> %d: a real run is never worth less than one question", helpers, got)
		}
	}

	// It must scale with the run. Doubling the fleet must move the figure, or it
	// is not derived from anything about the fleet.
	four, eight := ResearchQuotaMultiple(4), ResearchQuotaMultiple(8)
	if eight <= four {
		t.Errorf("4 helpers -> %d and 8 -> %d: the figure does not grow with the run", four, eight)
	}

	// THE DERIVATION ITSELF, checked against the same two quantities the function
	// uses. Not "helper < coder": that holds at shipped defaults and not on a
	// loadout the user has trimmed, because a helper keeps its four tools either
	// way. What must always hold is that the answer IS this division.
	helperStep := ResearchHelperBasisTokens() + ResearchOutputPerStep
	ordinary := LoadoutActiveTokens() + ResearchOutputPerStep
	wantOne := float64(ResearchStepsPerHelper) * float64(helperStep) / float64(ordinary)
	if got := float64(ResearchQuotaMultiple(20)) / 20; got < wantOne*0.9 || got > wantOne*1.1 {
		t.Errorf("per-helper multiple is %.2f, want about %.2f from the token model "+
			"(helper step %d tokens, ordinary question %d)", got, wantOne, helperStep, ordinary)
	}

	if ResearchQuotaMultiple(0) != 0 {
		t.Error("zero helpers must cost zero questions")
	}
}

// The helper-model warning must fire on a PROVIDER mismatch, not on any name
// difference. Warning that helpers use "Gemini 3.6" while you chat to
// "Gemini 3.7" is noise the user cannot act on; warning that they are on a
// different provider is a different bill and a different quota.
//
// SUPERSEDED IN PART, 2026-08-14 — kept, with the correction beside it rather
// than in place of it.
//
// What is still true: a different PROVIDER is a different bill, and that is
// worth saying on its own.
//
// What was wrong: this made the provider gate the ONLY thing that could surface
// a helper-model difference, and so the screen went blind on the axis that
// actually matters. The user moved to Claude Opus 4.6 (Thinking) and the
// research screen kept quoting Gemini prices without a word, because both live
// on Antigravity. Nobody chooses Opus to save money — they choose it because
// the work is hard, and research is the hard part. A weaker helper is a WORSE
// ANSWER, which is a different harm from a surprise bill and needs reporting
// even when the bill is identical.
//
// So the display compares MODELS (see ResearchModelChoice) and reports provider
// difference as an extra line. This test now guards that split.
func TestHelperModelWarningIsProviderBasedNotNameBased(t *testing.T) {
	src, err := os.ReadFile("loadout.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	i := strings.Index(body, "func ResearchHelperModel(")
	if i < 0 {
		t.Fatal("ResearchHelperModel is gone; this test now checks nothing")
	}
	fn := body[i:]
	if j := strings.Index(fn[1:], "\nfunc "); j > 0 {
		fn = fn[:j]
	}
	if !strings.Contains(fn, ".Provider != ") {
		t.Error("the comparison is not provider-based; a name-only difference will spam the user")
	}
	if strings.Contains(fn, "helper != chat") {
		t.Error("name comparison is back — that produced the confusing 3.6-vs-3.7 message")
	}
}

// The CAPABILITY axis must exist alongside the billing one, or the screen goes
// blind again the moment two models share a provider.
func TestResearchModelChoiceExposesTheChatModelForComparison(t *testing.T) {
	if _, err := Load(t.TempDir(), false); err != nil {
		t.Fatalf("Load: %v", err)
	}
	helper, chat, ok := ResearchModelChoice()
	if !ok {
		t.Skip("no coder/helper agent in this environment")
	}
	// The point of the function: the caller can compare the two models
	// directly, independently of provider.
	if helper.ID == "" || chat.ID == "" {
		t.Fatalf("ResearchModelChoice returned an empty model: helper=%q chat=%q", helper.ID, chat.ID)
	}
	src, err := os.ReadFile("loadout.go")
	if err != nil {
		t.Fatal(err)
	}
	i := strings.Index(string(src), "func ResearchModelChoice(")
	fn := string(src)[i:]
	if j := strings.Index(fn[1:], "\nfunc "); j > 0 {
		fn = fn[:j]
	}
	if strings.Contains(fn, ".Provider != ") {
		t.Error("ResearchModelChoice gates on provider; that is the bug it exists to fix — " +
			"it must hand back both models and let the caller compare them")
	}
}

// THE FLICKER BUG, kept as a note.
//
// models.SupportedModels is a map and Go randomises map iteration. The old
// cheapest/most-expensive range picked a different winner on every render, so
// the dialog cycled through model names continuously and looked like a runaway
// loop. That range is gone — replaced by pricing the user's OWN model — but the
// lesson stands: anything ranging over SupportedModels must sort first.
// ResearchPaidEquivalent does, via its lexicographic tie-break.
func TestPaidEquivalentIsDeterministic(t *testing.T) {
	var flat models.Model
	for _, m := range models.SupportedModels {
		if m.CostPer1MIn == 0 && m.DefaultMaxTokens > 0 {
			flat = m
			break
		}
	}
	if flat.ID == "" {
		t.Skip("no flat-rate model in this build")
	}
	_, _, firstVia, ok := ResearchPaidEquivalent(flat, 4)
	if !ok {
		t.Skip("no paid sibling for the sampled model")
	}
	for i := 0; i < 200; i++ {
		_, _, via, _ := ResearchPaidEquivalent(flat, 4)
		if via != firstVia {
			t.Fatalf("iteration %d: equivalent changed %q -> %q — this is the flicker", i, firstVia, via)
		}
	}
}

// The equivalent must be the SAME MODEL FAMILY, not a random catalogue entry.
// Offering BGE-M3 (an embedding model) as the price of Gemini was rejected as
// "hallucinated crap about unrelated models".
func TestPaidEquivalentStaysInTheFamily(t *testing.T) {
	for _, m := range models.SupportedModels {
		if m.CostPer1MIn != 0 || m.DefaultMaxTokens == 0 {
			continue
		}
		_, _, via, ok := ResearchPaidEquivalent(m, 4)
		if !ok || via == "" {
			continue // honestly reporting "no paid listing" is fine
		}
		shared := 0
		want := familyTokens(string(m.ID) + " " + m.Name)
		for tok := range familyTokens(via) {
			if want[tok] {
				shared++
			}
		}
		if shared < 2 {
			t.Errorf("%q priced via %q — only %d shared family words; that is an unrelated model",
				m.Name, via, shared)
		}
	}
}

// ROADMAP item 6: a helper does not carry the coder's loadout, and pricing it as
// if it did made every money figure on the research screen nearly double.
//
// Measured on the development machine at shipped defaults, 2026-08-23:
// base 1,791 + fetch 789 + websearch 749 + find 1,322 + view 595 = 5,246,
// against a coder basis of 10,380. A 1.98x overstatement.
func TestTheHelperBasisIsTheHelpersOwnToolsNotTheCoders(t *testing.T) {
	// Assert the DEFINITION, not a comparison against the coder.
	//
	// The first version of this test said "helper basis < coder basis", which is
	// true at shipped defaults and NOT true in general: a helper always gets its
	// four tools (agent.ResearchAgentTools builds them unconditionally, and the
	// note there says fetch and websearch are deliberately not loadout-gated),
	// while the coder's basis shrinks as the user switches things off. A user on
	// a trimmed loadout would have failed a test asserting a property of the
	// default configuration. It also broke against a sibling test that leaves the
	// package-global loadout trimmed, which is how it was caught.
	want := LoadoutBaseTokens()
	for _, c := range LoadoutComponents {
		for _, id := range researchHelperTools {
			if c.ID == id {
				want += ComponentTokens(c)
			}
		}
	}
	if got := ResearchHelperBasisTokens(); got != want {
		t.Errorf("helper basis %d, want %d (base prompt + the four helper tools).\n"+
			"  If this is now LoadoutActiveTokens again, every money figure on the\n"+
			"  research screen is back to pricing helpers on the coder's thirteen\n"+
			"  tools, which measured 1.98x too high on 2026-08-23.", got, want)
	}
	if got := ResearchHelperBasisTokens(); got <= LoadoutBaseTokens() {
		t.Errorf("helper basis %d is not above the base prompt %d, so no tool schemas "+
			"are being counted at all", got, LoadoutBaseTokens())
	}
}

// The list here mirrors agent.ResearchAgentTools, which config cannot import.
// Every id must name a real component: a typo would silently price a helper at
// the bare system prompt and report success.
func TestHelperToolListNamesRealComponents(t *testing.T) {
	known := map[string]bool{}
	for _, c := range LoadoutComponents {
		known[c.ID] = true
	}
	for _, id := range researchHelperTools {
		if !known[id] {
			t.Errorf("researchHelperTools names %q, which is not a registered component. "+
				"A typo here quietly drops a tool from the helper's price.", id)
		}
	}
	if len(researchHelperTools) < 4 {
		t.Errorf("only %d helper tools listed; agent.ResearchAgentTools builds four "+
			"unconditionally (fetch, websearch, find, view)", len(researchHelperTools))
	}
}

// ROADMAP item 6, the other half: "THIS RUN" counted helpers only. The synthesis
// turn carries the coder's context PLUS everything the helpers wrote, and runs
// on the coder's model.
func TestTheSynthesisTurnGrowsWithTheFleet(t *testing.T) {
	launch4, synth4 := ResearchOrchestratorTokens(4)
	launch20, synth20 := ResearchOrchestratorTokens(20)

	if launch4 != launch20 {
		t.Errorf("the launch turn moved with the fleet size (%d vs %d); it is one "+
			"ordinary coder turn either way", launch4, launch20)
	}
	if synth20 <= synth4 {
		t.Errorf("synthesis input is %d for 4 helpers and %d for 20; it must grow, "+
			"because it reads every answer back", synth4, synth20)
	}
	if synth4 <= launch4 {
		t.Errorf("synthesis (%d) is not larger than the launch turn (%d), so the "+
			"helper output is not being counted", synth4, launch4)
	}
	// A zero or negative fleet must not produce a nonsense figure.
	if l, s := ResearchOrchestratorTokens(0); l <= 0 || s <= 0 {
		t.Errorf("zero helpers gave (%d, %d); both must stay positive", l, s)
	}
}
