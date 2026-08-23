package agent

// GORILLA OVERRIDE (2026-08-23): print the bill.
//
// The owner, after a run that cost $7.64 and processed nearly twelve million
// tokens while the footer read $0.01: "we need to print this as well in the
// prompt... the total transparency... that should keep everyone honest."
//
// The numbers below are that run's real ones, taken from the session store.

import (
	"os"
	"strings"
	"testing"
)

func realRun() helperSpend {
	return helperSpend{
		cost: 7.64, inTokens: 11_935_525, outTokens: 56_923, toolCalls: 195,
	}
}

// THE FIGURE THAT WAS WRONG FOR EVERY RUN. helperSpend used to read
// PromptTokens, which is CURRENT CONTEXT OCCUPANCY, so the run reported the sum
// of eighteen final contexts (1,121,961) instead of the tokens actually
// processed (11,935,525). Low by 10.6x, in every token figure this tool has
// ever published.
func TestTheReceiptReportsTokensProcessedNotContextSize(t *testing.T) {
	r := newResearchReceipt(realRun(), 10, 18)

	if r.TokensIn != 11_935_525 {
		t.Errorf("tokens in = %d, want 11935525", r.TokensIn)
	}
	// The old wrong value, named so a regression is unmistakable.
	if r.TokensIn == 1_121_961 {
		t.Error("the receipt is summing FINAL CONTEXT SIZES again, not tokens " +
			"processed. An agent loop re-sends its whole context every turn, so " +
			"those two differ by an order of magnitude.")
	}
}

// The ratio is the number that EXPLAINS the total. Without it a reader sees a
// big bill and no reason for it.
func TestTheReceiptShowsWhyTheBillIsBig(t *testing.T) {
	r := newResearchReceipt(realRun(), 10, 18)
	if got := int(r.Ratio); got < 200 || got > 220 {
		t.Errorf("ratio %d:1, want about 210:1 for this run", got)
	}
	if r.CostPerMillion <= 0 {
		t.Error("no cost per million, so the total cannot be sanity-checked")
	}

	var b strings.Builder
	writeResearchReceipt(&b, realRun(), 10, 18)
	out := b.String()

	for _, want := range []string{
		"WHAT THIS RUN COST",
		"11,935,525", // grouped: an ungrouped 11935525 cannot be sized at a glance
		"56,923",
		"195",
		"210 : 1",
		// Plain, not markdown-bold. The receipt is a FENCED BLOCK now: the
		// first version used a markdown table and the renderer turned it into
		// a full-width bordered box with an empty first column and a rule
		// across the whole terminal, which is exactly the decoration this
		// project keeps being told not to draw.
		"TOTAL COST",
		"$7.64",
		"18 sessions",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the receipt is missing %q:\n%s", want, out)
		}
	}

	// Alignment is the whole reason for the fenced block, so assert the block
	// exists rather than trusting it.
	if !strings.Contains(out, "```") {
		t.Error("the receipt is not fenced, so its alignment is at the mercy of the " +
			"markdown renderer")
	}
	// The owner asked for this under the bottom line, which only the model can
	// do, so the report has to ask for it.
	if !strings.Contains(out, "REPRODUCE THE BLOCK ABOVE VERBATIM") {
		t.Error("nothing asks the model to repeat the receipt under its answer, so " +
			"the user only sees it in a tool result they may never scroll back to")
	}

	// The honesty line that already governs every other cost display here.
	if !strings.Contains(out, "free or flat-rate tier the real bill is $0") {
		t.Error("the receipt states a dollar figure without saying it is an estimate")
	}
	// The explanation only when it applies.
	if !strings.Contains(out, "re-reading, not thinking") {
		t.Error("a 210:1 ratio needs its one-line explanation")
	}
}

// A cheap run must not be lectured about re-reading. The note earns its place
// by being conditional; printed always, it becomes furniture nobody reads.
func TestACheapRunGetsNoLecture(t *testing.T) {
	var b strings.Builder
	writeResearchReceipt(&b, helperSpend{cost: 0.01, inTokens: 1000, outTokens: 900, toolCalls: 2}, 1, 1)
	if strings.Contains(b.String(), "re-reading, not thinking") {
		t.Error("a 1:1 run was told most of its spend was re-reading")
	}
}

// A free-tier run has zero cost and real tokens. The receipt must still be
// useful, because tokens are the only evidence the run happened at all.
func TestAFreeRunStillGetsAReceipt(t *testing.T) {
	var b strings.Builder
	writeResearchReceipt(&b, helperSpend{cost: 0, inTokens: 4_000_000, outTokens: 20_000, toolCalls: 60}, 8, 14)
	out := b.String()
	if !strings.Contains(out, "4,000,000") || !strings.Contains(out, "60") {
		t.Errorf("a zero-cost run lost its token and tool counts:\n%s", out)
	}
	if !strings.Contains(out, "$0.00") {
		t.Errorf("the total is missing entirely on a free tier:\n%s", out)
	}
}

func TestCommasGroupNumbersForReading(t *testing.T) {
	for in, want := range map[int64]string{
		0: "0", 7: "7", 999: "999", 1000: "1,000",
		56923: "56,923", 11935525: "11,935,525",
	} {
		if got := commas(in); got != want {
			t.Errorf("commas(%d) = %q, want %q", in, got, want)
		}
	}
}

// SOURCE GUARD. The tests above build helperSpend from literals, so they cannot
// see which SESSION FIELDS the run reads. That is exactly where the bug was:
// reading PromptTokens (current context occupancy) instead of
// CumulativePromptTokens (tokens actually processed) understated every run by
// about 10.6x and no unit test above would notice.
//
// Reading source is unusual and is used here for the same reason as the grant
// key and z-order guards: the property lives in which identifier appears at one
// place in one function, and nothing else can assert it.
func TestSpendIsReadFromTheCumulativeFields(t *testing.T) {
	src := readAgentSource(t, "research-tool.go")

	i := strings.Index(src, "spent = helperSpend{")
	if i < 0 {
		t.Fatal("cannot find where helperSpend is built; this guard's anchor needs updating")
	}
	block := src[i:]
	if j := strings.Index(block, "}"); j > 0 {
		block = block[:j]
	}

	if !strings.Contains(block, "CumulativePromptTokens") ||
		!strings.Contains(block, "CumulativeCompletionTokens") {
		t.Errorf("helperSpend is not built from the cumulative fields:\n%s\n\n"+
			"  PromptTokens is CURRENT CONTEXT OCCUPANCY, assigned every turn. A run\n"+
			"  that sums it reports the total of its helpers' FINAL CONTEXTS, not the\n"+
			"  tokens it processed. Measured 2026-08-23: 1,121,961 reported against a\n"+
			"  true 11,935,525.", block)
	}
}

func readAgentSource(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}
