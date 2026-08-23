package agent

// GORILLA OVERRIDE (2026-08-23): the evidence tiers stopped at the findings.
//
// The owner's question, after a run that answered correctly: "if we did not
// know who pete holmes is... how can we tell which one is the real answer?"
//
// From the report as printed, you could not. The run tiered its FINDINGS
// properly and even refused to repeat a claim eighteen helpers had made because
// the searches behind it were shaky. Then its bottom line said "Pete Holmes is
// the comedian" flat: no source, no tier. The source it really rested on, a
// Wikipedia page a helper genuinely fetched, appeared nowhere in the report.
//
// The machinery for honesty existed and did not reach the one line anybody
// reads.

import (
	"strings"
	"testing"
)

// The helper contract must require a basis on the ANSWER, not only on findings.
func TestTheAnswerContractDemandsABasis(t *testing.T) {
	c := researchOutputContract

	if !strings.Contains(c, "BASIS:") {
		t.Fatal("the ANSWER section does not ask for its basis, so a lane may state " +
			"a conclusion with nothing behind it and still parse as well-formed")
	}
	// The escape hatch must exist and must be explicit, or a lane with no source
	// will either invent one or omit the line.
	if !strings.Contains(c, "nothing consulted, this is the model's prior knowledge") {
		t.Error("no way to say honestly that an answer rests on nothing consulted; " +
			"a contract with no honest way to admit a gap teaches lanes to hide it")
	}
	if !strings.Contains(c, "unsourced") {
		t.Error("there is no tier for model prior knowledge, so it gets filed as " +
			"single_claim and looks like something somebody read")
	}
	// The basis line must be mandatory, not decorative.
	if !strings.Contains(c, "An answer with no basis line is malformed") {
		t.Error("the basis line is optional in practice; a lane will drop it")
	}
}

// The tier list must keep its ORDER, strongest first, and unsourced must be
// last. A tier list whose order is scrambled teaches the wrong hierarchy.
func TestUnsourcedIsTheWeakestTier(t *testing.T) {
	// Scope to the TIER LIST. The first version of this searched the whole
	// contract and failed against correct text, because "unsourced" also
	// appears earlier in the ANSWER section as an example. The code was right
	// and the assertion was looking in the wrong place, which is the second
	// time today I have written a test that way.
	c := researchOutputContract
	from := strings.Index(c, "TIER must be one of")
	if from < 0 {
		t.Fatal("cannot find the tier list; this test's anchor needs updating")
	}
	c = c[from:]
	order := []string{"primary_source", "config", "multiple_reports", "single_claim", "unsourced"}
	prev := -1
	for _, tier := range order {
		i := strings.Index(c, tier)
		if i < 0 {
			t.Fatalf("tier %q is missing from the contract", tier)
		}
		if i < prev {
			t.Errorf("tier %q appears before a stronger one; the list is meant to read "+
				"strongest first", tier)
		}
		prev = i
	}
}

// The synthesiser is told the things that decide whether a reader can check the
// answer. Each of these is a separate failure that actually happened.
func TestTheSynthesiserIsToldToCarryEvidenceIntoTheAnswer(t *testing.T) {
	var b strings.Builder
	writeSynthesisDuty(&b)
	duty := b.String()

	for name, want := range map[string]string{
		"evidence in the answer":     "names its EVIDENCE and TIER",
		"admit prior knowledge":      "unsourced",
		"separate checkable claims":  "VERIFY THEMSELVES",
		"agreement is not evidence":  "Helpers agreeing is NOT corroboration",
		"a source must be consulted": "actually consulted in this run",
	} {
		if !strings.Contains(duty, want) {
			t.Errorf("the synthesis duty does not cover %s (looking for %q):\n%s",
				name, want, duty)
		}
	}
}

// THE POINT ABOUT MULTI-AGENT RUNS, stated so it is not lost: eighteen helpers
// sharing one model are one opinion repeated, not eighteen witnesses. A run
// that reads convergence as corroboration will confidently launder a single
// model's mistake into a consensus.
func TestConvergenceIsNotTreatedAsCorroboration(t *testing.T) {
	var b strings.Builder
	writeSynthesisDuty(&b)
	if !strings.Contains(b.String(), "They share one model") {
		t.Error("nothing tells the synthesiser why helper agreement is worthless as " +
			"corroboration, which is the one statistical trap a fan-out design has")
	}
}
