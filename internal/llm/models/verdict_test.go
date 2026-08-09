package models

import (
	"strings"
	"testing"
)

// GORILLA OVERRIDE: the guard on the whole idea. Every label must be traceable
// to something — our own findings, a judgement already written, or the vendor's
// own words with the trigger quoted. A verdict with nothing behind it would be
// worse than the marketing it replaced, because it would carry this project's
// name.

func TestEarnedVerdictsCiteTheirEvidence(t *testing.T) {
	if len(earnedVerdicts) == 0 {
		t.Fatal("no verdicts loaded — the embedded file is missing or unparsable")
	}
	for id, v := range earnedVerdicts {
		if strings.TrimSpace(v.Evidence) == "" {
			t.Errorf("%s states a verdict with no evidence — that is an opinion, not a finding", id)
		}
		if len(v.Evidence) < 30 {
			t.Errorf("%s cites %q, which is too thin to check", id, v.Evidence)
		}
	}
}

// An earned verdict must beat both the vendor's copy and the classifier.
func TestEarnedVerdictWins(t *testing.T) {
	got := DescribeForPicker("google/gemini-3.6-flash",
		"Gemini 3.6 Flash is a fast model with excellent agentic coding for developers", 1000, 1, 2)
	if !strings.Contains(got, "shit tier") {
		t.Errorf("our own finding must override vendor marketing, got %q", got)
	}
	if strings.Contains(got, "CAN CODE") {
		t.Errorf("the classifier must not overrule a verdict we earned, got %q", got)
	}
}

// A model nobody has any information about must say so, not be guessed at.
func TestNoClaimIsStatedPlainly(t *testing.T) {
	got := DescribeForPicker("someone/unknown-model-xyz", "A large language model.", 1000, 1, 2)
	if !strings.Contains(got, "UNTESTED for coding work") {
		t.Errorf("silence must be reported as silence, got %q", got)
	}
}

// Every classifier label must quote the word that produced it, so a beginner
// can check the claim instead of trusting it.
func TestClassifierQuotesItsTrigger(t *testing.T) {
	got := DescribeForPicker("vendor/roleplay-bot",
		"A roleplay companion model for character chat", 1000, 1, 2)
	if !strings.Contains(got, "shit tier") {
		t.Errorf("a self-declared roleplay model is not a coding model: %q", got)
	}
	if !strings.Contains(got, `vendor: "`) {
		t.Errorf("the label must quote the vendor's own word, got %q", got)
	}
}

func TestClassifyForCodingOrdering(t *testing.T) {
	// A multimodal CODER is still a coder: the coding claim must win over a
	// passing mention of vision.
	label, _ := ClassifyForCoding("A vision-language model with strong agentic coding for developers")
	if label != "CAN CODE" {
		t.Errorf("coding claim should outrank a vision mention, got %q", label)
	}
}
