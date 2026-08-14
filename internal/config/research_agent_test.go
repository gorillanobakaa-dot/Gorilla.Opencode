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
// which nothing supports and which understates the real cost.
func TestQuotaMultipleIsDerivedNotTheHelperCount(t *testing.T) {
	for _, helpers := range []int{4, 10, 20} {
		got := ResearchQuotaMultiple(helpers)
		if got == helpers {
			t.Errorf("%d helpers -> %dx: that is the helper count, not a token-derived figure", helpers, got)
		}
		if want := helpers * ResearchStepsPerHelper; got != want {
			t.Errorf("%d helpers -> %d, want %d", helpers, got, want)
		}
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
